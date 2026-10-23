// Package connector provides an HTTP client for the Kafka Connect REST API.
package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// ListConnectors returns the names of all registered connectors.
func (c *Client) ListConnectors(ctx context.Context) ([]string, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, "/connectors", nil)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to list connectors: %s", errorMessage(body))
	}
	var connectors []string
	if err := json.Unmarshal(body, &connectors); err != nil {
		return nil, err
	}
	return connectors, nil
}

// ListConnectorStatuses returns status information for all connectors.
func (c *Client) ListConnectorStatuses(ctx context.Context) (ConnectorsStatusResponse, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, "/connectors?expand=status", nil)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to list connector statuses: %s", errorMessage(body))
	}
	// The expand=status response wraps each entry: { "name": { "info": {}, "status": {...} } }
	var expanded map[string]ExpandedEntry
	if err := json.Unmarshal(body, &expanded); err != nil {
		return nil, err
	}
	result := make(ConnectorsStatusResponse, len(expanded))
	for name, entry := range expanded {
		result[name] = entry.Status
	}
	return result, nil
}

// ListConnectorsExpanded returns status and config info for all connectors in one call.
func (c *Client) ListConnectorsExpanded(ctx context.Context) (map[string]ExpandedEntry, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, "/connectors?expand=status&expand=info", nil)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to list connectors: %s", errorMessage(body))
	}
	var result map[string]ExpandedEntry
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConnectorStatus returns the status of a single connector.
func (c *Client) GetConnectorStatus(ctx context.Context, name string) (Status, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/connectors/%s/status", url.PathEscape(name)), nil)
	if err != nil {
		return Status{}, err
	}
	if !isSuccess(status) {
		return Status{}, fmt.Errorf("failed to get status for %s: %s", name, errorMessage(body))
	}
	var s Status
	if err := json.Unmarshal(body, &s); err != nil {
		return Status{}, err
	}
	return s, nil
}

// DeleteConnector removes the named connector from Kafka Connect.
func (c *Client) DeleteConnector(ctx context.Context, name string) error {
	body, status, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/connectors/%s", url.PathEscape(name)), nil)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		return fmt.Errorf("failed to delete connector %s: a rebalance is in process", name)
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to delete connector %s: %s", name, errorMessage(body))
	}
	return nil
}

// CreateConnector creates a new connector from its config. The connector name
// is taken from cfg["name"], which must be set.
func (c *Client) CreateConnector(ctx context.Context, cfg map[string]string) (ConnectorInfo, error) {
	name := cfg["name"]
	if name == "" {
		return ConnectorInfo{}, errors.New(`connector config has no "name"`)
	}
	// POST /connectors expects {"name": ..., "config": {...}}.
	b, err := json.Marshal(map[string]any{"name": name, "config": cfg})
	if err != nil {
		return ConnectorInfo{}, err
	}
	body, status, err := c.doRequest(ctx, http.MethodPost, "/connectors", b)
	if err != nil {
		return ConnectorInfo{}, err
	}
	if !isSuccess(status) {
		return ConnectorInfo{}, fmt.Errorf("failed to submit connector configuration: %s", errorMessage(body))
	}
	var info ConnectorInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return ConnectorInfo{}, err
	}
	return info, nil
}

// UpdateConnectorConfig applies a new config map to the named connector.
func (c *Client) UpdateConnectorConfig(ctx context.Context, name string, cfg map[string]string) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	body, status, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/connectors/%s/config", url.PathEscape(name)), b)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to update connector %s: %s", name, errorMessage(body))
	}
	return nil
}

// GetConnectorConfig returns the config for the named connector as a map.
func (c *Client) GetConnectorConfig(ctx context.Context, name string) (map[string]string, error) {
	body, status, err := c.doRequest(
		ctx,
		http.MethodGet,
		"/connectors/"+url.PathEscape(name)+"/config",
		nil,
	)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to get config for %s: %s", name, errorMessage(body))
	}

	var cfg map[string]string
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ConfigDefinition describes a config key as reported by the validate
// endpoint. DefaultValue is any because plugins report typed defaults
// (string, number, boolean, or null).
type ConfigDefinition struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Required      bool   `json:"required"`
	DefaultValue  any    `json:"default_value"`
	Importance    string `json:"importance"`
	Documentation string `json:"documentation"`
	Group         string `json:"group"`
}

// ConfigValue holds the submitted value, server-recommended values, and
// per-field validation errors for a config key.
type ConfigValue struct {
	Name              string   `json:"name"`
	Value             any      `json:"value"`
	RecommendedValues []any    `json:"recommended_values"`
	Errors            []string `json:"errors"`
}

// ConfigEntry pairs a config key's definition with its validated value.
type ConfigEntry struct {
	Definition ConfigDefinition `json:"definition"`
	Value      ConfigValue      `json:"value"`
}

// ConfigValidationResponse is the connector config validation result.
type ConfigValidationResponse struct {
	Name       string        `json:"name"`
	ErrorCount int           `json:"error_count"`
	Configs    []ConfigEntry `json:"configs"`
}

// ValidateConnectorConfig validates a connector config against its plugin and
// returns per-field errors. connectorClass must match the config's connector.class.
func (c *Client) ValidateConnectorConfig(ctx context.Context, connectorClass string, cfg map[string]string) (ConfigValidationResponse, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return ConfigValidationResponse{}, err
	}
	path := fmt.Sprintf("/connector-plugins/%s/config/validate", url.PathEscape(connectorClass))
	body, status, err := c.doRequest(ctx, http.MethodPut, path, b)
	if err != nil {
		return ConfigValidationResponse{}, err
	}
	if !isSuccess(status) {
		return ConfigValidationResponse{}, fmt.Errorf("failed to validate config: %s", errorMessage(body))
	}
	var v ConfigValidationResponse
	if err := json.Unmarshal(body, &v); err != nil {
		return ConfigValidationResponse{}, err
	}
	return v, nil
}

// BackupConnectorConfig writes connector configs to a timestamped JSON file in outputDir.
func BackupConnectorConfig(
	ctx context.Context,
	client *Client,
	connectors []string,
	outputDir string,
) (string, error) {
	dumpConfig := make(map[string]map[string]string)

	for _, name := range connectors {
		cfg, err := client.GetConnectorConfig(ctx, name)
		if err != nil {
			return "", err
		}
		dumpConfig[name] = cfg
	}

	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	outputFile := filepath.Join(outputDir, fmt.Sprintf("config_%s.json", timestamp))

	// Backups can contain connector secrets, so restrict to owner read/write.
	file, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close() //nolint:errcheck

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(dumpConfig); err != nil {
		return "", fmt.Errorf("failed to encode config: %w", err)
	}

	return outputFile, nil
}
