package connector

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPauseConnector(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	err := client.PauseConnector(context.Background(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/connectors/alpha/pause", rec.path)
}

func TestResumeConnector(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	err := client.ResumeConnector(context.Background(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/connectors/alpha/resume", rec.path)
}

func TestRestartConnector(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.RestartConnector(context.Background(), "alpha", true, false)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/connectors/alpha/restart?includeTasks=true&onlyFailed=false", rec.path)
}

func TestRestartConnectorError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("unknown connector"))
	})

	err := client.RestartConnector(context.Background(), "ghost", false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown connector")
}
