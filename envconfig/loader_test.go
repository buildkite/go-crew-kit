package envconfig_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/buildkite/go-crew-kit/envconfig"
)

func TestLoadPrecedenceAndTypes(t *testing.T) {
	text, unchanged, empty := "default", "untouched", "default"
	enabled, count, size, timeout := true, 13, int64(17), time.Minute
	zeroTimeout, zeroUnitTimeout := time.Minute, 2*time.Minute
	list, cleared := []string{"default"}, []string{"default"}
	settings := []envconfig.Setting{
		{"TEXT", &text}, {"UNCHANGED", &unchanged}, {"EMPTY", &empty},
		{"ENABLED", &enabled}, {"COUNT", &count}, {"SIZE", &size},
		{"TIMEOUT", &timeout}, {"LIST", &list}, {"CLEARED", &cleared},
		{"ZERO_TIMEOUT", &zeroTimeout}, {"ZERO_UNIT_TIMEOUT", &zeroUnitTimeout},
	}
	path := writeEnv(t, "TEST_TEXT=file\nTEST_EMPTY=file\nTEST_ENABLED=true\nTEST_COUNT=not-an-integer\nTEST_SIZE=4294967297\nTEST_TIMEOUT=350ms\nTEST_LIST= first, second \nTEST_CLEARED=from-file\nTEST_ZERO_TIMEOUT=7s\nTEST_ZERO_UNIT_TIMEOUT=11s\n")
	loader := envconfig.Loader{Prefix: "TEST_", LookupEnv: lookup(map[string]string{
		"TEST_TEXT": "process", "TEST_EMPTY": "", "TEST_ENABLED": "false",
		"TEST_COUNT": "0", "TEST_CLEARED": "", "TEST_UNKNOWN": "ignored",
		"TEST_ZERO_TIMEOUT": "0", "TEST_ZERO_UNIT_TIMEOUT": "0s",
	})}
	if err := loader.Load(path, settings); err != nil {
		t.Fatal(err)
	}

	if text != "process" || unchanged != "untouched" || empty != "" || enabled || count != 0 || size != 4294967297 || timeout != 350*time.Millisecond {
		t.Fatalf("loaded values = %q, %q, %q, %t, %d, %d, %s", text, unchanged, empty, enabled, count, size, timeout)
	}
	if !reflect.DeepEqual(list, []string{"first", "second"}) || len(cleared) != 0 {
		t.Fatalf("lists = %q, %q", list, cleared)
	}
	if zeroTimeout != 0 || zeroUnitTimeout != 0 {
		t.Fatalf("zero duration overrides = %s, %s", zeroTimeout, zeroUnitTimeout)
	}
}

func TestLoadDoesNotDiscoverFilesOrMutateEnvironment(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("CREW_KIT_TEST_VALUE=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A zero Loader uses os.LookupEnv and accepts unprefixed setting names.
	t.Setenv("CREW_KIT_TEST_VALUE", "process")
	before := os.Environ()
	var value string
	settings := []envconfig.Setting{{"CREW_KIT_TEST_VALUE", &value}}
	if err := (envconfig.Loader{}).Load(".env", settings); err != nil || value != "process" {
		t.Fatalf("process value = %q, error = %v", value, err)
	}
	if !reflect.DeepEqual(before, os.Environ()) {
		t.Fatal("Load mutated the process environment")
	}

	value = "default"
	loader := envconfig.Loader{LookupEnv: lookup(nil)}
	if err := loader.Load("", settings); err != nil || value != "default" {
		t.Fatalf("empty path discovered .env: value = %q, error = %v", value, err)
	}
	if err := loader.Load(".env", settings); err != nil || value != "file" {
		t.Fatalf("explicit file value = %q, error = %v", value, err)
	}
}
