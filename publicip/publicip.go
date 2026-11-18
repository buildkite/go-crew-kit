// Package publicip classifies IP addresses using a conservative public-address
// policy for outbound connections. It doesn't resolve names or open connections.
package publicip

import "net/netip"

// IsPublic reports whether addr passes this package's public-address policy.
// Invalid and zoned addresses are rejected. IPv4-mapped IPv6 addresses are
// checked as IPv4. Addresses must be global unicast and not private, and must
// fall outside the additional special-use prefixes listed below.
//
// The exclusions cover shared, documentation, benchmarking, protocol-assignment,
// transition, deprecated, reserved, and selected special-purpose anycast space.
// This is a fixed conservative policy, not a live IANA registry or a guarantee
// of reachability. Changes to its exclusions can change which destinations
// callers can reach.
//
// To use this check as part of SSRF protection, callers must validate every
// resolved address and dial only those vetted addresses without resolving the
// hostname again. Destination authorization and internal-network exceptions
// remain the caller's responsibility.
func IsPublic(addr netip.Addr) bool {
	if !addr.IsValid() || addr.Zone() != "" {
		return false
	}

	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}

	for _, prefix := range nonPublic {
		if prefix.Contains(addr) {
			return false
		}
	}

	return true
}

// The standard library handles private, loopback, link-local, unspecified,
// multicast, and limited-broadcast addresses. These exclusions also reject
// globally unicast space that this policy doesn't treat as public.
var nonPublic = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.31.196.0/24"),
	netip.MustParsePrefix("192.52.193.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.175.48.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2620:4f:8000::/48"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fec0::/10"),
}
