package rcon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLI_Runner(t *testing.T) {
	called := false
	runner := func(ctx context.Context, cmd string) (string, error) {
		called = true
		if cmd != "say hello" {
			t.Errorf("expected cmd 'say hello', got %q", cmd)
		}
		return "hello from runner", nil
	}

	client := NewCLI("127.0.0.1", 0, nil, WithRunner(runner))
	defer client.Close()

	out, err := client.Exec("say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("runner was not invoked")
	}
	if out != "hello from runner" {
		t.Errorf("expected 'hello from runner', got %q", out)
	}
}

func TestCLI_ProcessExec_Success(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil, WithExecTimeout(5*time.Second))
	defer client.Close()

	out, err := client.Exec("echo 'server ready'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "server ready" {
		t.Errorf("expected 'server ready', got %q", out)
	}
}

func TestCLI_ProcessExec_ErrorExit(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil, WithExecTimeout(5*time.Second))
	defer client.Close()

	_, err := client.Exec("echo 'something broke' >&2; exit 1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("expected stderr in error message, got: %v", err)
	}
}

func TestCLI_ProcessExec_Timeout(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil, WithExecTimeout(50*time.Millisecond))
	defer client.Close()

	_, err := client.Exec("sleep 1")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestCLI_PipeExec(t *testing.T) {
	dir := t.TempDir()
	pipeFile := filepath.Join(dir, "cmd.pipe")

	if err := os.WriteFile(pipeFile, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create dummy pipe: %v", err)
	}

	client := NewCLI("127.0.0.1", 0, nil, WithPipePath(pipeFile), WithExecTimeout(2*time.Second))
	defer client.Close()

	out, err := client.Exec("save-all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string output from pipe write, got %q", out)
	}

	data, err := os.ReadFile(pipeFile)
	if err != nil {
		t.Fatalf("failed to read pipe file: %v", err)
	}
	if string(data) != "save-all\n" {
		t.Errorf("expected 'save-all\\n', got %q", string(data))
	}
}

func TestCLI_PipeExec_MissingFile(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil, WithPipePath("/path/to/nonexistent/pipe"), WithExecTimeout(1*time.Second))
	defer client.Close()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected error for nonexistent pipe, got nil")
	}
	if !strings.Contains(err.Error(), "open cli pipe") {
		t.Errorf("expected open cli pipe error, got %v", err)
	}
}

func TestCLI_LimitedWriter(t *testing.T) {
	var buf strings.Builder
	lw := &limitedWriter{w: &buf, limit: 10}

	n, err := lw.Write([]byte("12345"))
	if err != nil || n != 5 {
		t.Fatalf("Write(5) = (%d, %v), want (5, nil)", n, err)
	}

	n, err = lw.Write([]byte("6789012345"))
	if err != nil || n != 10 {
		t.Fatalf("Write(10) = (%d, %v), want (10, nil)", n, err)
	}

	if buf.String() != "1234567890" {
		t.Errorf("buf = %q, want '1234567890'", buf.String())
	}

	// Further writes are dropped without error
	n, err = lw.Write([]byte("extra"))
	if err != nil || n != 5 {
		t.Fatalf("Write(5) = (%d, %v), want (5, nil)", n, err)
	}
	if buf.String() != "1234567890" {
		t.Errorf("buf = %q, want '1234567890'", buf.String())
	}
}

func TestCLI_WithShellOption(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil, WithShell("/bin/bash", "-c"))
	defer client.Close()

	if client.shell != "/bin/bash" {
		t.Errorf("expected /bin/bash, got %s", client.shell)
	}
}
