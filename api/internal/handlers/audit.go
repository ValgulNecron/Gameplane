package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/api/internal/audit"
	"github.com/kestrel-gg/kestrel/api/internal/httperr"
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
}
