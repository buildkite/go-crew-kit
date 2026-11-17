package envconfig_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildkite/go-crew-kit/envconfig"
)

func TestLoadFileSyntaxAndSafety(t *testing.T) {
	for _, tt := range []struct{ name, line, reason string }{
		{"unknown", "SECRET_NAME=secret-value", "unknown environment setting"},
		{"duplicate", "TEST_VALUE=secret-value", "duplicate environment setting"},
		{"bare", "SECRET_NAME", "expected KEY=value"},
		{"empty name", "=secret-value", "expected KEY=value"},
		{"export", "export TEST_VALUE=secret-value", "export is not supported"},
		{"export tab", "export\tTEST_VALUE=secret-value", "export is not supported"},
		{"indented export", "  export   TEST_VALUE=secret-value", "export is not supported"},
		{"bare export", "export TEST_VALUE", "export is not supported"},
		{"export name", "export=secret-value", "unknown environment setting"},
		{"export prefix", "exportTEST_VALUE=secret-value", "unknown environment setting"},
		{"space in name", "TEST VALUE=secret-value", "malformed environment setting name"},
		{"space before equals", "TEST_VALUE =secret-value", "malformed environment setting name"},
		{"quote", "TEST_VALUE='secret-value", "malformed quoted value"},
		{"trailing", "TEST_VALUE='secret-value' junk", "trailing content"},
		{"nul", "TEST_VALUE=secret-value\x00", "NUL byte"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var value string
			path := writeEnv(t, "TEST_VALUE=first\n"+tt.line+"\n")
			loader := envconfig.Loader{Prefix: "TEST_", LookupEnv: lookup(map[string]string{"TEST_VALUE": "override"})}
			err := loader.Load(path, []envconfig.Setting{{"VALUE", &value}})
			if !errors.Is(err, envconfig.ErrInvalid) || !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("error = %v, want line 2 and %q", err, tt.reason)
			}
			if strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "SECRET_NAME") {
				t.Fatalf("error echoed file input: %v", err)
			}
		})
	}
}

func TestLoadFileLiteralValues(t *testing.T) {
	for _, tt := range []struct{ line, want string }{
		{"TEST_VALUE=host#port", "host#port"},
		{"TEST_VALUE=  plain text \t# comment", "plain text"},
		{`TEST_VALUE=  "$HOME # literal" # comment`, "$HOME # literal"},
		{`TEST_VALUE='$(touch never-created)'`, "$(touch never-created)"},
		{`TEST_VALUE='literal\nbytes'`, `literal\nbytes`},
		{`TEST_VALUE="literal\nbytes${HOME}$(command)"`, `literal\nbytes${HOME}$(command)`},
		{"TEST_VALUE= # disabled", ""},
		{"TEST_VALUE=''", ""},
		{"TEST_VALUE=a=b", "a=b"},
	} {
		t.Run(tt.line, func(t *testing.T) {
			var value string
			path := writeEnv(t, "# comment\r\n\r\n  "+tt.line+"\r\n")
			loader := envconfig.Loader{Prefix: "TEST_", LookupEnv: lookup(nil)}
			if err := loader.Load(path, []envconfig.Setting{{"VALUE", &value}}); err != nil || value != tt.want {
				t.Fatalf("value = %q, error = %v, want %q", value, err, tt.want)
			}
		})
	}
}

func TestLoadFileIOErrors(t *testing.T) {
	for _, tt := range []struct {
		path    string
		missing bool
	}{
		{filepath.Join(t.TempDir(), "missing"), true},
		{t.TempDir(), false},
	} {
		err := (envconfig.Loader{}).Load(tt.path, nil)
		if err == nil || errors.Is(err, envconfig.ErrInvalid) {
			t.Fatalf("I/O error = %v", err)
		}
		if errors.Is(err, os.ErrNotExist) != tt.missing {
			t.Fatalf("missing file classification = %t, want %t: %v", errors.Is(err, os.ErrNotExist), tt.missing, err)
		}
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("I/O cause not preserved: %v", err)
		}
	}
}

func TestLoadFileLineLimit(t *testing.T) {
	for _, n := range []int{64*1024 - 2, 64 * 1024} {
		line := "TEST_VALUE=" + strings.Repeat("x", n-len("TEST_VALUE="))
		var value string
		loader := envconfig.Loader{Prefix: "TEST_", LookupEnv: lookup(nil)}
		err := loader.Load(writeEnv(t, "# comment\n"+line+"\n"), []envconfig.Setting{{"VALUE", &value}})
		if n < 64*1024 {
			if err != nil || len(value) != n-len("TEST_VALUE=") {
				t.Fatalf("accepted line: length = %d, error = %v", len(value), err)
			}
		} else if !errors.Is(err, envconfig.ErrInvalid) || !strings.Contains(err.Error(), "line 2") || strings.Contains(err.Error(), "xxx") {
			t.Fatalf("oversized line error = %v", err)
		}
	}

	path := writeEnv(t, "# comment\n#"+strings.Repeat("x", 64*1024)+"\n")
	err := (envconfig.Loader{LookupEnv: lookup(nil)}).Load(path, nil)
	if !errors.Is(err, envconfig.ErrInvalid) || !strings.Contains(err.Error(), "line 2") || strings.Contains(err.Error(), "xxx") {
		t.Fatalf("oversized comment error = %v", err)
	}
}
