// Package filewrite creates, replaces, and syncs files for callers that own
// their directories and serialize conflicting writes. It doesn't create parent
// directories, validate ownership, or coordinate multi-file transactions.
//
// CreateExclusive exposes a new file while writing it. Replace publishes a
// complete private file by renaming it. SyncDir confirms directory changes
// separately. Durability depends on the filesystem's rename and sync semantics.
// These operations target Unix filesystems with directory syncing support.
// Errors preserve their causes and may contain paths, but not file contents.
package filewrite

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CreateExclusive creates path without replacing an existing file, directory,
// or symlink. It sets exactly mode.Perm(), writes data, syncs the file and closes
// it. Special mode bits aren't applied. The parent directory must already exist.
//
// The destination is visible before writing completes. On failure after creation,
// it attempts removal and returns any cleanup error alongside the original error.
// A crash or failed cleanup can leave a partial file. It doesn't sync the parent:
// call SyncDir separately if the new directory entry needs durability confirmation.
func CreateExclusive(path string, data []byte, mode fs.FileMode) error {
	return createExclusive(path, data, mode, writeAndClose, os.Remove)
}

// Replace stages data beside path, sets exactly 0600, syncs and closes the staged
// file, renames it over path, then syncs the parent directory. tempPattern follows
// os.CreateTemp naming rules. The parent directory must already exist.
//
// published reports whether this call's rename succeeded, even if the subsequent
// directory open, sync, or close fails. Before rename, failures leave the existing
// destination unchanged and attempt to remove the staged file. Staging cleanup is
// best effort. Cancellation is checked before staging and immediately before
// rename. It doesn't interrupt filesystem calls or roll back a published file.
func Replace(ctx context.Context, path string, data []byte, tempPattern string) (published bool, err error) {
	return replace(ctx, path, data, tempPattern, writeAndClose, SyncDir)
}

// SyncDir opens, syncs and closes the directory at path. It preserves sync and
// close errors, including when both fail. Callers supply the intended directory:
// this doesn't validate ownership, reject symlinks, or sync ancestor directories.
// Unsupported directory syncing returns an error rather than being ignored.
func SyncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	return syncAndClose(f)
}

func createExclusive(path string, data []byte, mode fs.FileMode, write func(writableFile, []byte, fs.FileMode) error, remove func(string) error) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}

	if err := write(f, data, mode.Perm()); err != nil {
		return errors.Join(err, remove(path))
	}

	return nil
}

func replace(ctx context.Context, path string, data []byte, tempPattern string, write func(writableFile, []byte, fs.FileMode) error, syncDir func(string) error) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name())

	if err := write(f, data, 0o600); err != nil {
		return false, err
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return false, err
	}

	return true, syncDir(dir)
}

// The small file interfaces and operation arguments keep deterministic failure
// injection private. Production calls always use os files and real operations.
type writableFile interface {
	io.Writer
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

func writeAndClose(f writableFile, data []byte, mode fs.FileMode) error {
	err := f.Chmod(mode)
	if err == nil {
		var n int
		n, err = f.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
	}

	if err == nil {
		err = f.Sync()
	}

	return errors.Join(err, f.Close())
}

func syncAndClose(f interface {
	Sync() error
	Close() error
}) error {
	return errors.Join(f.Sync(), f.Close())
}
