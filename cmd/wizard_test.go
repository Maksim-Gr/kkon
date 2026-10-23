package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Maksim-Gr/kkon/cmd/connector"
	"github.com/Maksim-Gr/kkon/internal/prompt/prompttest"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder collects "METHOD path" lines and request bodies seen by a fake
// Kafka Connect server.
type recorder struct {
	mu     sync.Mutex
	calls  []string
	bodies map[string][]byte
}

func (r *recorder) record(req *http.Request) []byte {
	b, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	key := req.Method + " " + req.URL.Path
	r.calls = append(r.calls, key)
	if r.bodies == nil {
		r.bodies = map[string][]byte{}
	}
	r.bodies[key] = b
	return b
}

func (r *recorder) body(key string) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bodies[key]
}

func TestExecute_TemplateWizardSubmitsConfig(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		switch {
		case strings.HasSuffix(r.URL.Path, "/config/validate"):
			_, _ = w.Write([]byte(`{"error_count": 0, "configs": []}`))
		case r.Method == http.MethodPost && r.URL.Path == "/connectors":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name": "s3"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)

	prompttest.New(t,
		prompttest.Select("Pick a connector", "S3 Sink Connector"),
		prompttest.Input("Enter s3.region", "eu-west-1"),
		prompttest.Input("Enter s3.bucket.name", "bucket"),
		prompttest.Input("Enter topics", "orders"),
		prompttest.Select("Value format", ""),
		prompttest.Confirm("change any field", true),
		prompttest.Select("Which field", "flush.size"),
		prompttest.Input("new value for flush.size", "10"),
		prompttest.Confirm("change any field", false),
		prompttest.Input("Connector name", "s3"),
		prompttest.Confirm("submit this connector", true),
	)

	captureStdout(t, func() { require.NoError(t, execute("connector", "create")) })

	var posted struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
	}
	require.NoError(t, json.Unmarshal(rec.body("POST /connectors"), &posted))
	assert.Equal(t, "s3", posted.Name)
	assert.Equal(t, "s3", posted.Config["name"])
	assert.Equal(t, "eu-west-1", posted.Config["s3.region"])
	assert.Equal(t, "bucket", posted.Config["s3.bucket.name"])
	assert.Equal(t, "orders", posted.Config["topics"])
	assert.Equal(t, "10", posted.Config["flush.size"])
	assert.NotContains(t, posted.Config, "value.converter")
}

func TestExecute_TemplateWizardCancelDoesNotSubmit(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)

	prompttest.New(t,
		prompttest.Select("Pick a connector", "S3 Sink Connector"),
		prompttest.Input("Enter s3.region", "eu-west-1"),
		prompttest.Cancel("Enter s3.bucket.name"),
	)

	err := execute("connector", "create")
	require.ErrorIs(t, err, util.ErrCanceled)
	assert.Empty(t, rec.calls)
}

func TestExecute_PluginWizardPromptsForRequiredFields(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		switch {
		case r.URL.Path == "/connector-plugins":
			_, _ = w.Write([]byte(`[{"class": "com.example.Sink", "type": "sink", "version": "1.0"}]`))
		case strings.HasSuffix(r.URL.Path, "/config/validate"):
			var cfg map[string]string
			_ = json.Unmarshal(body, &cfg)
			if cfg["db.password"] != "" {
				_, _ = w.Write([]byte(`{"error_count": 0, "configs": []}`))
				return
			}
			_, _ = w.Write([]byte(`{"error_count": 1, "configs": [
				{"definition": {"name": "db.password", "required": true}, "value": {"name": "db.password", "value": null, "errors": ["Missing required configuration"]}},
				{"definition": {"name": "mode", "required": true, "default_value": "upsert"}, "value": {"name": "mode", "value": null, "recommended_values": ["insert", "upsert"]}},
				{"definition": {"name": "batch.size", "required": false, "default_value": 100}, "value": {"name": "batch.size", "value": "100"}}
			]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/connectors":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name": "sink"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)
	t.Cleanup(func() { require.NoError(t, connector.CreateCmd.Flags().Set("plugin", "")) })

	prompttest.New(t,
		prompttest.Input("Connector name", "sink"),
		prompttest.Input("Topics to consume", "orders"),
		prompttest.Secret("Enter db.password", "pw"),
		prompttest.Select("Enter mode", ""),
		prompttest.Select("Value format", ""),
		prompttest.Confirm("change any field", false),
		prompttest.Confirm("submit this connector", true),
	)

	captureStdout(t, func() {
		require.NoError(t, execute("connector", "create", "--plugin", "com.example.Sink"))
	})

	var posted struct {
		Config map[string]string `json:"config"`
	}
	require.NoError(t, json.Unmarshal(rec.body("POST /connectors"), &posted))
	assert.Equal(t, map[string]string{
		"connector.class": "com.example.Sink",
		"name":            "sink",
		"tasks.max":       "1",
		"topics":          "orders",
		"db.password":     "pw",
		"mode":            "upsert",
	}, posted.Config)
}

func TestExecute_RestoreAsksBeforeOverwriting(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/connectors":
			_, _ = w.Write([]byte(`["alpha", "beta"]`))
		case r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)

	file := filepath.Join(t.TempDir(), "backup.json")
	require.NoError(t, os.WriteFile(file, []byte(`{
		"alpha": {"connector.class": "A"},
		"beta": {"connector.class": "B"},
		"gamma": {"connector.class": "C"}
	}`), 0o600))

	prompttest.New(t,
		prompttest.Confirm("Connector alpha already exists", true),
		prompttest.Confirm("Connector beta already exists", false),
	)

	captureStdout(t, func() { require.NoError(t, execute("connector", "restore", file)) })

	assert.Contains(t, rec.calls, "PUT /connectors/alpha/config")
	assert.NotContains(t, rec.calls, "PUT /connectors/beta/config")
	assert.Contains(t, rec.calls, "PUT /connectors/gamma/config")
}

func TestExecute_RestoreCancelStopsEverything(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		if r.Method == http.MethodGet && r.URL.Path == "/connectors" {
			_, _ = w.Write([]byte(`["alpha"]`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	setTestConfig(t, srv.URL)
	resetFlags(t)

	file := filepath.Join(t.TempDir(), "backup.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"alpha": {"connector.class": "A"}, "beta": {"connector.class": "B"}}`), 0o600))

	prompttest.New(t, prompttest.Cancel("Connector alpha already exists"))

	err := execute("connector", "restore", file)
	require.ErrorIs(t, err, util.ErrCanceled)
	assert.Equal(t, []string{"GET /connectors"}, rec.calls)
}
