// Package schemaregistry implements the Confluent Schema Registry workflows.
package schemaregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Reference identifies a previously registered schema dependency.
type Reference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

// Schema is the input shared by registration and compatibility checks.
type Schema struct {
	Schema     string      `json:"schema"`
	SchemaType string      `json:"schemaType"`
	References []Reference `json:"references"`
}

// Version is a registered schema and its registry identifiers.
type Version struct {
	Subject string `json:"subject"`
	ID      int    `json:"id"`
	Version int    `json:"version"`
	Schema
}

// Compatibility reports the registry's decision and any explanations.
type Compatibility struct {
	IsCompatible bool     `json:"is_compatible"`
	Messages     []string `json:"messages"`
}

// Registration identifies the result of publishing a schema.
type Registration struct {
	ID int `json:"id"`
}

// APIError preserves the registry's HTTP status, error code and message.
type APIError struct {
	Status  int    `json:"-"`
	Code    int    `json:"error_code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Schema Registry returned HTTP %d (error %d): %s", e.Status, e.Code, e.Message)
}

// Client talks directly to Schema Registry independently of Kafka Connect.
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewClient creates a client using standard TLS verification and optional Basic auth.
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), username: username, password: password,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) request(ctx context.Context, method, path string, input, output any) error {
	var body bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&body).Encode(input); err != nil {
			return fmt.Errorf("encode schema request: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &body)
	if err != nil {
		return fmt.Errorf("create Schema Registry request: %w", err)
	}
	const contentType = "application/vnd.schemaregistry.v1+json"
	req.Header.Set("Accept", contentType)
	if input != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Schema Registry (verify URL and credentials with `kkon config show`): %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if err != nil {
			return fmt.Errorf("read Schema Registry error: %w", err)
		}
		apiErr := &APIError{Status: resp.StatusCode}
		if json.Unmarshal(data, apiErr) != nil || apiErr.Message == "" {
			apiErr.Message = strings.TrimSpace(string(data))
		}
		return apiErr
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode Schema Registry response: %w", err)
	}
	return nil
}

func subjectPath(subject string) string {
	return "/subjects/" + url.PathEscape(subject)
}

// List returns the active subjects in sorted order.
func (c *Client) List(ctx context.Context) ([]string, error) {
	subjects := []string{}
	if err := c.request(ctx, http.MethodGet, "/subjects", nil, &subjects); err != nil {
		return nil, err
	}
	if subjects == nil {
		subjects = []string{}
	}
	sort.Strings(subjects)
	return subjects, nil
}

// Versions returns a subject's active versions in ascending order.
func (c *Client) Versions(ctx context.Context, subject string) ([]int, error) {
	versions := []int{}
	if err := c.request(ctx, http.MethodGet, subjectPath(subject)+"/versions", nil, &versions); err != nil {
		return nil, err
	}
	if versions == nil {
		versions = []int{}
	}
	sort.Ints(versions)
	return versions, nil
}

// Get retrieves schema content and metadata for a positive version or "latest".
func (c *Client) Get(ctx context.Context, subject, version string) (Version, error) {
	var result Version
	if version != "latest" {
		n, err := strconv.ParseInt(version, 10, 32)
		if err != nil || n < 1 {
			return result, errors.New("version must be a positive integer or latest")
		}
	}
	err := c.request(ctx, http.MethodGet, subjectPath(subject)+"/versions/"+url.PathEscape(version), nil, &result)
	// The registry omits schemaType for Avro and references when there are none.
	if result.SchemaType == "" {
		result.SchemaType = "AVRO"
	}
	if result.References == nil {
		result.References = []Reference{}
	}
	return result, err
}

// Check uses the subject's effective policy, including transitive checks.
// Existence is checked explicitly because some registries accept a check for a new subject.
func (c *Client) Check(ctx context.Context, subject string, schema Schema) (Compatibility, error) {
	result := Compatibility{Messages: []string{}}
	versions, err := c.Versions(ctx, subject)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == 40401 {
			return result, fmt.Errorf("compatibility cannot be evaluated: subject %q does not exist; publish can create it: %w", subject, err)
		}
		return result, err
	}
	if len(versions) == 0 {
		return result, fmt.Errorf("compatibility cannot be evaluated: subject %q has no registered versions", subject)
	}
	var response struct {
		IsCompatible *bool    `json:"is_compatible"`
		Messages     []string `json:"messages"`
	}
	err = c.request(ctx, http.MethodPost, "/compatibility"+subjectPath(subject)+"/versions?verbose=true", schema, &response)
	if err != nil {
		return result, err
	}
	if response.IsCompatible == nil {
		return result, errors.New("Schema Registry response is missing is_compatible")
	}
	result.IsCompatible = *response.IsCompatible
	if response.Messages != nil {
		result.Messages = response.Messages
	}
	return result, nil
}

// Publish registers a schema; the registry enforces its current compatibility policy.
func (c *Client) Publish(ctx context.Context, subject string, schema Schema) (Registration, error) {
	var result Registration
	err := c.request(ctx, http.MethodPost, subjectPath(subject)+"/versions", schema, &result)
	if err == nil && result.ID < 1 {
		err = errors.New("Schema Registry response is missing a valid schema ID")
	}
	return result, err
}
