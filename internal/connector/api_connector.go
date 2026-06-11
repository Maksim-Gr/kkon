package connector

import (
	"context"
	"fmt"
	"net/http"
)

// PauseConnector pauses the named connector and its tasks.
func (c *Client) PauseConnector(ctx context.Context, name string) error {
	body, status, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/connectors/%s/pause", name), nil)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to pause connector %s: %s", name, string(body))
	}
	return nil
}

// ResumeConnector resumes the named connector and its tasks.
func (c *Client) ResumeConnector(ctx context.Context, name string) error {
	body, status, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/connectors/%s/resume", name), nil)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to resume connector %s: %s", name, string(body))
	}
	return nil
}

// RestartConnector restarts the named connector. includeTasks also restarts the
// connector's tasks; onlyFailed restricts the restart to FAILED instances.
func (c *Client) RestartConnector(ctx context.Context, name string, includeTasks, onlyFailed bool) error {
	path := fmt.Sprintf("/connectors/%s/restart?includeTasks=%t&onlyFailed=%t", name, includeTasks, onlyFailed)
	body, status, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to restart connector %s: %s", name, string(body))
	}
	return nil
}
