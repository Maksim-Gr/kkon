// Package fileutil provides safe writes for files containing credentials.
package fileutil

import (
	"os"
	"path/filepath"
)

// WriteAtomic replaces path only after its private staging file is fully written.
// The staging file lives in the same directory so rename stays on one filesystem.
func WriteAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".kkon-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name()) //nolint:errcheck // Removed by rename on success.
	defer file.Close()           //nolint:errcheck // Explicitly closed before rename.
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
