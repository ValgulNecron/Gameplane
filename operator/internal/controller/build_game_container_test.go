package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// TestBuildGameContainer_ProbeOverride proves per-server probes win over
// the template one probe at a time, and unset per-server probes fall back
// to the template.
func TestBuildGameContainer_ProbeOverride(t *testing.T) {
	probe := func(delay int32) *corev1.Probe {
		return &corev1.Probe{InitialDelaySeconds: delay}
	}

	tmpl := &kestrelv1alpha1.GameTemplate{}
	tmpl.Name = "minecraft"
	tmpl.Spec.Game = "minecraft"
	tmpl.Spec.Probes = &kestrelv1alpha1.GameProbesSpec{
		Readiness: probe(1),
		Liveness:  probe(2),
		Startup:   probe(3),
	}

	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "alpha"
	// Override readiness + startup only; liveness should stay the template's.
	gs.Spec.Probes = &kestrelv1alpha1.GameProbesSpec{
		Readiness: probe(10),
		Startup:   probe(30),
	}

	c := buildGameContainer(gs, tmpl, "busybox:1.36", nil, &materializedConfig{})

	if c.ReadinessProbe == nil || c.ReadinessProbe.InitialDelaySeconds != 10 {
		t.Errorf("ReadinessProbe = %+v, want per-server (delay 10)", c.ReadinessProbe)
	}
	if c.LivenessProbe == nil || c.LivenessProbe.InitialDelaySeconds != 2 {
		t.Errorf("LivenessProbe = %+v, want template (delay 2)", c.LivenessProbe)
	}
	if c.StartupProbe == nil || c.StartupProbe.InitialDelaySeconds != 30 {
		t.Errorf("StartupProbe = %+v, want per-server (delay 30)", c.StartupProbe)
	}
}

// TestBuildGameContainer_NoServerProbes keeps the template probes when the
// server overrides none.
func TestBuildGameContainer_NoServerProbes(t *testing.T) {
	tmpl := &kestrelv1alpha1.GameTemplate{}
	tmpl.Name = "minecraft"
	tmpl.Spec.Game = "minecraft"
	tmpl.Spec.Probes = &kestrelv1alpha1.GameProbesSpec{
		Liveness: &corev1.Probe{InitialDelaySeconds: 7},
	}
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "alpha"

	c := buildGameContainer(gs, tmpl, "busybox:1.36", nil, &materializedConfig{})
	if c.LivenessProbe == nil || c.LivenessProbe.InitialDelaySeconds != 7 {
		t.Errorf("LivenessProbe = %+v, want template (delay 7)", c.LivenessProbe)
	}
}
