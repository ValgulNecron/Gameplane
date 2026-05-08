package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

func TestBuildAgentContainer_DefaultsAndOverrides(t *testing.T) {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "alpha"

	t.Run("falls back to default image and mountPath when template doesn't specify", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		tmpl.Name = "minecraft"
		tmpl.Spec.Game = "minecraft"
		c := buildAgentContainer(gs, tmpl, "ghcr.io/agent:fallback")
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
		c := buildAgentContainer(gs, tmpl, "ghcr.io/agent:fallback")
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
		c := buildAgentContainer(gs, tmpl, "fb")
		if c.VolumeMounts[0].MountPath != "/srv" {
			t.Fatalf("MountPath=%q", c.VolumeMounts[0].MountPath)
		}
	})

	t.Run("security context locks down the sidecar", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		c := buildAgentContainer(gs, tmpl, "fb")
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
		c := buildAgentContainer(gs, tmpl, "fb")
		envs := map[string]string{}
		for _, e := range c.Env {
			envs[e.Name] = e.Value
		}
		if envs["KESTREL_SERVER_NAME"] != "alpha" {
			t.Fatalf("server env=%q", envs["KESTREL_SERVER_NAME"])
		}
		if envs["KESTREL_TEMPLATE"] != "minecraft" {
			t.Fatalf("template env=%q", envs["KESTREL_TEMPLATE"])
		}
		if envs["KESTREL_GAME"] != "minecraft-java" {
			t.Fatalf("game env=%q", envs["KESTREL_GAME"])
		}
	})

	t.Run("port 8090 advertised by agent", func(t *testing.T) {
		tmpl := &kestrelv1alpha1.GameTemplate{}
		c := buildAgentContainer(gs, tmpl, "fb")
		if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 8090 {
			t.Fatalf("Ports=%+v", c.Ports)
		}
	})
}
