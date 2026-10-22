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
	"github.com/Maksim-Gr/kkon/cmd/task"
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

// statusServer serves GET /connectors?expand=status for name → state pairs.
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

func TestExecute_HealthCheckUsesTraceFromStatus(t *testing.T) {
	var taskRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tasks/") {
			taskRequests++
		}
		_, _ = w.Write([]byte(`{"alpha": {"status": {"name": "alpha", "connector": {"state": "RUNNING"},
			"tasks": [{"id": 0, "state": "FAILED", "trace": "java.lang.RuntimeException: boom\n\tat Foo.bar"}]}}}`))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	out := captureStdout(t, func() { require.NoError(t, execute("connector", "health-check")) })
	assert.Contains(t, out, "Task 0: FAILED")
	assert.Zero(t, taskRequests, "trace should come from the status response")
}

// resetFlags restores flags on the global command tree shared by tests.
func resetFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		restart, _, err := RootCmd.Find([]string{"task", "restart"})
		require.NoError(t, err)
		require.NoError(t, RootCmd.PersistentFlags().Set("output", "text"))
		require.NoError(t, RootCmd.PersistentFlags().Set("dry-run", "false"))
		require.NoError(t, task.Cmd.PersistentFlags().Set("connector", ""))
		require.NoError(t, task.Cmd.PersistentFlags().Set("id", "-1"))
		require.NoError(t, restart.Flags().Set("yes", "false"))
	})
}

// taskServer serves one connector with task 0 and records mutating requests.
func taskServer(t *testing.T) *[]string {
	t.Helper()
	var mutations []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != http.MethodGet:
			mutations = append(mutations, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/tasks"):
			_, _ = w.Write([]byte(`[{"id": {"connector": "alpha", "task": 0}, "config": {}}]`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"id": 0, "state": "RUNNING", "worker_id": "w1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	return &mutations
}

func TestExecute_TaskRestartYesJSON(t *testing.T) {
	mutations := taskServer(t)
	resetFlags(t)

	out := captureStdout(t, func() {
		require.NoError(t, execute("task", "restart", "--connector", "alpha", "--id", "0", "--yes", "--output", "json"))
	})
	assert.JSONEq(t, `{"name": "alpha", "task": 0, "action": "restart", "result": "ok"}`, out)
	assert.Equal(t, []string{"POST /connectors/alpha/tasks/0/restart"}, *mutations)
}

func TestExecute_TaskRestartJSONRequiresYes(t *testing.T) {
	mutations := taskServer(t)
	resetFlags(t)

	err := execute("task", "restart", "--connector", "alpha", "--id", "0", "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output json requires non-interactive usage")
	assert.Empty(t, *mutations)
}

func TestExecute_TaskGetIgnoresDryRun(t *testing.T) {
	taskServer(t)
	resetFlags(t)

	out := captureStdout(t, func() {
		require.NoError(t, execute("task", "get", "--connector", "alpha", "--id", "0", "--dry-run", "--output", "json"))
	})
	assert.Contains(t, out, `"state": "RUNNING"`)
}

func TestExecute_UpdateAcceptsName(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_code": 404, "message": "Connector missing not found"}`))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)

	err := execute("connector", "update", "missing")
	require.Error(t, err)
	assert.Equal(t, []string{"/connectors/missing/config"}, paths)
}

func TestExecute_CreateFromFlatFileWrapsConfig(t *testing.T) {
	var posted []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config/validate"):
			_, _ = w.Write([]byte(`{"name": "x", "error_count": 0, "configs": []}`))
		case r.Method == http.MethodPost && r.URL.Path == "/connectors":
			posted, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name": "flat", "config": {}, "tasks": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)
	t.Cleanup(func() { require.NoError(t, connector.CreateCmd.Flags().Set("file", "")) })

	file := filepath.Join(t.TempDir(), "flat.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"name": "flat", "connector.class": "com.example.Source", "tasks.max": "1"}`), 0o600))

	out := captureStdout(t, func() {
		require.NoError(t, execute("connector", "create", "--file", file, "--output", "json"))
	})
	assert.JSONEq(t, `{"name": "flat", "action": "create", "result": "ok"}`, out)
	assert.JSONEq(t, `{"name": "flat", "config": {"name": "flat", "connector.class": "com.example.Source", "tasks.max": "1"}}`, string(posted))
}

func TestExecute_CreateFromFlatFileRequiresName(t *testing.T) {
	setTestConfig(t, "http://127.0.0.1:1")
	resetFlags(t)
	t.Cleanup(func() { require.NoError(t, connector.CreateCmd.Flags().Set("file", "")) })

	file := filepath.Join(t.TempDir(), "flat.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"connector.class": "com.example.Source"}`), 0o600))

	err := execute("connector", "create", "--file", file, "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `has no "name"`)
}

func TestExecute_CreateFromWrappedFileStringifiesValues(t *testing.T) {
	var posted []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config/validate"):
			_, _ = w.Write([]byte(`{"name": "x", "error_count": 0, "configs": []}`))
		case r.Method == http.MethodPost && r.URL.Path == "/connectors":
			posted, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name": "wrapped", "config": {}, "tasks": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)
	t.Cleanup(func() { require.NoError(t, connector.CreateCmd.Flags().Set("file", "")) })

	file := filepath.Join(t.TempDir(), "wrapped.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"name": "wrapped", "config": {"connector.class": "com.example.Source", "tasks.max": 2}}`), 0o600))

	captureStdout(t, func() {
		require.NoError(t, execute("connector", "create", "--file", file, "--output", "json"))
	})
	assert.JSONEq(t, `{"name": "wrapped", "config": {"name": "wrapped", "connector.class": "com.example.Source", "tasks.max": "2"}}`, string(posted))
}

func TestExecute_CreateFromFileRejectsUnknownTopLevelField(t *testing.T) {
	setTestConfig(t, "http://127.0.0.1:1")
	resetFlags(t)
	t.Cleanup(func() { require.NoError(t, connector.CreateCmd.Flags().Set("file", "")) })

	file := filepath.Join(t.TempDir(), "extra.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"name": "x", "config": {}, "initial_state": "PAUSED"}`), 0o600))

	err := execute("connector", "create", "--file", file, "--output", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported top-level field "initial_state"`)
}
