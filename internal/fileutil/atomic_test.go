package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAtomicFailedReplacementCleansUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.Mkdir(target, 0o700))
	previous := filepath.Join(target, "previous")
	require.NoError(t, os.WriteFile(previous, []byte("previous config"), 0o600))

	require.Error(t, WriteAtomic(target, []byte("new config")))
	data, err := os.ReadFile(previous) //nolint:gosec // Fixture in t.TempDir.
	require.NoError(t, err)
	assert.Equal(t, "previous config", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "failed replacement must clean up its staging file")
}
