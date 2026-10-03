package target

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGuardRejectsReplacementAndUnconfiguredAgents(t *testing.T) {
	for _, tc := range []struct {
		name, expected, requested string
		status                    int
	}{
		{"matching incarnation", "uid-original", "uid-original", http.StatusNoContent},
		{"recreated server", "uid-replacement", "uid-original", http.StatusNotFound},
		{"legacy operator", "", "uid-original", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := chi.NewRouter()
			router.Route("/v1/targets/{uid}", func(r chi.Router) {
				r.Use(Guard(tc.expected))
				r.Post("/files/write", func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) })
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/targets/"+tc.requested+"/files/write", nil))
			if response.Code != tc.status {
				t.Fatalf("status=%d, want %d", response.Code, tc.status)
			}
			if called != (tc.status == http.StatusNoContent) {
				t.Fatal("a mismatched target reached the mutation handler")
			}
		})
	}
}
