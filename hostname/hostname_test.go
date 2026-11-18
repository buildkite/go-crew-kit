package hostname_test

import (
	"strings"
	"testing"

	"github.com/buildkite/go-crew-kit/hostname"
)

func TestValid(t *testing.T) {
	for _, name := range []string{
		"a", "7", "localhost", "GitHub.COM", "a-b.example", "xn--bcher-kva.example",
		"127.0.0.1", "010.0.0.1", "999.0.0.1",
		strings.Repeat("a", 63),
		strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61),
	} {
		t.Run(name, func(t *testing.T) {
			if !hostname.Valid(name) {
				t.Fatal("valid hostname syntax rejected")
			}
		})
	}

	for _, name := range []string{
		"", ".", ".example", "example.", "a..example", "-a.example", "a-.example", "a.-.example",
		"a_b.example", "a/b.example", "a\\b.example", "a@b.example", "a?b.example", "a#b.example",
		"a%b.example", "*.example", "a:443", "::1", "[::1]", "bücher.example", "K.example",
		" example", "example ", "a\tb", "a\nb", "a\x00b", "a\xffb",
		strings.Repeat("a", 64),
		strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 62),
	} {
		t.Run(name, func(t *testing.T) {
			if hostname.Valid(name) {
				t.Fatal("invalid hostname syntax accepted")
			}
		})
	}
}
