package envconfig_test

import (
	"fmt"
	"time"

	"github.com/buildkite/go-crew-kit/envconfig"
)

func ExampleLoader_Load() {
	listen := "127.0.0.1:8080"
	timeout := 30 * time.Second
	loader := envconfig.Loader{
		Prefix: "APP_",
		LookupEnv: func(name string) (string, bool) {
			// Omit LookupEnv to read the process environment instead.
			values := map[string]string{"APP_TIMEOUT": "500ms"}
			value, present := values[name]
			return value, present
		},
	}
	if err := loader.Load("", []envconfig.Setting{
		{Name: "LISTEN", Target: &listen},
		{Name: "TIMEOUT", Target: &timeout},
	}); err != nil {
		panic(err)
	}

	fmt.Println(listen, timeout)
	// Output: 127.0.0.1:8080 500ms
}
