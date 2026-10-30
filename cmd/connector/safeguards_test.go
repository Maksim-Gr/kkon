package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDisplayAndPromptsHideSecrets(t *testing.T) {
	cfg := map[string]string{
		"connection.password": "example-password",
		"auth.token":          "example-token",
		"sasl.jaas.config":    "example-jaas",
		"topics":              "events",
	}
	shown := redactConfig(cfg)
	for _, key := range []string{"connection.password", "auth.token", "sasl.jaas.config"} {
		assert.Equal(t, "********", shown[key])
		assert.NotEqual(t, "********", cfg[key], "display must not alter the submitted config")
		assert.Equal(t, "********", displayConfigValue(key, "replacement-secret"))
		assert.IsType(t, &survey.Password{}, configValuePrompt(key))
	}
	assert.Equal(t, "events", shown["topics"])
	assert.IsType(t, &survey.Input{}, configValuePrompt("topics"))
}

func TestVerifyConnectorOrRevertCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := verifyConnectorOrRevert(ctx, nil, "alpha", nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestVerifyConnectorOrRevertUnavailable(t *testing.T) {
	client := newStatusClient(t, func(_ int) (int, string) {
		return http.StatusServiceUnavailable, "unavailable"
	})
	err := verifyConnectorOrRevert(context.Background(), client, "alpha", nil)
	require.ErrorContains(t, err, "could not verify updated connector alpha")
}

func TestRevertUnhealthyConnector(t *testing.T) {
	for _, tc := range []struct {
		name       string
		confirm    bool
		confirmErr error
		status     int
		wantError  string
	}{
		{name: "declined", wantError: "keeping new config"},
		{name: "prompt failed", confirmErr: assert.AnError, wantError: "confirmation failed"},
		{name: "rollback failed", confirm: true, status: http.StatusInternalServerError, wantError: "failed to revert config"},
		{name: "rolled back", confirm: true, status: http.StatusOK, wantError: "previous config restored"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := map[string]string{"name": "alpha", "topics": "original"}
			var submitted map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/connectors/alpha/config", r.URL.Path)
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&submitted))
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(srv.Close)
			err := revertUnhealthyConnector(context.Background(), connector.NewClient(srv.URL), "alpha", original, func() (bool, error) {
				return tc.confirm, tc.confirmErr
			})
			require.ErrorContains(t, err, tc.wantError)
			if tc.confirm {
				assert.Equal(t, original, submitted)
			} else {
				assert.Nil(t, submitted)
			}
		})
	}
}
