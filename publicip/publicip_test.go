package publicip_test

import (
	"net/netip"
	"testing"

	"github.com/buildkite/go-crew-kit/publicip"
)

func TestIsPublicRejectsNonPublic(t *testing.T) {
	// First and last addresses exercise the extent of each excluded range,
	// including the ranges handled by the standard library.
	for _, raw := range []string{
		"0.0.0.0", "0.255.255.255", "10.0.0.0", "10.255.255.255",
		"100.64.0.0", "100.127.255.255", "127.0.0.0", "127.255.255.255",
		"169.254.0.0", "169.254.255.255", "172.16.0.0", "172.31.255.255",
		"192.0.0.0", "192.0.0.255", "192.0.2.0", "192.0.2.255",
		"192.31.196.0", "192.31.196.255", "192.52.193.0", "192.52.193.255",
		"192.88.99.0", "192.88.99.255", "192.168.0.0", "192.168.255.255",
		"192.175.48.0", "192.175.48.255", "198.18.0.0", "198.19.255.255",
		"198.51.100.0", "198.51.100.255", "203.0.113.0", "203.0.113.255",
		"224.0.0.0", "239.255.255.255", "240.0.0.0", "255.255.255.255",
		"::", "::1", "::ffff:ffff", "64:ff9b::", "64:ff9b::ffff:ffff",
		"64:ff9b:1::", "64:ff9b:1:ffff:ffff:ffff:ffff:ffff",
		"100::", "100::ffff:ffff:ffff:ffff", "2001::", "2001:1ff:ffff:ffff:ffff:ffff:ffff:ffff",
		"2001:db8::", "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff",
		"2002::", "2002:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"2620:4f:8000::", "2620:4f:8000:ffff:ffff:ffff:ffff:ffff",
		"3fff::", "3fff:fff:ffff:ffff:ffff:ffff:ffff:ffff",
		"5f00::", "5f00:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"fc00::", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"fe80::", "febf:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"fec0::", "feff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		"ff00::", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
	} {
		t.Run(raw, func(t *testing.T) {
			addr := netip.MustParseAddr(raw)
			if publicip.IsPublic(addr) {
				t.Fatal("non-public address accepted")
			}

			if addr.Is4() && publicip.IsPublic(netip.MustParseAddr("::ffff:"+raw)) {
				t.Fatal("IPv4-mapped non-public address accepted")
			}
		})
	}
}

func TestIsPublicAcceptsPublic(t *testing.T) {
	// Neighbors of excluded ranges guard against overly broad masks.
	for _, raw := range []string{
		"1.0.0.0", "8.8.8.8", "100.63.255.255", "100.128.0.0",
		"172.15.255.255", "172.32.0.0", "192.0.1.0", "192.0.3.0",
		"192.31.195.255", "192.31.197.0", "192.52.192.255", "192.52.194.0",
		"192.88.98.255", "192.88.100.0", "192.175.47.255", "192.175.49.0",
		"198.17.255.255", "198.20.0.0", "198.51.99.255", "198.51.101.0",
		"203.0.112.255", "203.0.114.0", "223.255.255.255",
		"::1:0:0", "64:ff9a:ffff:ffff:ffff:ffff:ffff:ffff", "64:ff9b::1:0:0",
		"64:ff9b:0:ffff:ffff:ffff:ffff:ffff", "64:ff9b:2::",
		"ff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "100:0:0:1::",
		"2000:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "2001:200::",
		"2001:db7:ffff:ffff:ffff:ffff:ffff:ffff", "2001:db9::", "2003::",
		"2606:4700:4700::1111", "2620:4f:7fff:ffff:ffff:ffff:ffff:ffff", "2620:4f:8001::",
		"3ffe:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "3fff:1000::",
		"5eff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "5f01::",
	} {
		t.Run(raw, func(t *testing.T) {
			addr := netip.MustParseAddr(raw)
			if !publicip.IsPublic(addr) {
				t.Fatal("public address rejected")
			}

			if addr.Is4() && !publicip.IsPublic(netip.MustParseAddr("::ffff:"+raw)) {
				t.Fatal("IPv4-mapped public address rejected")
			}
		})
	}
}

func TestIsPublicRejectsInvalidAndZoned(t *testing.T) {
	if publicip.IsPublic(netip.Addr{}) {
		t.Fatal("zero address accepted")
	}

	for _, raw := range []string{"2606:4700:4700::1111%eth0", "::ffff:8.8.8.8%eth0", "fe80::1%eth0"} {
		if publicip.IsPublic(netip.MustParseAddr(raw)) {
			t.Errorf("zoned address %s accepted", raw)
		}
	}
}
