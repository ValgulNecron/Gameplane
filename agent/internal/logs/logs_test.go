package logs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

func mountServer(t *testing.T, path string) string {
	t.Helper()
	r := chi.NewRouter()
	Mount(r, path)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv.URL
}

func testGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return http.DefaultClient.Do(req)
}

func TestTail_NotConfigured(t *testing.T) {
	url := mountServer(t, "")
	resp, err := testGet(t, url+"/logs/tail")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestTail_StreamsLinesAndRotates(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "latest.log")
	if err := os.WriteFile(logPath, []byte(""), 0o600); err != nil {
		t.Fatalf("create: %v", err)
	}

	url := mountServer(t, logPath)
	wsURL := "ws" + strings.TrimPrefix(url, "http") + "/logs/tail?from=start"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	// Append a line.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()

	// Read first line.
	mt, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageText || string(b) != "hello\n" {
		t.Fatalf("got %d %q", mt, b)
	}

	// Rotate: rename current and create fresh file with a new line.
	rotated := logPath + ".1"
	if err := os.Rename(logPath, rotated); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	f2, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if _, err := f2.WriteString("after-rotate\n"); err != nil {
		t.Fatalf("write2: %v", err)
	}
	_ = f2.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read after rotate: %v", err)
		}
		if string(b) == "after-rotate\n" {
			return
		}
	}
	t.Fatal("did not see post-rotate line")
}

func TestTail_FileMissingThenAppears(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "latest.log")
	url := mountServer(t, logPath)
	wsURL := "ws" + strings.TrimPrefix(url, "http") + "/logs/tail?from=start"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	// Wait briefly so streamFile hits ENOENT once and enters its 1s
	// retry sleep, then create the file.
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(logPath, []byte("late-start\n"), 0o600); err != nil {
		t.Fatalf("create: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(b) == "late-start\n" {
			return
		}
	}
	t.Fatal("did not see line after file appeared")
}

func TestCheckRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	t.Run("same file", func(t *testing.T) {
		rot, err := checkRotation(f)
		if err != nil || rot {
			t.Fatalf("rot=%v err=%v", rot, err)
		}
	})

	t.Run("file deleted", func(t *testing.T) {
		_ = os.Remove(path)
		rot, err := checkRotation(f)
		if err != nil || !rot {
			t.Fatalf("rot=%v err=%v", rot, err)
		}
	})

	t.Run("file replaced (rotated)", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("b"), 0o600); err != nil {
			t.Fatalf("recreate: %v", err)
		}
		rot, err := checkRotation(f)
		if err != nil || !rot {
			t.Fatalf("rot=%v err=%v", rot, err)
		}
	})
}

func TestSleep_Cancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleep(ctx, time.Hour); err == nil {
		t.Fatal("expected ctx err")
	}
}

func TestSleep_Elapses(t *testing.T) {
	if err := sleep(context.Background(), 10*time.Millisecond); err != nil {
		t.Fatalf("err=%v", err)
	}
}
