package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// ClientIPFromTrustedProxies records the client IP of each request in the
// request context, where middleware.GetClientIP reads it back. The login
// rate limiters and the audit log key on that value.
//
// The TCP peer is checked first. A peer outside the trusted prefixes is the
// client, and any X-Forwarded-For header on its request is ignored. Only
// when the peer is a trusted proxy is X-Forwarded-For read, right to left:
// each entry is the address the hop to its right saw, so the walk goes on
// while the address reached so far is trusted and stops at the first one
// that is not. When every hop is trusted, the leftmost entry is the client.
// An entry that isn't an IP address ends the walk at the last address
// already reached (the peer itself if no entry was read).
//
// With no trusted prefixes the peer is always the client. A peer address
// that doesn't parse records nothing, so callers keep their RemoteAddr
// fallback. IPv4-mapped IPv6 addresses fold to IPv4 and IPv6 zones are
// dropped before the trust check and before the address is recorded.
func ClientIPFromTrustedProxies(trusted []netip.Prefix) func(http.Handler) http.Handler {
	prefixes := make([]netip.Prefix, len(trusted))
	for i, p := range trusted {
		prefixes[i] = p.Masked()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client, ok := derivedClientIP(r, prefixes)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			// chi keeps its context key private, so the address is stored
			// through chi's own ClientIPFromRemoteAddr, run on a copy of the
			// request whose RemoteAddr is the derived client. The original
			// RemoteAddr is put back before the request reaches next.
			remoteAddr := r.RemoteAddr
			record := middleware.ClientIPFromRemoteAddr(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				req.RemoteAddr = remoteAddr
				next.ServeHTTP(rw, req)
			}))
			withClient := r.WithContext(r.Context())
			withClient.RemoteAddr = client.String()
			record.ServeHTTP(w, withClient)
		})
	}
}

// derivedClientIP applies the rules documented on ClientIPFromTrustedProxies.
// ok is false only when the TCP peer address doesn't parse.
func derivedClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr // a bare address with no port
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	client := peer.Unmap().WithZone("")
	if !inTrustedPrefixes(client, trusted) {
		return client, true
	}
	// Several X-Forwarded-For headers read as one chain, in the order
	// received, so the walk starts at the last entry of the last header.
	headers := r.Header.Values("X-Forwarded-For")
	for hi := len(headers) - 1; hi >= 0; hi-- {
		entries := strings.Split(headers[hi], ",")
		for ei := len(entries) - 1; ei >= 0; ei-- {
			entry := strings.TrimSpace(entries[ei])
			if entry == "" {
				continue
			}
			hop, perr := netip.ParseAddr(entry)
			if perr != nil {
				return client, true
			}
			client = hop.Unmap().WithZone("")
			if !inTrustedPrefixes(client, trusted) {
				return client, true
			}
		}
	}
	return client, true
}

func inTrustedPrefixes(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
