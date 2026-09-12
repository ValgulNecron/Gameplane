package rcon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCLI_Runner verifies custom command runner invocation.
func TestCLI_Runner(t *testing.T) {
	called := false
	runner := func(_ context.Context, cmd string) (string, error) {
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

// TestCLI_PipeExec verifies writing commands to a FIFO pipe file.
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

// TestCLI_PipeExec_MissingFile verifies error handling when the target pipe is missing.
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

// TestCLI_NoPipeOrRunner verifies error handling when neither pipe nor runner is configured.
func TestCLI_NoPipeOrRunner(t *testing.T) {
	client := NewCLI("127.0.0.1", 0, nil)
	defer client.Close()

	_, err := client.Exec("status")
	if err == nil {
		t.Fatal("expected error when neither pipe nor runner is configured, got nil")
	}
	if !strings.Contains(err.Error(), "requires a configured stdin pipe or runner") {
		t.Errorf("expected missing config error, got %v", err)
	}
}

// TestCLI_EnvPipe verifies configuring the pipe path via environment variable.
func TestCLI_EnvPipe(t *testing.T) {
	t.Setenv("GAMEPLANE_CLI_PIPE", "/tmp/test-env.pipe")
	c := NewCLI("127.0.0.1", 0, nil)
	defer c.Close()

	if c.pipePath != "/tmp/test-env.pipe" {
		t.Errorf("expected pipe /tmp/test-env.pipe, got %s", c.pipePath)
	}
}

// TestCLI_WithExecTimeout verifies configuring execution timeout option.
func TestCLI_WithExecTimeout(t *testing.T) {
	c := NewCLI("127.0.0.1", 0, nil, WithExecTimeout(42*time.Second))
	defer c.Close()

	if c.timeout != 42*time.Second {
		t.Errorf("expected timeout 42s, got %v", c.timeout)
	}
}
