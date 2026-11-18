package outbound_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/buildkite/go-crew-kit/outbound"
)

func TestDialContextDestinationPolicy(t *testing.T) {
	denied := errors.New("host not configured")
	for _, tc := range []struct {
		name      string
		network   string
		address   string
		authorize func(string, string) (func(netip.Addr) bool, error)
	}{
		{name: "no policy", network: "tcp", address: "registry.example:443"},
		{
			name: "denied host", network: "tcp", address: "other.example:443",
			authorize: func(host, port string) (func(netip.Addr) bool, error) {
				if host != "other.example" || port != "443" {
					t.Fatalf("policy target = %q, %q", host, port)
				}

				return nil, denied
			},
		},
		{name: "missing port", network: "tcp", address: "registry.example"},
		{name: "empty host", network: "tcp", address: ":443"},
		{name: "empty port", network: "tcp", address: "registry.example:"},
		{name: "non TCP", network: "unix", address: "registry.example:443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := outbound.Dialer{
				Authorize: tc.authorize,
				LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
					t.Fatal("denied destination reached DNS")
					return nil, nil
				},
				Dial: func(context.Context, string, string) (net.Conn, error) {
					t.Fatal("denied destination reached dialing")
					return nil, nil
				},
			}
			conn, err := d.DialContext(context.Background(), tc.network, tc.address)
			if conn != nil || outbound.Classify(err) != outbound.DestinationDenied {
				t.Fatalf("DialContext = %v, %v", conn, err)
			}
			if tc.name == "denied host" && !errors.Is(err, denied) {
				t.Fatalf("lost authorization error: %v", err)
			}
		})
	}
}

func TestDialContextAddressPolicy(t *testing.T) {
	internal := netip.MustParsePrefix("10.0.0.0/8")
	for _, tc := range []struct {
		name    string
		answers []netip.Addr
		allow   func(netip.Addr) bool
		want    []string
	}{
		{
			name:    "public addresses are normalized and tried in order",
			answers: []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111"), netip.MustParseAddr("::ffff:8.8.8.8")},
			want:    []string{"[2606:4700:4700::1111]:5443", "8.8.8.8:5443"},
		},
		{
			name:    "mapped internal address uses custom CIDR policy",
			answers: []netip.Addr{netip.MustParseAddr("::ffff:10.1.2.3")},
			allow:   internal.Contains,
			want:    []string{"10.1.2.3:5443"},
		},
		{
			name:    "explicit exception permits loopback",
			answers: []netip.Addr{netip.MustParseAddr("127.0.0.1")},
			allow:   func(netip.Addr) bool { return true },
			want:    []string{"127.0.0.1:5443"},
		},
		{
			name:    "mixed public and private answer is entirely refused",
			answers: []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")},
		},
		{
			name:    "conservative public IP exclusions apply",
			answers: []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("3fff::1")},
		},
		{
			name:    "internal policy replaces rather than extends public policy",
			answers: []netip.Addr{netip.MustParseAddr("10.1.2.3"), netip.MustParseAddr("8.8.8.8")},
			allow:   internal.Contains,
		},
		{
			name:    "invalid address cannot bypass validation",
			answers: []netip.Addr{{}},
			allow:   func(netip.Addr) bool { return true },
		},
		{
			name:    "zone cannot bypass validation",
			answers: []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111%eth0")},
			allow:   func(netip.Addr) bool { return true },
		},
		{
			name:    "zone is checked before unmapping",
			answers: []netip.Addr{netip.MustParseAddr("::ffff:8.8.8.8%eth0")},
			allow:   func(netip.Addr) bool { return true },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := slices.Clone(tc.answers)
			d := outbound.Dialer{
				Authorize: func(host, port string) (func(netip.Addr) bool, error) {
					if host != "registry.example" || port != "5443" {
						t.Fatalf("policy target = %q, %q", host, port)
					}

					return tc.allow, nil
				},
				LookupNetIP: func(_ context.Context, network, host string) ([]netip.Addr, error) {
					if network != "ip" || host != "registry.example" {
						t.Fatalf("lookup target = %q, %q", network, host)
					}

					return tc.answers, nil
				},
			}
			refused := errors.New("connection refused")
			var attempts []string
			d.Dial = func(_ context.Context, network, address string) (net.Conn, error) {
				if network != "tcp" {
					t.Fatalf("dial network = %q", network)
				}

				attempts = append(attempts, address)
				if len(attempts) < len(tc.want) {
					return nil, errors.New("earlier attempt failed")
				}

				return nil, refused
			}
			conn, err := d.DialContext(context.Background(), "tcp", "registry.example:5443")
			if conn != nil || !slices.Equal(attempts, tc.want) {
				t.Fatalf("attempts = %v, want %v, conn = %v", attempts, tc.want, conn)
			}
			if tc.want == nil {
				if outbound.Classify(err) != outbound.AddressDenied {
					t.Fatalf("policy refusal = %v", err)
				}
			} else if outbound.Classify(err) != outbound.Connect || !errors.Is(err, refused) {
				t.Fatalf("connection failure lost its cause: %v", err)
			}
			if !slices.Equal(original, tc.answers) {
				t.Fatal("modified lookup answer")
			}
		})
	}
}

func TestDialContextResolvesAfreshForEachCall(t *testing.T) {
	answers := []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}
	lookups := 0
	var attempts []string
	d := outbound.Dialer{
		Authorize: allowPublic,
		LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			if lookups >= len(answers) {
				t.Fatal("unexpected extra DNS lookup")
			}
			answer := answers[lookups]
			lookups++

			return []netip.Addr{answer}, nil
		},
		Dial: func(_ context.Context, _, address string) (net.Conn, error) {
			attempts = append(attempts, address)
			server, client := net.Pipe()
			t.Cleanup(func() { server.Close() })

			return client, nil
		},
	}

	for range 2 {
		conn, err := d.DialContext(context.Background(), "tcp", "registry.example:443")
		if err != nil {
			t.Fatalf("DialContext = %v", err)
		}
		conn.Close()
	}

	want := []string{"8.8.8.8:443", "1.1.1.1:443"}
	if lookups != 2 || !slices.Equal(attempts, want) {
		t.Fatalf("DNS lookups = %d, dial targets = %v, want 2 lookups and %v", lookups, attempts, want)
	}
}

func TestDialContextLookupFailures(t *testing.T) {
	lookupErr := &net.DNSError{Err: "no such host", Name: "registry.example"}
	for _, cause := range []error{nil, lookupErr} {
		d := outbound.Dialer{
			Authorize: allowPublic,
			LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
				return nil, cause
			},
			Dial: func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("failed lookup reached dialing")
				return nil, nil
			},
		}
		conn, err := d.DialContext(context.Background(), "tcp", "registry.example:443")
		if conn != nil || outbound.Classify(err) != outbound.DNS {
			t.Fatalf("DialContext = %v, %v", conn, err)
		}
		if cause != nil && !errors.Is(err, cause) {
			t.Fatalf("lost lookup cause: %v", err)
		}
	}
}

func TestDialContextRejectsZonedLiteralBeforeDefaultLookup(t *testing.T) {
	for _, allow := range []func(netip.Addr) bool{nil, func(netip.Addr) bool { return true }} {
		attempts := 0
		d := outbound.Dialer{
			Authorize: func(string, string) (func(netip.Addr) bool, error) { return allow, nil },
			Dial: func(context.Context, string, string) (net.Conn, error) {
				attempts++
				return nil, errors.New("dialed zoned literal")
			},
		}
		conn, err := d.DialContext(context.Background(), "tcp", "[2606:4700:4700::1111%eth0]:443")
		if conn != nil || attempts != 0 || outbound.Classify(err) != outbound.AddressDenied {
			t.Fatalf("zoned literal = %v, %v, dial attempts = %d", conn, err, attempts)
		}
	}
}

func TestDialContextSharesRemainingBudget(t *testing.T) {
	for _, first := range []string{"timeout", "refused", "canceled"} {
		t.Run(first, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			lookups := 0
			attempts := 0
			var attemptContexts []context.Context
			server, fallback := net.Pipe()
			defer server.Close()
			defer fallback.Close()
			d := outbound.Dialer{
				Timeout:   time.Minute,
				Authorize: allowPublic,
				LookupNetIP: func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
					lookups++
					until, _ := ctx.Deadline()
					if !until.Equal(deadline) {
						t.Fatal("lookup did not inherit shorter caller deadline")
					}

					return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}, nil
				},
				Dial: func(attempt context.Context, _, address string) (net.Conn, error) {
					attempts++
					attemptContexts = append(attemptContexts, attempt)
					until, ok := attempt.Deadline()
					if !ok || until.After(deadline) {
						t.Fatal("attempt exceeds overall deadline")
					}
					if attempts == 1 {
						if address != "8.8.8.8:443" {
							t.Fatalf("first target = %q", address)
						}
						remaining := time.Until(deadline)
						share := time.Until(until)
						if share <= remaining/3 || share > remaining*2/3 {
							t.Fatalf("first share = %v, remaining = %v", share, remaining)
						}

						switch first {
						case "timeout":
							<-attempt.Done()
							return nil, attempt.Err()
						case "canceled":
							cancel()
							return nil, attempt.Err()
						default:
							return nil, errors.New("connection refused")
						}
					}
					if attempts != 2 || address != "1.1.1.1:443" || attempt.Err() != nil {
						t.Fatalf("fallback target = %q, error = %v", address, attempt.Err())
					}
					if deadline.Sub(until) > time.Millisecond {
						t.Fatal("fallback did not receive all remaining time")
					}

					return fallback, nil
				},
			}
			conn, err := d.DialContext(ctx, "tcp", "registry.example:443")
			if first == "canceled" {
				if conn != nil || !errors.Is(err, context.Canceled) || attempts != 1 {
					t.Fatalf("canceled dial = %v, %v, attempts = %d", conn, err, attempts)
				}
			} else if err != nil || conn != fallback {
				t.Fatalf("fallback = %v, %v", conn, err)
			}
			if lookups != 1 {
				t.Fatalf("DNS lookups = %d, want 1", lookups)
			}
			for _, attempt := range attemptContexts {
				if attempt.Err() == nil {
					t.Fatal("attempt context was not released")
				}
			}
		})
	}
}

func TestDialContextBoundsDNS(t *testing.T) {
	d := outbound.Dialer{
		Timeout:   20 * time.Millisecond,
		Authorize: allowPublic,
		LookupNetIP: func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("lookup has no deadline")
			}

			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	conn, err := d.DialContext(context.Background(), "tcp", "registry.example:443")
	var failure *outbound.Error
	if conn != nil || outbound.Classify(err) != outbound.Timeout || !errors.Is(err, context.DeadlineExceeded) ||
		!errors.As(err, &failure) || failure.Kind != outbound.DNS {
		t.Fatalf("DNS timeout = %v, %v", conn, err)
	}
}

func TestDialContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := (outbound.Dialer{}).DialContext(ctx, "tcp", "registry.example:443")
	if conn != nil || outbound.Classify(err) != outbound.Canceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled dial = %v, %v", conn, err)
	}
}

func TestDialContextDefaultLookupAndDial(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	d := outbound.Dialer{
		Authorize: func(host, port string) (func(netip.Addr) bool, error) {
			if net.JoinHostPort(host, port) != listener.Addr().String() {
				return nil, errors.New("unexpected endpoint")
			}

			return netip.MustParsePrefix("127.0.0.0/8").Contains, nil
		},
	}
	conn, err := d.DialContext(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write([]byte("connected")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("connected"))
	if _, err := io.ReadFull(conn, got); err != nil || string(got) != "connected" {
		t.Fatalf("connection data = %q, %v", got, err)
	}
}

func TestDialContextHTTPTransportPreservesTLSHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS.ServerName != "example.com" || r.Host != "example.com" {
			t.Errorf("TLS name = %q, HTTP host = %q", r.TLS.ServerName, r.Host)
		}

		_, _ = io.WriteString(w, "verified origin")
	}))
	defer server.Close()

	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	d := outbound.Dialer{
		Authorize: func(name, service string) (func(netip.Addr) bool, error) {
			if name != "example.com" || service != "443" {
				return nil, errors.New("unexpected destination")
			}

			return netip.MustParsePrefix("127.0.0.0/8").Contains, nil
		},
		LookupNetIP: func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr(host)}, nil
		},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != net.JoinHostPort(host, "443") {
				t.Errorf("dial target = %q", address)
			}

			return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(host, port))
		},
	}
	client := server.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = d.DialContext
	defer transport.CloseIdleConnections()
	client.Transport = transport
	client.Timeout = 5 * time.Second
	resp, err := client.Get("https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "verified origin" {
		t.Fatalf("response = %q, %v", body, err)
	}
}

func allowPublic(string, string) (func(netip.Addr) bool, error) { return nil, nil }
