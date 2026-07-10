// Package audit records every mutating request to the API for later
// review by administrators. Records land in the audit_events table
// and are exposed via /admin/audit.
package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// webhookEvents counts audit-event webhook deliveries by outcome. A "dropped"
// or "failed" delta is operationally important — it means the external audit
// mirror has a gap — so it's surfaced at /metrics (default registry, served by
// promhttp).
var webhookEvents = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "gameplane_audit_webhook_events_total",
	Help: "Audit-event webhook deliveries by result (sent, failed, dropped).",
}, []string{"result"})

type Auditor struct {
	db      *db.Store
	sink    *slog.Logger // structured stdout sink; nil disables it
	webhook *WebhookSink // outbound HTTP push sink; nil disables it
	s3      *S3Sink      // S3 batch sink; nil disables it
}

// Option configures an Auditor.
type Option func(*Auditor)

// WithStdoutSink mirrors each audited event to logger as a structured log
// line, so cluster log aggregation (Loki/ELK/CloudWatch scraping the pod's
// stdout) captures the audit trail — the Kubernetes-native "external sink".
// A nil logger leaves the sink disabled (the default; events still land in
// the database).
func WithStdoutSink(logger *slog.Logger) Option {
	return func(a *Auditor) { a.sink = logger }
}

// WithWebhookSink pushes each audited event to an external HTTP endpoint. A nil
// sink leaves it disabled (the default). The sink's worker must be started
// (sink.Start) by the caller; see NewWebhookSink.
func WithWebhookSink(s *WebhookSink) Option {
	return func(a *Auditor) { a.webhook = s }
}

// WithS3Sink batches each audited event into NDJSON objects written to an
// S3-compatible endpoint. A nil sink leaves it disabled (the default). The
// sink's worker must be started (sink.Start) by the caller; see NewS3Sink.
func WithS3Sink(s *S3Sink) Option {
	return func(a *Auditor) { a.s3 = s }
}

// webhookBuffer bounds how many unsent events the webhook sink holds before it
// starts dropping. Audit events are low-rate (one per mutating request), so a
// healthy endpoint never approaches this; the bound exists so a stalled
// endpoint can't grow memory without limit.
const webhookBuffer = 1024

// WebhookSink mirrors each audit event to an external HTTP endpoint by POSTing
// it as JSON. Delivery is best-effort and fully decoupled from the request
// path: events are handed to a bounded buffer and shipped by a single
// background worker, so a slow or unreachable endpoint never blocks — or fails
// — an audited request. The database stays the source of truth; this is the
// same "mirror, don't gate" contract as the stdout sink, just pushed rather
// than scraped.
type WebhookSink struct {
	url    string
	auth   string // optional Authorization header value; "" omits the header
	client *http.Client
	ch     chan Event
}

// NewWebhookSink returns a sink that POSTs audit events as JSON to url.
// authHeader, when non-empty, is sent verbatim as the Authorization header
// (e.g. "Bearer <token>"). Call Start to launch the delivery worker.
func NewWebhookSink(url, authHeader string) *WebhookSink {
	return &WebhookSink{
		url:    url,
		auth:   authHeader,
		client: &http.Client{Timeout: 5 * time.Second},
		ch:     make(chan Event, webhookBuffer),
	}
}

// Enqueue hands an event to the worker without ever blocking. When the buffer
// is full (a stalled endpoint backing up), the event is dropped and counted —
// a dropped mirror leaves a hole in the external trail, so it must be visible.
func (s *WebhookSink) Enqueue(e Event) {
	select {
	case s.ch <- e:
	default:
		webhookEvents.WithLabelValues("dropped").Inc()
	}
}

// Start runs the delivery worker until ctx is cancelled, then best-effort
// drains whatever is already buffered within a short deadline. It blocks; run
// it in a goroutine.
func (s *WebhookSink) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.drain()
			return
		case e := <-s.ch:
			s.post(e)
		}
	}
}

// drain ships already-buffered events on shutdown, bounded by a short deadline
// so a wedged endpoint can't stall process exit.
func (s *WebhookSink) drain() {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-s.ch:
			s.post(e)
		case <-deadline:
			return
		default:
			return
		}
	}
}

// post delivers one event. It deliberately uses a detached context (bounded by
// the client's own timeout) rather than the worker's lifecycle context: at
// shutdown the select in Start can still pick a buffered event after ctx is
// cancelled, and a cancelled context would fail that delivery even though the
// event could have been shipped. The client timeout still bounds each attempt.
func (s *WebhookSink) post(e Event) {
	body, err := json.Marshal(webhookPayload{
		TS: e.TS, Actor: e.Actor, Method: e.Method, Path: e.Path,
		Target: e.Target, Status: e.Status, IP: e.IP,
	})
	if err != nil {
		webhookEvents.WithLabelValues("failed").Inc()
		slog.Warn("audit webhook marshal failed", "err", err)
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		webhookEvents.WithLabelValues("failed").Inc()
		slog.Warn("audit webhook build request failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if s.auth != "" {
		req.Header.Set("Authorization", s.auth)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		webhookEvents.WithLabelValues("failed").Inc()
		slog.Warn("audit webhook post failed", "err", err, "url", s.url)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		webhookEvents.WithLabelValues("failed").Inc()
		slog.Warn("audit webhook non-2xx", "status", resp.StatusCode, "url", s.url)
		return
	}
	webhookEvents.WithLabelValues("sent").Inc()
}

// webhookPayload is the JSON shipped to the webhook: the audit event minus the
// database row id, which isn't known at emit time and is meaningless to an
// external sink (which keys on ts/actor/path).
type webhookPayload struct {
	TS     string `json:"ts"`
	Actor  string `json:"actor"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Target string `json:"target,omitempty"`
	Status int    `json:"status"`
	IP     string `json:"ip,omitempty"`
}

func New(store *db.Store, opts ...Option) *Auditor {
	a := &Auditor{db: store}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Middleware logs every mutating request after the handler returns.
// Reads and health probes are skipped to keep the audit log signal-dense.
func Middleware(a *Auditor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Install an actor holder before the chain runs. Authenticate
			// sets it on this same context; the user it puts on a child
			// context never propagates back up here, which is why audit
			// rows used to record "anonymous" for authenticated actions.
			ctx, holder := auth.WithActorHolder(req.Context())
			req = req.WithContext(ctx)
			rw := &responseRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, req)
			if !shouldLog(req) {
				return
			}
			actor := "anonymous"
			if name := holder.Name(); name != "" {
				actor = name
			} else if u := auth.UserFromContext(req.Context()); u != nil && u.Username != "" {
				// Fallback for callers that put the user directly on this
				// context instead of via the actor holder. In the normal
				// chain Authenticate fills the holder, so this never overrides
				// it; in production the authenticated user lives on a child
				// context the audit middleware can't see, so this stays nil.
				actor = u.Username
			}
			target := req.URL.Query().Get("name")
			// Stamp once so the DB row, stdout line, and webhook payload all
			// agree on the event time.
			ts := time.Now().UTC().Format(time.RFC3339)
			_, err := a.db.DB.ExecContext(req.Context(),
				`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				ts,
				actor, req.Method, req.URL.Path, target,
				rw.status, req.RemoteAddr,
			)
			if err != nil {
				// A dropped security-audit write must not be silent — surface it
				// so an operator notices the trail has a hole.
				slog.Warn("audit insert failed",
					"err", err, "actor", actor, "method", req.Method, "path", req.URL.Path)
			}
			if a.sink != nil {
				a.sink.Info("audit",
					"actor", actor, "method", req.Method, "path", req.URL.Path,
					"target", target, "status", rw.status, "ip", req.RemoteAddr)
			}
			if a.webhook != nil {
				a.webhook.Enqueue(Event{
					TS: ts, Actor: actor, Method: req.Method, Path: req.URL.Path,
					Target: target, Status: rw.status, IP: req.RemoteAddr,
				})
			}
			if a.s3 != nil {
				a.s3.Enqueue(Event{
					TS: ts, Actor: actor, Method: req.Method, Path: req.URL.Path,
					Target: target, Status: rw.status, IP: req.RemoteAddr,
				})
			}
		})
	}
}

func shouldLog(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	switch {
	case req.URL.Path == "/healthz", req.URL.Path == "/metrics":
		return false
	case strings.HasPrefix(req.URL.Path, "/auth/oidc/"):
		// Login events are audited only on success via the session
		// creation path; OIDC callbacks themselves are too noisy.
		return false
	}
	return true
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying writer. Without this, the embedded
// interface hides the concrete writer's Hijacker and every WebSocket
// upgrade behind this middleware fails with 501 (websocket.Accept
// type-asserts http.Hijacker).
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("audit: underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Flush forwards so streaming responses keep working through the
// recorder.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ---- query API ----

type Event struct {
	ID     int64  `json:"id"`
	TS     string `json:"ts"`
	Actor  string `json:"actor"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Target string `json:"target,omitempty"`
	Status int    `json:"status"`
	IP     string `json:"ip,omitempty"`
}

// Page returns the most recent events, oldest-first within the page.
// `before` is an ID cursor (exclusive); 0 means "from latest".
func (a *Auditor) Page(req *http.Request, limit int, before int64) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := a.db.DB.QueryContext(req.Context(),
		`SELECT id, ts, actor, method, path, COALESCE(target,''), status, COALESCE(ip,'')
		 FROM audit_events
		 WHERE (? = 0 OR id < ?)
		 ORDER BY id DESC
		 LIMIT ?`, before, before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Event, 0, limit)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.TS, &e.Actor, &e.Method, &e.Path, &e.Target, &e.Status, &e.IP); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StreamFilter narrows an export. The zero value streams everything.
//
// It deliberately mirrors the dashboard's audit-page filters so an export
// contains exactly the rows the operator is looking at — the page filters
// client-side over the pages it has scrolled, which is a different (and
// smaller) set than the table holds.
type StreamFilter struct {
	Since  string // RFC3339 lower bound, inclusive; "" = unbounded
	Until  string // RFC3339 upper bound, inclusive; "" = unbounded
	Actor  string // case-insensitive substring; "" = any
	Method string // exact HTTP method; "" = any
	// StatusMin/StatusMax bound the HTTP status inclusively. StatusMax == 0 means no status filter.
	StatusMin int
	StatusMax int
}

// likeEscape neutralizes the LIKE wildcards so an actor containing "%" or
// "_" matches literally instead of broadening the export.
func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// Stream invokes fn for every audit event matching f, oldest-first. It
// iterates rows rather than buffering, so the whole table can be exported
// without holding it in memory. ts is stored as fixed-width RFC3339 (see
// Middleware), so the lexicographic comparison is chronological — no per-row
// parsing, and it stays portable across the sqlite and pgx drivers.
func (a *Auditor) Stream(ctx context.Context, f StreamFilter, fn func(Event) error) error {
	actorPattern := "%" + likeEscape(strings.ToLower(f.Actor)) + "%"
	rows, err := a.db.DB.QueryContext(ctx,
		`SELECT id, ts, actor, method, path, COALESCE(target,''), status, COALESCE(ip,'')
		 FROM audit_events
		 WHERE (? = '' OR ts >= ?) AND (? = '' OR ts <= ?)
		   AND (? = '' OR LOWER(actor) LIKE ? ESCAPE '\')
		   AND (? = '' OR method = ?)
		   AND (? = 0 OR (status >= ? AND status <= ?))
		 ORDER BY id ASC`,
		f.Since, f.Since, f.Until, f.Until,
		f.Actor, actorPattern,
		f.Method, f.Method,
		f.StatusMax, f.StatusMin, f.StatusMax,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.TS, &e.Actor, &e.Method, &e.Path, &e.Target, &e.Status, &e.IP); err != nil {
			return err
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ---- retention ----

// Prune deletes audit events whose ts predates the cutoff and returns the
// number of rows removed. ts is stored as RFC3339 (see Middleware), a
// fixed-width zero-padded UTC format, so a lexicographic "<" comparison is
// also a chronological one — no per-row time parsing needed, and the query
// stays portable across the sqlite and postgres drivers.
func (a *Auditor) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := a.db.DB.ExecContext(ctx,
		`DELETE FROM audit_events WHERE ts < ?`,
		cutoff.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("prune audit events: %w", err)
	}
	return res.RowsAffected()
}

// RunRetention periodically prunes audit events older than the retention
// window, blocking until ctx is cancelled. A sweep runs immediately on start
// (a long-lived process shouldn't wait a full interval to first prune) and
// then every interval. A retention of zero or less disables retention and
// returns immediately, preserving the keep-forever default.
func (a *Auditor) RunRetention(ctx context.Context, retention, interval time.Duration, logger *slog.Logger) {
	if retention <= 0 {
		return
	}
	sweep := func() {
		cutoff := time.Now().Add(-retention)
		n, err := a.Prune(ctx, cutoff)
		if err != nil {
			logger.Warn("audit retention sweep failed", "err", err)
			return
		}
		if n > 0 {
			logger.Info("audit retention sweep",
				"deleted", n, "olderThan", cutoff.UTC().Format(time.RFC3339))
		}
	}
	sweep()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}

// WriteJSON is a convenience for handlers.
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
