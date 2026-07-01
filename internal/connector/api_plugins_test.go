package connector

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListConnectorPlugins(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"class": "io.confluent.connect.s3.S3SinkConnector", "type": "sink", "version": "10.0.0"},
			{"class": "io.debezium.connector.postgresql.PostgresConnector", "type": "source", "version": "2.5.0"}
		]`))
	})

	got, err := client.ListConnectorPlugins(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "io.confluent.connect.s3.S3SinkConnector", got[0].Class)
	assert.Equal(t, "sink", got[0].Type)
	assert.Equal(t, "10.0.0", got[0].Version)
	assert.Equal(t, "source", got[1].Type)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/connector-plugins", rec.path)
}

func TestListConnectorPluginsError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	_, err := client.ListConnectorPlugins(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
