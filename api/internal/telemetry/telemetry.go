// Package telemetry implements an opt-in, off-by-default reporter that
// periodically POSTs anonymous, aggregate usage counts to a configurable
// endpoint. It exists so the admin "Send anonymous metrics" toggle does
// something — previously it persisted a value nothing read.
//
// Privacy: the payload carries only the control-plane version and total
// counts. No names, namespaces, hostnames, IPs, or identifiers. Reporting
// happens only when BOTH an endpoint is configured (operator-set) AND the
// sendMetrics config toggle is on (admin-set) — opt-in on two axes.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

type Reporter struct {
	store    *db.Store
	k        *kube.Client
	endpoint string
	version  string
	interval time.Duration
	client   *http.Client
}

func New(store *db.Store, k *kube.Client, endpoint, version string, interval time.Duration) *Reporter {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Reporter{
		store:    store,
		k:        k,
		endpoint: endpoint,
		version:  version,
		interval: interval,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// payload is the anonymous, aggregate telemetry. Deliberately carries no
// names, namespaces, hosts, or identifiers.
type payload struct {
	Version   string `json:"version"`
	Servers   int    `json:"servers"`
	Templates int    `json:"templates"`
}

// Run reports on an interval until ctx is cancelled. It returns
// immediately when no endpoint is configured (telemetry off).
func (r *Reporter) Run(ctx context.Context) {
	if r.endpoint == "" {
		slog.Info("telemetry disabled: no endpoint configured")
		return
	}
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.reportOnce(ctx); err != nil {
				slog.Warn("telemetry report", "err", err)
			}
		}
	}
}

// enabled reads the admin sendMetrics toggle on each tick so flipping it
// in the UI takes effect without a restart.
func (r *Reporter) enabled(ctx context.Context) bool {
	raw, ok, err := r.store.ConfigValue(ctx, "telemetry")
	if err != nil || !ok {
		return false
	}
	var c struct {
		SendMetrics bool `json:"sendMetrics"`
	}
	if json.Unmarshal([]byte(raw), &c) != nil {
		return false
	}
	return c.SendMetrics
}

// reportOnce collects and POSTs one telemetry sample, or does nothing
// when the admin toggle is off.
func (r *Reporter) reportOnce(ctx context.Context) error {
	if !r.enabled(ctx) {
		return nil
	}
	p := payload{Version: r.version}
	if n, err := r.count(ctx, "servers"); err == nil {
		p.Servers = n
	}
	if n, err := r.count(ctx, "templates"); err == nil {
		p.Templates = n
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry endpoint returned %d", resp.StatusCode)
	}
	return nil
}

func (r *Reporter) count(ctx context.Context, kind string) (int, error) {
	gvr, ok := kube.GVRs[kind]
	if !ok {
		return 0, nil
	}
	list, err := r.k.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(list.Items), nil
}
