package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
	Parameters  map[string]interface{} `json:"parameters"`
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
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// The HTTP transport returns tools in a wrapper object
	var response struct {
		Tools []ToolInfo `json:"tools"`
		Count int        `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return response.Tools, nil
}

// CallTool executes a tool via MCP protocol
func (c *httpMCPTestClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	// The HTTP transport expects the args directly, not wrapped
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshaling args: %w", err)
	}

	// Use the correct URL pattern for the HTTP transport
	url := fmt.Sprintf("%s/api/v1/tools/%s", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tool call failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result, nil
}

// GetHealth retrieves server health status
func (c *httpMCPTestClient) GetHealth(ctx context.Context) (*HealthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/health", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("decoding health response: %w", err)
	}

	return &health, nil
}

// Ping tests basic connectivity to the MCP server
func (c *httpMCPTestClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/status", nil)
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetSessionWorkspace retrieves the workspace directory for a session
func (c *httpMCPTestClient) GetSessionWorkspace(sessionID string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/%s", c.baseURL, sessionID), nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("workspace request failed with status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	workspace, ok := result["workspace_dir"].(string)
	if !ok {
		return "", fmt.Errorf("workspace_dir not found in response")
	}

	return workspace, nil
}

// InspectSessionState retrieves detailed session state for validation
func (c *httpMCPTestClient) InspectSessionState(sessionID string) (*SessionState, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/sessions/%s", c.baseURL, sessionID), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session inspection failed with status %d", resp.StatusCode)
	}

	var session SessionState
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decoding session state: %w", err)
	}

	return &session, nil
}

// ValidateToolResponse checks that a tool response contains expected fields
func (c *httpMCPTestClient) ValidateToolResponse(response map[string]interface{}, expectedFields []string) error {
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			return fmt.Errorf("expected field '%s' not found in response", field)
		}
	}
	return nil
}

// ExtractSessionID safely extracts session_id from tool response
func (c *httpMCPTestClient) ExtractSessionID(response map[string]interface{}) (string, error) {
	sessionID, exists := response["session_id"]
	if !exists {
		return "", fmt.Errorf("session_id not found in response")
	}

	sessionIDStr, ok := sessionID.(string)
	if !ok {
		return "", fmt.Errorf("session_id is not a string: %T", sessionID)
	}

	if sessionIDStr == "" {
		return "", fmt.Errorf("session_id is empty")
	}

	return sessionIDStr, nil
}

// Close cleans up the test client
func (c *httpMCPTestClient) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}
