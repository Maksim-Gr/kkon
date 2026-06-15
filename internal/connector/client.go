package connector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the Kafka Connect REST API.
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewClient creates a Kafka Connect client for the given base URL.
func NewClient(kafkaConnectURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(kafkaConnectURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBasicAuth configures HTTP Basic Auth credentials for all requests.
func (c *Client) SetBasicAuth(username, password string) {
	c.username = username
	c.password = password
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("could not reach Kafka Connect at %s (is it running? verify the URL/credentials with `kkon config show`): %w", c.baseURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}
