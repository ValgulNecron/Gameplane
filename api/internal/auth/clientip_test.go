package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

// defaultTrustedProxies mirrors the API's default --trusted-proxies list.
var defaultTrustedProxies = mustPrefixes(
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"169.254.0.0/16", "::1/128", "fc00::/7", "fe80::/10",
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

// recordedClientIP runs one request through ClientIPFromTrustedProxies and
// returns the client IP the next handler sees. It also checks that the
// next handler still sees the request's original RemoteAddr.
func recordedClientIP(t *testing.T, trusted []netip.Prefix, remoteAddr string, xff ...string) string {
	t.Helper()
	var got, gotRemote string
	h := ClientIPFromTrustedProxies(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = middleware.GetClientIP(r.Context())
		gotRemote = r.RemoteAddr
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	if gotRemote != remoteAddr {
		t.Fatalf("next handler saw RemoteAddr %q, want the original %q", gotRemote, remoteAddr)
	}
	return got
}

func TestClientIPFromTrustedProxies_ClientAddress(t *testing.T) {
	proxyOnly := mustPrefixes("10.42.0.0/16")
	cases := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		want    string
	}{
		{
			name:    "untrusted peer is the client and its forwarded header is ignored",
			trusted: defaultTrustedProxies,
			remote:  "203.0.113.9:40000",
			xff:     []string{"198.51.100.1"},
			want:    "203.0.113.9",
		},
		{
			name:    "private-range peer outside a narrowed list is the client",
			trusted: proxyOnly,
			remote:  "192.168.1.20:40000",
			xff:     []string{"10.42.0.7"},
			want:    "192.168.1.20",
		},
		{
			name:    "trusted peer forwarding a private-range client records that client",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"192.168.1.50"},
			want:    "192.168.1.50",
		},
		{
			name:    "public client behind two trusted hops is recorded",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"203.0.113.7, 10.42.0.9"},
			want:    "203.0.113.7",
		},
		{
			name:    "walk stops at the first address outside the trusted list",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"192.168.9.9, 198.51.100.4, 10.42.0.9"},
			want:    "198.51.100.4",
		},
		{
			name:    "when every hop is trusted the leftmost address is the client",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"192.168.1.50, 10.42.0.9"},
			want:    "192.168.1.50",
		},
		{
			name:    "trusted peer without a forwarded header is the client",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			want:    "10.42.0.5",
		},
		{
			name:    "several forwarded headers read as one chain",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"203.0.113.7", "10.42.0.9"},
			want:    "203.0.113.7",
		},
		{
			name:    "entry that is not an address ends the walk at the last address reached",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"203.0.113.7, not-an-ip, 10.42.0.9"},
			want:    "10.42.0.9",
		},
		{
			name:    "rightmost entry that is not an address leaves the peer as the client",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"not-an-ip"},
			want:    "10.42.0.5",
		},
		{
			name:    "empty entries are skipped",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{" , 192.168.1.50 ,"},
			want:    "192.168.1.50",
		},
		{
			name:    "IPv4-mapped peer is checked as IPv4",
			trusted: defaultTrustedProxies,
			remote:  "[::ffff:10.42.0.5]:8000",
			xff:     []string{"203.0.113.7"},
			want:    "203.0.113.7",
		},
		{
			name:    "IPv4-mapped untrusted peer is recorded as IPv4",
			trusted: defaultTrustedProxies,
			remote:  "[::ffff:203.0.113.9]:40000",
			xff:     []string{"198.51.100.1"},
			want:    "203.0.113.9",
		},
		{
			name:    "IPv4-mapped forwarded entry is recorded as IPv4",
			trusted: defaultTrustedProxies,
			remote:  "10.42.0.5:8000",
			xff:     []string{"::ffff:203.0.113.7"},
			want:    "203.0.113.7",
		},
		{
			name:    "IPv6 trusted peer forwarding an IPv6 client",
			trusted: defaultTrustedProxies,
			remote:  "[fd00::5]:8000",
			xff:     []string{"2001:db8::7"},
			want:    "2001:db8::7",
		},
		{
			name:    "bare peer address without a port",
			trusted: defaultTrustedProxies,
			remote:  "203.0.113.9",
			want:    "203.0.113.9",
		},
		{
			name:    "with no trusted prefixes the peer is always the client",
			trusted: nil,
			remote:  "10.42.0.5:8000",
			xff:     []string{"203.0.113.7"},
			want:    "10.42.0.5",
		},
		{
			name:    "peer address that does not parse records nothing",
			trusted: defaultTrustedProxies,
			remote:  "not-an-address",
			xff:     []string{"203.0.113.7"},
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordedClientIP(t, tc.trusted, tc.remote, tc.xff...); got != tc.want {
				t.Fatalf("client IP = %q, want %q", got, tc.want)
			}
		})
	}
}

// limitedHandler puts a one-request-per-key limiter (no refill) behind
// ClientIPFromTrustedProxies with the default trusted list.
func limitedHandler() http.Handler {
	limiter := newTokenBucket(0, 1)
	return ClientIPFromTrustedProxies(defaultTrustedProxies)(limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
}

func sendFrom(h http.Handler, remoteAddr, xff string) int {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr.Code
}

func TestClientIPFromTrustedProxies_ClientsBehindOneProxyGetSeparateLimiterBuckets(t *testing.T) {
	h := limitedHandler()
	const proxy = "10.42.0.5:8000" // the same trusted proxy for every client
	if got := sendFrom(h, proxy, "192.168.1.50"); got != http.StatusNoContent {
		t.Fatalf("first client, first request: code=%d, want 204", got)
	}
	if got := sendFrom(h, proxy, "192.168.1.51"); got != http.StatusNoContent {
		t.Fatalf("second client, first request: code=%d, want 204 (own bucket)", got)
	}
	if got := sendFrom(h, proxy, "192.168.1.50"); got != http.StatusTooManyRequests {
		t.Fatalf("first client, second request: code=%d, want 429", got)
	}
}

func TestClientIPFromTrustedProxies_UntrustedPeerKeepsOneLimiterBucket(t *testing.T) {
	h := limitedHandler()
	const peer = "203.0.113.9:40000"
	if got := sendFrom(h, peer, "198.51.100.1"); got != http.StatusNoContent {
		t.Fatalf("first request: code=%d, want 204", got)
	}
	if got := sendFrom(h, peer, "198.51.100.2"); got != http.StatusTooManyRequests {
		t.Fatalf("second request with another forwarded address: code=%d, want 429 (same peer, same bucket)", got)
	}
}
