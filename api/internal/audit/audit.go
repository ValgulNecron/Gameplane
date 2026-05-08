// Package audit records every mutating request to the API for later
// review by administrators. Records land in the audit_events table
// and are exposed via /admin/audit.
package audit

import (
	"encoding/json"
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
			rw := &responseRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, req)
			if !shouldLog(req) {
				return
			}
			actor := "anonymous"
			if u := auth.UserFromContext(req.Context()); u != nil {
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
