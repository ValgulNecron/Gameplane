package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// upsertCondition replaces a condition by Type if present, otherwise
// appends. Sets LastTransitionTime to now when the status flips.
func upsertCondition(conds []metav1.Condition, c metav1.Condition) []metav1.Condition {
	now := metav1.Now()
	for i := range conds {
		if conds[i].Type != c.Type {
			continue
		}
		if conds[i].Status != c.Status {
			c.LastTransitionTime = now
		} else {
			c.LastTransitionTime = conds[i].LastTransitionTime
		}
		conds[i] = c
		return conds
	}
	if c.LastTransitionTime.IsZero() {
		c.LastTransitionTime = now
	}
	return append(conds, c)
}

// sameConditions compares two condition slices for equivalence,
// ignoring LastTransitionTime (which changes only on status flips).
func sameConditions(a, b []metav1.Condition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type ||
			a[i].Status != b[i].Status ||
			a[i].Reason != b[i].Reason ||
			a[i].Message != b[i].Message ||
			a[i].ObservedGeneration != b[i].ObservedGeneration {
			return false
		}
	}
	return true
}

// EffectiveConsoleMode resolves a GameTemplate's console mode, applying
// the default when ConsoleMode is unset:
//   - if RCON is configured with a non-"none" protocol → "rcon"
//   - otherwise → "none"
//
// Exported so the API service can consume the same defaulting rule when
// it decides which console WebSocket to expose for a given template.
func EffectiveConsoleMode(tmpl *kestrelv1alpha1.GameTemplate) string {
	if tmpl == nil {
		return "none"
	}
	if tmpl.Spec.ConsoleMode != "" {
		return tmpl.Spec.ConsoleMode
	}
	if tmpl.Spec.RCON != nil && tmpl.Spec.RCON.Protocol != "" && tmpl.Spec.RCON.Protocol != "none" {
		return "rcon"
	}
	return "none"
}

// enqueueTemplateForServer maps a GameServer change to a reconcile on
// the GameTemplate it references, so the template's InUseCount is kept
// up to date.
func enqueueTemplateForServer() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		gs, ok := obj.(*kestrelv1alpha1.GameServer)
		if !ok || gs.Spec.TemplateRef.Name == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: gs.Spec.TemplateRef.Name}}}
	})
}

// templateHasRCON reports whether the game actually exposes an RCON
// console the agent can dial. Mirrors EffectiveConsoleMode's defaulting:
// an absent RCON block or protocol "none" means no console port exists.
func templateHasRCON(tmpl *kestrelv1alpha1.GameTemplate) bool {
	return tmpl != nil && tmpl.Spec.RCON != nil &&
		tmpl.Spec.RCON.Protocol != "" && tmpl.Spec.RCON.Protocol != "none"
}
