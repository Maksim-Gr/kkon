package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setTestConfig points the CLI config at url via a temp HOME so commands run
// against an httptest server without touching the user's real config.
func setTestConfig(t *testing.T, url string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "kkon")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	cfg := fmt.Sprintf("kafkaConnect:\n  url: %s\n", url)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o600))
}

func execute(args ...string) error {
	RootCmd.SetArgs(args)
	return RootCmd.ExecuteContext(context.Background())
}

func TestExecute_FailedMutationReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	err := execute("connector", "pause", "some-connector")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pause some-connector")
}

func TestExecute_SuccessfulMutationReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	assert.NoError(t, execute("connector", "pause", "some-connector"))
}

func TestExecute_JSONModeRejectsInteractiveUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	// No connector name: pause would need to prompt, which json mode forbids.
	err := execute("connector", "pause", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output json requires non-interactive usage")

	// delete with a name but without --yes would need a confirm prompt.
	err = execute("connector", "delete", "some-connector", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output json requires non-interactive usage")

	// Reset the output flag for other tests sharing the global RootCmd.
	require.NoError(t, RootCmd.PersistentFlags().Set("output", "text"))
}

func TestExecute_DryRunSkipsMutation(t *testing.T) {
	var mutated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutated = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	require.NoError(t, execute("connector", "pause", "some-connector", "--dry-run"))
	assert.False(t, mutated, "dry-run must not send mutating requests")

	require.NoError(t, RootCmd.PersistentFlags().Set("dry-run", "false"))
}

func TestExecute_NothingToDoIsNotAFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	// No connectors exist, so there is nothing to pause; Execute treats
	// ErrNothingToDo as exit 0.
	err := execute("connector", "pause")
	assert.True(t, errors.Is(err, util.ErrNothingToDo))
}

func TestExecute_MalformedConfigIsReported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "kkon")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("kafkaConnect: [not a map"), 0o600))

	err := execute("connector", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")

	// The broken file must be left for the user to fix, not overwritten.
	b, readErr := os.ReadFile(path) //nolint:gosec
	require.NoError(t, readErr)
	assert.Equal(t, "kafkaConnect: [not a map", string(b))
}

func TestExecute_MissingURLWithoutTerminalFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(util.EnvConnectURL, "")

	err := execute("connector", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Kafka Connect URL configured")
	assert.Contains(t, err.Error(), util.EnvConnectURL)
}
