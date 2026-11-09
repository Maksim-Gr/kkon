package schemaregistry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientRequests(t *testing.T) {
	subject := "tenant/orders value+%"
	input := Schema{Schema: `syntax = "proto3";`, SchemaType: "PROTOBUF", References: []Reference{{Name: "common.proto", Subject: "common", Version: 2}}}
	var compatibilityChecks int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "key", user)
		assert.Equal(t, "secret", pass)
		assert.Equal(t, "application/vnd.schemaregistry.v1+json", r.Header.Get("Accept"))
		path := r.URL.EscapedPath()
		switch {
		case path == "/subjects":
			_, _ = w.Write([]byte(`["z", "a"]`))
		case r.Method == http.MethodGet && path == subjectPath(subject)+"/versions":
			_, _ = w.Write([]byte(`[3, 1, 2]`))
		case r.Method == http.MethodGet && path == subjectPath(subject)+"/versions/latest":
			_, _ = w.Write([]byte(`{"subject":"tenant/orders value+%", "id":10,"version":3,"schema":"\"string\""}`))
		case r.Method == http.MethodPost:
			assert.Equal(t, "application/vnd.schemaregistry.v1+json", r.Header.Get("Content-Type"))
			var got Schema
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, input, got)
			if path == "/compatibility"+subjectPath(subject)+"/versions" {
				compatibilityChecks++
				assert.Equal(t, "true", r.URL.Query().Get("verbose"))
				_, _ = w.Write([]byte(`{"is_compatible":false,"messages":["field type changed"]}`))
			} else {
				assert.Equal(t, subjectPath(subject)+"/versions", path)
				_, _ = w.Write([]byte(`{"id":11}`))
			}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL+"/", "key", "secret")
	ctx := context.Background()
	subjects, err := client.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "z"}, subjects)
	versions, err := client.Versions(ctx, subject)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, versions)
	got, err := client.Get(ctx, subject, "latest")
	require.NoError(t, err)
	assert.Equal(t, "AVRO", got.SchemaType)
	assert.Equal(t, []Reference{}, got.References)
	assert.Equal(t, 3, got.Version)
	result, err := client.Check(ctx, subject, input)
	require.NoError(t, err)
	assert.False(t, result.IsCompatible)
	assert.Equal(t, []string{"field type changed"}, result.Messages)
	assert.Equal(t, 1, compatibilityChecks)
	registration, err := client.Publish(ctx, subject, input)
	require.NoError(t, err)
	assert.Equal(t, 11, registration.ID)
}

func TestClientFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{"authentication", 401, `{"error_code":40101,"message":"Unauthorized"}`},
		{"invalid schema", 422, `{"error_code":42201,"message":"Invalid schema"}`},
		{"incompatible registration", 409, `{"error_code":409,"message":"incompatible"}`},
		{"unavailable", 503, "registry unavailable"},
		{"malformed response", 200, "not JSON"},
		{"missing id", 200, `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := NewClient(server.URL, "", "").Publish(context.Background(), "orders", Schema{})
			require.Error(t, err)
			if test.status != 200 {
				var apiErr *APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, test.status, apiErr.Status)
			}
		})
	}
}

func TestCheckUnknownSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "must not treat a new subject as compatible")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_code":40401,"message":"Subject not found"}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL, "", "").Check(context.Background(), "new", Schema{})
	require.ErrorContains(t, err, "compatibility cannot be evaluated")
}

func TestClientCancellationAndTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "", "")
	_, err := client.List(context.Background())
	require.Error(t, err, "must verify TLS certificates")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.List(ctx)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestCheckRequiresRegistryDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[1]`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL, "", "").Check(context.Background(), "orders", Schema{})
	require.ErrorContains(t, err, "missing is_compatible")
}
