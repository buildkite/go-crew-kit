package filewrite_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildkite/go-crew-kit/filewrite"
)

func Example() {
	dir, err := os.MkdirTemp("", "filewrite-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "state")
	if err := filewrite.CreateExclusive(path, []byte("initial"), 0o600); err != nil {
		panic(err)
	}
	if err := filewrite.SyncDir(dir); err != nil {
		// Creation succeeded, but directory durability wasn't confirmed.
		panic(err)
	}

	published, err := filewrite.Replace(context.Background(), path, []byte("replacement"), ".state-")
	if err != nil {
		// Don't assume the old file remains when published is true.
		panic(fmt.Errorf("replace (published=%t): %w", published, err))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	fmt.Println(published, string(data))
	// Output: true replacement
}
