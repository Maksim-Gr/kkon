package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	// Point HOME at a temp dir so GetConfigPath/LoadConfig stay isolated.
	t.Setenv("HOME", t.TempDir())

	path, err := GetConfigPath()
	require.NoError(t, err)

	want := RestAPIConfig{
		KafkaConnect: KafkaConnectConfig{
			URL:      "http://localhost:8083",
			Username: "admin",
			Password: "s3cret",
		},
	}
	require.NoError(t, SaveConfig(want, path))

	got, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadConfigMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := LoadConfig()
	require.Error(t, err)
}

func TestResolveConfigEnvOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(EnvConnectURL, "http://connect:8083")
	t.Setenv(EnvConnectUsername, "ci")
	t.Setenv(EnvConnectPassword, "token")
	t.Setenv(EnvSchemaRegistryURL, "registry:8081")

	got, err := ResolveConfig()
	require.NoError(t, err)
	assert.Equal(t, RestAPIConfig{
		KafkaConnect:   KafkaConnectConfig{URL: "http://connect:8083", Username: "ci", Password: "token"},
		SchemaRegistry: SchemaRegistryConfig{URL: "http://registry:8081"},
	}, got)
}

func TestResolveConfigEnvOverridesFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := GetConfigPath()
	require.NoError(t, err)
	require.NoError(t, SaveConfig(RestAPIConfig{
		KafkaConnect: KafkaConnectConfig{URL: "http://file:8083", Username: "admin", Password: "s3cret"},
	}, path))
	t.Setenv(EnvConnectURL, "env:8083")

	got, err := ResolveConfig()
	require.NoError(t, err)
	assert.Equal(t, "http://env:8083", got.KafkaConnect.URL)
	assert.Equal(t, "admin", got.KafkaConnect.Username)
	assert.Equal(t, "s3cret", got.KafkaConnect.Password)
	assert.Equal(t, []string{EnvConnectURL}, EnvOverrides())
}

func TestResolveConfigMalformedFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := GetConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("kafkaConnect: [not a map"), 0o600))
	t.Setenv(EnvConnectURL, "http://env:8083")

	_, err = ResolveConfig()
	require.Error(t, err)
}
