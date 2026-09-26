package logs

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestDownload_StatNonNotExistError covers download's non-ENOENT os.Stat
// error branch (the 500 path, distinct from the 404 ErrNotExist path
// other tests already cover): the configured path's parent component is
// a regular file, so os.Stat fails with ENOTDIR rather than ENOENT.
func TestDownload_StatNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notadir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	badPath := filepath.Join(file, "child") // "notadir/child": ENOTDIR, not ENOENT

	url := mountServer(t, badPath)
	resp, err := testGet(t, url+"/logs/download")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestStreamFile_DirectoryPathReturnsError covers streamFile's final
// error-propagation return after tailLoop fails on a non-rotation error:
// reading from a directory FD fails with EISDIR, which tailLoop's default
// switch case surfaces, and streamFile must close the handle and return
// that error rather than looping or swallowing it.
func TestStreamFile_DirectoryPathReturnsError(t *testing.T) {
	dir := t.TempDir() // a directory used as the "log file" path

	err := streamFile(context.Background(), nil, dir, false)
	if err == nil {
		t.Fatal("streamFile on a directory path should fail once ReadString hits EISDIR")
	}
}

// TestTailLoop_CtxCanceledImmediately covers tailLoop's OWN top-of-loop
// ctx.Err() guard. This is distinct from
// TestStreamFile_CtxCanceledImmediately, which cancels before streamFile
// ever opens the file and reaches tailLoop at all; here the file is
// opened successfully and tailLoop itself is called directly with an
// already-canceled context.
func TestTailLoop_CtxCanceledImmediately(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := os.WriteFile(path, []byte("line\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// conn is nil: the ctx.Err() check must fire before any conn use.
	if err := tailLoop(ctx, nil, f); !errors.Is(err, context.Canceled) {
		t.Fatalf("tailLoop with a pre-canceled ctx = %v, want context.Canceled", err)
	}
}

// TestTailLoop_WriteFailsOnClosedConn covers tailLoop's conn.Write-error
// return: a non-empty file gives ReadString a line to deliver, but the WS
// conn was closed by the client itself before tailLoop ever touches it,
// so the Write fails deterministically (no server-timing dependency).
func TestTailLoop_WriteFailsOnClosedConn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := os.WriteFile(path, []byte("line one\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	srv := dummyWSServer(t)
	defer srv.Close()
	cli, dialResp, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := cli.CloseNow(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := tailLoop(context.Background(), cli, f); err == nil {
		t.Fatal("tailLoop should fail when the WS write fails on an already-closed conn")
	}
}

// TestTailLoop_EmptyFileWaitsThenCtxExpires covers the sleep(ctx, 200ms)
// != nil branch inside tailLoop's EOF case: an empty, unrotated file
// keeps hitting EOF, so tailLoop sleeps between polls; a context that
// expires well inside that 200ms window makes the sleep return the
// context's error deterministically. conn is nil: an empty file never
// produces a line, so conn.Write is never reached.
func TestTailLoop_EmptyFileWaitsThenCtxExpires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = tailLoop(ctx, nil, f)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("tailLoop on an empty, unrotated file = %v, want context.DeadlineExceeded", err)
	}
}

// TestCheckRotation_ClosedFileStatError covers checkRotation's OWN f.Stat()
// error branch, distinct from TestCheckRotation_StatError which covers
// the SECOND os.Stat(f.Name()) call: closing f first makes the file's own
// Stat fail immediately.
func TestCheckRotation_ClosedFileStatError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	rot, err := checkRotation(f)
	if err == nil {
		t.Fatal("expected an error from Stat on a closed file")
	}
	if rot {
		t.Error("rot should be false when f.Stat fails, not treated as a rotation")
	}
}
