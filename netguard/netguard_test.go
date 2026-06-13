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

// TestIsPublic locks in the agent's strict policy. The split with IsAllowed is
// intentional: IsPublic blocks RFC1918/loopback/CGNAT that IsAllowed permits,
// so a future refactor cannot silently collapse the two.
func TestIsPublic(t *testing.T) {
	blocked := []string{
		// stdlib-covered: loopback, private, link-local, unspecified
		"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1",
		"169.254.169.254", // cloud metadata endpoint (link-local)
		"::1", "0.0.0.0", "fd00::1", "fc00::1", "fe80::1",
		// IPv4-mapped IPv6 of the above must also be blocked
		"::ffff:169.254.169.254", "::ffff:127.0.0.1", "::ffff:10.0.0.1",
		// reserved/special-use ranges the stdlib predicates miss
		"100.64.0.1", "100.127.255.255", // RFC 6598 CGNAT (k8s)
		"192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1",
		"240.0.0.1", "255.255.255.255",
		"64:ff9b::1.2.3.4", "2002::1", "2001:db8::1", "fec0::1",
	}
	for _, s := range blocked {
		if ip := net.ParseIP(s); ip == nil || IsPublic(ip) {
			t.Errorf("IsPublic(%s) = true, want false", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "9.9.9.9", "2606:4700:4700::1111"} {
		if !IsPublic(net.ParseIP(s)) {
			t.Errorf("IsPublic(%s) = false, want true", s)
		}
	}
	if IsPublic(nil) {
		t.Error("IsPublic(nil) = true, want false")
	}
	// The two policies must genuinely differ: a private address is fine for
	// the operator but never for the agent.
	priv := net.ParseIP("10.0.0.1")
	if !IsAllowed(priv) || IsPublic(priv) {
		t.Errorf("policy split broken: IsAllowed(10.0.0.1)=%v IsPublic=%v, want true/false",
			IsAllowed(priv), IsPublic(priv))
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
	// Operator policy: IP literals resolve without DNS; private/loopback OK.
	for _, h := range []string{"8.8.8.8", "10.0.0.1", "127.0.0.1"} {
		if err := CheckHostAllowed(ctx, h, IsAllowed); err != nil {
			t.Errorf("CheckHostAllowed(%q, IsAllowed) = %v, want nil", h, err)
		}
	}
	for _, h := range []string{"169.254.169.254", "metadata.google.internal", "0.0.0.0", ""} {
		if err := CheckHostAllowed(ctx, h, IsAllowed); !errors.Is(err, ErrBlockedAddr) {
			t.Errorf("CheckHostAllowed(%q, IsAllowed) = %v, want ErrBlockedAddr", h, err)
		}
	}
	// "localhost" resolves to loopback, which IsAllowed permits.
	if err := CheckHostAllowed(ctx, "localhost", IsAllowed); err != nil {
		t.Errorf("CheckHostAllowed(localhost, IsAllowed) = %v, want nil", err)
	}
	// Agent policy rejects loopback/private even though they resolve.
	for _, h := range []string{"127.0.0.1", "10.0.0.1"} {
		if err := CheckHostAllowed(ctx, h, IsPublic); !errors.Is(err, ErrBlockedAddr) {
			t.Errorf("CheckHostAllowed(%q, IsPublic) = %v, want ErrBlockedAddr", h, err)
		}
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

	// Loopback is allowed by IsAllowed, so the guarded client reaches it.
	resp, err := get(t, HTTPClient(5*time.Second, IsAllowed), srv.URL)
	if err != nil {
		t.Fatalf("guarded GET of loopback failed: %v", err)
	}
	_ = resp.Body.Close()

	// A deny-all classifier makes the dial Control hook short-circuit with
	// ErrBlockedAddr. (IsPublic would also reject loopback, but a deny-all
	// func exercises the hook directly.)
	resp, err = get(t, HTTPClient(5*time.Second, func(net.IP) bool { return false }), srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrBlockedAddr) {
		t.Errorf("guarded GET error = %v, want ErrBlockedAddr", err)
	}
}

func TestHTTPClientNoProxy(t *testing.T) {
	tr, ok := HTTPClient(time.Minute, IsAllowed).Transport.(*http.Transport)
	if !ok {
		t.Fatal("transport is not *http.Transport")
	}
	if tr.Proxy != nil {
		t.Error("Proxy must be nil so the dial guard sees the real destination IP")
	}
}
