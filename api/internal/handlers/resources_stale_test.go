package handlers

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func gsWithHeartbeat(age time.Duration, players int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"agent": map[string]any{
				"lastHeartbeat": time.Now().Add(-age).UTC().Format(time.RFC3339),
				"playersOnline": players,
			},
		},
	}}
}

func TestGateStaleAgent_BlanksStale(t *testing.T) {
	gs := gsWithHeartbeat(5*time.Minute, 5)
	gateStaleAgent(gs)
	agent, _, _ := unstructured.NestedMap(gs.Object, "status", "agent")
	if _, ok := agent["playersOnline"]; ok {
		t.Fatalf("stale playersOnline should be dropped, got %v", agent["playersOnline"])
	}
	if agent["stale"] != true {
		t.Fatalf("expected stale=true, got %v", agent["stale"])
	}
}

func TestGateStaleAgent_KeepsFresh(t *testing.T) {
	gs := gsWithHeartbeat(2*time.Second, 5)
	gateStaleAgent(gs)
	agent, _, _ := unstructured.NestedMap(gs.Object, "status", "agent")
	if agent["playersOnline"] == nil {
		t.Fatal("fresh playersOnline should be preserved")
	}
	if _, ok := agent["stale"]; ok {
		t.Fatal("fresh agent should not be marked stale")
	}
}

func TestGateStaleAgent_NoAgentNoPanic(t *testing.T) {
	gs := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{"phase": "Pending"},
	}}
	gateStaleAgent(gs) // must not panic or add an agent
	if _, found, _ := unstructured.NestedMap(gs.Object, "status", "agent"); found {
		t.Fatal("gateStaleAgent should not synthesize an agent block")
	}
}
