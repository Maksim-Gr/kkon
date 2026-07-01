package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreConnectorConfigs(t *testing.T) {
	client, rec := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	configs := map[string]map[string]string{
		"alpha": {"name": "alpha", "connector.class": "com.example.Alpha"},
	}

	restored, err := RestoreConnectorConfigs(context.Background(), client, configs)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, restored)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/connectors/alpha/config", rec.path)

	var sent map[string]string
	require.NoError(t, json.Unmarshal(rec.body, &sent))
	assert.Equal(t, "com.example.Alpha", sent["connector.class"])
}

func TestRestoreConnectorConfigsError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	configs := map[string]map[string]string{
		"alpha": {"name": "alpha"},
	}

	_, err := RestoreConnectorConfigs(context.Background(), client, configs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
