package connector

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
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

func TestConnectorHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status connector.Status
		want   bool
	}{
		{
			name: "connector and tasks running",
			status: connector.Status{
				Connector: connector.ConnectorState{State: "RUNNING"},
				Tasks:     []connector.TaskState{{ID: 0, State: "RUNNING"}, {ID: 1, State: "RUNNING"}},
			},
			want: true,
		},
		{
			name: "running with no tasks",
			status: connector.Status{
				Connector: connector.ConnectorState{State: "RUNNING"},
			},
			want: true,
		},
		{
			name: "connector failed",
			status: connector.Status{
				Connector: connector.ConnectorState{State: "FAILED"},
				Tasks:     []connector.TaskState{{ID: 0, State: "RUNNING"}},
			},
			want: false,
		},
		{
			name: "task failed",
			status: connector.Status{
				Connector: connector.ConnectorState{State: "RUNNING"},
				Tasks:     []connector.TaskState{{ID: 0, State: "RUNNING"}, {ID: 1, State: "FAILED"}},
			},
			want: false,
		},
		{
			name: "connector paused",
			status: connector.Status{
				Connector: connector.ConnectorState{State: "PAUSED"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connectorHealthy(tt.status))
		})
	}
}

// newStatusClient returns a client whose status endpoint replies with the JSON
// produced by body(callCount) on each call, so tests can simulate a connector
// that changes state over successive polls.
func newStatusClient(t *testing.T, body func(call int) (int, string)) *connector.Client {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		code, payload := body(calls)
		calls++
		w.WriteHeader(code)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	return connector.NewClient(srv.URL)
}

func TestWaitForConnectorRunning_BecomesHealthy(t *testing.T) {
	client := newStatusClient(t, func(call int) (int, string) {
		if call < 2 {
			return http.StatusOK, `{"name":"alpha","connector":{"state":"RESTARTING"},"tasks":[{"id":0,"state":"RESTARTING"}]}`
		}
		return http.StatusOK, `{"name":"alpha","connector":{"state":"RUNNING"},"tasks":[{"id":0,"state":"RUNNING"}]}`
	})

	status, healthy := waitForConnectorRunning(context.Background(), client, "alpha", 5, 0)
	require.True(t, healthy)
	assert.Equal(t, "RUNNING", status.Connector.State)
}

func TestWaitForConnectorRunning_NeverHealthy(t *testing.T) {
	client := newStatusClient(t, func(_ int) (int, string) {
		return http.StatusOK, `{"name":"alpha","connector":{"state":"FAILED"},"tasks":[]}`
	})

	status, healthy := waitForConnectorRunning(context.Background(), client, "alpha", 3, 0)
	assert.False(t, healthy)
	assert.Equal(t, "FAILED", status.Connector.State)
	assert.Equal(t, "alpha", status.Name)
}

func TestFieldsToPrompt(t *testing.T) {
	res := connector.ConfigValidationResponse{
		Configs: []connector.ConfigEntry{
			{Definition: connector.ConfigDefinition{Name: "connector.class", Required: true}},
			{Definition: connector.ConfigDefinition{Name: "name", Required: true}},
			{Definition: connector.ConfigDefinition{Name: "connection.url", Required: true, Documentation: "URL."}},
			{Definition: connector.ConfigDefinition{Name: "auto.create", Required: false, DefaultValue: false}},
			{Definition: connector.ConfigDefinition{Name: "tasks.max", Required: true, DefaultValue: float64(1)}},
			{
				Definition: connector.ConfigDefinition{Name: "topics", Required: true},
				Value:      connector.ConfigValue{Name: "topics", Value: "bad topic", Errors: []string{"invalid"}},
			},
			{
				Definition: connector.ConfigDefinition{Name: "connection.password", Required: true},
				Value:      connector.ConfigValue{Name: "connection.password"},
			},
			{
				Definition: connector.ConfigDefinition{Name: "insert.mode", Required: true},
				Value:      connector.ConfigValue{Name: "insert.mode", RecommendedValues: []any{"insert", "upsert"}},
			},
		},
	}

	fields := fieldsToPrompt(res)
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	// connector.class and name are wizard-managed and always skipped;
	// non-required fields (auto.create) are skipped; every other required
	// field is included, even ones with a default (tasks.max) — the default
	// becomes the suggested answer instead of being silently accepted.
	assert.Equal(t, []string{"connection.url", "tasks.max", "topics", "connection.password", "insert.mode"}, names)

	assert.Equal(t, "URL.", fields[0].Doc)
	assert.Equal(t, "1", fields[1].Default, "required field with a default is suggested, not silently accepted")
	assert.Equal(t, "bad topic", fields[2].Default, "errored field re-prompts with its current value")
	assert.Equal(t, []string{"invalid"}, fields[2].Errors)
	assert.True(t, fields[3].Secret, "password fields must be masked")
	assert.Equal(t, []string{"insert", "upsert"}, fields[4].Recommended)
}

func TestIsSecretField(t *testing.T) {
	assert.True(t, isSecretField("connection.password"))
	assert.True(t, isSecretField("aws.secret.access.key"))
	assert.True(t, isSecretField("sasl.jaas.config"))
	assert.True(t, isSecretField("datadog.api.key"))
	assert.True(t, isSecretField("auth.token"))
	assert.True(t, isSecretField("gcs.credentials.json"))
	assert.False(t, isSecretField("connection.url"))
	assert.False(t, isSecretField("key.converter"))
	assert.False(t, isSecretField("aws.access.key.id"))
}

func TestRunBulk_AllSucceed(t *testing.T) {
	var attempted []string
	err := runBulk(context.Background(), false, "pause", []string{"a", "b"}, func(_ context.Context, name string) error {
		attempted = append(attempted, name)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, attempted)
}

func TestRunBulk_ContinuesPastFailure(t *testing.T) {
	var attempted []string
	err := runBulk(context.Background(), false, "pause", []string{"a", "bad", "c"}, func(_ context.Context, name string) error {
		attempted = append(attempted, name)
		if name == "bad" {
			return assert.AnError
		}
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pause 1 of 3 connector(s)")
	assert.Equal(t, []string{"a", "bad", "c"}, attempted, "all names must still be attempted")
}

func TestRunBulk_CanceledContextSkipsRemaining(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var attempted []string
	err := runBulk(ctx, false, "pause", []string{"a", "b"}, func(_ context.Context, name string) error {
		attempted = append(attempted, name)
		return nil
	})
	require.Error(t, err)
	assert.Empty(t, attempted, "canceled context must not invoke the operation")
}

func TestPrintDryRunPreview_JSONMode_SingleName(t *testing.T) {
	out := captureStdout(t, func() {
		printDryRunPreview(true, "pause", []string{"alpha"}, "")
	})

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &result), "dry-run json output must be valid JSON: %q", out)
	assert.Equal(t, "alpha", result["name"])
	assert.Equal(t, "pause", result["action"])
	assert.Equal(t, "dry-run", result["result"])
}

func TestPrintDryRunPreview_JSONMode_MultipleNames(t *testing.T) {
	out := captureStdout(t, func() {
		printDryRunPreview(true, "restart", []string{"alpha", "beta"}, "")
	})

	var results []map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &results), "dry-run json output must be valid JSON array: %q", out)
	require.Len(t, results, 2)
	assert.Equal(t, "alpha", results[0]["name"])
	assert.Equal(t, "beta", results[1]["name"])
	for _, r := range results {
		assert.Equal(t, "restart", r["action"])
		assert.Equal(t, "dry-run", r["result"])
	}
}

func TestPrintResultLines_ReturnsFailureCount(t *testing.T) {
	failed := printResultLines(false, nil, []string{"a", "b", "c"}, []string{"", "boom", ""})
	assert.Equal(t, 1, failed)
}

func TestPrintResultLines_JSONModePrintsGivenBytes(t *testing.T) {
	out := captureStdout(t, func() {
		printResultLines(true, []byte(`{"custom":"payload"}`), nil, nil)
	})
	assert.JSONEq(t, `{"custom":"payload"}`, out)
}

func TestWaitForConnectorRunning_AllErrors(t *testing.T) {
	client := newStatusClient(t, func(_ int) (int, string) {
		return http.StatusInternalServerError, "boom"
	})

	status, healthy := waitForConnectorRunning(context.Background(), client, "alpha", 3, 0)
	assert.False(t, healthy)
	assert.Empty(t, status.Name)
}

func TestWaitForConnectorRunning_StopsOnCancel(t *testing.T) {
	client := newStatusClient(t, func(_ int) (int, string) {
		return http.StatusOK, `{"name":"alpha","connector":{"state":"RESTARTING"},"tasks":[]}`
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, healthy := waitForConnectorRunning(ctx, client, "alpha", 5, time.Second)
	assert.False(t, healthy)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestConfigMapFromFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    map[string]string
		wantErr string
	}{
		{
			name: "flat",
			file: `{"name": "a", "connector.class": "X", "tasks.max": "1"}`,
			want: map[string]string{"name": "a", "connector.class": "X", "tasks.max": "1"},
		},
		{
			name: "wrapped folds name into config",
			file: `{"name": "a", "config": {"connector.class": "X"}}`,
			want: map[string]string{"name": "a", "connector.class": "X"},
		},
		{
			name: "wrapped with matching config name",
			file: `{"name": "a", "config": {"name": "a", "connector.class": "X"}}`,
			want: map[string]string{"name": "a", "connector.class": "X"},
		},
		{
			name: "non-string scalars become strings",
			file: `{"name": "a", "config": {"tasks.max": 1, "big": 12345678901234567890, "flag": true, "unset": null}}`,
			want: map[string]string{"name": "a", "tasks.max": "1", "big": "12345678901234567890", "flag": "true", "unset": ""},
		},
		{
			name:    "missing name",
			file:    `{"connector.class": "X"}`,
			wantErr: `has no "name"`,
		},
		{
			name:    "wrapped with mismatched names",
			file:    `{"name": "a", "config": {"name": "b"}}`,
			wantErr: `has name "a" but config name "b"`,
		},
		{
			name:    "unsupported top-level field",
			file:    `{"name": "a", "config": {}, "initial_state": "PAUSED"}`,
			wantErr: `unsupported top-level field "initial_state"`,
		},
		{
			name:    "nested value",
			file:    `{"name": "a", "transforms": {"x": 1}}`,
			wantErr: `nested value for "transforms"`,
		},
		{
			name:    "config not an object",
			file:    `{"name": "a", "config": "x"}`,
			wantErr: `"config" that is not a JSON object`,
		},
		{
			name:    "not an object",
			file:    `["a"]`,
			wantErr: "is not a JSON object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configMapFromFile([]byte(tt.file))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
