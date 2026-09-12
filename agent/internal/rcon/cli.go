// Package rcon implements remote console clients for dedicated game servers.
// cli.go implements local console access over standard input / Unix PTY / process execution.
//
// # CLI Protocol Architecture & Semantics
//
// In accordance with the resolution in OPEN-DECISIONS.md (Section 1, Option A):
//   - Web UI / Dashboard Console: Continues to attach directly via Kubernetes
//     pod-attach / PTY (consoleMode: pty), giving administrators interactive terminal
//     access to the game process.
//   - Agent Console Client (CLI): Enables scheduled commands, health checks, quiesce
//     sequences, and lifecycle stop sequences to be driven directly by the agent
//     sidecar via standard input / Unix FIFO pipe or local process execution, without
//     requiring remote TCP networking or an external RCON password secret.
package rcon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultCLITimeout = 15 * time.Second
)

// CLIOption configures a CLI client.
type CLIOption func(*CLI)

// WithPipePath configures the path to a named FIFO or stdin pipe where commands
// are written.
func WithPipePath(path string) CLIOption {
	return func(c *CLI) {
		c.pipePath = path
	}
}

// WithExecTimeout configures the execution deadline for CLI commands.
func WithExecTimeout(d time.Duration) CLIOption {
	return func(c *CLI) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithRunner overrides the execution mechanism with a custom command runner
// (primarily used for unit testing).
func WithRunner(runner func(ctx context.Context, cmd string) (string, error)) CLIOption {
	return func(c *CLI) {
		c.runner = runner
	}
}

// CLI is an agent console client that drives game commands via container stdin,
// FIFO pipe, or local process execution.
type CLI struct {
	pipePath string
	timeout  time.Duration
	runner   func(ctx context.Context, cmd string) (string, error)

	mu sync.Mutex
}

// NewCLI creates a new CLI console client. The host, port, and pass arguments
// are accepted to satisfy the standard RCON constructor signature used by the agent
// dispatch switch.
func NewCLI(_ string, _ int, _ PassFn, opts ...CLIOption) *CLI {
	c := &CLI{
		timeout: defaultCLITimeout,
	}

	if envPipe := os.Getenv("GAMEPLANE_CLI_PIPE"); envPipe != "" {
		c.pipePath = envPipe
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Close releases any resources held by the CLI client.
func (c *CLI) Close() error {
	return nil
}

// Exec executes a command locally via the configured runner or FIFO pipe.
func (c *CLI) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if c.runner != nil {
		return c.runner(ctx, cmd)
	}

	if c.pipePath != "" {
		if err := c.execPipe(ctx, cmd); err != nil {
			return "", err
		}
		return "", nil
	}

	return "", errors.New("cli rcon requires a configured stdin pipe or runner")
}

// execPipe writes a command string into the configured stdin FIFO pipe.
func (c *CLI) execPipe(ctx context.Context, cmd string) error {
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(c.pipePath, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			done <- fmt.Errorf("open cli pipe %q: %w", c.pipePath, err)
			return
		}
		defer func() { _ = f.Close() }()

		data := []byte(strings.TrimRight(cmd, "\r\n") + "\n")
		if _, err := f.Write(data); err != nil {
			done <- fmt.Errorf("write cli pipe %q: %w", c.pipePath, err)
			return
		}
		done <- nil
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("cli pipe write %q timed out: %w", cmd, ctx.Err())
	case err := <-done:
		return err
	}
}
