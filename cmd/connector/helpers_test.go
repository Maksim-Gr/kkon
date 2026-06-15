package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestWaitForConnectorRunning_AllErrors(t *testing.T) {
	client := newStatusClient(t, func(_ int) (int, string) {
		return http.StatusInternalServerError, "boom"
	})

	status, healthy := waitForConnectorRunning(context.Background(), client, "alpha", 3, 0)
	assert.False(t, healthy)
	assert.Empty(t, status.Name)
}
