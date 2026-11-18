// Package hostname validates ASCII hostname label syntax without normalization
// or network lookup. Callers decide whether to allow IP literals, trailing dots,
// wildcards, or ports and handle those before calling Valid.
package hostname

import "strings"

// Valid reports whether name has 1–253 bytes and dot-separated labels of 1–63
// bytes. Labels contain only ASCII letters, digits, and hyphens, and must start
// and end with a letter or digit. Uppercase letters and single-label names are
// accepted. Trailing dots, whitespace, Unicode, wildcards, and ports aren't.
//
// Valid checks syntax only. It doesn't establish that a name exists or is safe
// to connect to. Dotted-decimal text such as "127.0.0.1" and "010.0.0.1" meets
// the label grammar. Callers that distinguish IP literals from DNS names must
// apply that policy separately.
func Valid(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || !alphaNumeric(label[0]) || !alphaNumeric(label[len(label)-1]) {
			return false
		}

		for i := 1; i < len(label)-1; i++ {
			if !alphaNumeric(label[i]) && label[i] != '-' {
				return false
			}
		}
	}

	return true
}

func alphaNumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
