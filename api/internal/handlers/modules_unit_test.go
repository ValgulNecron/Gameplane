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

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"a", "b"}, "c")
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("got %+v", got)
	}
	got = appendUnique([]string{"a", "b"}, "a")
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
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
