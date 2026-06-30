package handlers

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/audit"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
)

func MountAudit(r chi.Router, a *audit.Auditor) {
	r.Get("/admin/audit", func(w http.ResponseWriter, req *http.Request) {
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		before, _ := strconv.ParseInt(req.URL.Query().Get("before"), 10, 64)
		events, err := a.Page(req, limit, before)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		audit.WriteJSON(w, events)
	})

	// Export streams the full audit log (optionally bounded by RFC3339
	// since/until) as a CSV or JSON download. Unlike the paginated /admin/audit
	// view (and the dashboard's client-side CSV, which only covers the loaded
	// page), this gives the entire trail in one request — for compliance
	// archival or an external pipeline. Streams row-by-row so a large table
	// doesn't have to be buffered in memory.
	r.Get("/admin/audit/export", func(w http.ResponseWriter, req *http.Request) {
		format := strings.ToLower(req.URL.Query().Get("format"))
		if format == "" {
			format = "csv"
		}
		since := req.URL.Query().Get("since")
		until := req.URL.Query().Get("until")
		stamp := time.Now().UTC().Format("20060102-150405")

		switch format {
		case "csv":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition",
				fmt.Sprintf(`attachment; filename="gameplane-audit-%s.csv"`, stamp))
			cw := csv.NewWriter(w)
			_ = cw.Write([]string{"id", "ts", "actor", "method", "path", "target", "status", "ip"})
			err := a.Stream(req.Context(), since, until, func(e audit.Event) error {
				return cw.Write([]string{
					strconv.FormatInt(e.ID, 10), e.TS, e.Actor, e.Method, e.Path,
					e.Target, strconv.Itoa(e.Status), e.IP,
				})
			})
			cw.Flush()
			// Headers are already sent, so a mid-stream failure can't change the
			// status code; surface it in the log so the operator knows the export
			// is truncated rather than complete.
			if err != nil {
				slog.Warn("audit export failed mid-stream", "format", "csv", "err", err)
			}
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition",
				fmt.Sprintf(`attachment; filename="gameplane-audit-%s.json"`, stamp))
			bw := bufio.NewWriter(w)
			_ = bw.WriteByte('[')
			first := true
			err := a.Stream(req.Context(), since, until, func(e audit.Event) error {
				if !first {
					_ = bw.WriteByte(',')
				}
				first = false
				b, mErr := json.Marshal(e)
				if mErr != nil {
					return mErr
				}
				_, wErr := bw.Write(b)
				return wErr
			})
			_ = bw.WriteByte(']')
			_ = bw.Flush()
			if err != nil {
				slog.Warn("audit export failed mid-stream", "format", "json", "err", err)
			}
		default:
			httperr.WriteCode(w, req, http.StatusBadRequest,
				errors.New("format must be csv or json"))
		}
	})
}
