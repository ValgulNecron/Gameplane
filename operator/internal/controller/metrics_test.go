package controller

import (
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func metricsScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := gameplanev1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add gameplane scheme: %v", err)
	}
	return s
}

func gsWithPhase(name string, phase gameplanev1alpha1.GameServerPhase) *gameplanev1alpha1.GameServer {
	return &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "gameplane-games"},
		Status:     gameplanev1alpha1.GameServerStatus{Phase: phase},
	}
}

func backupWithPhase(name string, phase gameplanev1alpha1.BackupPhase) *gameplanev1alpha1.Backup {
	return &gameplanev1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "gameplane-games"},
		Status:     gameplanev1alpha1.BackupStatus{Phase: phase},
	}
}

// collectPhaseCounts registers the collector with a throwaway registry, gathers
// once, and returns the named metric's value keyed by its phase label. Going
// through a real Gather exercises Describe/Collect end to end without pulling in
// the testutil package (and its extra module deps).
func collectPhaseCounts(t *testing.T, c prometheus.Collector, metricName string) map[string]float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	if err := reg.Register(c); err != nil {
		t.Fatalf("register collector: %v", err)
	}
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	out := map[string]float64{}
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			phase := ""
			for _, l := range m.GetLabel() {
				if l.GetName() == "phase" {
					phase = l.GetValue()
				}
			}
			out[phase] = m.GetGauge().GetValue()
		}
	}
	return out
}

func TestGameServerCollector(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(metricsScheme(t)).WithObjects(
		gsWithPhase("a", gameplanev1alpha1.GameServerPhaseRunning),
		gsWithPhase("b", gameplanev1alpha1.GameServerPhaseRunning),
		gsWithPhase("c", gameplanev1alpha1.GameServerPhaseRunning),
		gsWithPhase("d", gameplanev1alpha1.GameServerPhaseFailed),
		gsWithPhase("e", gameplanev1alpha1.GameServerPhasePending),
		// No phase yet (just created, not reconciled) — bucketed as Pending.
		gsWithPhase("f", ""),
	).Build()

	got := collectPhaseCounts(t, NewGameServerCollector(cl), "gameplane_gameservers")

	// Every phase reports a sample (0 when empty), and the counts sum to the
	// fleet size (6): 3 Running, 1 Failed, 2 Pending (one explicit + the
	// phase-less server bucketed as Pending).
	want := map[string]float64{
		"Pending":   2,
		"Starting":  0,
		"Running":   3,
		"Stopping":  0,
		"Stopped":   0,
		"Suspended": 0,
		"Failed":    1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("phase counts:\n got %v\nwant %v", got, want)
	}
}

func TestGameServerCollectorEmptyFleet(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(metricsScheme(t)).Build()

	got := collectPhaseCounts(t, NewGameServerCollector(cl), "gameplane_gameservers")

	// An empty fleet still reports one zero-valued series per known phase, so
	// the dashboard shows a flat 0 line rather than a gap.
	if len(got) != len(allGameServerPhases) {
		t.Fatalf("empty fleet: got %d series, want %d (%v)", len(got), len(allGameServerPhases), got)
	}
	for _, phase := range allGameServerPhases {
		if v := got[string(phase)]; v != 0 {
			t.Errorf("empty fleet: phase %q = %v, want 0", phase, v)
		}
	}
}

func TestBackupCollector(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(metricsScheme(t)).WithObjects(
		backupWithPhase("a", gameplanev1alpha1.BackupPhaseSucceeded),
		backupWithPhase("b", gameplanev1alpha1.BackupPhaseSucceeded),
		backupWithPhase("c", gameplanev1alpha1.BackupPhaseFailed),
		backupWithPhase("d", gameplanev1alpha1.BackupPhaseRunning),
		// No phase yet (just created, not reconciled) — bucketed as Pending.
		backupWithPhase("e", ""),
	).Build()

	got := collectPhaseCounts(t, NewBackupCollector(cl), "gameplane_backups")

	want := map[string]float64{
		"Pending":   1,
		"Running":   1,
		"Succeeded": 2,
		"Failed":    1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("backup phase counts:\n got %v\nwant %v", got, want)
	}
}
