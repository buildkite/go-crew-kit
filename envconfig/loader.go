// Package envconfig loads explicitly selected dotenv files and environment
// overrides into caller-provided settings. Callers supply defaults before
// loading and perform application-specific validation afterward.
package envconfig

import (
	"errors"
	"fmt"
	"os"
)

// ErrInvalid identifies invalid file syntax, unknown or duplicate file settings,
// and values that cannot be parsed. File I/O errors do not wrap ErrInvalid.
var ErrInvalid = errors.New("invalid configuration")

// Setting binds an unprefixed name to a non-nil pointer. Target must be a
// *string, *bool, *time.Duration, *int, *int64, or *[]string. Names must be unique.
type Setting struct {
	Name   string
	Target any
}

// Loader selects settings by prefix. Its zero value uses unprefixed names and
// the process environment.
type Loader struct {
	Prefix string

	// LookupEnv replaces os.LookupEnv when non-nil. A present empty value
	// overrides the file value, just like any other present value.
	LookupEnv func(string) (string, bool)
}

// Load applies an optional file and then environment overrides to settings.
// Existing target values provide defaults. An empty path skips file loading.
// Load never discovers files or changes the process environment.
//
// The entire file is checked for syntax, unknown names, and duplicates before
// values are applied. Only the winning value for each setting is parsed.
// Unknown process variables are ignored. Errors name settings or file lines,
// never configuration values. File I/O errors may include the selected path.
//
// On error, targets may be partially updated. Load panics if an applied setting
// has an unsupported target type or a nil target pointer.
func (l Loader) Load(path string, settings []Setting) error {
	values, err := l.readEnvFile(path, settings)
	if err != nil {
		return err
	}

	lookup := l.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}

	for _, setting := range settings {
		name := l.Prefix + setting.Name
		value, present := lookup(name)
		if !present {
			value, present = values[name]
		}
		if !present {
			continue
		}

		if reason := setValue(setting.Target, value); reason != "" {
			return fmt.Errorf("%w: %s: %s", ErrInvalid, name, reason)
		}
	}

	return nil
}
