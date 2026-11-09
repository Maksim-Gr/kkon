package schema

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Maksim-Gr/kkon/internal/schemaregistry"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolatedConfig(t *testing.T, registryURL string) {
	t.Helper()
	key := "HOME"
	if runtime.GOOS == "windows" {
		key = "USERPROFILE"
	}
	t.Setenv(key, t.TempDir())
	t.Setenv(util.EnvConnectURL, "")
	t.Setenv(util.EnvSchemaRegistryURL, registryURL)
	t.Setenv(util.EnvSchemaRegistryUsername, "")
	t.Setenv(util.EnvSchemaRegistryPassword, "")
}

func run(args ...string) (string, error) {
	root := &cobra.Command{Use: "kkon", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("output", "text", "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.AddCommand(NewCommand())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"schema"}, args...))
	err := root.Execute()
	return out.String(), err
}

func schemaFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestSchemaFileWorkflows(t *testing.T) {
	for _, test := range []struct {
		flag, wireType, content string
	}{
		{"avro", "AVRO", `{"type":"record","name":"Order","fields":[]}`},
		{"json", "JSON", `{"type":"object","properties":{"id":{"type":"integer"}}}`},
		{"protobuf", "PROTOBUF", `syntax = "proto3"; message Order { string id = 1; }`},
	} {
		t.Run(test.flag, func(t *testing.T) {
			var published int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`[1]`))
					return
				}
				var input schemaregistry.Schema
				require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
				assert.Equal(t, test.wireType, input.SchemaType)
				assert.Equal(t, test.content, input.Schema)
				assert.Equal(t, []schemaregistry.Reference{{Name: "common", Subject: "common-value", Version: 1}}, input.References)
				if r.URL.Path == "/subjects/orders-value/versions" {
					published++
					_, _ = w.Write([]byte(`{"id":42}`))
				} else {
					assert.Equal(t, "/compatibility/subjects/orders-value/versions", r.URL.Path)
					_, _ = w.Write([]byte(`{"is_compatible":false,"messages":["field id changed"]}`))
				}
			}))
			defer server.Close()
			isolatedConfig(t, server.URL)
			file := schemaFile(t, test.content)
			refs := schemaFile(t, `[{"name":"common","subject":"common-value","version":1}]`)
			flags := []string{"orders-value", "-f", file, "--type", test.flag, "--references", refs, "--output", "json"}
			out, err := run(append([]string{"publish"}, flags...)...)
			require.NoError(t, err)
			assert.JSONEq(t, `{"subject":"orders-value","id":42}`, out)
			assert.Equal(t, 1, published)
			out, err = run(append([]string{"check"}, flags...)...)
			require.ErrorContains(t, err, "incompatible")
			assert.JSONEq(t, `{"is_compatible":false,"messages":["field id changed"]}`, out)
			assert.Equal(t, 1, published, "checking must not register")
			out, err = run(append(append([]string{"publish"}, flags...), "--dry-run")...)
			require.NoError(t, err)
			assert.Contains(t, out, `"dry_run": true`)
			assert.Equal(t, 1, published, "dry-run must not register")
		})
	}
}

func TestSchemaBrowse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subjects":
			_, _ = w.Write([]byte(`["z", "a"]`))
		case "/subjects/orders/versions":
			_, _ = w.Write([]byte(`[2, 1]`))
		case "/subjects/orders/versions/latest", "/subjects/orders/versions/1":
			_, _ = w.Write([]byte(`{"subject":"orders","id":7,"version":1,"schema":"\"string\""}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	isolatedConfig(t, server.URL)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"list"}, "a\nz\n"},
		{[]string{"versions", "orders"}, "1\n2\n"},
		{[]string{"get", "orders"}, "\"string\"\n"},
		{[]string{"get", "orders", "--version", "1"}, "\"string\"\n"},
	} {
		out, err := run(test.args...)
		require.NoError(t, err)
		assert.Equal(t, test.want, out)
	}
	out, err := run("get", "orders", "--output", "json", "--dry-run")
	require.NoError(t, err)
	assert.JSONEq(t, `{"subject":"orders","id":7,"version":1,"schema":"\"string\"","schemaType":"AVRO","references":[]}`, out)
}

func TestSchemaNoninteractiveValidation(t *testing.T) {
	isolatedConfig(t, "http://127.0.0.1:1")
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"get", "--output", "json"}, "subject argument is required"},
		{[]string{"publish", "orders", "--output", "json"}, "--file is required"},
		{[]string{"publish", "", "--output", "json"}, "subject cannot be empty"},
		{[]string{"get", "orders", "--version", "../config"}, "positive integer or latest"},
		{[]string{"list", "--output", "yaml"}, "output must be text or json"},
	} {
		_, err := run(test.args...)
		require.ErrorContains(t, err, test.want)
	}
	t.Setenv(util.EnvSchemaRegistryURL, "")
	_, err := run("list", "--output", "json")
	require.ErrorContains(t, err, "no Schema Registry URL")
}

func TestSchemaDryRunNeedsNoServer(t *testing.T) {
	isolatedConfig(t, "http://127.0.0.1:1")
	out, err := run("publish", "new-subject", "-f", schemaFile(t, `"string"`), "--dry-run", "--output", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"subject":"new-subject","dry_run":true,"schema":"\"string\"","schemaType":"AVRO","references":[]}`, out)
}

func TestInvalidSchemaInputs(t *testing.T) {
	file := schemaFile(t, `"string"`)
	_, err := readSchema(file, "xml", "")
	require.ErrorContains(t, err, "type must be")
	_, err = readSchema(file+"-missing", "avro", "")
	require.ErrorContains(t, err, "read schema file")
	_, err = readSchema(schemaFile(t, " \n"), "avro", "")
	require.ErrorContains(t, err, "cannot be empty")
	for _, refs := range []string{
		`null`, `{}`, `[] []`, `[{"name":"x","subject":"y","version":0}]`,
		`[{"name":"x","subject":"y","version":1,"extra":true}]`,
		`[{"name":"x","subject":"y","version":1},{"name":"x","subject":"z","version":2}]`,
	} {
		_, err := readSchema(file, "avro", schemaFile(t, refs))
		require.Error(t, err, refs)
	}
}
