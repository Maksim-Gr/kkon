package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListConnectorPlugins lists the connector plugins installed on the cluster
// via GET /connector-plugins.
func (c *Client) ListConnectorPlugins(ctx context.Context) ([]Plugin, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, "/connector-plugins", nil)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to list connector plugins: %s", string(body))
	}

	var plugins []Plugin
	if err := json.Unmarshal(body, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}
