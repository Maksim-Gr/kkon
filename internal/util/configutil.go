// Package util provides shared configuration loading and helper utilities.
package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"gopkg.in/yaml.v3"
)

// KeysFromMap extracts and returns a slice of keys from the given map.
func KeysFromMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// ToPrettyJSON marshals v to an indented JSON string.
func ToPrettyJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetConfigPath returns the path to the kkon config file.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kkon", "config.yaml"), nil
}

// ValidateURL returns an error if input is not a valid http/https URL.
func ValidateURL(input string) error {
	if input == "" {
		return errors.New("URL cannot be empty")
	}

	// Reject explicit non-http/https schemes (e.g. ftp://)
	if strings.Contains(input, "://") {
		if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
			return errors.New("URL scheme must be http or https")
		}
	}

	testURL := input
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		testURL = "http://" + input
	}

	parsed, err := url.ParseRequestURI(testURL)
	if err != nil {
		return errors.New("invalid URL format")
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return errors.New("URL must contain a host (e.g. localhost:8083 or example.com)")
	}

	// Reject ambiguous single-character bare hostnames (e.g. "d")
	// while still allowing service names like "kafkaconnect:8083"
	if len(hostname) <= 1 && !strings.Contains(hostname, ".") && hostname != "localhost" {
		return fmt.Errorf("invalid host: %q", hostname)
	}

	return nil
}

// SaveConfig saves config to file.
func SaveConfig(cfg RestAPIConfig, configPath string) error {
	// Create a directory if not exists
	err := os.MkdirAll(filepath.Dir(configPath), 0o700)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0o600)
}

// LoadConfig reads and returns the config from the default config path.
func LoadConfig() (RestAPIConfig, error) {
	var cfg RestAPIConfig
	configPath, err := GetConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}

// Environment variables that override config file values.
const (
	EnvConnectURL        = "KKON_CONNECT_URL"
	EnvConnectUsername   = "KKON_CONNECT_USERNAME"
	EnvConnectPassword   = "KKON_CONNECT_PASSWORD" //nolint:gosec // env var name, not a credential
	EnvSchemaRegistryURL = "KKON_SCHEMA_REGISTRY_URL"
)

// ResolveConfig returns the config file values overridden by any non-empty
// KKON_* environment variables. A missing config file is not an error.
func ResolveConfig() (RestAPIConfig, error) {
	cfg, err := LoadConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}

	if v := os.Getenv(EnvConnectURL); v != "" {
		cfg.KafkaConnect.URL = withDefaultScheme(v)
	}
	if v := os.Getenv(EnvConnectUsername); v != "" {
		cfg.KafkaConnect.Username = v
	}
	if v := os.Getenv(EnvConnectPassword); v != "" {
		cfg.KafkaConnect.Password = v
	}
	if v := os.Getenv(EnvSchemaRegistryURL); v != "" {
		cfg.SchemaRegistry.URL = withDefaultScheme(v)
	}
	return cfg, nil
}

// EnvOverrides returns the names of the KKON_* environment variables that are set.
func EnvOverrides() []string {
	var set []string
	for _, name := range []string{EnvConnectURL, EnvConnectUsername, EnvConnectPassword, EnvSchemaRegistryURL} {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	return set
}

func withDefaultScheme(u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return "http://" + u
}

// IsSurveyInterrupt reports whether err is a survey terminal interrupt (Ctrl+C).
func IsSurveyInterrupt(err error) bool {
	return errors.Is(err, terminal.InterruptErr)
}
