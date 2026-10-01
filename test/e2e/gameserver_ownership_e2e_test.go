//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGameServer_UnownedNamesakeSurvives is a regression test:
// the GameServer reconciler's fixed-name delete paths (config Secret,
// files Secret, RCON Secret, managed BackupSchedule) must check
// ownership (deleteIfControlledBy / metav1.IsControlledBy) before
// deleting, not just match on name. An unowned object that happens to
// share one of those fixed names must survive reconciliation.
//
// This test creates the config Secret and the managed BackupSchedule
// (the two fixed names cheapest to pre-seed) WITHOUT any OwnerReference
// before the GameServer exists, then creates a GameServer whose
// template has no config values and no BackupPolicy — both conditions
// that make the reconciler take the delete-if-unused branch for these
// objects. If ownership is enforced correctly, both survive; a
// name-only delete would remove them.
func TestGameServer_UnownedNamesakeSurvives(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	suffix := time.Now().UnixNano()
	tmplName := fmt.Sprintf("e2e-ownership-namesake-tmpl-%d", suffix)
	gsName := fmt.Sprintf("e2e-ownership-namesake-gs-%d", suffix)

	applyBusyboxTemplate(t, tmplName)

	unownedConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gsName + "-config",
			Namespace: ns,
			// Deliberately NO OwnerReferences — this object must never
			// be treated as belonging to the GameServer created below.
		},
		Data: map[string][]byte{"marker": []byte("unowned-config")},
	}
	if _, err := envInstance.K8s.CoreV1().Secrets(ns).
		Create(ctx, unownedConfigSecret, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create unowned config secret: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.K8s.CoreV1().Secrets(ns).
			Delete(context.Background(), gsName+"-config", metav1.DeleteOptions{})
	})

	unownedBackupSchedule := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "BackupSchedule",
		"metadata":   map[string]any{"name": gsName + "-auto", "namespace": ns},
		"spec": map[string]any{
			// serverRef intentionally points at a DIFFERENT server name —
			// this BackupSchedule is not this GameServer's managed one,
			// it just happens to share its fixed "<gs>-auto" name.
			"serverRef": map[string]any{"name": "not-" + gsName},
			"schedule":  "0 0 * * *",
			"repoRef":   map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":  "restic-snapshot",
			"suspend":   true,
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Create(ctx, unownedBackupSchedule, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create unowned backupschedule: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Delete(context.Background(), gsName+"-auto", metav1.DeleteOptions{})
	})

	// This GameServer has no spec.config values and no spec.backupPolicy,
	// so reconcileConfigSecret and reconcileBackupSchedule both take the
	// "nothing to manage, delete if we own it" branch against the two
	// fixed names above on every reconcile pass.
	applyBusyboxGameServer(t, ns, gsName, tmplName)

	// Give the reconciler several passes to reach and settle on a
	// steady state, then assert both namesakes are still present and
	// still unowned by this GameServer.
	envInstance.Consistently(t, 30*time.Second, 5*time.Second, func() (bool, string) {
		sec, err := envInstance.K8s.CoreV1().Secrets(ns).Get(ctx, gsName+"-config", metav1.GetOptions{})
		if err != nil {
			return false, "unowned config secret disappeared: " + err.Error()
		}
		for _, owner := range sec.OwnerReferences {
			if owner.Kind == "GameServer" && owner.Name == gsName && owner.Controller != nil && *owner.Controller {
				return false, "unowned config secret was adopted by " + gsName
			}
		}

		bs, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Get(ctx, gsName+"-auto", metav1.GetOptions{})
		if err != nil {
			return false, "unowned backupschedule disappeared: " + err.Error()
		}
		for _, owner := range bs.GetOwnerReferences() {
			if owner.Kind == "GameServer" && owner.Name == gsName && owner.Controller != nil && *owner.Controller {
				return false, "unowned backupschedule was adopted by " + gsName
			}
		}
		return true, ""
	})
}
