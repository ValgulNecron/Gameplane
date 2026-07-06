// Package heartbeat periodically patches the owning GameServer's
// status.agent.{lastHeartbeat, playersOnline, playersMax, gameVersion} —
// plus the agent's own cpu/memory/disk usage — so the control plane can
// distinguish "pod ready" from "game actually up" and surface live
// resource usage without a cluster metrics pipeline.
//
// The agent uses its in-pod ServiceAccount to authenticate to the
// Kubernetes API directly; no traffic flows through the Gameplane API
// for this. The operator must grant the agent's SA permission to
// update gameservers/status — wired up during agent-injection.
package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/agent/internal/players"
	"github.com/ValgulNecron/gameplane/agent/internal/usage"
)

type Rcon interface {
	Exec(cmd string) (string, error)
}

// UsageReader yields the agent's own resource consumption. It never
// errors; unknown values are flagged on the Sample so they can be
// reported as null.
type UsageReader interface {
	Read() usage.Sample
}

type Config struct {
	ServerName   string
	Namespace    string
	Template     string
	Game         string
	Version      string
	Interval     time.Duration
	RCON         Rcon
	Usage        UsageReader
	PlayerList   *caps.PlayerList
	PlayerListRE *regexp.Regexp
}

var gvr = schema.GroupVersionResource{
	Group:    "gameplane.local",
	Version:  "v1alpha1",
	Resource: "gameservers",
}

// Run loops until ctx is cancelled. If anything in the setup fails
// (e.g. no in-cluster config available because we're in a dev env),
// it logs and returns gracefully — the server can still serve other
// endpoints without heartbeats.
func Run(ctx context.Context, cfg Config) {
	if cfg.ServerName == "" {
		slog.Info("heartbeat disabled: no GAMEPLANE_SERVER_NAME")
		return
	}
	if cfg.Namespace == "" {
		cfg.Namespace = readNamespace()
	}
	if cfg.Namespace == "" {
		slog.Info("heartbeat disabled: no namespace available")
		return
	}
	if cfg.Interval == 0 {
		cfg.Interval = 20 * time.Second
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		slog.Warn("heartbeat disabled: not in-cluster", "err", err)
		return
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		slog.Error("heartbeat dynamic client", "err", err)
		return
	}

	// Compile the player list regex if configured.
	if cfg.PlayerList != nil && cfg.PlayerList.EntryRegex != "" {
		re, err := regexp.Compile(cfg.PlayerList.EntryRegex)
		switch {
		case err != nil:
			slog.Warn("invalid player list entryRegex in heartbeat; using default parser", "err", err)
		default:
			cfg.PlayerListRE = re
		}
	}

	t := time.NewTicker(cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := sendOnce(ctx, dyn, cfg); err != nil {
				slog.Warn("heartbeat", "err", err)
			}
		}
	}
}

func sendOnce(ctx context.Context, dyn dynamic.Interface, cfg Config) error {
	agent := map[string]any{
		"lastHeartbeat": metav1.Now().UTC().Format(time.RFC3339),
		"version":       cfg.Version,
		"gameVersion":   cfg.Game,
	}
	// playersOnline/playersMax are null ("unknown") unless the game actually
	// answered a player-count query. A failing "list" is common on startup
	// and for games without RCON; emitting a sentinel like -1 here is wrong
	// because the dashboard sums playersOnline across servers (-1 + -1 =
	// -2). null/absent is the contract for "unknown" — a JSON merge patch
	// with null clears any prior value. playersMax stays null when the game
	// reports an online count but no maximum, so the dashboard shows "—"
	// rather than a bogus cap of 0.
	if online, maxN, err := queryPlayerCounts(cfg); err == nil {
		agent["playersOnline"] = online
		agent["playersMax"] = nullable(int64(maxN), maxN > 0)
	} else {
		agent["playersOnline"] = nil
		agent["playersMax"] = nil
	}
	// Resource usage follows the same null-on-unknown contract: an
	// unreadable source patches null so the dashboard renders "—" rather
	// than a frozen value. The keys are always present so a merge patch
	// clears a prior reading once a source goes away.
	if cfg.Usage != nil {
		s := cfg.Usage.Read()
		agent["cpuMillicores"] = nullable(s.CPUMillicores, s.CPUKnown)
		agent["cpuLimitMillicores"] = nullable(s.CPULimitMillicores, s.CPULimitKnown)
		agent["memoryBytes"] = nullable(s.MemoryBytes, s.MemoryKnown)
		agent["memoryLimitBytes"] = nullable(s.MemoryLimitBytes, s.MemoryLimitKnown)
		agent["diskUsedBytes"] = nullable(s.DiskUsedBytes, s.DiskKnown)
		agent["diskTotalBytes"] = nullable(s.DiskTotalBytes, s.DiskKnown)
	}
	patch := map[string]any{
		"status": map[string]any{"agent": agent},
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = dyn.Resource(gvr).
		Namespace(cfg.Namespace).
		Patch(ctx, cfg.ServerName, types.MergePatchType, body, metav1.PatchOptions{}, "status")
	return err
}

// nullable returns v when known, else nil, so a JSON merge patch with a
// null clears any stale value the dashboard would otherwise show forever.
func nullable(v int64, known bool) any {
	if !known {
		return nil
	}
	return v
}

// queryPlayerCounts issues an RCON command and derives the online count
// and, when the response is in a recognized Minecraft format, the configured
// maximum (max is 0 when no maximum is reported). The command and optional
// regex come from cfg.PlayerList; defaults to "list" with a loose parse.
// When EntryRegex is configured, the online count is the number of regex
// matches. err is non-nil only when RCON fails or no online count can be
// parsed at all.
func queryPlayerCounts(cfg Config) (online, max int, err error) {
	cmd := "list"
	if cfg.PlayerList != nil && cfg.PlayerList.Command != "" {
		cmd = cfg.PlayerList.Command
	}
	raw, err := cfg.RCON.Exec(cmd)
	if err != nil {
		return 0, 0, err
	}

	// If a custom regex is configured, count matches for online and set max to -1.
	if cfg.PlayerListRE != nil {
		online = players.CountWithRegex(raw, cfg.PlayerListRE)
		return online, -1, nil
	}

	// Default behavior: use ParseCounts for Minecraft, loose first-number parse for online.
	if _, m, ok := players.ParseCounts(raw); ok {
		max = m
	}
	// Very loose parse for online — we only care about the first number.
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			continue
		}
		n := 0
		for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
			n = n*10 + int(raw[i]-'0')
			i++
		}
		return n, max, nil
	}
	return 0, max, fmt.Errorf("no player count in %q", raw)
}

// readNamespace reads the SA-projected namespace file written into
// every pod.
func readNamespace() string {
	b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return ""
	}
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return string(b)
}
