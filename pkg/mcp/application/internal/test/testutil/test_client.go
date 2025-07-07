package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MCPTestClient provides utilities for testing real MCP protocol interactions
// This is NOT a mock - it implements real MCP client functionality for tests
type MCPTestClient interface {
	// Core MCP operations
	ListTools(ctx context.Context) ([]ToolInfo, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error)
	GetHealth(ctx context.Context) (*HealthStatus, error)
	Ping(ctx context.Context) error

	// Session management helpers
	GetSessionWorkspace(sessionID string) (string, error)
	InspectSessionState(sessionID string) (*SessionState, error)

	// Test utilities
	ValidateToolResponse(response map[string]interface{}, expectedFields []string) error
	ExtractSessionID(response map[string]interface{}) (string, error)

	// Cleanup
	Close() error
}

// ToolInfo represents MCP tool metadata
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`  // Legacy field name
	InputSchema map[string]interface{} `json:"inputSchema"` // MCP spec field name
}

// HealthStatus represents server health information
type HealthStatus struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// SessionState represents session state for inspection
type SessionState struct {
	ID           string                 `json:"session_id"`
	WorkspaceDir string                 `json:"workspace_dir"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"last_accessed"`
	Status       string                 `json:"status"`
	RepoURL      string                 `json:"repo_url,omitempty"`
	Labels       []string               `json:"labels,omitempty"`
}

// httpMCPTestClient implements MCPTestClient using HTTP transport
type httpMCPTestClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// NewMCPTestClient creates a new real MCP test client
func NewMCPTestClient(serverURL string) (MCPTestClient, error) {
	client := &httpMCPTestClient{
		baseURL: strings.TrimSuffix(serverURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		timeout: 30 * time.Second,
	}

	return client, nil
}

// ListTools retrieves available tools via MCP protocol
func (c *httpMCPTestClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/tools", nil)
	if err != nil {
		return nil, errors.NewError().Message("creating request").Cause(err).Build()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewError().Message("making request").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewError().Messagef("unexpected status %d: %s", resp.StatusCode, string(body)).WithLocation(

		// The HTTP transport returns tools in a wrapper object
		).Build()
	}

	var response struct {
		Tools []ToolInfo `json:"tools"`
		Count int        `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, errors.NewError().Message("decoding response").Cause(err).Build()
	}

	return response.Tools, nil
}

// CallTool executes a tool via MCP protocol
func (c *httpMCPTestClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	// The HTTP transport expects the args directly, not wrapped
	body, err := json.Marshal(args)
	if err != nil {
		return nil, errors.NewError().Message("marshaling args").Cause(err).WithLocation(

		// Use the correct URL pattern for the HTTP transport
		).Build()
	}

	url := fmt.Sprintf("%s/api/v1/tools/%s", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, errors.NewError().Message("creating request").Cause(err).Build()
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewError().Message("making request").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewError().Messagef("tool call failed with status %d: %s", resp.StatusCode, string(body)).Build()
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.NewError().Message("decoding response").Cause(err).Build()
	}

	return result, nil
}

// GetHealth retrieves server health status
func (c *httpMCPTestClient) GetHealth(ctx context.Context) (*HealthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/health", nil)
	if err != nil {
		return nil, errors.NewError().Message("creating request").Cause(err).Build()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewError().Message("making request").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewError().Messagef("health check failed with status %d", resp.StatusCode).Build()
	}

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, errors.NewError().Message("decoding health response").Cause(err).Build()
	}

	return &health, nil
}

// Ping tests basic connectivity to the MCP server
func (c *httpMCPTestClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/status", nil)
	if err != nil {
		return errors.NewError().Message("creating ping request").Cause(err).Build()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.NewError().Message("ping request failed").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.NewError().Messagef("ping failed with status %d", resp.StatusCode).Build(

		// GetSessionWorkspace retrieves the workspace directory for a session
		)
	}

	return nil
}

func (c *httpMCPTestClient) GetSessionWorkspace(sessionID string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/%s", c.baseURL, sessionID), nil)
	if err != nil {
		return "", errors.NewError().Message("creating request").Cause(err).Build()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.NewError().Message("making request").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.NewError().Messagef("workspace request failed with status %d", resp.StatusCode).Build()
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", errors.NewError().Message("decoding response").Cause(err).Build()
	}

	workspace, ok := result["workspace_dir"].(string)
	if !ok {
		return "", errors.NewError().Messagef("workspace_dir not found in response").Build()
	}

	return workspace, nil
}

// InspectSessionState retrieves detailed session state for validation
func (c *httpMCPTestClient) InspectSessionState(sessionID string) (*SessionState, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/%s", c.baseURL, sessionID), nil)
	if err != nil {
		return nil, errors.NewError().Message("creating request").Cause(err).Build()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewError().Message("making request").Cause(err).Build()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewError().Messagef("session inspection failed with status %d", resp.StatusCode).Build()
	}

	var session SessionState
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, errors.NewError().Message("decoding session state").Cause(err).Build()
	}

	return &session, nil
}

// ValidateToolResponse checks that a tool response contains expected fields
func (c *httpMCPTestClient) ValidateToolResponse(response map[string]interface{}, expectedFields []string) error {
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			return errors.NewError().Messagef("expected field '%s' not found in response", field).Build()
		}
	}
	return nil
}

// ExtractSessionID safely extracts session_id from tool response
func (c *httpMCPTestClient) ExtractSessionID(response map[string]interface{}) (string, error) {
	sessionID, exists := response["session_id"]
	if !exists {
		return "", errors.NewError().Messagef("session_id not found in response").Build()
	}

	sessionIDStr, ok := sessionID.(string)
	if !ok {
		return "", errors.NewError().Messagef("session_id is not a string: %T", sessionID).Build()
	}

	if sessionIDStr == "" {
		return "", errors.NewError().Messagef("session_id is empty").Build()
	}

	return sessionIDStr, nil
}

// Close cleans up the test client
func (c *httpMCPTestClient) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}
