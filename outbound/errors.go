package outbound

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
)

// Kind is a stable failure category, not an HTTP status or retry decision.
type Kind string

const (
	DestinationDenied Kind = "destination_denied"
	AddressDenied     Kind = "address_denied"
	DNS               Kind = "dns"
	Connect           Kind = "connect"
	Canceled          Kind = "canceled"
	Timeout           Kind = "timeout"
	TLS               Kind = "tls"
	Transport         Kind = "transport"
)

// Error records the stage at which a protected dial failed. Classify may report
// Canceled or Timeout instead of Kind when the underlying cause indicates it.
// Err is preserved for errors.Is and errors.As. Error text can contain sensitive
// details from underlying errors. Use Classify for safe diagnostic categories.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}

	return string(e.Kind) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// Classify identifies wrapped dial and standard-library transport failures
// without inspecting error messages. Cancellation and timeout take precedence
// over the dial stage. TLS identifies typed certificate, record, and alert
// failures. Other unrecognized errors, including untyped TLS errors, return
// Transport. A nil error returns the empty Kind.
//
// Callers own HTTP status mapping, retries, metrics, and logging. Redirect and
// protocol-specific failures are also the caller's responsibility.
func Classify(err error) Kind {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}

	if isTimeout(err) {
		return Timeout
	}

	var failure *Error
	if errors.As(err, &failure) {
		return failure.Kind
	}

	var dns *net.DNSError
	if errors.As(err, &dns) {
		return DNS
	}

	var verification *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificate x509.CertificateInvalidError
	var record tls.RecordHeaderError
	var alert tls.AlertError
	if errors.As(err, &verification) || errors.As(err, &authority) || errors.As(err, &hostname) ||
		errors.As(err, &certificate) || errors.As(err, &record) || errors.As(err, &alert) {
		return TLS
	}

	var operation *net.OpError
	if errors.As(err, &operation) && operation.Op == "dial" {
		return Connect
	}

	return Transport
}

// Check every cause, including joined errors. A net.Error wrapper can report
// false from Timeout even when a deeper cause reports true.
func isTimeout(err error) bool {
	if network, ok := err.(net.Error); ok && network.Timeout() {
		return true
	}

	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, cause := range wrapped.Unwrap() {
			if isTimeout(cause) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return isTimeout(wrapped.Unwrap())
	}

	return false
}
