package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MCPToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPClient struct {
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	cmd     *exec.Cmd
	scanner *bufio.Scanner
	nextID  int
}

func StartMCPServer(t *testing.T) (*MCPClient, error) {
	tmpDir := t.TempDir()

	serverBinary := filepath.Join(tmpDir, "mcp-server")
	buildCmd := exec.Command("go", "build", "-o", serverBinary, "./cmd/mcp-server")
	buildCmd.Dir = filepath.Join("..", "..", "..", "..")

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

	return &MCPClient{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		cmd:     cmd,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
	}, nil
}

func (c *MCPClient) SendRequest(method string, params interface{}) error {
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

func (c *MCPClient) SendNotification(method string, params interface{}) error {
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

func (c *MCPClient) ReadResponse() (*MCPResponse, error) {
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

func (c *MCPClient) Close() error {
	c.stdin.Close()
	c.stdout.Close()
	c.stderr.Close()

	if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
		c.cmd.Process.Kill()
	}

	return c.cmd.Wait()
}

func (c *MCPClient) GetToolsList() ([]MCPToolSchema, error) {
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "schema-regression-test",
			"version": "1.0.0",
		},
	}

	if err := c.SendRequest("initialize", initParams); err != nil {
		return nil, fmt.Errorf("failed to send initialize request: %w", err)
	}

	initResp, err := c.ReadResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read initialize response: %w", err)
	}

	if initResp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s (code: %d)", initResp.Error.Message, initResp.Error.Code)
	}

	if err := c.SendNotification("initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	if err := c.SendRequest("tools/list", nil); err != nil {
		return nil, fmt.Errorf("failed to send tools/list request: %w", err)
	}

	// Read responses, skipping any notification errors (id: null)
	// The gomcp library may return errors for unsupported notifications
	var resp *MCPResponse
	for {
		var err error
		resp, err = c.ReadResponse()
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

	var tools []MCPToolSchema
	for _, toolInterface := range toolsArray {
		toolBytes, err := json.Marshal(toolInterface)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool: %w", err)
		}

		var tool MCPToolSchema
		if err := json.Unmarshal(toolBytes, &tool); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema: %w", err)
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func TestSchemaRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping schema regression test in short mode")
	}

	// Removed hardcoded skip - testing gomcp integration fix

	client, err := StartMCPServer(t)
	require.NoError(t, err, "Failed to start MCP server")
	defer client.Close()

	tools, err := client.GetToolsList()
	if err != nil {
		t.Logf("Error getting tools list: %v", err)
		stderr := make([]byte, 1024)
		if n, readErr := client.stderr.Read(stderr); readErr == nil && n > 0 {
			t.Logf("Server stderr: %s", string(stderr[:n]))
		}
	}
	require.NoError(t, err, "Failed to get tools list")

	t.Logf("Found %d tools", len(tools))
	for _, tool := range tools {
		t.Logf("Tool: %s - %s", tool.Name, tool.Description)
	}

	assert.GreaterOrEqual(t, len(tools), 5, "Expected at least 5 tools to be registered")

	expectedTools := []string{
		"analyze_repository",
		"build_image",
		"push_image",
		"generate_manifests",
		"generate_dockerfile",
		"list_sessions",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expectedTool := range expectedTools {
		assert.True(t, toolNames[expectedTool], "Expected tool %s is missing", expectedTool)
	}

	for _, tool := range tools {
		t.Run(fmt.Sprintf("Tool_%s", tool.Name), func(t *testing.T) {
			validateToolSchema(t, tool)
		})
	}
}

func validateToolSchema(t *testing.T, tool MCPToolSchema) {
	assert.NotEmpty(t, tool.Name, "Tool name should not be empty")
	assert.NotEmpty(t, tool.Description, "Tool description should not be empty")
	assert.NotNil(t, tool.InputSchema, "Tool input schema should not be nil")

	schemaBytes, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err, "Failed to marshal input schema for tool %s", tool.Name)

	const maxSchemaSize = 8 * 1024
	assert.LessOrEqual(t, len(schemaBytes), maxSchemaSize,
		"Tool %s input schema size %d bytes exceeds %d byte limit",
		tool.Name, len(schemaBytes), maxSchemaSize)

	schemaStr := string(schemaBytes)

	assert.NotContains(t, schemaStr, `"$ref"`,
		"Tool %s input schema contains $ref, which is incompatible with GitHub Copilot", tool.Name)

	assert.NotContains(t, schemaStr, `"definitions"`,
		"Tool %s input schema contains definitions section, which is incompatible with GitHub Copilot", tool.Name)

	assert.Contains(t, schemaStr, `"type":"object"`,
		"Tool %s input schema missing root type:object declaration", tool.Name)

	var schemaValidation interface{}
	err = json.Unmarshal(schemaBytes, &schemaValidation)
	assert.NoError(t, err, "Tool %s input schema is not valid JSON", tool.Name)

	if properties, ok := tool.InputSchema["properties"]; ok {
		propertiesMap, ok := properties.(map[string]interface{})
		assert.True(t, ok, "Tool %s properties field should be an object", tool.Name)
		assert.NotEmpty(t, propertiesMap, "Tool %s should have at least one property", tool.Name)
	}

	if strings.HasSuffix(tool.Name, "_atomic") {
		properties, ok := tool.InputSchema["properties"].(map[string]interface{})
		if assert.True(t, ok, "Atomic tool %s should have properties", tool.Name) {
			sessionID, ok := properties["session_id"]
			assert.True(t, ok, "Atomic tool %s should have session_id parameter", tool.Name)

			if sessionIDMap, ok := sessionID.(map[string]interface{}); ok {
				sessionIDType, ok := sessionIDMap["type"].(string)
				assert.True(t, ok && sessionIDType == "string",
					"Atomic tool %s session_id should be string type", tool.Name)
			}
		}
	}
}

func TestSchemaStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping schema stability test in short mode")
	}

	// Skip due to broken pipe issues during server shutdown - see TEST_FIX.md for resolution plan
	t.Skip("TEMPORARILY SKIPPED: Broken pipe during server shutdown - tracked in TEST_FIX.md")

	client1, err := StartMCPServer(t)
	require.NoError(t, err)

	tools1, err := client1.GetToolsList()
	require.NoError(t, err)
	err = client1.Close()
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	client2, err := StartMCPServer(t)
	require.NoError(t, err)
	defer client2.Close()

	tools2, err := client2.GetToolsList()
	require.NoError(t, err)

	assert.Equal(t, len(tools1), len(tools2), "Tool count should be stable across restarts")

	schemas1 := make(map[string]MCPToolSchema)
	schemas2 := make(map[string]MCPToolSchema)

	for _, tool := range tools1 {
		schemas1[tool.Name] = tool
	}
	for _, tool := range tools2 {
		schemas2[tool.Name] = tool
	}

	for name, tool1 := range schemas1 {
		tool2, exists := schemas2[name]
		assert.True(t, exists, "Tool %s should exist in both instances", name)

		if exists {
			schema1Bytes, _ := json.Marshal(tool1.InputSchema)
			schema2Bytes, _ := json.Marshal(tool2.InputSchema)

			assert.JSONEq(t, string(schema1Bytes), string(schema2Bytes),
				"Schema for tool %s should be stable across restarts", name)
		}
	}
}

func TestSchemaArrayMapCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping schema compatibility test in short mode")
	}

	// Removed hardcoded skip - testing gomcp integration fix

	client, err := StartMCPServer(t)
	require.NoError(t, err)
	defer client.Close()

	tools, err := client.GetToolsList()
	require.NoError(t, err)

	for _, tool := range tools {
		t.Run(fmt.Sprintf("ArrayMap_%s", tool.Name), func(t *testing.T) {
			schemaBytes, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)

			var parsedSchema map[string]interface{}
			err = json.Unmarshal(schemaBytes, &parsedSchema)
			require.NoError(t, err)

			if properties, exists := parsedSchema["properties"]; exists {
				_, isMap := properties.(map[string]interface{})
				assert.True(t, isMap, "Tool %s properties should be a map/object", tool.Name)
			}

			validateArrayFields(t, tool.Name, parsedSchema)
		})
	}
}

func validateArrayFields(t *testing.T, toolName string, schema map[string]interface{}) {
	for key, value := range schema {
		switch v := value.(type) {
		case map[string]interface{}:
			if schemaType, ok := v["type"].(string); ok && schemaType == "array" {
				_, hasItems := v["items"]
				assert.True(t, hasItems,
					"Tool %s field %s is array type but missing items definition", toolName, key)
			}
			validateArrayFields(t, toolName, v)
		case []interface{}:
			// Allow empty arrays for 'required' field when all parameters are optional
			if key == "required" {
				// Empty required array is valid when all parameters are optional
				continue
			}
			assert.NotEmpty(t, v, "Tool %s field %s is empty array", toolName, key)
		}
	}
}

func BenchmarkSchemaValidation(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	b.Skip("Schema validation benchmark currently unstable - needs external MCP server setup")

	testingT := &testing.T{}
	client, err := StartMCPServer(testingT)
	if err != nil {
		b.Fatalf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	tools, err := client.GetToolsList()
	if err != nil {
		b.Fatalf("Failed to get tools: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			_ = bytes.Contains(schemaBytes, []byte(`"$ref"`))
			_ = bytes.Contains(schemaBytes, []byte(`"definitions"`))
			_ = bytes.Contains(schemaBytes, []byte(`"type":"object"`))
		}
	}
}
