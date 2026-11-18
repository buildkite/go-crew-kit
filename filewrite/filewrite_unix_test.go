//go:build unix

package filewrite

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestExactPermissionsUnderUmask(t *testing.T) {
	const helper = "CREW_KIT_FILEWRITE_UMASK_HELPER"
	if os.Getenv(helper) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestExactPermissionsUnderUmask$")
		cmd.Env = append(os.Environ(), helper+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("restrictive umask: %v\n%s", err, output)
		}
		return
	}

	dir := t.TempDir()
	previous := syscall.Umask(0o777)
	defer syscall.Umask(previous)
	for _, mode := range []os.FileMode{0o600, 0o644} {
		path := filepath.Join(dir, mode.String())
		if err := CreateExclusive(path, []byte("new"), mode); err != nil {
			t.Fatal(err)
		}
		assertFile(t, path, "new", mode)
	}
	path := filepath.Join(dir, "replacement")
	if published, err := Replace(context.Background(), path, []byte("private"), ".stage-"); !published || err != nil {
		t.Fatalf("Replace = %t, %v", published, err)
	}
	assertFile(t, path, "private", 0o600)
}
