package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsAllowed(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},                 // public
		{"1.1.1.1", true},                 // public
		{"10.0.0.1", true},                // RFC1918 — self-hosted infra, allowed
		{"192.168.1.10", true},            // RFC1918
		{"172.16.5.5", true},              // RFC1918
		{"127.0.0.1", true},               // loopback — kind/k3d registry, allowed
		{"::1", true},                     // loopback v6
		{"fc00::1", true},                 // ULA — allowed
		{"169.254.169.254", false},        // link-local: cloud metadata — BLOCKED
		{"169.254.0.1", false},            // link-local
		{"fe80::1", false},                // link-local v6
		{"0.0.0.0", false},                // unspecified
		{"::", false},                     // unspecified v6
		{"224.0.0.1", false},              // multicast
		{"ff02::1", false},                // link-local multicast v6
		{"::ffff:169.254.169.254", false}, // IPv4-mapped link-local
		{"64:ff9b::a9fe:a9fe", false},     // NAT64-wrapped 169.254.169.254
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("bad test ip %q", tc.ip)
		}
		if got := IsAllowed(ip); got != tc.want {
			t.Errorf("IsAllowed(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
	if IsAllowed(nil) {
		t.Error("IsAllowed(nil) = true, want false")
	}
}

func TestHostIsMetadata(t *testing.T) {
	for _, h := range []string{"metadata.google.internal", "METADATA.GOOGLE.INTERNAL", "metadata", "metadata.google.internal."} {
		if !HostIsMetadata(h) {
			t.Errorf("HostIsMetadata(%q) = false, want true", h)
		}
	}
	for _, h := range []string{"github.com", "registry.example.com", ""} {
		if HostIsMetadata(h) {
			t.Errorf("HostIsMetadata(%q) = true, want false", h)
		}
	}
}

func TestCheckHostAllowed(t *testing.T) {
	ctx := context.Background()
	// IP literals resolve without DNS.
	for _, h := range []string{"8.8.8.8", "10.0.0.1", "127.0.0.1"} {
		if err := CheckHostAllowed(ctx, h); err != nil {
			t.Errorf("CheckHostAllowed(%q) = %v, want nil", h, err)
		}
	}
	for _, h := range []string{"169.254.169.254", "metadata.google.internal", "0.0.0.0", ""} {
		if err := CheckHostAllowed(ctx, h); !errors.Is(err, ErrBlockedAddr) {
			t.Errorf("CheckHostAllowed(%q) = %v, want ErrBlockedAddr", h, err)
		}
	}
	// "localhost" resolves to loopback, which is allowed.
	if err := CheckHostAllowed(ctx, "localhost"); err != nil {
		t.Errorf("CheckHostAllowed(localhost) = %v, want nil", err)
	}
}

func get(t *testing.T, c *http.Client, rawURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return c.Do(req)
}

func TestHTTPClientGuardsDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Loopback is normally allowed, so the guarded client reaches the server.
	resp, err := get(t, HTTPClient(5*time.Second), srv.URL)
	if err != nil {
		t.Fatalf("guarded GET of loopback failed: %v", err)
	}
	_ = resp.Body.Close()

	// Flip the classifier to reject everything and confirm the dial Control
	// hook short-circuits the connection with ErrBlockedAddr.
	prev := ipAllowed
	ipAllowed = func(net.IP) bool { return false }
	defer func() { ipAllowed = prev }()
	resp, err = get(t, HTTPClient(5*time.Second), srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrBlockedAddr) {
		t.Errorf("guarded GET error = %v, want ErrBlockedAddr", err)
	}
}

func TestHTTPClientNoProxy(t *testing.T) {
	tr, ok := HTTPClient(time.Minute).Transport.(*http.Transport)
	if !ok {
		t.Fatal("transport is not *http.Transport")
	}
	if tr.Proxy != nil {
		t.Error("Proxy must be nil so the dial guard sees the real destination IP")
	}
}
