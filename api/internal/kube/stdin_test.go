package kube

import (
	"context"
	"io"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// TestWriteStdinLines_BadConfigErrors covers the "build stdin attach
// executor" failure branch: a rest.Config whose TLS material fails to
// parse makes remotecommand.NewSPDYExecutor error before any network I/O,
// so this is deterministic and doesn't need a real pod or apiserver — the
// same shape the operator's equivalent (gameserver_stop_attach.go) is
// exercised at, since neither has a unit test that opens a real SPDY
// connection.
func TestWriteStdinLines_BadConfigErrors(t *testing.T) {
	c := &Client{
		Typed: fake.NewSimpleClientset(),
		Config: &rest.Config{
			Host: "https://127.0.0.1:6443",
			TLSClientConfig: rest.TLSClientConfig{
				CAData: []byte("not a valid PEM certificate"),
			},
		},
	}

	err := c.WriteStdinLines(context.Background(), "gameplane-games", "alpha-0", "game", []string{"say hi"})
	if err == nil {
		t.Fatal("expected an error from a bad TLS config, got nil")
	}
	if !strings.Contains(err.Error(), "build stdin attach executor") {
		t.Fatalf("err = %q, want it to mention the executor build step", err.Error())
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
