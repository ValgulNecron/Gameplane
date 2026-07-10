package main

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mutatingVerbs is the denylist asserted against both the Client method set
// and the registered tool names: this server's entire reason for existing
// is that none of these ever appear.
var mutatingVerbs = []string{"Create", "Update", "Delete", "Patch", "Apply", "Remove"}

// TestClientHasNoMutatingMethods is the structural half of the read-only
// invariant described in main.go's package doc: Client must not expose any
// method a tool could use to mutate the cluster, regardless of what tools
// happen to be registered.
func TestClientHasNoMutatingMethods(t *testing.T) {
	typ := reflect.TypeOf(&Client{})
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		for _, verb := range mutatingVerbs {
			if strings.HasPrefix(name, verb) {
				t.Errorf("Client has mutating-looking method %q (matches verb %q)", name, verb)
			}
		}
	}
}

func connectInMemory(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestRegisteredToolsAreReadOnly is the registration half of the read-only
// invariant: every tool this server advertises must carry ReadOnlyHint, must
// not be flagged destructive, and its name must not look like a mutating
// verb — checked over the wire via a real client session, not just against
// the in-process registry.
func TestRegisteredToolsAreReadOnly(t *testing.T) {
	server := newMCPServer(&Client{})
	cs := connectInMemory(t, server)
	ctx := context.Background()

	var gotNames []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools: %v", err)
		}
		gotNames = append(gotNames, tool.Name)

		for _, verb := range mutatingVerbs {
			if strings.Contains(strings.ToLower(tool.Name), strings.ToLower(verb)) {
				t.Errorf("tool %q name looks mutating (matches verb %q)", tool.Name, verb)
			}
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q: want Annotations.ReadOnlyHint=true, got %+v", tool.Name, tool.Annotations)
		}
		if tool.Annotations != nil && tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint {
			t.Errorf("tool %q: want DestructiveHint=false, got true", tool.Name)
		}
	}

	sort.Strings(gotNames)
	want := append([]string(nil), registeredToolNames...)
	sort.Strings(want)
	if !reflect.DeepEqual(gotNames, want) {
		t.Errorf("registered tools = %v, want %v", gotNames, want)
	}
}

func callToolText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		var texts []string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				texts = append(texts, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) returned a tool error: %v", name, texts)
	}
	if len(res.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", name)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): want *mcp.TextContent, got %T", name, res.Content[0])
	}
	return tc.Text
}

func testClientWithFixtures(t *testing.T) *Client {
	t.Helper()
	scheme := newScheme()
	gs := newUnstructuredCRD("GameServer", "games", "my-server", map[string]any{"phase": "Running"})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKindMap(), gs)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server-0", Namespace: "games"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev1", Namespace: "games"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "my-server-0"},
		Reason:         "Started",
		Message:        "Started container agent",
	}
	typed := k8sfake.NewSimpleClientset(pod, ev)

	return &Client{Typed: typed, Dynamic: dyn, Scheme: scheme}
}

func TestToolsListAndGetHappyPath(t *testing.T) {
	c := testClientWithFixtures(t)
	server := newMCPServer(c)
	cs := connectInMemory(t, server)

	t.Run("list_gameplane_resources", func(t *testing.T) {
		text := callToolText(t, cs, "list_gameplane_resources", map[string]any{"kind": "GameServer", "namespace": "games"})
		if !strings.Contains(text, "my-server") {
			t.Errorf("want my-server in output, got %q", text)
		}
	})

	t.Run("get_gameplane_resource", func(t *testing.T) {
		text := callToolText(t, cs, "get_gameplane_resource", map[string]any{"kind": "GameServer", "namespace": "games", "name": "my-server"})
		if !strings.Contains(text, "Running") {
			t.Errorf("want status phase Running in output, got %q", text)
		}
	})

	t.Run("get_gameplane_resource unknown kind is a tool error", func(t *testing.T) {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "get_gameplane_resource",
			Arguments: map[string]any{"kind": "NotAKind", "namespace": "games", "name": "x"},
		})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !res.IsError {
			t.Error("want IsError=true for an unknown kind")
		}
	})

	t.Run("list_pods", func(t *testing.T) {
		text := callToolText(t, cs, "list_pods", map[string]any{"namespace": "games"})
		if !strings.Contains(text, "my-server-0") {
			t.Errorf("want my-server-0 in output, got %q", text)
		}
	})

	t.Run("get_pod", func(t *testing.T) {
		text := callToolText(t, cs, "get_pod", map[string]any{"namespace": "games", "name": "my-server-0"})
		if !strings.Contains(text, "Running") {
			t.Errorf("want phase Running in output, got %q", text)
		}
	})

	t.Run("list_events", func(t *testing.T) {
		text := callToolText(t, cs, "list_events", map[string]any{"namespace": "games"})
		if !strings.Contains(text, "Started") {
			t.Errorf("want reason Started in output, got %q", text)
		}
	})

	t.Run("get_pod_logs", func(t *testing.T) {
		text := callToolText(t, cs, "get_pod_logs", map[string]any{"namespace": "games", "pod": "my-server-0"})
		if !strings.Contains(text, "fake logs") {
			t.Errorf("want fake clientset log text, got %q", text)
		}
	})
}

func TestProposeFixTool(t *testing.T) {
	c := testClientWithFixtures(t)
	server := newMCPServer(c)
	cs := connectInMemory(t, server)

	t.Run("matches a heuristic and stays read-only", func(t *testing.T) {
		text := callToolText(t, cs, "propose_fix", map[string]any{
			"kind": "Pod", "namespace": "games", "name": "my-server-0",
			"symptom": "the container keeps hitting CrashLoopBackOff",
		})
		if !strings.Contains(text, "CrashLoopBackOff") {
			t.Errorf("want CrashLoopBackOff advice, got %q", text)
		}
		if !strings.Contains(strings.ToLower(text), "read-only") {
			t.Errorf("want a read-only disclaimer, got %q", text)
		}
		if !strings.Contains(text, "Running") {
			t.Errorf("want the observed pod status folded in, got %q", text)
		}
	})

	t.Run("no heuristic match still returns generic advice", func(t *testing.T) {
		text := callToolText(t, cs, "propose_fix", map[string]any{
			"symptom": "the leaderboard shows the wrong high score",
		})
		if !strings.Contains(text, "No specific heuristic matched") {
			t.Errorf("want the generic fallback, got %q", text)
		}
	})

	t.Run("missing resource is reported, not a tool error", func(t *testing.T) {
		text := callToolText(t, cs, "propose_fix", map[string]any{
			"kind": "GameServer", "namespace": "games", "name": "does-not-exist",
			"symptom": "backup keeps failing",
		})
		if !strings.Contains(text, "could not read") {
			t.Errorf("want a could-not-read note, got %q", text)
		}
		if !strings.Contains(text, "restic") {
			t.Errorf("want backup advice, got %q", text)
		}
	})
}

func TestRunIdleReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- runIdle(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runIdle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runIdle did not return after context cancellation")
	}
}
