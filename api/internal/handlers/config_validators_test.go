package handlers

import (
	"strings"
	"testing"
)

// These tests pin the per-section validators directly so error branches
// don't need a full HTTP round-trip through PUT.

func TestValidateGeneral(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
		errs string
	}{
		{"missing instanceName", `{"defaultNamespace":"x"}`, false, "instanceName"},
		{"missing default ns", `{"instanceName":"k"}`, false, "defaultNamespace"},
		{"bad ns label", `{"instanceName":"k","defaultNamespace":"BadCAPS"}`, false, "RFC1123"},
		{"bad external url", `{"instanceName":"k","defaultNamespace":"n","externalURL":"ftp://x"}`, false, "http"},
		{"happy path", `{"instanceName":"k","defaultNamespace":"n","externalURL":"https://example.com"}`, true, ""},
		{"happy path no url", `{"instanceName":"k","defaultNamespace":"n"}`, true, ""},
		{"bad json", `not json`, false, "invalid json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateGeneral([]byte(tc.in))
			if tc.ok && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.errs)) {
				t.Fatalf("got %v, want substring %q", err, tc.errs)
			}
		})
	}
}

func TestValidateAuth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
		errs string
	}{
		{"empty providers", `{"providers":[]}`, true, ""},
		{"missing name", `{"providers":[{"kind":"local"}]}`, false, "name is required"},
		{"duplicate name", `{"providers":[{"name":"a","kind":"local"},{"name":"a","kind":"oidc"}]}`, false, "duplicate"},
		{"unknown kind", `{"providers":[{"name":"a","kind":"weird"}]}`, false, "kind must"},
		{"bad configRef", `{"providers":[{"name":"a","kind":"local","configRef":"BAD"}]}`, false, "configRef"},
		{"happy path", `{"providers":[{"name":"a","kind":"oidc","configRef":"my-secret"}]}`, true, ""},
		{"bad json", `{`, false, "invalid json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateAuth([]byte(tc.in))
			if tc.ok && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.errs)) {
				t.Fatalf("got %v want %q", err, tc.errs)
			}
		})
	}
}

func TestValidateNotifications(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
		errs string
	}{
		{"empty", `{"sinks":[]}`, true, ""},
		{"missing name", `{"sinks":[{"kind":"discord"}]}`, false, "name is required"},
		{"duplicate", `{"sinks":[{"name":"a","kind":"discord"},{"name":"a","kind":"slack"}]}`, false, "duplicate"},
		{"bad kind", `{"sinks":[{"name":"a","kind":"weird"}]}`, false, "kind must"},
		{"happy", `{"sinks":[{"name":"x","kind":"smtp"}]}`, true, ""},
		{"bad json", `{`, false, "invalid json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateNotifications([]byte(tc.in))
			if tc.ok && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.errs)) {
				t.Fatalf("got %v want %q", err, tc.errs)
			}
		})
	}
}

func TestValidateTelemetry(t *testing.T) {
	if _, err := validateTelemetry([]byte(`{"sendMetrics":true}`)); err != nil {
		t.Fatalf("happy: %v", err)
	}
	if _, err := validateTelemetry([]byte(`bogus`)); err == nil {
		t.Fatal("expected json error")
	}
}

func TestValidateUpdates(t *testing.T) {
	if _, err := validateUpdates([]byte(`{"channel":"stable"}`)); err != nil {
		t.Fatalf("stable: %v", err)
	}
	if _, err := validateUpdates([]byte(`{"channel":"beta"}`)); err != nil {
		t.Fatalf("beta: %v", err)
	}
	if _, err := validateUpdates([]byte(`{"channel":"nightly"}`)); err != nil {
		t.Fatalf("nightly: %v", err)
	}
	if _, err := validateUpdates([]byte(`{"channel":"weird"}`)); err == nil ||
		!strings.Contains(err.Error(), "channel must") {
		t.Fatalf("got %v", err)
	}
	if _, err := validateUpdates([]byte(`bogus`)); err == nil {
		t.Fatal("expected json error")
	}
}
