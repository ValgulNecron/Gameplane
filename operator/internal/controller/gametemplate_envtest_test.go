//go:build envtest

package controller

import (
	"context"
	"testing"
	"time"
)

// TestTemplate_InUseCountTracksServers — InUseCount reflects the
// number of GameServers that reference the template across
// create/delete events.
func TestTemplate_InUseCountTracksServers(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameTemplateReconciler())

	tmpl := buildGameTemplate(uniqueName("counted"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	// 0 servers initially.
	eventually(t, func() (bool, string) {
		got := getTemplateInUseCount(t, tmpl.Name)
		return got == 0, "InUseCount = " + intToStr(got)
	})

	// Two servers using this template.
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp-a", tmpl.Name)); err != nil {
		t.Fatalf("create gs a: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp-b", tmpl.Name)); err != nil {
		t.Fatalf("create gs b: %v", err)
	}

	eventually(t, func() (bool, string) {
		got := getTemplateInUseCount(t, tmpl.Name)
		return got == 2, "InUseCount = " + intToStr(got)
	})

	// Delete one — InUseCount drops to 1.
	gs := getGameServer(t, ns, "smp-a")
	if err := k8sClient.Delete(context.Background(), gs); err != nil {
		t.Fatalf("delete gs a: %v", err)
	}

	eventually(t, func() (bool, string) {
		got := getTemplateInUseCount(t, tmpl.Name)
		return got == 1, "InUseCount = " + intToStr(got)
	})
}

// TestTemplate_NoChurnAtSteadyState — once InUseCount has converged,
// repeated reconciles do not bump the template's ResourceVersion.
func TestTemplate_NoChurnAtSteadyState(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withGameTemplateReconciler())

	tmpl := buildGameTemplate(uniqueName("steady"))
	if err := k8sClient.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	deleteCleanup(t, tmpl)

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "one", tmpl.Name)); err != nil {
		t.Fatalf("create gs: %v", err)
	}

	// Wait until InUseCount has converged on 1.
	eventually(t, func() (bool, string) {
		got := getTemplateInUseCount(t, tmpl.Name)
		return got == 1, "InUseCount = " + intToStr(got)
	})

	// Capture RV after a brief settling window, then assert it stays put.
	time.Sleep(500 * time.Millisecond)
	rv := getTemplateResourceVersion(t, tmpl.Name)

	consistently(t, 2*time.Second, func() (bool, string) {
		got := getTemplateResourceVersion(t, tmpl.Name)
		if got != rv {
			return false, "RV bumped " + rv + " → " + got + " (controller is churning Status updates)"
		}
		return true, ""
	})
}

func getTemplateInUseCount(t *testing.T, name string) int32 {
	t.Helper()
	tmpl := getTemplateByName(t, name)
	return tmpl.Status.InUseCount
}

func getTemplateResourceVersion(t *testing.T, name string) string {
	t.Helper()
	tmpl := getTemplateByName(t, name)
	return tmpl.ResourceVersion
}
