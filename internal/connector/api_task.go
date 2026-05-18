package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TaskRef represents an entry returned by GET /connectors/{name}/tasks.
type TaskRef struct {
	Connector string `json:"connector"`
	Task      int    `json:"task"`
}

// TaskStatus represents the response from GET /connectors/{name}/tasks/{id}/status.
type TaskStatus struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
	Trace    string `json:"trace,omitempty"`
}

// ListConnectorTasks lists tasks for a connector.
// GET /connectors/{name}/tasks returns [{id:{connector,task}, config:{}}],
// so we unwrap the nested id before returning flat TaskRefs.
func (c *Client) ListConnectorTasks(ctx context.Context, connectorName string) ([]TaskRef, error) {
	path := fmt.Sprintf("/connectors/%s/tasks", connectorName)

	body, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if !isSuccess(status) {
		return nil, fmt.Errorf("failed to list tasks for %s: %s", connectorName, string(body))
	}

	var raw []struct {
		ID TaskRef `json:"id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	tasks := make([]TaskRef, len(raw))
	for i, r := range raw {
		tasks[i] = r.ID
	}
	return tasks, nil
}

// GetConnectorTaskStatus fetches the status of a single task.
func (c *Client) GetConnectorTaskStatus(ctx context.Context, connectorName string, taskID int) (TaskStatus, error) {
	path := fmt.Sprintf("/connectors/%s/tasks/%d/status", connectorName, taskID)

	body, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return TaskStatus{}, err
	}
	if !isSuccess(status) {
		return TaskStatus{}, fmt.Errorf("failed to get task status for %s task %d: %s", connectorName, taskID, string(body))
	}

	var ts TaskStatus
	if err := json.Unmarshal(body, &ts); err != nil {
		return TaskStatus{}, err
	}
	return ts, nil
}

// RestartConnectorTask restarts a single task.
func (c *Client) RestartConnectorTask(ctx context.Context, connectorName string, taskID int) error {
	path := fmt.Sprintf("/connectors/%s/tasks/%d/restart", connectorName, taskID)

	body, status, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return fmt.Errorf("failed to restart %s task %d: %s", connectorName, taskID, string(body))
	}
	return nil
}
