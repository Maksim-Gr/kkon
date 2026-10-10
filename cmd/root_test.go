package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Maksim-Gr/kkon/cmd/connector"
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

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// statusServer serves GET /connectors?expand=status with the given
// connector name → connector state pairs (all tasks RUNNING).
func statusServer(t *testing.T, states map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := map[string]any{}
		for name, state := range states {
			body[name] = map[string]any{"status": map[string]any{
				"name":      name,
				"connector": map[string]string{"state": state},
				"tasks":     []map[string]any{{"id": 0, "state": "RUNNING"}},
			}}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
}

func TestExecute_HealthCheckSortedOutput(t *testing.T) {
	statusServer(t, map[string]string{"delta": "RUNNING", "alpha": "RUNNING", "charlie": "RUNNING", "bravo": "RUNNING"})

	for range 5 {
		out := captureStdout(t, func() {
			require.NoError(t, execute("connector", "health-check"))
		})
		a, b, c, d := strings.Index(out, "alpha"), strings.Index(out, "bravo"), strings.Index(out, "charlie"), strings.Index(out, "delta")
		require.True(t, a >= 0 && a < b && b < c && c < d, "connectors not sorted:\n%s", out)
	}
}

func TestExecute_HealthCheckFailOnUnhealthy(t *testing.T) {
	statusServer(t, map[string]string{"alpha": "RUNNING", "bravo": "FAILED"})
	t.Cleanup(func() { require.NoError(t, connector.HealthCheckCmd.Flags().Set("fail-on-unhealthy", "false")) })

	// Without the flag an unhealthy connector is reported but not an error.
	captureStdout(t, func() { require.NoError(t, execute("connector", "health-check")) })

	var err error
	captureStdout(t, func() { err = execute("connector", "health-check", "--fail-on-unhealthy") })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 of 2 connector(s) unhealthy")

	// Filtering to healthy connectors only passes.
	captureStdout(t, func() { err = execute("connector", "health-check", "--fail-on-unhealthy", "--state", "RUNNING") })
	require.NoError(t, err)
	require.NoError(t, connector.HealthCheckCmd.Flags().Set("state", ""))
}
