package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestDerivePhase(t *testing.T) {
	table := []struct {
		name     string
		suspend  bool
		ssExists bool
		ssReady  bool
		hbFresh  bool
		want     gameplanev1alpha1.GameServerPhase
	}{
		{"pending when no ss", false, false, false, false, gameplanev1alpha1.GameServerPhasePending},
		{"starting when ss not ready", false, true, false, false, gameplanev1alpha1.GameServerPhaseStarting},
		{"starting when ready but no heartbeat", false, true, true, false, gameplanev1alpha1.GameServerPhaseStarting},
		{"running when ready and fresh heartbeat", false, true, true, true, gameplanev1alpha1.GameServerPhaseRunning},
		{"suspended when suspend + ss gone", true, false, false, false, gameplanev1alpha1.GameServerPhaseSuspended},
		{"stopping when suspend + ss still ready", true, true, true, true, gameplanev1alpha1.GameServerPhaseStopping},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			gs := &gameplanev1alpha1.GameServer{
				Spec: gameplanev1alpha1.GameServerSpec{Suspend: tc.suspend},
			}
			got := derivePhase(gs, tc.ssExists, tc.ssReady, tc.hbFresh, idleAwake)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestHeartbeatFresh(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{}
	if heartbeatFresh(gs) {
		t.Error("no heartbeat should be stale")
	}
	now := metav1.Now()
	gs.Status.Agent = &gameplanev1alpha1.AgentStatus{LastHeartbeat: &now}
	if !heartbeatFresh(gs) {
		t.Error("heartbeat now should be fresh")
	}
	old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	gs.Status.Agent.LastHeartbeat = &old
	if heartbeatFresh(gs) {
		t.Error("heartbeat 10m ago should be stale")
	}
}
