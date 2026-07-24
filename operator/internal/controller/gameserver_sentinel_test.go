package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// singlePortTemplate builds a GameTemplate with one port, letting each case
// control whether it's advertised and what wakeProtocol it declares.
func singlePortTemplate(advertise bool, wakeProtocol string) *gameplanev1alpha1.GameTemplate {
	return &gameplanev1alpha1.GameTemplate{
		Spec: gameplanev1alpha1.GameTemplateSpec{
			Ports: []gameplanev1alpha1.GamePort{{
				Name:          "game",
				ContainerPort: 25565,
				Protocol:      corev1.ProtocolTCP,
				Advertise:     advertise,
				WakeProtocol:  wakeProtocol,
			}},
		},
	}
}

// TestWakeOnConnectEligible covers the eligibility gate shared by the
// sentinel Deployment (reconcileSentinel, via planSentinel) and the
// game-direct Service (reconcileGameDirectServiceFromTemplate). It used to
// be tested only indirectly, through envtest subtests that called
// reconcileSentinel with an idleState and asserted on the resulting K8s
// object — this is the fast, deterministic equivalent of those eligibility
// cases (armed/disarmed, wakeProtocol: none), pulled out as a pure-function
// table test the same way gameserver_idle_test.go tests idleDecide.
func TestWakeOnConnectEligible(t *testing.T) {
	cases := []struct {
		name string
		idle *gameplanev1alpha1.IdleSpec
		tmpl *gameplanev1alpha1.GameTemplate
		want bool
	}{
		{
			name: "armed with a wakeable advertised port",
			idle: &gameplanev1alpha1.IdleSpec{Enabled: true, WakeOnConnect: true},
			tmpl: singlePortTemplate(true, "minecraft"),
			want: true,
		},
		{
			name: "idle spec unset",
			idle: nil,
			tmpl: singlePortTemplate(true, "minecraft"),
			want: false,
		},
		{
			name: "idle policy disabled",
			idle: &gameplanev1alpha1.IdleSpec{Enabled: false, WakeOnConnect: true},
			tmpl: singlePortTemplate(true, "minecraft"),
			want: false,
		},
		{
			name: "wakeOnConnect not armed",
			idle: &gameplanev1alpha1.IdleSpec{Enabled: true, WakeOnConnect: false},
			tmpl: singlePortTemplate(true, "minecraft"),
			want: false,
		},
		{
			name: "every advertised port has wakeProtocol none",
			idle: &gameplanev1alpha1.IdleSpec{Enabled: true, WakeOnConnect: true},
			tmpl: singlePortTemplate(true, "none"),
			want: false,
		},
		{
			name: "wakeable port exists but is not advertised",
			idle: &gameplanev1alpha1.IdleSpec{Enabled: true, WakeOnConnect: true},
			tmpl: singlePortTemplate(false, "minecraft"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := &gameplanev1alpha1.GameServer{
				Spec: gameplanev1alpha1.GameServerSpec{Idle: tc.idle},
			}
			if got := wakeOnConnectEligible(gs, tc.tmpl); got != tc.want {
				t.Errorf("wakeOnConnectEligible() = %v, want %v", got, tc.want)
			}
		})
	}
}
