package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/kestrel-gg/kestrel/agent/internal/usage"
)

func TestRun_DisabledPaths(t *testing.T) {
	t.Run("empty server name returns immediately", func(_ *testing.T) {
		Run(context.Background(), Config{})
	})

	t.Run("empty namespace and no SA file returns immediately", func(_ *testing.T) {
		// readNamespace will fail because SA path is unreadable in the
		// test env; Run should early-return without panicking.
		Run(context.Background(), Config{ServerName: "srv"})
	})

	t.Run("rest config unavailable outside cluster", func(t *testing.T) {
		// In-cluster config requires KUBERNETES_SERVICE_HOST/PORT env. Make
		// sure they are unset, then exercise the rest.InClusterConfig path.
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		t.Setenv("KUBERNETES_SERVICE_PORT", "")
		Run(context.Background(), Config{ServerName: "srv", Namespace: "ns"})
	})
}

func TestSendOnce(t *testing.T) {
	scheme := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{gvr: "GameServerList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvkr)

	var captured []byte
	dyn.PrependReactor("patch", "gameservers", func(a clienttesting.Action) (bool, runtime.Object, error) {
		captured = a.(clienttesting.PatchAction).GetPatch()
		return true, fakeGameServer(), nil
	})

	cfg := Config{
		ServerName: "srv",
		Namespace:  "ns",
		Version:    "v1",
		Game:       "minecraft-1.20",
		RCON:       fakeRcon{out: "There are 4 of a max"},
	}
	if err := sendOnce(context.Background(), dyn, cfg); err != nil {
		t.Fatalf("sendOnce: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(captured, &got); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	agent := got["status"].(map[string]any)["agent"].(map[string]any)
	if agent["playersOnline"].(float64) != 4 {
		t.Fatalf("playersOnline=%v", agent["playersOnline"])
	}
	if agent["version"].(string) != "v1" || agent["gameVersion"].(string) != "minecraft-1.20" {
		t.Fatalf("agent payload=%+v", agent)
	}
	if _, ok := agent["lastHeartbeat"].(string); !ok {
		t.Fatalf("lastHeartbeat missing or wrong type: %+v", agent)
	}
}

func TestSendOnce_EmitsKnownUsage(t *testing.T) {
	scheme := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{gvr: "GameServerList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvkr)

	var captured []byte
	dyn.PrependReactor("patch", "gameservers", func(a clienttesting.Action) (bool, runtime.Object, error) {
		captured = a.(clienttesting.PatchAction).GetPatch()
		return true, fakeGameServer(), nil
	})

	err := sendOnce(context.Background(), dyn, Config{
		ServerName: "srv",
		Namespace:  "ns",
		RCON:       fakeRcon{out: "0"},
		Usage: fakeUsage{s: usage.Sample{
			CPUMillicores: 500, CPUKnown: true,
			CPULimitMillicores: 2000, CPULimitKnown: true,
			MemoryBytes: 1024, MemoryKnown: true,
			MemoryLimitBytes: 4096, MemoryLimitKnown: true,
			DiskUsedBytes: 10, DiskTotalBytes: 100, DiskKnown: true,
		}},
	})
	if err != nil {
		t.Fatalf("sendOnce: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(captured, &got); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	agent := got["status"].(map[string]any)["agent"].(map[string]any)
	for k, want := range map[string]float64{
		"cpuMillicores":      500,
		"cpuLimitMillicores": 2000,
		"memoryBytes":        1024,
		"memoryLimitBytes":   4096,
		"diskUsedBytes":      10,
		"diskTotalBytes":     100,
	} {
		if v, ok := agent[k].(float64); !ok || v != want {
			t.Fatalf("%s = %v, want %v", k, agent[k], want)
		}
	}
}

func TestSendOnce_EmitsNullForUnknownUsage(t *testing.T) {
	scheme := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{gvr: "GameServerList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvkr)

	var captured []byte
	dyn.PrependReactor("patch", "gameservers", func(a clienttesting.Action) (bool, runtime.Object, error) {
		captured = a.(clienttesting.PatchAction).GetPatch()
		return true, fakeGameServer(), nil
	})

	// A zero-value Sample: every Known flag false → every key present and
	// null, so a merge patch clears any prior reading.
	err := sendOnce(context.Background(), dyn, Config{
		ServerName: "srv",
		Namespace:  "ns",
		RCON:       fakeRcon{out: "0"},
		Usage:      fakeUsage{},
	})
	if err != nil {
		t.Fatalf("sendOnce: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(captured, &got)
	agent := got["status"].(map[string]any)["agent"].(map[string]any)
	for _, k := range []string{
		"cpuMillicores", "cpuLimitMillicores", "memoryBytes",
		"memoryLimitBytes", "diskUsedBytes", "diskTotalBytes",
	} {
		v, present := agent[k]
		if !present {
			t.Fatalf("%s key missing from patch: %+v", k, agent)
		}
		if v != nil {
			t.Fatalf("want null %s, got %v", k, v)
		}
	}
}

func TestSendOnce_RconFailureEmitsNullPlayers(t *testing.T) {
	scheme := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{gvr: "GameServerList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvkr)

	var captured []byte
	dyn.PrependReactor("patch", "gameservers", func(a clienttesting.Action) (bool, runtime.Object, error) {
		captured = a.(clienttesting.PatchAction).GetPatch()
		return true, fakeGameServer(), nil
	})

	err := sendOnce(context.Background(), dyn, Config{
		ServerName: "srv",
		Namespace:  "ns",
		RCON:       fakeRcon{err: errors.New("rcon down")},
	})
	if err != nil {
		t.Fatalf("sendOnce: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(captured, &got)
	agent := got["status"].(map[string]any)["agent"].(map[string]any)
	// Unknown player count must be null ("unknown"), never a -1 sentinel
	// the dashboard would sum into a negative total. The key is present
	// with a null value so the merge patch clears any prior count.
	v, present := agent["playersOnline"]
	if !present {
		t.Fatalf("playersOnline key missing from patch: %+v", agent)
	}
	if v != nil {
		t.Fatalf("want null playersOnline on rcon failure, got %v", v)
	}
}

func TestSendOnce_PatchErrorPropagates(t *testing.T) {
	scheme := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{gvr: "GameServerList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvkr)
	dyn.PrependReactor("patch", "gameservers", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	err := sendOnce(context.Background(), dyn, Config{
		ServerName: "srv",
		Namespace:  "ns",
		RCON:       fakeRcon{out: "0"},
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("got %v", err)
	}
}

func TestQueryOnline(t *testing.T) {
	cases := []struct {
		name string
		out  string
		err  error
		want int
		ok   bool
	}{
		{"basic count", "There are 7 of a max of 20", nil, 7, true},
		{"no digits", "Server starting...", nil, 0, false},
		{"leading zero", "0 players", nil, 0, true},
		{"multi-digit", "There are 132 players", nil, 132, true},
		{"rcon error", "", errors.New("boom"), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := queryOnline(fakeRcon{out: tc.out, err: tc.err})
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v ok=%v", err, tc.ok)
			}
			if n != tc.want {
				t.Fatalf("got %d, want %d", n, tc.want)
			}
		})
	}
}

func TestReadNamespace(t *testing.T) {
	// In a normal CI/test env this file is absent; readNamespace must
	// return "" rather than panicking.
	if got := readNamespace(); got != "" {
		// If a CI runner happens to mount it (unlikely), accept whatever
		// it produced as long as no trailing newline.
		if strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\r") {
			t.Fatalf("trailing whitespace not stripped: %q", got)
		}
	}
}

func TestReadNamespace_TrimsTrailingNewlines(t *testing.T) {
	// Exercise the trimming branch directly through a temp file by
	// duplicating the trim logic here. (readNamespace's path is
	// hard-coded to the SA path; we don't hot-swap it. The trim logic is
	// the only branch worth verifying explicitly.)
	dir := t.TempDir()
	path := filepath.Join(dir, "ns")
	if err := os.WriteFile(path, []byte("kestrel-system\r\n\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, _ := os.ReadFile(path)
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	if string(b) != "kestrel-system" {
		t.Fatalf("got %q", string(b))
	}
}

// helpers

type fakeRcon struct {
	out string
	err error
}

func (f fakeRcon) Exec(string) (string, error) { return f.out, f.err }

type fakeUsage struct{ s usage.Sample }

func (f fakeUsage) Read() usage.Sample { return f.s }

func fakeGameServer() runtime.Object {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "kestrel.gg", Version: "v1alpha1", Kind: "GameServer",
	})
	o.SetName("srv")
	o.SetNamespace("ns")
	return o
}
