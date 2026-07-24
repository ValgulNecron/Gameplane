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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	err := Retry(ctx, "test op", 10*time.Millisecond, func(_ context.Context) error {
		attempts++
		return fmt.Errorf("attempt %d: always fails", attempts)
	})

	if err == nil {
		t.Fatalf("Retry should fail on deadline exhaustion")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Retry should wrap context.DeadlineExceeded, got: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected multiple attempts before deadline, got %d", attempts)
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
