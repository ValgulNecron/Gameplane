package controller

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestBuildAgentContainer_DefaultsAndOverrides(t *testing.T) {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "alpha"

	t.Run("falls back to default image and mountPath when template doesn't specify", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		tmpl.Name = "minecraft"
		tmpl.Spec.Game = "minecraft"
		c := buildAgentContainer(gs, tmpl, nil, "ghcr.io/agent:fallback")
		if c.Image != "ghcr.io/agent:fallback" {
			t.Fatalf("Image=%q", c.Image)
		}
		if c.VolumeMounts[0].MountPath != "/data" {
			t.Fatalf("MountPath=%q", c.VolumeMounts[0].MountPath)
		}
	})

	t.Run("template Agent.Image overrides fallback", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		tmpl.Name = "minecraft"
		tmpl.Spec.Game = "minecraft"
		tmpl.Spec.Agent = &kestrelv1alpha1.AgentSpec{
			Image: "ghcr.io/agent:custom",
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			},
		}
		c := buildAgentContainer(gs, tmpl, nil, "ghcr.io/agent:fallback")
		if c.Image != "ghcr.io/agent:custom" {
			t.Fatalf("Image=%q", c.Image)
		}
		if c.Resources.Limits.Cpu().String() != "500m" {
			t.Fatalf("Resources=%+v", c.Resources)
		}
	})

	t.Run("template Storage.MountPath overrides /data", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		tmpl.Spec.Storage.MountPath = "/srv"
		c := buildAgentContainer(gs, tmpl, nil, "fb")
		if c.VolumeMounts[0].MountPath != "/srv" {
			t.Fatalf("MountPath=%q", c.VolumeMounts[0].MountPath)
		}
	})

	t.Run("security context locks down the sidecar", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		c := buildAgentContainer(gs, tmpl, nil, "fb")
		sc := c.SecurityContext
		if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
			t.Fatalf("RunAsNonRoot not set: %+v", sc)
		}
		if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
			t.Fatal("ReadOnlyRootFilesystem not set")
		}
		if len(sc.Capabilities.Drop) == 0 || sc.Capabilities.Drop[0] != "ALL" {
			t.Fatalf("Capabilities.Drop=%v", sc.Capabilities.Drop)
		}
	})

	t.Run("env vars include server/template/game", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		tmpl.Name = "minecraft"
		tmpl.Spec.Game = "minecraft-java"
		c := buildAgentContainer(gs, tmpl, nil, "fb")
		envs := map[string]string{}
		for _, e := range c.Env {
			envs[e.Name] = e.Value
		}
		if envs["GAMEPLANE_SERVER_NAME"] != "alpha" {
			t.Fatalf("server env=%q", envs["GAMEPLANE_SERVER_NAME"])
		}
		if envs["GAMEPLANE_TEMPLATE"] != "minecraft" {
			t.Fatalf("template env=%q", envs["GAMEPLANE_TEMPLATE"])
		}
		if envs["GAMEPLANE_GAME"] != "minecraft-java" {
			t.Fatalf("game env=%q", envs["GAMEPLANE_GAME"])
		}
	})

	t.Run("port 8090 advertised by agent", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		c := buildAgentContainer(gs, tmpl, nil, "fb")
		if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 8090 {
			t.Fatalf("Ports=%+v", c.Ports)
		}
	})
}

func TestBuildAgentContainer_RconEnabledEnv(t *testing.T) {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "smp"
	gs.Namespace = "g"

	envOf := func(c corev1.Container) map[string]string {
		out := map[string]string{}
		for _, e := range c.Env {
			out[e.Name] = e.Value
		}
		return out
	}

	t.Run("rcon template advertises enabled", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{Spec: kestrelv1alpha1.GameTemplateSpec{
			RCON: &kestrelv1alpha1.RCONSpec{Protocol: "source"},
		}}
		if got := envOf(buildAgentContainer(gs, tmpl, nil, "fb"))["GAMEPLANE_RCON_ENABLED"]; got != "true" {
			t.Fatalf("GAMEPLANE_RCON_ENABLED=%q, want true", got)
		}
	})

	t.Run("no rcon block disables", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		if got := envOf(buildAgentContainer(gs, tmpl, nil, "fb"))["GAMEPLANE_RCON_ENABLED"]; got != "false" {
			t.Fatalf("GAMEPLANE_RCON_ENABLED=%q, want false", got)
		}
	})

	t.Run("protocol none disables", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{Spec: kestrelv1alpha1.GameTemplateSpec{
			RCON: &kestrelv1alpha1.RCONSpec{Protocol: "none"},
		}}
		if got := envOf(buildAgentContainer(gs, tmpl, nil, "fb"))["GAMEPLANE_RCON_ENABLED"]; got != "false" {
			t.Fatalf("GAMEPLANE_RCON_ENABLED=%q, want false", got)
		}
	})
}

func TestBuildAgentContainer_GameLogPathArg(t *testing.T) {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "smp"
	gs.Namespace = "g"

	hasArg := func(c corev1.Container, want string) bool {
		for _, a := range c.Args {
			if a == want {
				return true
			}
		}
		return false
	}

	tmpl := &kestrelv1alpha1.GameTemplate{Spec: kestrelv1alpha1.GameTemplateSpec{
		LogPath: "/data/logs/latest.log",
	}}
	if !hasArg(buildAgentContainer(gs, tmpl, nil, "fb"), "--game-log-path=/data/logs/latest.log") {
		t.Fatal("expected --game-log-path arg when template declares logPath")
	}

	bare := &kestrelv1alpha1.GameTemplate{}
	for _, a := range buildAgentContainer(gs, bare, nil, "fb").Args {
		if strings.HasPrefix(a, "--game-log-path") {
			t.Fatalf("unexpected log-path arg without template logPath: %s", a)
		}
	}
}

func TestBuildAgentContainer_CapabilitiesEnv(t *testing.T) {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "smp"
	gs.Namespace = "g"

	envValue := func(c corev1.Container, name string) (string, bool) {
		for _, e := range c.Env {
			if e.Name == name {
				return e.Value, true
			}
		}
		return "", false
	}

	tmpl := &kestrelv1alpha1.GameTemplate{Spec: kestrelv1alpha1.GameTemplateSpec{
		Capabilities: &kestrelv1alpha1.CapabilitiesSpec{
			Players: &kestrelv1alpha1.PlayerActionsSpec{
				Kick: "kick {{.Player}}",
			},
			Quiesce: &kestrelv1alpha1.QuiesceSpec{
				Quiesce:   []string{"save-off", "save-all flush"},
				Unquiesce: []string{"save-on"},
			},
		},
	}}
	got, ok := envValue(buildAgentContainer(gs, tmpl, nil, "fb"), "GAMEPLANE_CAPABILITIES")
	if !ok {
		t.Fatal("expected GAMEPLANE_CAPABILITIES env when template declares capabilities")
	}
	var parsed kestrelv1alpha1.CapabilitiesSpec
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("env is not valid JSON: %v", err)
	}
	if parsed.Players == nil || parsed.Players.Kick != "kick {{.Player}}" {
		t.Errorf("players round-trip = %+v", parsed.Players)
	}
	if parsed.Quiesce == nil || len(parsed.Quiesce.Quiesce) != 2 {
		t.Errorf("quiesce round-trip = %+v", parsed.Quiesce)
	}

	bare := &kestrelv1alpha1.GameTemplate{}
	if _, ok := envValue(buildAgentContainer(gs, bare, nil, "fb"), "GAMEPLANE_CAPABILITIES"); ok {
		t.Fatal("unexpected GAMEPLANE_CAPABILITIES env without declared capabilities")
	}
}
