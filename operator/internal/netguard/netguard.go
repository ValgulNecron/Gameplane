// Package netguard restricts outbound connections made on behalf of
// user-configured ModuleSources so they cannot be aimed at the cloud
// instance-metadata endpoint — the classic SSRF credential-theft target.
//
// It is the operator-side counterpart to the agent's mod-download guard
// (agent/internal/mods), but deliberately weaker: the agent only ever
// fetches untrusted public mod URLs, so it demands a public-only policy,
// whereas ModuleSources are admin-configured infrastructure endpoints that
// are frequently and legitimately internal — a self-hosted GitLab or Harbor
// on an RFC1918 address, or a kind/k3d registry on loopback. Blocking every
// private range would break those, so this guard blocks only addresses that
// are never a legitimate module store and are high-value SSRF targets:
// link-local (where 169.254.169.254 lives), the unspecified address,
// multicast, and the NAT64/6to4 prefixes that can wrap a link-local address.
//
// The two implementations live in separate Go modules (linked by go.work),
// so the logic is duplicated rather than shared; keep them in sync.
package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// ErrBlockedAddr is returned when a connection targets a blocked address.
var ErrBlockedAddr = errors.New("address is not an allowed module-source destination")

// blockedV6Prefixes are IPv6 ranges that can embed or translate to a
// link-local/metadata address and so are refused defensively.
var blockedV6Prefixes = func() []*net.IPNet {
	out := make([]*net.IPNet, 0, 2)
	for _, c := range []string{
		"64:ff9b::/96", // NAT64 well-known prefix (can wrap 169.254.0.0/16)
		"2002::/16",    // 6to4 (can wrap a link-local IPv4)
	} {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

// IsAllowed reports whether ip is an acceptable module-source destination.
// It refuses link-local (the cloud metadata range), the unspecified address,
// multicast, and NAT64/6to4 prefixes that can wrap them. Loopback and
// private/ULA addresses are allowed because self-hosted module stores
// legitimately live there.
func IsAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	// Normalize an IPv4-mapped IPv6 address (e.g. ::ffff:a.b.c.d) to its
	// 4-byte form so the IPv4 reserved prefixes match it.
	norm := ip
	if v4 := ip.To4(); v4 != nil {
		norm = v4
	}
	for _, blk := range blockedV6Prefixes {
		if blk.Contains(norm) {
			return false
		}
	}
	return true
}

// ipAllowed is the classifier the dial guard consults. It is a package var
// only so this package's own tests can exercise the rejection path against a
// loopback listener; production never reassigns it.
var ipAllowed = IsAllowed

// metadataHosts are DNS names for cloud instance-metadata services. They
// resolve to a link-local IP (and so are already blocked by address), but are
// refused by name too for a clearer error.
var metadataHosts = map[string]bool{
	"metadata.google.internal": true,
	"metadata":                 true,
}

// HostIsMetadata reports whether host is a known instance-metadata hostname.
func HostIsMetadata(host string) bool {
	return metadataHosts[strings.ToLower(strings.TrimSuffix(host, "."))]
}

// dialControl is a net.Dialer.Control hook that rejects a dial whose resolved
// address is blocked. Running at dial time means the check sees the real
// destination IP, defeating DNS rebinding past a name-based allowlist.
func dialControl(_, addr string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ipAllowed(ip) {
		return ErrBlockedAddr
	}
	return nil
}

// Dialer returns a net.Dialer that refuses blocked destinations.
func Dialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: dialControl}
}

// HTTPClient returns an http.Client that refuses to connect to blocked
// addresses and never honours a proxy: a forward proxy from the pod
// environment would hide the real destination IP from the dial guard, so it
// dials directly to keep the guard authoritative.
func HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           Dialer(30 * time.Second).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConns:          2,
		},
	}
}

// CheckHostAllowed resolves host and returns ErrBlockedAddr if it is a
// metadata hostname or resolves to any blocked address. It is the pre-flight
// for transports (notably git over ssh) where the dial is buried and a
// Control hook cannot be injected; the residual TOCTOU window is bounded by
// the host-key pinning those transports already require.
func CheckHostAllowed(ctx context.Context, host string) error {
	if host == "" {
		return ErrBlockedAddr
	}
	if HostIsMetadata(host) {
		return ErrBlockedAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ipAllowed(ip) {
			return ErrBlockedAddr
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return ErrBlockedAddr
	}
	for _, a := range addrs {
		if !ipAllowed(a.IP) {
			return ErrBlockedAddr
		}
	}
	return nil
}
