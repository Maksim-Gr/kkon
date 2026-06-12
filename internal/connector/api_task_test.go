package connector

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConnectorTasks(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id": {"connector": "alpha", "task": 0}, "config": {}},
			{"id": {"connector": "alpha", "task": 1}, "config": {}}
		]`))
	})

	got, err := client.ListConnectorTasks(context.Background(), "alpha")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Connector)
	assert.Equal(t, 0, got[0].Task)
	assert.Equal(t, 1, got[1].Task)
	assert.Equal(t, "/connectors/alpha/tasks", rec.path)
}

func TestGetConnectorTaskStatus(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id": 0, "state": "RUNNING", "worker_id": "w1:8083"}`))
	})

	got, err := client.GetConnectorTaskStatus(context.Background(), "alpha", 0)
	require.NoError(t, err)
	assert.Equal(t, "RUNNING", got.State)
	assert.Equal(t, "w1:8083", got.WorkerID)
	assert.Equal(t, "/connectors/alpha/tasks/0/status", rec.path)
}

func TestRestartConnectorTask(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.RestartConnectorTask(context.Background(), "alpha", 2)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/connectors/alpha/tasks/2/restart", rec.path)
}
