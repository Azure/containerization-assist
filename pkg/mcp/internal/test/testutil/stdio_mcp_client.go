package testutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// StdioMCPClient provides MCP protocol testing over stdio transport
// This reuses the proven working approach from schema regression tests
type StdioMCPClient interface {
	ListTools() ([]ToolInfo, error)
	CallTool(name string, args map[string]interface{}) (map[string]interface{}, error)
	Close() error
}

// stdioMCPClient implements StdioMCPClient using the same pattern as schema regression test
type stdioMCPClient struct {
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	cmd     *exec.Cmd
	scanner *bufio.Scanner
	nextID  int
}

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// StartMCPServerForTest starts an MCP server for testing using the proven approach
func StartMCPServerForTest(t *testing.T) (StdioMCPClient, error) {
	tmpDir := t.TempDir()

	serverBinary := filepath.Join(tmpDir, "mcp-server")

	// Find the project root by looking for go.mod
	currentDir, _ := os.Getwd()
	projectRoot := currentDir
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			// Reached filesystem root without finding go.mod
			projectRoot = filepath.Join("..", "..", "..", "..")
			break
		}
		projectRoot = parent
	}

	buildCmd := exec.Command("go", "build", "-o", serverBinary, "./cmd/mcp-server")
	buildCmd.Dir = projectRoot

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build MCP server: %w", err)
	}

	cmd := exec.Command(serverBinary, "--transport=stdio")
	cmd.Env = append(os.Environ(), "LOG_LEVEL=error")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	return &stdioMCPClient{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		cmd:     cmd,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
	}, nil
}

// ListTools retrieves available tools via MCP protocol
func (c *stdioMCPClient) ListTools() ([]ToolInfo, error) {
	// Initialize the server first
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "integration-test-client",
			"version": "1.0.0",
		},
	}

	if err := c.sendRequest("initialize", initParams); err != nil {
		return nil, fmt.Errorf("failed to send initialize request: %w", err)
	}

	initResp, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read initialize response: %w", err)
	}

	if initResp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s (code: %d)", initResp.Error.Message, initResp.Error.Code)
	}

	if err := c.sendNotification("initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	if err := c.sendRequest("tools/list", nil); err != nil {
		return nil, fmt.Errorf("failed to send tools/list request: %w", err)
	}

	// Read responses, skipping any notification errors (id: null)
	var resp *MCPResponse
	for {
		var err error
		resp, err = c.readResponse()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		// Skip notification error responses (id: null)
		if resp.ID == nil {
			continue
		}
		// This is a response to a request (has non-null ID) - use it
		break
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", resp.Result)
	}

	toolsInterface, ok := resultMap["tools"]
	if !ok {
		return nil, fmt.Errorf("no 'tools' field in response")
	}

	toolsArray, ok := toolsInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("tools field is not an array: %T", toolsInterface)
	}

	var tools []ToolInfo
	for _, toolInterface := range toolsArray {
		toolBytes, err := json.Marshal(toolInterface)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool: %w", err)
		}

		var tool ToolInfo
		if err := json.Unmarshal(toolBytes, &tool); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema: %w", err)
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// CallTool calls a tool via MCP protocol
func (c *stdioMCPClient) CallTool(name string, args map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	if err := c.sendRequest("tools/call", params); err != nil {
		return nil, fmt.Errorf("failed to send tools/call request: %w", err)
	}

	resp, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", resp.Result)
	}

	return result, nil
}

// Close shuts down the MCP client and server
func (c *stdioMCPClient) Close() error {
	c.stdin.Close()
	c.stdout.Close()
	c.stderr.Close()

	if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
		c.cmd.Process.Kill()
	}

	return c.cmd.Wait()
}

// sendRequest sends an MCP request
func (c *stdioMCPClient) sendRequest(method string, params interface{}) error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  method,
		Params:  params,
	}
	c.nextID++

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// sendNotification sends an MCP notification
func (c *stdioMCPClient) sendNotification(method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// readResponse reads an MCP response
func (c *stdioMCPClient) readResponse() (*MCPResponse, error) {
	if c.scanner.Scan() {
		var resp MCPResponse
		if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w, data: %s", err, c.scanner.Text())
		}
		return &resp, nil
	}

	if err := c.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}
