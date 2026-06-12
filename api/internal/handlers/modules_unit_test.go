package handlers

import (
	"reflect"
	"testing"
)

func TestJoinNames(t *testing.T) {
	if got := joinNames(nil); got != "" {
		t.Errorf("nil: got %q", got)
	}
	if got := joinNames([]string{"a"}); got != "a" {
		t.Errorf("one: got %q", got)
	}
	if got := joinNames([]string{"a", "b", "c"}); got != "a, b, c" {
		t.Errorf("three: got %q", got)
	}
}

func TestAppendUniqueSource(t *testing.T) {
	a, b := SourceRef{Name: "a", Type: "oci"}, SourceRef{Name: "b", Type: "git"}
	got := appendUniqueSource([]SourceRef{a, b}, SourceRef{Name: "c", Type: "upload"})
	if !reflect.DeepEqual(got, []SourceRef{a, b, {Name: "c", Type: "upload"}}) {
		t.Fatalf("got %+v", got)
	}
	got = appendUniqueSource([]SourceRef{a, b}, SourceRef{Name: "a", Type: "oci"})
	if !reflect.DeepEqual(got, []SourceRef{a, b}) {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeVersions(t *testing.T) {
	got := mergeVersions([]string{"1.0.0", "1.1.0"}, []string{"1.1.0", "1.2.0"})
	want := []string{"1.2.0", "1.1.0", "1.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestMergeVersions_Empty(t *testing.T) {
	if got := mergeVersions(nil, nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
