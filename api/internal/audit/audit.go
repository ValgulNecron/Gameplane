// Package audit records every mutating request to the API for later
// review by administrators. Records land in the audit_events table
// and are exposed via /admin/audit.
package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/db"
)

type Auditor struct {
	db *db.Store
}

func New(store *db.Store) *Auditor { return &Auditor{db: store} }

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
			_, _ = a.db.DB.ExecContext(req.Context(),
				`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				time.Now().UTC().Format(time.RFC3339),
				actor, req.Method, req.URL.Path, req.URL.Query().Get("name"),
				rw.status, req.RemoteAddr,
			)
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

// WriteJSON is a convenience for handlers.
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
