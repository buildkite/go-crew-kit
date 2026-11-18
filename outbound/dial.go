// Package outbound provides policy-checked TCP dialing and network-error
// classification. Callers own destination rules and HTTP transport settings.
package outbound

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/buildkite/go-crew-kit/publicip"
)

// Dialer authorizes destinations, validates every DNS answer, and connects only
// to the validated IPs. Configure it before use. Concurrent calls are safe if
// its callbacks are safe for concurrent use.
type Dialer struct {
	// Timeout bounds authorization, DNS, and all connection attempts together.
	// Values <= 0 use 15 seconds. A shorter context deadline takes precedence.
	Timeout time.Duration

	// Authorize checks host and port before DNS. It must be a local, nonblocking
	// policy check. An error denies the destination. A nil callback denies all
	// destinations, so the zero Dialer cannot connect anywhere.
	//
	// On success, a nil address predicate uses publicip.IsPublic. A non-nil
	// predicate replaces that policy and receives IPv4-mapped addresses as IPv4.
	// It can restrict answers to configured CIDRs or explicitly permit private
	// addresses. Invalid and zoned addresses are always rejected.
	//
	// Host and port come from net.SplitHostPort without canonicalization.
	// Callers must validate their input syntax and normalize it as needed.
	Authorize func(host, port string) (allow func(netip.Addr) bool, err error)

	// LookupNetIP replaces net.DefaultResolver.LookupNetIP when non-nil.
	// It must honor context cancellation and deadlines.
	LookupNetIP func(context.Context, string, string) ([]netip.Addr, error)

	// Dial replaces net.Dialer.DialContext when non-nil. It receives only vetted
	// IP:port addresses and must honor context cancellation and deadlines.
	Dial func(context.Context, string, string) (net.Conn, error)
}

// DialContext implements the dial hook for http.Transport. It accepts tcp, tcp4,
// and tcp6 networks and an explicit host:port address. It doesn't canonicalize
// authorities, follow redirects, use environment proxies, perform TLS, or set
// deadlines on the returned connection. Keep the original hostname in the HTTP
// transport for TLS verification.
//
// DNS is looked up once using network "ip". An empty answer or any denied
// address fails the entire call before any connection attempt. The remaining
// time is divided evenly among addresses not yet attempted, in answer order.
// No answers are cached. Errors wrap *Error and preserve the underlying cause.
func (d Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, &Error{Kind: Connect, Err: err}
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" || (network != "tcp" && network != "tcp4" && network != "tcp6") {
		return nil, &Error{Kind: DestinationDenied, Err: errors.New("expected TCP host:port destination")}
	}
	if d.Authorize == nil {
		return nil, &Error{Kind: DestinationDenied, Err: errors.New("no destination policy")}
	}

	allow, err := d.Authorize(host, port)
	if err != nil {
		return nil, &Error{Kind: DestinationDenied, Err: err}
	}
	if allow == nil {
		allow = publicip.IsPublic
	}

	// LookupNetIP discards zones on literal hosts, so reject them before lookup.
	if literal, err := netip.ParseAddr(host); err == nil && literal.Zone() != "" {
		return nil, &Error{Kind: AddressDenied, Err: errors.New("zoned destination address")}
	}

	lookup := d.LookupNetIP
	if lookup == nil {
		lookup = net.DefaultResolver.LookupNetIP
	}
	addresses, err := lookup(ctx, "ip", host)
	if err != nil {
		return nil, &Error{Kind: DNS, Err: err}
	}
	if len(addresses) == 0 {
		return nil, &Error{Kind: DNS, Err: errors.New("empty DNS answer")}
	}

	// Validate the whole answer before dialing. Don't mutate a lookup's slice.
	ips := make([]netip.Addr, 0, len(addresses))
	for _, raw := range addresses {
		if !raw.IsValid() || raw.Zone() != "" || !allow(raw.Unmap()) {
			return nil, &Error{Kind: AddressDenied, Err: errors.New("DNS answer contains a forbidden address")}
		}

		ips = append(ips, raw.Unmap())
	}

	dial := d.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	deadline, _ := ctx.Deadline()
	var last error
	for i, ip := range ips {
		if err := ctx.Err(); err != nil {
			return nil, &Error{Kind: Connect, Err: err}
		}

		attempt, cancelAttempt := context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(ips)-i))
		conn, err := dial(attempt, network, net.JoinHostPort(ip.String(), port))
		cancelAttempt()
		if err == nil {
			return conn, nil
		}

		last = err
	}

	return nil, &Error{Kind: Connect, Err: last}
}
