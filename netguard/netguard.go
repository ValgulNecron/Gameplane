// Package netguard restricts outbound connections made on behalf of
// user-influenced inputs (module-source fetches, mod downloads) so they
// cannot be aimed at the cloud instance-metadata endpoint or other
// cluster-internal SSRF targets.
//
// It is shared by the operator and the agent, which enforce two deliberately
// different policies over the same machinery:
//
//   - IsAllowed (operator, permissive): ModuleSources are admin-configured
//     infrastructure endpoints that are frequently and legitimately internal —
//     a self-hosted GitLab/Harbor on an RFC1918 address, or a kind/k3d
//     registry on loopback. So it blocks only addresses that are never a
//     legitimate module store and are high-value SSRF targets: link-local
//     (where 169.254.169.254 lives), the unspecified address, multicast, and
//     the NAT64/6to4 prefixes that can wrap a link-local address.
//
//   - IsPublic (agent, strict): mod URLs are untrusted, so only globally
//     routable unicast addresses are allowed — loopback, private (RFC1918 /
//     ULA), link-local, multicast and the reserved/special-use ranges below
//     (notably RFC 6598 CGNAT, used by EKS/GKE/Cilium pod networks) are all
//     refused.
//
// The two policies must stay separately selectable: collapsing them would
// either re-open the SSRF the agent guards against or break the private
// registries the operator must reach. Callers pick a policy and pass it to
// the dialer/client constructors, which apply it at dial time (defeating DNS
// rebinding past a name-based allowlist).
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
var ErrBlockedAddr = errors.New("address is not an allowed destination")

// parseCIDRs parses a list of CIDR strings, skipping any that fail to parse.
func parseCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// normalize maps an IPv4-mapped IPv6 address (e.g. ::ffff:a.b.c.d) to its
// 4-byte form so the IPv4 reserved prefixes match it.
func normalize(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// blockedV6Prefixes are IPv6 ranges that can embed or translate to a
// link-local/metadata address and so are refused defensively by IsAllowed.
var blockedV6Prefixes = parseCIDRs(
	"64:ff9b::/96", // NAT64 well-known prefix (can wrap 169.254.0.0/16)
	"2002::/16",    // 6to4 (can wrap a link-local IPv4)
)

// reservedBlocks are non-globally-routable ranges that Go's net.IP predicates
// (IsPrivate/IsLoopback/…) miss but that IsPublic must still refuse: most
// importantly RFC 6598 CGNAT (100.64.0.0/10), which several Kubernetes setups
// (EKS custom networking, GKE, Cilium/Calico) use for node/pod addressing, so
// a mod URL resolving there would reach cluster-internal hosts. The NAT64/6to4
// translation prefixes and the TEST-NET / reserved blocks are denied for the
// same defense-in-depth reason.
var reservedBlocks = parseCIDRs(
	"100.64.0.0/10",   // RFC 6598 CGNAT (k8s node/pod ranges)
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved / future use (incl. 255.255.255.255)
	"64:ff9b::/96",    // NAT64 well-known prefix
	"2001:db8::/32",   // documentation
	"2002::/16",       // 6to4
	"fec0::/10",       // deprecated site-local
)

// IsAllowed is the operator policy: it refuses link-local (the cloud metadata
// range), the unspecified address, multicast, and NAT64/6to4 prefixes that can
// wrap them. Loopback and private/ULA addresses are allowed because
// self-hosted module stores legitimately live there.
func IsAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	norm := normalize(ip)
	for _, blk := range blockedV6Prefixes {
		if blk.Contains(norm) {
			return false
		}
	}
	return true
}

// IsPublic is the agent policy: it reports whether ip is a globally routable
// unicast address — i.e. not loopback, private (RFC1918 / ULA), link-local,
// multicast, unspecified, or one of the reserved/special-use ranges above.
// This is what blocks the agent from being tricked into fetching
// cluster-internal services or the cloud metadata endpoint.
func IsPublic(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	norm := normalize(ip)
	for _, blk := range reservedBlocks {
		if blk.Contains(norm) {
			return false
		}
	}
	return true
}

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

// dialControl returns a net.Dialer.Control hook that rejects a dial whose
// resolved address fails the policy. Running at dial time means the check sees
// the real destination IP, defeating DNS rebinding past a name-based
// allowlist.
func dialControl(allow func(net.IP) bool) func(string, string, syscall.RawConn) error {
	return func(_, addr string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || !allow(ip) {
			return ErrBlockedAddr
		}
		return nil
	}
}

// Dialer returns a net.Dialer that refuses destinations rejected by allow.
func Dialer(timeout time.Duration, allow func(net.IP) bool) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: dialControl(allow)}
}

// HTTPClient returns an http.Client that refuses to connect to addresses
// rejected by allow and never honours a proxy: a forward proxy from the pod
// environment would hide the real destination IP from the dial guard, so it
// dials directly to keep the guard authoritative. The returned client has no
// CheckRedirect — callers that re-validate redirect hosts set their own.
func HTTPClient(timeout time.Duration, allow func(net.IP) bool) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           Dialer(30*time.Second, allow).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConns:          2,
		},
	}
}

// CheckHostAllowed resolves host and returns ErrBlockedAddr if it is a
// metadata hostname or resolves to any address rejected by allow. It is the
// pre-flight for transports (notably git over ssh) where the dial is buried
// and a Control hook cannot be injected; the residual TOCTOU window is bounded
// by the host-key pinning those transports already require.
func CheckHostAllowed(ctx context.Context, host string, allow func(net.IP) bool) error {
	if host == "" {
		return ErrBlockedAddr
	}
	if HostIsMetadata(host) {
		return ErrBlockedAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		if !allow(ip) {
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
		if !allow(a.IP) {
			return ErrBlockedAddr
		}
	}
	return nil
}
