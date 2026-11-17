package envconfig_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/buildkite/go-crew-kit/envconfig"
)

func TestLoadRejectsInvalidValues(t *testing.T) {
	var boolean bool
	var integer int
	var wide int64
	var duration time.Duration
	var list []string
	for _, tt := range []struct {
		name   string
		target any
		values []string
	}{
		{"BOOL", &boolean, []string{"TRUE", "1", "", " true", "secret-value"}},
		{"INT", &integer, []string{"01", "00", "+5", "-0", "-1", "0x10", "1_000", "1.5", "", "secret-value"}},
		{"INT64", &wide, []string{"9223372036854775808", "secret-value"}},
		{"DURATION", &duration, []string{"", "9223372036854775808ns", "secret-value"}},
		{"LIST", &list, []string{"first,", ",first", "first, \t,second", "secret-value,"}},
	} {
		for _, value := range tt.values {
			t.Run(tt.name+"="+value, func(t *testing.T) {
				loader := envconfig.Loader{Prefix: "TEST_", LookupEnv: lookup(map[string]string{"TEST_" + tt.name: value})}
				err := loader.Load("", []envconfig.Setting{{tt.name, tt.target}})
				if !errors.Is(err, envconfig.ErrInvalid) || !strings.Contains(err.Error(), "TEST_"+tt.name+":") {
					t.Fatalf("error = %v", err)
				}
				if strings.Contains(err.Error(), "secret-value") {
					t.Fatalf("error echoed value: %v", err)
				}
			})
		}
	}
}

func TestLoadIntegerBounds(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	var integer int
	var wide int64
	settings := []envconfig.Setting{{"INT", &integer}, {"INT64", &wide}}
	loader := envconfig.Loader{LookupEnv: lookup(map[string]string{
		"INT": strconv.Itoa(maxInt), "INT64": "9223372036854775807",
	})}
	if err := loader.Load("", settings); err != nil || integer != maxInt || wide != 9223372036854775807 {
		t.Fatalf("integers = %d, %d, error = %v", integer, wide, err)
	}

	loader.LookupEnv = lookup(map[string]string{"INT": strconv.FormatUint(uint64(maxInt)+1, 10)})
	if err := loader.Load("", settings); !errors.Is(err, envconfig.ErrInvalid) {
		t.Fatalf("overflow accepted: %v", err)
	}
}

func TestLoadLeavesSemanticValidationToCaller(t *testing.T) {
	var duration time.Duration
	var enabled bool
	loader := envconfig.Loader{LookupEnv: lookup(map[string]string{"DURATION": "-3s", "BOOL": "true"})}
	if err := loader.Load("", []envconfig.Setting{{"DURATION", &duration}, {"BOOL", &enabled}}); err != nil || duration != -3*time.Second || !enabled {
		t.Fatalf("parsed values = %s, %t, error = %v", duration, enabled, err)
	}
}
