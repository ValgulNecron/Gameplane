package kube

import (
	"context"
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
