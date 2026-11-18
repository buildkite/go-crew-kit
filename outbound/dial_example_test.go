package outbound_test

import (
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/buildkite/go-crew-kit/outbound"
)

func ExampleDialer_DialContext() {
	internal := netip.MustParsePrefix("10.0.0.0/8")
	dialer := outbound.Dialer{
		Timeout: 10 * time.Second,
		Authorize: func(host, port string) (func(netip.Addr) bool, error) {
			if port != "443" {
				return nil, errors.New("port not permitted")
			}

			switch host {
			case "registry.example.com":
				return nil, nil // Use the shared public-IP policy.
			case "git.internal.example.com":
				return internal.Contains, nil // Use only the configured CIDR.
			default:
				return nil, errors.New("host not permitted")
			}
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Don't bypass the protected dialer via a proxy.
	transport.DialContext = dialer.DialContext
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer transport.CloseIdleConnections()

	_ = client // Use the original hostname in request URLs for TLS verification.
}
