package filewrite

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new")
	if err := CreateExclusive(path, []byte("new bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	assertFile(t, path, "new bytes", 0o640)
	if err := CreateExclusive(path, []byte("replacement"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing destination: %v", err)
	}
	assertFile(t, path, "new bytes", 0o640)

	for _, target := range []string{path, filepath.Join(dir, "absent")} {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if err := CreateExclusive(link, []byte("replacement"), 0o600); !errors.Is(err, fs.ErrExist) {
			t.Fatalf("existing symlink: %v", err)
		}
		if got, err := os.Readlink(link); err != nil || got != target {
			t.Fatalf("symlink changed: %q, %v", got, err)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
	assertFile(t, path, "new bytes", 0o640)
	if _, err := os.Stat(filepath.Join(dir, "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dangling symlink target created: %v", err)
	}
	if err := CreateExclusive(dir, nil, 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing directory: %v", err)
	}
	if err := CreateExclusive(filepath.Join(dir, "missing", "file"), nil, 0o600); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing parent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("parent created: %v", err)
	}
}

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	if err := os.WriteFile(path, []byte("old public file"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"first", "different replacement bytes", ""} {
		published, err := Replace(context.Background(), path, []byte(data), ".state-*.json")
		if !published || err != nil {
			t.Fatalf("Replace = %t, %v", published, err)
		}
		assertFile(t, path, data, 0o600)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "state" {
		t.Fatalf("staged file left over: %v, %v", entries, err)
	}

	for _, destination := range []string{dir, filepath.Join(dir, "missing", "state")} {
		if published, err := Replace(context.Background(), destination, []byte("bad"), ".stage-"); published || err == nil {
			t.Fatalf("invalid destination: %t, %v", published, err)
		}
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(dir), ".stage-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("failed rename left stage: %v, %v", staged, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if published, err := Replace(ctx, filepath.Join(dir, "missing", "state"), nil, ".stage-"); published || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation must precede staging: %t, %v", published, err)
	}
}

func TestReplaceCancellationAndPublication(t *testing.T) {
	for _, when := range []string{"before staging", "before rename", "after rename", "directory sync failure"} {
		t.Run(when, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "state")
			if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if when == "before staging" {
				cancel()
			}
			write := func(f writableFile, data []byte, mode fs.FileMode) error {
				// Inspect the real staged file before it is renamed or cleaned up.
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 2 {
					t.Fatalf("staging: %v, %v", entries, err)
				}
				if !strings.HasPrefix(entries[0].Name(), ".peers-") {
					t.Fatalf("wrong staging pattern: %s", entries[0].Name())
				}
				err = writeAndClose(f, data, mode)
				if when == "before rename" {
					cancel()
				}
				return err
			}
			failure := &os.PathError{Op: "sync", Path: dir, Err: fs.ErrPermission}
			syncDir := func(path string) error {
				if when == "directory sync failure" {
					return failure
				}
				cancel()
				return SyncDir(path)
			}
			published, err := replace(ctx, path, []byte("new"), ".peers-", write, syncDir)
			switch when {
			case "before staging", "before rename":
				if published || !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation: %t, %v", published, err)
				}
				assertFile(t, path, "old", 0o600)
			case "after rename":
				if !published || err != nil {
					t.Fatalf("late cancellation: %t, %v", published, err)
				}
				assertFile(t, path, "new", 0o600)
			case "directory sync failure":
				var pathErr *os.PathError
				if !published || !errors.Is(err, fs.ErrPermission) || !errors.As(err, &pathErr) {
					t.Fatalf("published but unsynced: %t, %v", published, err)
				}
				assertFile(t, path, "new", 0o600)
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 1 {
				t.Fatalf("left staged files: %v, %v", entries, readErr)
			}
		})
	}
}

func TestWriteFailures(t *testing.T) {
	failure := errors.New("injected file error")
	closeFailure := errors.New("injected close error")
	for _, operation := range []string{"create", "replace"} {
		for _, stage := range []string{"chmod", "write", "short write", "sync", "close"} {
			t.Run(operation+"/"+stage, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "state")
				if operation == "replace" {
					if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var handle writableFile
				write := func(f writableFile, data []byte, mode fs.FileMode) error {
					handle = f
					return writeAndClose(faultFile{f, stage, failure, closeFailure}, data, mode)
				}
				var err error
				if operation == "create" {
					err = createExclusive(path, []byte("replacement"), 0o600, write, os.Remove)
				} else {
					var published bool
					published, err = replace(context.Background(), path, []byte("replacement"), ".stage-", write, SyncDir)
					if published {
						t.Fatal("published failed write")
					}
				}
				want := failure
				if stage == "short write" {
					want = io.ErrShortWrite
				}
				if !errors.Is(err, want) || !errors.Is(err, closeFailure) {
					t.Fatalf("lost write/close error: %v", err)
				}
				if _, err := handle.Write([]byte("still open?")); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("file not closed: %v", err)
				}
				if operation == "replace" {
					assertFile(t, path, "previous", 0o600)
				} else if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("incomplete file retained: %v", err)
				}
				staged, err := filepath.Glob(filepath.Join(dir, ".stage-*"))
				if err != nil || len(staged) != 0 {
					t.Fatalf("staged file retained: %v, %v", staged, err)
				}
			})
		}
	}
}

func TestCreateExclusiveCleanupFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial")
	failure := errors.New("write failed")
	cleanupFailure := errors.New("remove failed")
	write := func(f writableFile, _ []byte, _ fs.FileMode) error {
		_, err := f.Write([]byte("partial"))
		return errors.Join(failure, err, f.Close())
	}
	err := createExclusive(path, []byte("complete"), 0o600, write, func(string) error { return cleanupFailure })
	if !errors.Is(err, failure) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("lost failure: %v", err)
	}
	assertFile(t, path, "partial", 0o600)
}

func TestSyncDir(t *testing.T) {
	if err := SyncDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	err := SyncDir(filepath.Join(t.TempDir(), "missing"))
	var pathErr *os.PathError
	if !errors.Is(err, fs.ErrNotExist) || !errors.As(err, &pathErr) {
		t.Fatalf("missing directory: %v", err)
	}

	for _, stage := range []string{"sync", "close"} {
		t.Run(stage, func(t *testing.T) {
			f, err := os.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			failure, closeFailure := errors.New("sync failed"), errors.New("close failed")
			err = syncAndClose(faultFile{f, stage, failure, closeFailure})
			if !errors.Is(err, closeFailure) || (stage == "sync" && !errors.Is(err, failure)) {
				t.Fatalf("lost sync/close failure: %v", err)
			}
			if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("directory not closed: %v", err)
			}
		})
	}
}

func TestReplaceReadersSeeCompleteFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	first, second := strings.Repeat("a", 8193), strings.Repeat("bc", 6151)
	if published, err := Replace(context.Background(), path, []byte(first), ".stage-"); !published || err != nil {
		t.Fatalf("initial write: %t, %v", published, err)
	}
	done := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		for {
			data, err := os.ReadFile(path)
			if err != nil {
				done <- err
				return
			}
			if !bytes.Equal(data, []byte(first)) && !bytes.Equal(data, []byte(second)) {
				done <- errors.New("reader observed partial replacement")
				return
			}
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
		}
	}()
	for i := 0; i < 30; i++ {
		data := first
		if i%2 == 0 {
			data = second
		}
		if published, err := Replace(context.Background(), path, []byte(data), ".stage-"); !published || err != nil {
			close(stop)
			<-done
			t.Fatalf("replacement: %t, %v", published, err)
		}
	}
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, want string, mode fs.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("file contents = %q, %v, want %q", data, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != mode {
		t.Fatalf("file mode = %v, %v, want %o", info, err, mode)
	}
}

type faultFile struct {
	writableFile
	stage        string
	failure      error
	closeFailure error
}

func (f faultFile) Chmod(mode fs.FileMode) error {
	if f.stage == "chmod" {
		return f.failure
	}
	return f.writableFile.Chmod(mode)
}

func (f faultFile) Write(data []byte) (int, error) {
	if f.stage == "write" {
		return 0, f.failure
	}
	if f.stage == "short write" {
		return f.writableFile.Write(data[:len(data)/2])
	}
	return f.writableFile.Write(data)
}

func (f faultFile) Sync() error {
	if f.stage == "sync" {
		return f.failure
	}
	return f.writableFile.Sync()
}

func (f faultFile) Close() error {
	err := f.writableFile.Close()
	if f.stage == "close" {
		err = errors.Join(err, f.failure)
	}
	return errors.Join(err, f.closeFailure)
}
