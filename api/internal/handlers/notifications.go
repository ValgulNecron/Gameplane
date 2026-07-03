package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/notify"
)

// MountNotifications exposes notification-sink operations under
// /admin/notifications. Sink CRUD stays in the config store
// (PUT /admin/config/notifications); this surface only adds the
// test-send, which dials an external endpoint synchronously and so
// carries its own tight rate limit on top of the global mutation cap.
func MountNotifications(r chi.Router, n *notify.Notifier) {
	r.With(auth.NotifyTestLimiter.Middleware).
		Post("/admin/notifications/sinks/{name}/test", func(w http.ResponseWriter, req *http.Request) {
			err := n.DeliverTest(req.Context(), chi.URLParam(req, "name"))
			switch {
			case err == nil:
				writeJSON(w, map[string]any{"delivered": true})
			case errors.Is(err, notify.ErrUnknownSink):
				http.Error(w, err.Error(), http.StatusNotFound)
			case errors.Is(err, notify.ErrSinkNotConfigured):
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			default:
				// Sink exists and is configured but delivery failed — relay
				// the (URL-sanitized) cause so the admin can fix the endpoint
				// or the Secret.
				http.Error(w, err.Error(), http.StatusBadGateway)
			}
		})
}
