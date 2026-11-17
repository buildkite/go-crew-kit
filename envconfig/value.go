package envconfig

import (
	"strconv"
	"strings"
	"time"
)

// setValue returns fixed diagnostics instead of parser errors containing values.
func setValue(target any, value string) string {
	// Select parsing rules from the concrete target pointer type.
	switch target := target.(type) {
	case *string:
		// Assign strings literally.
		*target = value
	case *bool:
		// Accept only the lowercase boolean spellings.
		if value != "true" && value != "false" {
			return "must be true or false"
		}
		*target = value == "true"
	case *time.Duration:
		// Parse Go duration syntax without exposing parser errors containing values.
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return "must be a duration string"
		}
		*target = parsed
	case *int, *int64:
		// Match the parsing range to the target's integer width.
		bits := 64
		if _, ok := target.(*int); ok {
			bits = strconv.IntSize
		}

		// Require canonical decimal digits within the target's signed range.
		parsed, err := strconv.ParseInt(value, 10, bits)
		if err != nil || !canonicalDigits(value) {
			return "must be a decimal integer without a sign or leading zeros within the target integer range"
		}

		// Store the validated integer in the concrete target.
		switch target := target.(type) {
		case *int:
			*target = int(parsed)
		case *int64:
			*target = parsed
		}
	case *[]string:
		// Clear the existing list, treating an empty value as an empty list.
		*target = nil
		if value == "" {
			return ""
		}

		// Split comma-separated entries, trimming whitespace and rejecting blanks.
		for _, entry := range strings.Split(value, ",") {
			text := strings.TrimSpace(entry)
			if text == "" {
				return "must not contain empty list entries"
			}
			*target = append(*target, text)
		}
	default:
		// Treat unsupported targets as caller programming errors.
		panic("unsupported environment setting type")
	}

	return ""
}

func canonicalDigits(value string) bool {
	// Reject empty input and leading zeros except for zero itself.
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}

	// Require every byte to be an ASCII digit, excluding signs and separators.
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}

	return true
}
