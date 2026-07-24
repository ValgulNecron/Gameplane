package probe

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetrySuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempts := 0
	err := Retry(ctx, "test op", 100*time.Millisecond, func(_ context.Context) error {
		attempts++
		if attempts < 2 {
			return fmt.Errorf("attempt %d: not ready yet", attempts)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Retry should succeed after retry, got: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryErrFatalShortCircuit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempts := 0
	err := Retry(ctx, "test op", 100*time.Millisecond, func(_ context.Context) error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("transient error")
		}
		return fmt.Errorf("misconfigured server: %w", ErrFatal)
	})

	if !errors.Is(err, ErrFatal) {
		t.Fatalf("Retry should return ErrFatal, got: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts before short-circuit, got %d", attempts)
	}
}

func TestRetryDeadlineExhaustion(t *testing.T) {
	// Deadline must be long enough for at least one retryInterval (3 seconds in probe.go)
	// so the loop gets multiple attempts. Use 10 seconds to ensure we get multiple attempts
	// before timing out.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sentinel error to verify it gets wrapped when fn fails before deadline.
	testErr := fmt.Errorf("test sentinel")

	attempts := 0
	err := Retry(ctx, "test op", 100*time.Millisecond, func(_ context.Context) error {
		attempts++
		return testErr
	})

	if err == nil {
		t.Fatalf("Retry should fail on deadline exhaustion")
	}
	// When fn fails at least once before the deadline expires, Retry wraps
	// the last fn error, not context.DeadlineExceeded. This is deliberate:
	// the useful debugging info is why the server kept refusing, not "context expired".
	if !errors.Is(err, testErr) {
		t.Fatalf("Retry should wrap the last fn error, got: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected multiple attempts before deadline, got %d", attempts)
	}
}

func TestRetryDeadlineExhaustedNoAttempts(t *testing.T) {
	// Test the rare case where context is already expired before fn ever runs.
	// Use a very short deadline (1ms) that will expire before fn is called.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give the context time to actually expire.
	time.Sleep(2 * time.Millisecond)

	err := Retry(ctx, "test op", 10*time.Millisecond, func(_ context.Context) error {
		// This should never be called since context is already expired.
		return fmt.Errorf("fn should not run")
	})

	if err == nil {
		t.Fatalf("Retry should fail when context is already expired")
	}
	// When fn never runs (or never returns an error), Retry wraps context.DeadlineExceeded.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Retry should wrap context.DeadlineExceeded when fn never fails, got: %v", err)
	}
}

func TestDepthConstants(t *testing.T) {
	tests := []struct {
		depth Depth
		want  string
	}{
		{Joined, "JOINED"},
		{Partial, "PARTIAL"},
		{Query, "QUERY"},
	}
	for _, tt := range tests {
		if string(tt.depth) != tt.want {
			t.Errorf("Depth %v = %q, want %q", tt.depth, string(tt.depth), tt.want)
		}
	}
}

func TestDepthEquality(t *testing.T) {
	if Joined != Depth("JOINED") {
		t.Fatalf("Joined should equal Depth(\"JOINED\")")
	}
	if Joined == Partial {
		t.Fatalf("Joined should not equal Partial")
	}
	if Partial == Query {
		t.Fatalf("Partial should not equal Query")
	}
}
