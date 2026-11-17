package envconfig

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const maxEnvLineBytes = 64 * 1024

func (l Loader) readEnvFile(path string, settings []Setting) (map[string]string, error) {
	// Skip file loading when no path was explicitly selected.
	values := make(map[string]string)
	if path == "" {
		return values, nil
	}

	// Open the selected file and close it when loading finishes.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read environment file: %w", err)
	}
	defer f.Close()

	// Restrict file assignments to the caller's prefixed setting names.
	allowed := make(map[string]bool, len(settings))
	for _, setting := range settings {
		allowed[l.Prefix+setting.Name] = true
	}

	// Bound line size and track line numbers for diagnostics.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), maxEnvLineBytes)
	line := 1
	for ; scanner.Scan(); line++ {
		// Validate syntax and skip blank lines and full-line comments.
		name, value, skip, reason := parseEnvLine(scanner.Text())
		if reason != "" {
			return nil, fmt.Errorf("%w at line %d: %s", ErrInvalid, line, reason)
		}
		if skip {
			continue
		}

		// Reject unknown or repeated keys before recording the literal value.
		if !allowed[name] {
			return nil, fmt.Errorf("%w at line %d: unknown environment setting", ErrInvalid, line)
		}
		if _, duplicate := values[name]; duplicate {
			return nil, fmt.Errorf("%w at line %d: duplicate environment setting", ErrInvalid, line)
		}
		values[name] = value
	}

	// Classify oversized lines as invalid input while preserving other I/O causes.
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("%w at line %d: environment file line is too long", ErrInvalid, line)
		}

		return nil, fmt.Errorf("read environment file: %w", err)
	}

	return values, nil
}

func parseEnvLine(line string) (name, value string, skip bool, reason string) {
	// Reject NUL bytes anywhere in the line, including comments.
	if strings.IndexByte(line, 0) >= 0 {
		return "", "", false, "NUL byte"
	}

	// Ignore surrounding whitespace, blank lines, and full-line comments.
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", true, ""
	}

	// Reject shell-style export declarations.
	if strings.HasPrefix(trimmed, "export ") || strings.HasPrefix(trimmed, "export\t") {
		return "", "", false, "export is not supported"
	}

	// Split the assignment and reject missing names or disallowed whitespace.
	name, rest, found := strings.Cut(trimmed, "=")
	if !found || name == "" {
		return "", "", false, "expected KEY=value"
	}
	if strings.TrimSpace(name) != name || strings.ContainsAny(name, " \t") {
		return "", "", false, "malformed environment setting name"
	}

	// Quotes group literal bytes without escapes, interpolation, or execution.
	rest = strings.TrimSpace(rest)
	if len(rest) > 0 && (rest[0] == '\'' || rest[0] == '"') {
		// Extract the bytes up to the first matching closing quote.
		end := strings.IndexByte(rest[1:], rest[0])
		if end < 0 {
			return "", "", false, "malformed quoted value"
		}
		end++
		value = rest[1:end]

		// Allow only whitespace and an optional comment after the closing quote.
		trailing := strings.TrimSpace(rest[end+1:])
		if trailing != "" && !strings.HasPrefix(trailing, "#") {
			return "", "", false, "trailing content after quoted value"
		}

		return name, value, false, ""
	}

	// Remove unquoted comments only at value start or after a space or tab.
	for i := range rest {
		if rest[i] == '#' && (i == 0 || rest[i-1] == ' ' || rest[i-1] == '\t') {
			rest = rest[:i]
			break
		}
	}

	// Return the unquoted value without surrounding whitespace.
	return name, strings.TrimSpace(rest), false, ""
}
