package util

import (
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
