package connector

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConnectors(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`["alpha","beta"]`))
	})

	got, err := client.ListConnectors(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, got)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/connectors", rec.path)
}

func TestListConnectorsError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	_, err := client.ListConnectors(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestListConnectorStatuses(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"alpha": {"info": {}, "status": {"name": "alpha", "connector": {"state": "RUNNING"}, "tasks": [{"id": 0, "state": "RUNNING"}]}}
		}`))
	})

	got, err := client.ListConnectorStatuses(context.Background())
	require.NoError(t, err)
	require.Contains(t, got, "alpha")
	assert.Equal(t, "RUNNING", got["alpha"].Connector.State)
	assert.Len(t, got["alpha"].Tasks, 1)
	assert.Equal(t, "/connectors?expand=status", rec.path)
}

func TestGetConnectorStatus(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name": "alpha", "connector": {"state": "FAILED"}, "tasks": []}`))
	})

	got, err := client.GetConnectorStatus(context.Background(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, "FAILED", got.Connector.State)
	assert.Equal(t, "/connectors/alpha/status", rec.path)
}

func TestSubmitConnector(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name": "alpha", "config": {"connector.class": "X"}, "type": "source"}`))
	})

	got, err := client.SubmitConnector(context.Background(), `{"name":"alpha"}`)
	require.NoError(t, err)
	assert.Equal(t, "alpha", got.Name)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.JSONEq(t, `{"name":"alpha"}`, string(rec.body))
}

func TestUpdateConnectorConfig(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	err := client.UpdateConnectorConfig(context.Background(), "alpha", map[string]string{"tasks.max": "2"})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/connectors/alpha/config", rec.path)
	assert.JSONEq(t, `{"tasks.max":"2"}`, string(rec.body))
}

func TestDeleteConnector(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteConnector(context.Background(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/connectors/alpha", rec.path)
}

func TestDeleteConnectorConflict(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})

	err := client.DeleteConnector(context.Background(), "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebalance")
}

func TestValidateConnectorConfig(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"name": "X",
			"error_count": 1,
			"configs": [
				{"value": {"name": "topics", "errors": ["Missing required configuration \"topics\""]}}
			]
		}`))
	})

	res, err := client.ValidateConnectorConfig(context.Background(), "io.example.X", map[string]string{"connector.class": "io.example.X"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.ErrorCount)
	require.Len(t, res.Configs, 1)
	assert.Equal(t, "topics", res.Configs[0].Value.Name)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/connector-plugins/io.example.X/config/validate", rec.path)
}
