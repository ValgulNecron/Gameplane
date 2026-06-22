package controller

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func wipeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := kestrelv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("kestrel scheme: %v", err)
	}
	if err := batchv1.AddToScheme(s); err != nil {
		t.Fatalf("batch scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("core scheme: %v", err)
	}
	return s
}

func wipeGameServer(suspend bool, req string) *kestrelv1alpha1.GameServer {
	gs := &kestrelv1alpha1.GameServer{}
	gs.Name = "alpha"
	gs.Namespace = "ns"
	gs.Annotations = map[string]string{WipeRequestedAnnotation: req}
	gs.Spec.Suspend = suspend
	gs.Spec.TemplateRef.Name = "mc"
	return gs
}

func TestReconcileWipe_CreatesJobWhenSuspended(t *testing.T) {
	s := wipeScheme(t)
	gs := wipeGameServer(true, "tok1")
	tmpl := &kestrelv1alpha1.GameTemplate{}
	tmpl.Name = "mc"

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs).Build()
	r := &GameServerReconciler{Client: cl, Scheme: s}
	if err := r.reconcileWipe(context.Background(), gs, tmpl); err != nil {
		t.Fatalf("reconcileWipe: %v", err)
	}

	var job batchv1.Job
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "alpha-wipe", Namespace: "ns"}, &job); err != nil {
		t.Fatalf("expected wipe job: %v", err)
	}
	if job.Labels[wipeTokenLabel] != "tok1" {
		t.Errorf("token label = %q, want tok1", job.Labels[wipeTokenLabel])
	}
	if got := job.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName; got != "alpha-data" {
		t.Errorf("data claim = %q, want alpha-data", got)
	}
}

func TestReconcileWipe_SkipsWhenNotSuspended(t *testing.T) {
	s := wipeScheme(t)
	gs := wipeGameServer(false, "tok1")
	tmpl := &kestrelv1alpha1.GameTemplate{}
	tmpl.Name = "mc"

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs).Build()
	r := &GameServerReconciler{Client: cl, Scheme: s}
	if err := r.reconcileWipe(context.Background(), gs, tmpl); err != nil {
		t.Fatalf("reconcileWipe: %v", err)
	}
	var job batchv1.Job
	err := cl.Get(context.Background(), types.NamespacedName{Name: "alpha-wipe", Namespace: "ns"}, &job)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected no wipe job, got err=%v", err)
	}
}

func TestReconcileWipe_AcksWhenJobSucceeded(t *testing.T) {
	s := wipeScheme(t)
	gs := wipeGameServer(true, "tok1")
	tmpl := &kestrelv1alpha1.GameTemplate{}
	tmpl.Name = "mc"

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-wipe",
			Namespace: "ns",
			Labels:    map[string]string{wipeTokenLabel: "tok1"},
		},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs, job).Build()
	r := &GameServerReconciler{Client: cl, Scheme: s}
	if err := r.reconcileWipe(context.Background(), gs, tmpl); err != nil {
		t.Fatalf("reconcileWipe: %v", err)
	}

	// The request is acked on the GameServer.
	var got kestrelv1alpha1.GameServer
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "alpha", Namespace: "ns"}, &got); err != nil {
		t.Fatalf("get gs: %v", err)
	}
	if got.Annotations[WipeCompletedAnnotation] != "tok1" {
		t.Errorf("completed annotation = %q, want tok1", got.Annotations[WipeCompletedAnnotation])
	}
	// The finished Job is cleaned up.
	var leftover batchv1.Job
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "alpha-wipe", Namespace: "ns"}, &leftover); !apierrors.IsNotFound(err) {
		t.Errorf("expected wipe job deleted, got err=%v", err)
	}
}
