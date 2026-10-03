package httpserver

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedBoundCapture(t *testing.T, s *Server, id string) {
	t.Helper()
	if _, err := s.prepareCaptureBinding(id, startRequest{ServerUID: "server-uid", CaptureUID: "capture-uid"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.captureFilePath(id), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBoundCaptureSurvivesRestartAndRejectsReplacement(t *testing.T) {
	s, _ := newTestServer(t)
	s.serverUID = "server-uid"
	seedBoundCapture(t, s, "cap-one")
	// A new server has neither active state nor the bounded completed cache.
	restarted := NewServer(context.Background(), s.captureDataDir, 0, "server-uid")
	path := "/v1/targets/server-uid/captures/cap-one/uids/capture-uid/file"
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		for _, wrong := range []string{strings.Replace(path, "server-uid", "new-server", 1), strings.Replace(path, "capture-uid", "new-capture", 1), path + "?url=http://elsewhere"} {
			rr := httptest.NewRecorder()
			restarted.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), method, wrong, nil))
			if rr.Code != http.StatusNotFound {
				t.Fatalf("%s %s: %d", method, wrong, rr.Code)
			}
		}
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req.Header.Set("Range", "bytes=2-5")
	rr := httptest.NewRecorder()
	restarted.Routes(nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusPartialContent || rr.Body.String() != "2345" || rr.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %v", rr.Code, rr.Body.String(), rr.Header())
	}
	rr = httptest.NewRecorder()
	restarted.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, path, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatal(rr.Code)
	}
	if _, err := os.Stat(s.captureFilePath("cap-one")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("PCAP retained: %v", err)
	}
	// Lost-response retry preserves an explicit acknowledgement for the same UID.
	rr = httptest.NewRecorder()
	restarted.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, path, nil))
	if rr.Code != 204 {
		t.Fatal(rr.Code)
	}
	rr = httptest.NewRecorder()
	restarted.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if rr.Code != 410 {
		t.Fatal(rr.Code)
	}
}

func TestBoundCaptureRefusesUnboundAndRetainedNameReuse(t *testing.T) {
	s, _ := newTestServer(t)
	s.serverUID = "server-uid"
	seedBoundCapture(t, s, "cap-bound")
	for _, identity := range []startRequest{{}, {ServerUID: "server-uid", CaptureUID: "capture-uid"}, {ServerUID: "server-uid", CaptureUID: "new-uid"}} {
		if _, err := s.prepareCaptureBinding("cap-bound", identity); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("reused bound name: %v", err)
		}
	}
	if err := os.WriteFile(s.captureFilePath("cap-legacy"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rr := httptest.NewRecorder()
		s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), method, "/v1/targets/server-uid/captures/cap-legacy/uids/capture-uid/file", nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("legacy %s: %d", method, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/captures/cap-legacy/file", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "legacy" {
		t.Fatalf("legacy local download: %d %q", rr.Code, rr.Body.String())
	}
}

func TestCaptureStartIdentityAndAuthenticatedBoundRoutes(t *testing.T) {
	s, factory := newTestServer(t)
	s.serverUID = "server-uid"
	for _, identity := range []string{`"serverUID":"other","captureUID":"capture-uid"`, `"serverUID":"server-uid"`, `"captureUID":"../escape"`} {
		body := `{` + identity + `,"filter":"tcp port 12345","maxDurationSeconds":60,"maxSizeBytes":4096}`
		rr := httptest.NewRecorder()
		s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/captures/cap-start/start", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest || factory.count() != 0 {
			t.Fatalf("identity opened capture: %d sources=%d", rr.Code, factory.count())
		}
	}
	seedBoundCapture(t, s, "cap-auth")
	wrapped := s.Routes(func(_ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "unauthorized", http.StatusUnauthorized) })
	})
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), method, "/v1/targets/server-uid/captures/cap-auth/uids/capture-uid/file", nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("unwrapped route: %d", rr.Code)
		}
	}
}

func TestControlIdentityCannotAdoptOrStopNamesake(t *testing.T) {
	s, _ := newTestServer(t)
	s.serverUID = "server-uid"
	body := `{"serverUID":"server-uid","captureUID":"capture-uid","filter":"tcp port 12345","maxDurationSeconds":60,"maxSizeBytes":4096}`
	router := s.Routes(nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/captures/cap-one/start", strings.NewReader(body)))
	if rr.Code != 200 {
		t.Fatalf("start: %d %s", rr.Code, rr.Body)
	}
	for _, op := range []struct{ method, path string }{{http.MethodGet, "/captures/cap-one/status"}, {http.MethodPost, "/captures/cap-one/stop"}, {http.MethodDelete, "/captures/cap-one"}} {
		req := httptest.NewRequestWithContext(t.Context(), op.method, op.path, nil)
		req.Header.Set("X-Gameplane-Server-UID", "server-uid")
		req.Header.Set("X-Gameplane-Capture-UID", "replacement")
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != 404 {
			t.Fatalf("replacement %s: %d", op.path, rr.Code)
		}
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/captures/cap-one/stop", nil)
	req.Header.Set("X-Gameplane-Server-UID", "server-uid")
	req.Header.Set("X-Gameplane-Capture-UID", "capture-uid")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("bound stop: %d %s", rr.Code, rr.Body)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/captures/cap-one/start", strings.NewReader(strings.Replace(body, "capture-uid", "new-uid", 1))))
	if rr.Code != 409 {
		t.Fatalf("reused retained name: %d", rr.Code)
	}
}

func TestCaptureTombstoneRetentionAndFinalization(t *testing.T) {
	s, _ := newTestServer(t)
	s.serverUID = "server-uid"
	seedBoundCapture(t, s, "cap-one")
	s.finishingCapture = &captureState{id: "cap-one", status: statusRunning}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		rr := httptest.NewRecorder()
		s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), method, "/v1/targets/server-uid/captures/cap-one/uids/capture-uid/file", nil))
		if rr.Code != 409 {
			t.Fatalf("finalizing %s: %d", method, rr.Code)
		}
	}
	s.finishingCapture = nil
	if err := os.Remove(s.captureFilePath("cap-one")); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(s.captureIdentityPath("cap-one"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := s.prepareCaptureBinding("cap-one", startRequest{ServerUID: "server-uid", CaptureUID: "replacement"}); err != nil {
		t.Fatalf("expired tombstone was not reclaimed: %v", err)
	}
	rr := httptest.NewRecorder()
	s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/targets/server-uid/captures/cap-one/uids/capture-uid/file", nil))
	if rr.Code != 404 {
		t.Fatalf("stale UID after reuse: %d", rr.Code)
	}
}

func TestCaptureFileOpenCannotEscapeRoot(t *testing.T) {
	s, _ := newTestServer(t)
	for _, name := range []string{"../outside", filepath.Join(t.TempDir(), "outside")} {
		if file, err := s.openCaptureRegular(name); err == nil {
			_ = file.Close()
			t.Fatalf("accepted outside path %q", name)
		}
	}
	outside := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(s.captureDataDir, "capture-link.pcapng")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if file, err := s.openCaptureRegular("capture-link.pcapng"); err == nil {
		_ = file.Close()
		t.Fatal("followed capture symlink")
	}
}

func TestBoundDeleteAcknowledgesAbsentCapture(t *testing.T) {
	for _, scenario := range []string{"absent", "wrong server", "query", "invalid UID", "corrupt identity", "wrong identity", "unbound PCAP", "identity symlink", "PCAP symlink", "unavailable directory", "active", "finishing", "flushing"} {
		t.Run(scenario, func(t *testing.T) {
			s, _ := newTestServer(t)
			s.serverUID = "server-uid"
			path := "/v1/targets/server-uid/captures/cap-one/uids/capture-uid/file"
			want := http.StatusNotFound
			t.Cleanup(func() {
				s.currentCapture, s.finishingCapture = nil, nil
				delete(s.completed, "cap-one")
			})
			write := func(path, data string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "absent":
				want = http.StatusGone
			case "wrong server":
				path = strings.Replace(path, "server-uid", "replacement", 1)
			case "query":
				path += "?extra=true"
			case "invalid UID":
				path = strings.Replace(path, "capture-uid", "bad_uid", 1)
			case "corrupt identity":
				write(s.captureIdentityPath("cap-one"), "{")
			case "wrong identity":
				write(s.captureIdentityPath("cap-one"), `{"serverUID":"server-uid","captureUID":"replacement"}`)
			case "unbound PCAP":
				write(s.captureFilePath("cap-one"), "legacy")
			case "identity symlink", "PCAP symlink":
				link := s.captureIdentityPath("cap-one")
				if scenario == "PCAP symlink" {
					link = s.captureFilePath("cap-one")
				}
				// A dangling symlink is not positive evidence of an absent entry.
				if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), link); err != nil {
					t.Fatal(err)
				}
			case "unavailable directory":
				s.captureDataDir = filepath.Join(s.captureDataDir, "missing")
			case "active":
				s.currentCapture = &captureState{id: "cap-one", status: statusRunning}
				want = http.StatusConflict
			case "finishing":
				s.finishingCapture = &captureState{id: "cap-one", status: statusRunning}
				want = http.StatusConflict
			case "flushing":
				s.completed["cap-one"] = &captureState{id: "cap-one", status: statusRunning}
				want = http.StatusConflict
			}
			rr := httptest.NewRecorder()
			s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, path, nil))
			if rr.Code != want {
				t.Fatalf("delete=%d %s; want=%d", rr.Code, rr.Body, want)
			}
			if scenario == "absent" {
				rr = httptest.NewRecorder()
				s.Routes(nil).ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
				if rr.Code != http.StatusNotFound {
					t.Fatalf("unbound download=%d", rr.Code)
				}
			}
		})
	}
}
