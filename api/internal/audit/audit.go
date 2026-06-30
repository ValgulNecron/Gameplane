// Package audit records every mutating request to the API for later
// review by administrators. Records land in the audit_events table
// and are exposed via /admin/audit.
package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

type Auditor struct {
	db   *db.Store
	sink *slog.Logger // structured stdout sink; nil disables it
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
			_, err := a.db.DB.ExecContext(req.Context(),
				`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				time.Now().UTC().Format(time.RFC3339),
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

// Stream invokes fn for every audit event within the optional time window,
// oldest-first. since/until are RFC3339 bounds (inclusive); an empty string
// means unbounded. It iterates rows rather than buffering, so the whole table
// can be exported without holding it in memory. ts is stored as fixed-width
// RFC3339 (see Middleware), so the lexicographic comparison is chronological —
// no per-row parsing, and it stays portable across the sqlite and pgx drivers.
func (a *Auditor) Stream(ctx context.Context, since, until string, fn func(Event) error) error {
	rows, err := a.db.DB.QueryContext(ctx,
		`SELECT id, ts, actor, method, path, COALESCE(target,''), status, COALESCE(ip,'')
		 FROM audit_events
		 WHERE (? = '' OR ts >= ?) AND (? = '' OR ts <= ?)
		 ORDER BY id ASC`, since, since, until, until,
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
