// Package target binds versioned agent routes to an immutable GameServer UID.
package target

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Guard rejects a route for any server other than the one that started this
// agent. Legacy routes stay available for local callers during rolling upgrades.
func Guard(serverUID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if serverUID == "" || chi.URLParam(req, "uid") != serverUID {
				http.NotFound(w, req)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}
