package outbound_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/buildkite/go-crew-kit/outbound"
)

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want outbound.Kind
	}{
		{name: "no error"},
		{name: "destination policy", err: &outbound.Error{Kind: outbound.DestinationDenied}, want: outbound.DestinationDenied},
		{name: "address policy", err: &outbound.Error{Kind: outbound.AddressDenied}, want: outbound.AddressDenied},
		{name: "DNS stage", err: &outbound.Error{Kind: outbound.DNS, Err: errors.New("lookup failed")}, want: outbound.DNS},
		{name: "connect stage", err: &outbound.Error{Kind: outbound.Connect, Err: errors.New("connection failed")}, want: outbound.Connect},
		{name: "DNS type", err: &net.DNSError{Err: "no such host"}, want: outbound.DNS},
		{name: "canceled", err: context.Canceled, want: outbound.Canceled},
		{name: "DNS canceled", err: &outbound.Error{Kind: outbound.DNS, Err: context.Canceled}, want: outbound.Canceled},
		{name: "deadline", err: context.DeadlineExceeded, want: outbound.Timeout},
		{name: "connect deadline", err: &outbound.Error{Kind: outbound.Connect, Err: context.DeadlineExceeded}, want: outbound.Timeout},
		{name: "DNS timeout", err: &outbound.Error{Kind: outbound.DNS, Err: &net.DNSError{IsTimeout: true}}, want: outbound.Timeout},
		{name: "joined DNS timeout", err: errors.Join(errors.New("context"), &net.DNSError{IsTimeout: true}), want: outbound.Timeout},
		{name: "joined dial timeout", err: &outbound.Error{Kind: outbound.Connect, Err: errors.Join(errors.New("context"), &net.DNSError{IsTimeout: true})}, want: outbound.Timeout},
		{name: "TLS verification", err: &tls.CertificateVerificationError{Err: errors.New("bad certificate")}, want: outbound.TLS},
		{name: "unknown authority", err: x509.UnknownAuthorityError{}, want: outbound.TLS},
		{name: "hostname mismatch", err: x509.HostnameError{}, want: outbound.TLS},
		{name: "invalid certificate", err: x509.CertificateInvalidError{}, want: outbound.TLS},
		{name: "TLS record", err: tls.RecordHeaderError{}, want: outbound.TLS},
		{name: "TLS alert", err: tls.AlertError(40), want: outbound.TLS},
		{name: "network dial", err: &net.OpError{Op: "dial", Err: errors.New("refused")}, want: outbound.Connect},
		{name: "other network operation", err: &net.OpError{Op: "read", Err: errors.New("reset")}, want: outbound.Transport},
		{name: "message is not a type", err: errors.New("dns resolve tls timeout"), want: outbound.Transport},
		{name: "unknown error", err: ioFailure{}, want: outbound.Transport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := outbound.Classify(tc.err); got != tc.want {
				t.Fatalf("Classify = %q, want %q", got, tc.want)
			}

			if tc.err != nil {
				wrapped := &url.Error{Op: "Get", URL: "https://example.com/?secret=value", Err: fmt.Errorf("wrapped: %w", tc.err)}
				if got := outbound.Classify(wrapped); got != tc.want {
					t.Fatalf("wrapped Classify = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// A classifier must not need to render an error that might contain secrets.
type ioFailure struct{}

func (ioFailure) Error() string { panic("classifier rendered error text") }
