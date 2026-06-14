package auth

import (
	"context"
	"testing"
)

func TestActorHolder_NilSafe(t *testing.T) {
	var h *ActorHolder
	h.Set("bob") // must not panic on a nil receiver
	if got := h.Name(); got != "" {
		t.Fatalf("nil holder Name() = %q, want empty", got)
	}
}

func TestActorHolder_SetThenName(t *testing.T) {
	h := &ActorHolder{}
	if got := h.Name(); got != "" {
		t.Fatalf("fresh holder Name() = %q, want empty", got)
	}
	h.Set("alice")
	if got := h.Name(); got != "alice" {
		t.Fatalf("Name() = %q, want alice", got)
	}
}

func TestWithActorHolder_SetActor(t *testing.T) {
	ctx, h := WithActorHolder(context.Background())
	if h == nil {
		t.Fatal("WithActorHolder returned a nil holder")
	}
	SetActor(ctx, "carol")
	if got := h.Name(); got != "carol" {
		t.Fatalf("holder Name() = %q after SetActor, want carol", got)
	}
}

func TestSetActor_NoHolderInContext(t *testing.T) {
	// A context without a holder makes SetActor a no-op (the resolved
	// holder is nil, and *ActorHolder.Set is nil-safe).
	SetActor(context.Background(), "nobody") // must not panic
}
