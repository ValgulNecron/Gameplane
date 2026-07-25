//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fastGameSet lists the games run by default (unset GAMEPLANE_E2E_GAMES).
// These four cover a JOINED Java protocol (Minecraft), a JOINED .NET protocol
// (Terraria), a hand-rolled UDP protocol (Factorio), and the shared Source
// family (Garry's Mod). They boot quickly (minutes at most) and fit within a
// single kind node.
var fastGameSet = []string{"minecraft-java", "terraria", "factorio", "garrys-mod"}

// heavyGameSet lists the games not in the fast set. These are opt-in only
// (GAMEPLANE_E2E_GAMES=all) due to large disk/network requirements; they are
// not run in CI. Note: a game appearing here does not mean a client exists for
// it yet; skipUnlessGameInScope only decides scope, not test existence.
var heavyGameSet = []string{
	"cs2", "7-days-to-die", "project-zomboid", "valheim", "palworld", "rust",
	"v-rising", "dayz", "ark-survival-ascended", "dont-starve-together",
	"enshrouded", "satisfactory",
}

// parseGameScope parses GAMEPLANE_E2E_GAMES and returns the set of games to test,
// or nil if the gate is not enabled. The env var can be:
//   - unset  => the fast set
//   - "all"  => every game (fast + heavy)
//   - comma-separated list => those games (whitespace trimmed around entries)
func parseGameScope() []string {
	games := os.Getenv("GAMEPLANE_E2E_GAMES")
	if games == "" {
		return fastGameSet
	}
	if games == "all" {
		// Return fast + heavy concatenated. Build a fresh slice to avoid
		// aliasing fastGameSet's backing array.
		result := make([]string, 0, len(fastGameSet)+len(heavyGameSet))
		result = append(result, fastGameSet...)
		result = append(result, heavyGameSet...)
		return result
	}
	// Parse comma-separated list, trim whitespace.
	parts := strings.Split(games, ",")
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// skipUnlessGameInScope skips the test if GAMEPLANE_E2E_GAME_BOT is unset,
// or if the game is not in scope according to GAMEPLANE_E2E_GAMES.
// It logs a clear reason in each case.
func skipUnlessGameInScope(t *testing.T, game string) {
	t.Helper()
	if os.Getenv("GAMEPLANE_E2E_GAME_BOT") == "" {
		t.Skip("heavy: set GAMEPLANE_E2E_GAME_BOT=1 to run the game bot tests")
	}
	scope := parseGameScope()
	for _, allowed := range scope {
		if allowed == game {
			return
		}
	}
	t.Skipf("game %q not in scope (GAMEPLANE_E2E_GAMES=%q)", game, os.Getenv("GAMEPLANE_E2E_GAMES"))
}

// gamePort describes a single port exported by a game container.
type gamePort struct {
	Name     string
	Port     int
	Protocol string // "TCP" | "UDP"
}

// gameResources describes CPU/memory requests and limits for a game container.
type gameResources struct {
	ReqCPU, ReqMem, LimCPU, LimMem string
}

// pathAControl declares how a game is driven THROUGH Gameplane after Path B proves
// the server is running. Empty/unset means no Path A test (server is unchecked for
// control channel availability).
type pathAControl struct {
	Mode   string // "rcon" | "stdin" | "" (no control channel)
	Action string // action id to run
}

// gameBotSpec fully describes a game for bot testing: template config,
// container config, readiness expectations, and probe parameters.
type gameBotSpec struct {
	Game          string // module dir name, e.g. "minecraft-java"
	Template      string // GameTemplate name
	DisplayName   string
	Image         string
	Env           map[string]string
	Ports         []gamePort
	StorageSize   string
	MountPath     string
	Resources     gameResources
	ReadyTimeout  time.Duration
	ProbePort     int
	ProbeDeadline time.Duration
	ExpectDepth   string
	ProbeArgs     []string
	Probes        map[string]any // spec.probes; nil means the operator sets no probe
	RCON          map[string]any // spec.rcon
	ConsoleMode   string         // spec.consoleMode
	Actions       []any          // spec.capabilities.actions; nil means no actions
	Control       pathAControl   // Path A: drive the server through Gameplane (optional)
}

// runGameBotTest creates the GameTemplate + GameServer, waits for
// status.phase == Running, then runs the in-cluster probe. It expects
// the probe to reach ExpectDepth or the test fails.
func runGameBotTest(t *testing.T, s gameBotSpec) {
	t.Helper()
	ctx := context.Background()
	ns := "gameplane-games"

	// Build the env array from the map, sorted for determinism.
	var envKeys []string
	for k := range s.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	var envArray []any
	for _, k := range envKeys {
		envArray = append(envArray, map[string]any{
			"name":  k,
			"value": s.Env[k],
		})
	}

	// Build the ports array.
	var portsArray []any
	for _, p := range s.Ports {
		portsArray = append(portsArray, map[string]any{
			"name":          p.Name,
			"containerPort": int64(p.Port),
			"advertise":     true,
			"protocol":      p.Protocol,
		})
	}

	// Create the GameTemplate.
	spec := map[string]any{
		"displayName": s.DisplayName,
		"game":        s.Game,
		"version":     "1",
		"image":       s.Image,
		"env":         envArray,
		"ports":       portsArray,
		"storage": map[string]any{
			"size":      s.StorageSize,
			"mountPath": s.MountPath,
		},
		"resources": map[string]any{
			"requests": map[string]any{
				"cpu":    s.Resources.ReqCPU,
				"memory": s.Resources.ReqMem,
			},
			"limits": map[string]any{
				"cpu":    s.Resources.LimCPU,
				"memory": s.Resources.LimMem,
			},
		},
	}

	// Set optional probes, rcon, consoleMode, and actions if provided.
	if s.Probes != nil {
		spec["probes"] = s.Probes
	}
	if s.RCON != nil {
		spec["rcon"] = s.RCON
	}
	if s.ConsoleMode != "" {
		spec["consoleMode"] = s.ConsoleMode
	}
	if s.Actions != nil {
		spec["capabilities"] = map[string]any{
			"actions": s.Actions,
		}
	}

	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": s.Template},
		"spec":       spec,
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), s.Template, metav1.DeleteOptions{})
	})

	// Create the GameServer, deriving its name from the template name.
	gsName := s.Template + "-bot"
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": s.Template},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Create(ctx, gs, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create gameserver: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	// Wait for the server to reach Running phase.
	envInstance.Eventually(t, s.ReadyTimeout, func() (bool, string) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Get(ctx, gsName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Sprintf("get gs: %v", err)
		}
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == "Running" {
			return true, ""
		}
		return false, "phase=" + phase
	})

	// Run the in-cluster probe.
	envInstance.RunGameProbe(t, GameProbe{
		GameNS:      ns,
		GSName:      gsName,
		Game:        s.Game,
		Port:        s.ProbePort,
		Deadline:    s.ProbeDeadline,
		ExpectDepth: s.ExpectDepth,
		Args:        s.ProbeArgs,
	})

	// Path A: drive the server THROUGH Gameplane if a control channel is declared.
	// This proves the API, agent, and control protocol integration work end-to-end.
	if s.Control.Mode != "" {
		runGameBotPathA(t, gsName, ns, s.Control)
	}
}

// runGameBotPathA drives the SAME already-running server through Gameplane
// (API → agent → game protocol) to prove end-to-end control integration.
// It expects the server to already be in Running phase and reachable.
// All control modes run an action and assert its effect via log tailing.
func runGameBotPathA(t *testing.T, gsName, ns string, ctrl pathAControl) {
	t.Helper()
	ctx := context.Background()

	// Log in once; reuse the same client for all Path A calls to stay
	// within the login budget (~2 for the entire bot bucket).
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	// RunGameProbe (Path B), which runs before this, only proves the GAME
	// container's protocol port is serving — it says nothing about the
	// agent sidecar. actions/run (rcon transport) and the log-tail
	// WebSocket both dial the agent over the <gs>-agent Service/mTLS, and
	// "GameServer phase == Running" does not mean that path is up yet:
	// the agent container can still be starting, and even once
	// Ready=true the Service's EndpointSlice + kube-proxy dataplane
	// programming lag behind (see requireAgentReady/waitAgentReachable
	// doc comments in api_agent_e2e_test.go, the same pair TestAPI_
	// LogsTailWS waits on before dialing this exact route). Skipping
	// this wait is what made both Path A tests fail in CI with the WS
	// closing "agent dial failed" (StatusBadGateway) on first Read.
	requireAgentReady(t, ns, gsName)
	waitAgentReachable(t, cli, gsName)

	if ctrl.Mode != "" {
		runControlAction(t, ctx, cli, gsName, ns, ctrl)
	}
}

// runControlAction fires an action via the API and asserts its effect
// by tailing logs for a unique message derived from the GameServer name.
// This works for both RCON and stdin/PTY games: RCON outputs through logs,
// stdin outputs through console which is captured in logs.
func runControlAction(t *testing.T, ctx context.Context, cli *APIClient, gsName, ns string, ctrl pathAControl) {
	t.Helper()

	// Derive a unique message from the GameServer name (e.g., gsName="e2e-minecraft-bot" → "e2e-minecraft-bot-pathA").
	// This allows concurrent tests to distinguish their messages in shared logs.
	uniqueMessage := gsName + "-pathA"

	// POST /servers/{gsName}/actions/run with the broadcast action.
	// The action receives the unique message as a parameter.
	resp, body, err := cli.Post(
		fmt.Sprintf("/servers/%s/actions/run", gsName),
		map[string]any{
			"id": ctrl.Action,
			"params": map[string]string{
				"message": uniqueMessage,
			},
		},
	)
	if err != nil {
		t.Fatalf("API rejected the action: POST /actions/run transport error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API rejected the action: POST /servers/%s/actions/run returned %d: %s", gsName, resp.StatusCode, string(body))
	}

	// Parse the response to confirm the API accepted the action.
	var respBody struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &respBody); err != nil {
		t.Fatalf("API rejected the action: decode action response: %v\n%s", err, string(body))
	}
	if !respBody.OK {
		t.Fatalf("API rejected the action: response body reported ok=false")
	}

	// Tail the logs via WebSocket looking for the unique message.
	// The action output is captured in the game's console/log output.
	wsConn, stop := dialAuthedWS(t, cli, fmt.Sprintf("/ws/servers/%s/logs", gsName))
	defer stop()

	readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readCancel()
	for {
		_, frame, err := wsConn.Read(readCtx)
		if err != nil {
			// The API closes the downstream WS with StatusBadGateway and
			// reason "agent dial failed" (api/internal/ws/dialer.go) when
			// its own dial to the agent sidecar's WS endpoint fails — a
			// distinct failure from "the marker never showed up in time".
			// requireAgentReady/waitAgentReachable above should have
			// already ruled this out; distinguish it clearly if it still
			// happens (e.g. the agent restarted between the check and now).
			if websocket.CloseStatus(err) == websocket.StatusBadGateway {
				t.Fatalf("agent unreachable: log-tail WS closed by the API (%v) before %q appeared; the agent sidecar's dial failed even though requireAgentReady/waitAgentReachable passed", err, uniqueMessage)
			}
			t.Fatalf("action ran but its effect never appeared in logs within 30s (want %q): %v", uniqueMessage, err)
		}
		if strings.Contains(string(frame), uniqueMessage) {
			return // Success: found the message in logs.
		}
	}
}
