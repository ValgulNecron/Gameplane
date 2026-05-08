package handlers

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProject_FromDataField(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "alpha",
			CreationTimestamp: metav1.Time{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		Data: map[string][]byte{
			"url":      []byte("s3://bucket/path"),
			"password": []byte("secret"),
		},
	}
	got := project(s)
	if got.Name != "alpha" || got.URL != "s3://bucket/path" || !got.HasPassword {
		t.Fatalf("got %+v", got)
	}
}

func TestProject_FromStringDataField(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "x"},
		StringData: map[string]string{"url": "rclone://x", "password": "p"},
	}
	got := project(s)
	if got.URL != "rclone://x" || !got.HasPassword {
		t.Fatalf("got %+v", got)
	}
}

func TestProject_NoPassword(t *testing.T) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "y"},
		Data:       map[string][]byte{"url": []byte("local:/data")},
	}
	if project(s).HasPassword {
		t.Fatal("expected HasPassword false")
	}
}

func TestIsDestination(t *testing.T) {
	if isDestination(nil) {
		t.Fatal("nil should not be a destination")
	}
	yes := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{destinationLabel: "true"}},
	}
	if !isDestination(yes) {
		t.Fatal("labelled secret should be a destination")
	}
	no := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"x": "y"}},
	}
	if isDestination(no) {
		t.Fatal("unlabelled secret should not be a destination")
	}
}

func TestDestinationNameRE(t *testing.T) {
	good := []string{"a", "ab", "alpha-beta", "x1-y2"}
	bad := []string{"", "A", "-leading", "trailing-", "foo_bar"}
	for _, n := range good {
		if !nameRE.MatchString(n) {
			t.Errorf("rejected good name %q", n)
		}
	}
	for _, n := range bad {
		if nameRE.MatchString(n) {
			t.Errorf("accepted bad name %q", n)
		}
	}
}
