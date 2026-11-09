package util

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setTestHome isolates the config using the platform's home directory variable.
func setTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	key := "HOME"
	if runtime.GOOS == "windows" {
		key = "USERPROFILE"
	}
	t.Setenv(key, dir)
	return dir
}

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	// Point the home directory at a temp dir so config stays isolated.
	setTestHome(t)

	path, err := GetConfigPath()
	require.NoError(t, err)

	want := RestAPIConfig{
		KafkaConnect: KafkaConnectConfig{
			URL:      "http://localhost:8083",
			Username: "admin",
			Password: "s3cret",
		},
		SchemaRegistry: SchemaRegistryConfig{URL: "https://registry:8081", Username: "key", Password: "secret"},
	}
	require.NoError(t, SaveConfig(want, path))

	got, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadConfigMissingFile(t *testing.T) {
	setTestHome(t)

	_, err := LoadConfig()
	require.Error(t, err)
}

func TestResolveConfigEnvOnly(t *testing.T) {
	setTestHome(t)
	t.Setenv(EnvConnectURL, "http://connect:8083")
	t.Setenv(EnvConnectUsername, "ci")
	t.Setenv(EnvConnectPassword, "token")
	t.Setenv(EnvSchemaRegistryURL, "registry:8081")
	t.Setenv(EnvSchemaRegistryUsername, "schema-key")
	t.Setenv(EnvSchemaRegistryPassword, "schema-secret")

	got, err := ResolveConfig()
	require.NoError(t, err)
	assert.Equal(t, RestAPIConfig{
		KafkaConnect:   KafkaConnectConfig{URL: "http://connect:8083", Username: "ci", Password: "token"},
		SchemaRegistry: SchemaRegistryConfig{URL: "http://registry:8081", Username: "schema-key", Password: "schema-secret"},
	}, got)
}

func TestResolveConfigEnvOverridesFile(t *testing.T) {
	setTestHome(t)
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

func TestResolveRegistryCredentialsIndependently(t *testing.T) {
	setTestHome(t)
	path, err := GetConfigPath()
	require.NoError(t, err)
	require.NoError(t, SaveConfig(RestAPIConfig{
		KafkaConnect:   KafkaConnectConfig{URL: "http://connect:8083", Username: "connect", Password: "connect-secret"},
		SchemaRegistry: SchemaRegistryConfig{URL: "https://registry:8081", Username: "old", Password: "old-secret"},
	}, path))
	t.Setenv(EnvSchemaRegistryUsername, "key")
	t.Setenv(EnvSchemaRegistryPassword, "secret")
	got, err := ResolveConfig()
	require.NoError(t, err)
	assert.Equal(t, SchemaRegistryConfig{URL: "https://registry:8081", Username: "key", Password: "secret"}, got.SchemaRegistry)
	assert.Equal(t, "connect-secret", got.KafkaConnect.Password)
	assert.Equal(t, []string{EnvSchemaRegistryUsername, EnvSchemaRegistryPassword}, EnvOverrides())
	t.Setenv(EnvSchemaRegistryPassword, "")
	got, err = ResolveConfig()
	require.NoError(t, err)
	assert.Equal(t, "old-secret", got.SchemaRegistry.Password, "empty env values retain file settings")
}

func TestResolveConfigMalformedFile(t *testing.T) {
	setTestHome(t)
	path, err := GetConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("kafkaConnect: [not a map"), 0o600))
	t.Setenv(EnvConnectURL, "http://env:8083")

	_, err = ResolveConfig()
	require.Error(t, err)
}

func TestKeysFromMapSorted(t *testing.T) {
	m := map[string]string{"topics": "", "connector.class": "", "name": "", "tasks.max": ""}
	for range 20 {
		assert.Equal(t, []string{"connector.class", "name", "tasks.max", "topics"}, KeysFromMap(m))
	}
}

func TestSaveConfigReplacesExistingFilePrivately(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("old config"), 0o644)) //nolint:gosec // Exercise replacement of an insecure existing file.
	require.NoError(t, SaveConfig(RestAPIConfig{KafkaConnect: KafkaConnectConfig{Password: "example"}}, path))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
