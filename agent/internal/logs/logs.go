// Package logs streams the game container's log file over a WebSocket.
//
// In-cluster plumbing: the operator mounts the game container's log as a
// readable file into the agent via a shared emptyDir + symlink, or the
// game itself is configured to write to /data/logs/latest.log. This
// package doesn't care — it just tails whatever path it's handed.
package logs

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	path string
}

func Mount(r chi.Router, path string) {
	h := &handler{path: path}
	r.Get("/logs/tail", h.tail)
}

func (h *handler) tail(w http.ResponseWriter, req *http.Request) {
	if h.path == "" {
		http.Error(w, "log tailing not configured (set --game-log-path)", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	// Read any "follow from now" marker: ?from=end (default) vs ?from=start.
	from := req.URL.Query().Get("from")
	fromEnd := from != "start"

	if err := streamFile(ctx, conn, h.path, fromEnd); err != nil && !errors.Is(err, context.Canceled) {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}
}

// streamFile tails path, delivering each full line as a text WS frame.
// Reopens the file on rotation (ENOENT or inode change) with a short
// backoff so logrotate-style setups keep working.
func streamFile(ctx context.Context, conn *websocket.Conn, path string, fromEnd bool) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if sleep(ctx, time.Second) != nil {
					return ctx.Err()
				}
				continue
			}
			return err
		}
		if fromEnd {
			_, _ = f.Seek(0, io.SeekEnd)
		}
		if err := tailLoop(ctx, conn, f); err != nil {
			_ = f.Close()
			if errors.Is(err, errRotated) {
				fromEnd = false
				continue
			}
			return err
		}
		_ = f.Close()
		return nil
	}
}

var errRotated = errors.New("log file rotated")

func tailLoop(ctx context.Context, conn *websocket.Conn, f *os.File) error {
	reader := bufio.NewReader(f)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if werr := conn.Write(ctx, websocket.MessageText, []byte(line)); werr != nil {
				return werr
			}
		}
		switch {
		case err == nil:
			continue
		case errors.Is(err, io.EOF):
			if rotated, rerr := checkRotation(f); rerr != nil {
				return rerr
			} else if rotated {
				return errRotated
			}
			if sleep(ctx, 200*time.Millisecond) != nil {
				return ctx.Err()
			}
		default:
			return err
		}
	}
}

func checkRotation(f *os.File) (bool, error) {
	fi1, err := f.Stat()
	if err != nil {
		return false, err
	}
	fi2, err := os.Stat(f.Name())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return !os.SameFile(fi1, fi2), nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
