package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupConnectorConfigKeepsEveryBackup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"alpha","connection.password":"example"}`))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	client := NewClient(srv.URL)
	seen := map[string]bool{}
	for range 5 {
		path, err := BackupConnectorConfig(context.Background(), client, []string{"alpha"}, dir)
		require.NoError(t, err)
		assert.False(t, seen[path], "backups must have unique paths")
		seen[path] = true
		data, err := os.ReadFile(path) //nolint:gosec // Path returned from a backup in t.TempDir.
		require.NoError(t, err)
		assert.JSONEq(t, `{"alpha":{"name":"alpha","connection.password":"example"}}`, string(data))
		info, err := os.Stat(path)
		require.NoError(t, err)
		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 5, "no staging files should remain")
}

func TestBackupConnectorConfigFailurePreservesBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "previous.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"previous":{}}`), 0o600))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	_, err := BackupConnectorConfig(context.Background(), NewClient(srv.URL), []string{"alpha"}, dir)
	require.Error(t, err)
	data, err := os.ReadFile(path) //nolint:gosec // Fixture in t.TempDir.
	require.NoError(t, err)
	assert.True(t, json.Valid(data))
	assert.JSONEq(t, `{"previous":{}}`, string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
