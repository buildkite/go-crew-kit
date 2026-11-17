package envconfig

import (
	"strconv"
	"strings"
	"time"
)

// setValue returns fixed diagnostics instead of parser errors containing values.
func setValue(target any, value string) string {
	switch target := target.(type) {
	case *string:
		*target = value
	case *bool:
		if value != "true" && value != "false" {
			return "must be true or false"
		}
		*target = value == "true"
	case *time.Duration:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return "must be a duration string"
		}
		*target = parsed
	case *int, *int64:
		bits := 64
		if _, ok := target.(*int); ok {
			bits = strconv.IntSize
		}

		parsed, err := strconv.ParseInt(value, 10, bits)
		if err != nil || !canonicalDigits(value) {
			return "must be a decimal integer without a sign or leading zeros within the target integer range"
		}

		switch target := target.(type) {
		case *int:
			*target = int(parsed)
		case *int64:
			*target = parsed
		}
	case *[]string:
		*target = nil
		if value == "" {
			return ""
		}

		for _, entry := range strings.Split(value, ",") {
			text := strings.TrimSpace(entry)
			if text == "" {
				return "must not contain empty list entries"
			}
			*target = append(*target, text)
		}
	default:
		panic("unsupported environment setting type")
	}

	return ""
}

func canonicalDigits(value string) bool {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}

	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}

	return true
}
