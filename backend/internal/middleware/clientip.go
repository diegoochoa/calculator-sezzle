package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// IPResolver derives the client address from a request.
type IPResolver struct {
	trusted []netip.Prefix
}

// NewIPResolver parses the trusted proxy list. Entries may be CIDR blocks
// (10.0.0.0/8) or single addresses.
func NewIPResolver(trustedProxies []string) (*IPResolver, error) {
	resolver := &IPResolver{}

	for _, entry := range trustedProxies {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if prefix, err := netip.ParsePrefix(entry); err == nil {
			resolver.trusted = append(resolver.trusted, prefix)
			continue
		}
		address, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is not an IP address or CIDR block", entry)
		}
		resolver.trusted = append(resolver.trusted, netip.PrefixFrom(address, address.BitLen()))
	}

	return resolver, nil
}

// ClientIP returns the address to attribute a request to.
//
// X-Forwarded-For is honoured only when the immediate peer is a configured
// proxy. Trusting it unconditionally would let any client forge the header and
// walk straight through a per-IP rate limit.
func (r *IPResolver) ClientIP(req *http.Request) string {
	peer := peerAddress(req.RemoteAddr)

	if len(r.trusted) == 0 || !r.isTrusted(peer) {
		return peer.String()
	}

	forwarded := req.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return peer.String()
	}

	// The left-most entry is the original client, as appended by the first proxy.
	first, _, _ := strings.Cut(forwarded, ",")
	address, err := netip.ParseAddr(strings.TrimSpace(first))
	if err != nil {
		return peer.String()
	}
	return address.String()
}

func (r *IPResolver) isTrusted(address netip.Addr) bool {
	if !address.IsValid() {
		return false
	}
	for _, prefix := range r.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func peerAddress(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return address.Unmap()
}
