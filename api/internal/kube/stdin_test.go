package kube

import (
	"context"
	"io"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestWriteStdinLines_UnreachableErrors covers the attach-failure branch
// deterministically without a cluster: a VALID rest.Config pointing at a dead
// port builds the clientset and the SPDY executor fine, then StreamWithContext
// dials, gets connection-refused, and WriteStdinLines wraps it. This proves
// the error is surfaced (wrapped, not a panic) and — with the drain goroutine
// in play — that a fast transport error still returns promptly.
//
// The clientset MUST be built from a real rest.Config: fake.NewSimpleClientset's
// CoreV1().RESTClient() is nil, so .Post() on it panics before the URL is even
// built. And the CA must be valid PEM / Insecure, else kubernetes.NewForConfig
// itself fails on the TLS material before we reach WriteStdinLines.
func TestWriteStdinLines_UnreachableErrors(t *testing.T) {
	cfg := &rest.Config{
		Host:            "https://127.0.0.1:1", // nothing listens on port 1
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	}
	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("build clientset: %v", err)
	}
	c := &Client{Typed: typed, Config: cfg}

	err = c.WriteStdinLines(context.Background(), "gameplane-games", "alpha-0", "game", []string{"say hi"})
	if err == nil {
		t.Fatal("expected an error attaching to an unreachable apiserver, got nil")
	}
	if !strings.Contains(err.Error(), "pod gameplane-games/alpha-0") {
		t.Fatalf("err = %q, want it to name the target pod", err.Error())
	}
}

// TestNotifyOnEOF verifies the drain signal that lets WriteStdinLines tear
// the attach session down promptly: drained closes exactly once, the first
// time the wrapped reader reports EOF, and the bytes pass through unchanged.
func TestNotifyOnEOF(t *testing.T) {
	drained := make(chan struct{})
	n := &notifyOnEOF{r: strings.NewReader("say hi\n"), drained: drained}

	// Before EOF, drained must stay open.
	buf := make([]byte, 3)
	if _, err := n.Read(buf); err != nil {
		t.Fatalf("first read: %v", err)
	}
	select {
	case <-drained:
		t.Fatal("drained closed before the reader hit EOF")
	default:
	}

	// Drain the rest until EOF; drained must then be closed.
	if _, err := io.ReadAll(n); err != nil {
		t.Fatalf("drain: %v", err)
	}
	select {
	case <-drained:
	default:
		t.Fatal("drained not closed after EOF")
	}

	// A read after EOF must not double-close (that would panic).
	if _, err := n.Read(buf); err != io.EOF {
		t.Fatalf("post-EOF read err = %v, want io.EOF", err)
	}
}
