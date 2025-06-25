package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	stdioutils "github.com/Azure/container-copilot/pkg/mcp/internal/transport/stdio"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStdioClient simulates a JSON-RPC client over stdio
type MockStdioClient struct {
	reader *bytes.Buffer
	writer *bytes.Buffer
}

func NewMockStdioClient() *MockStdioClient {
	return &MockStdioClient{
		reader: &bytes.Buffer{},
		writer: &bytes.Buffer{},
	}
}

// SimulateResponse simulates a JSON-RPC response from the client
func (m *MockStdioClient) SimulateResponse(id interface{}, result interface{}, err error) error {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
	}

	if err != nil {
		response["error"] = map[string]interface{}{
			"code":    -32603,
			"message": err.Error(),
		}
	} else {
		resultBytes, _ := json.Marshal(result)
		response["result"] = json.RawMessage(resultBytes)
	}

	responseBytes, _ := json.Marshal(response)
	_, writeErr := m.reader.Write(append(responseBytes, '\n'))
	return writeErr
}

// GetLastRequest returns the last JSON-RPC request sent
func (m *MockStdioClient) GetLastRequest() (map[string]interface{}, error) {
	data := m.writer.Bytes()
	if len(data) == 0 {
		return nil, io.EOF
	}

	// Find the last newline-delimited JSON
	lines := bytes.Split(data, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) > 0 {
			var req map[string]interface{}
			if err := json.Unmarshal(lines[i], &req); err == nil {
				return req, nil
			}
		}
	}

	return nil, io.EOF
}

func TestStdioLLMTransport_InvokeTool(t *testing.T) {
	// Create stdio transport using factory for consistent configuration
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stdioTransport := stdioutils.NewDefaultStdioTransport(logger)

	// Create LLM transport using factory for consistent configuration
	llmTransport := stdioutils.NewDefaultLLMTransport(stdioTransport, logger)

	// Note: This test validates the basic functionality without actual stdio communication
	// In a real environment, the stdio transport would be connected to an MCP client

	t.Run("successful tool invocation creation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		payload := map[string]any{
			"query": "test query",
			"limit": 10,
		}

		// Invoke tool - this should create the response channel successfully
		responseCh, err := llmTransport.InvokeTool(ctx, "search", payload, false)
		require.NoError(t, err)
		require.NotNil(t, responseCh)

		// Wait for response - we expect this to contain an error since
		// there's no actual JSON-RPC server listening on stdin/stdout in tests
		select {
		case resp := <-responseCh:
			require.NotNil(t, resp)
			var result map[string]interface{}
			err := json.Unmarshal(resp, &result)
			assert.NoError(t, err)
			// Should contain an error response since stdio isn't connected to an MCP client
			assert.Contains(t, result, "Error")
			assert.Contains(t, result["Error"], "Failed to invoke tool")
		case <-ctx.Done():
			// This is expected in test environment - stdio isn't connected to anything
			t.Log("Context timeout expected in test environment without MCP client")
		}
	})

	t.Run("streaming tool invocation creation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		payload := map[string]any{"test": "data"}

		// Invoke tool with streaming - this should create the response channel successfully
		responseCh, err := llmTransport.InvokeTool(ctx, "stream_tool", payload, true)
		require.NoError(t, err)
		require.NotNil(t, responseCh)

		// Wait for response - similar to above, we expect either an error response
		// or a timeout due to no MCP client being connected
		select {
		case resp := <-responseCh:
			require.NotNil(t, resp)
			var result map[string]interface{}
			err := json.Unmarshal(resp, &result)
			assert.NoError(t, err)
		case <-ctx.Done():
			// This is expected in test environment - stdio isn't connected to anything
			t.Log("Context timeout expected in test environment without MCP client")
		}
	})
}

func TestJSONRPCProtocol(t *testing.T) {
	// This test verifies the JSON-RPC protocol format
	mockClient := NewMockStdioClient()

	// Simulate a request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "test_tool",
			"arguments": map[string]interface{}{
				"key": "value",
			},
		},
	}

	requestBytes, err := json.Marshal(request)
	require.NoError(t, err)

	_, err = mockClient.writer.Write(append(requestBytes, '\n'))
	require.NoError(t, err)

	// Verify we can read it back
	lastReq, err := mockClient.GetLastRequest()
	require.NoError(t, err)
	assert.Equal(t, "2.0", lastReq["jsonrpc"])
	assert.Equal(t, float64(1), lastReq["id"])
	assert.Equal(t, "tools/call", lastReq["method"])

	// Simulate a response
	err = mockClient.SimulateResponse(1, map[string]interface{}{"content": "Tool executed successfully"}, nil)
	require.NoError(t, err)

	// Read the response
	responseBytes := mockClient.reader.Bytes()
	var response map[string]interface{}
	err = json.Unmarshal(bytes.TrimSpace(responseBytes), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response["jsonrpc"])
	assert.Equal(t, float64(1), response["id"])
	assert.Contains(t, response, "result")
	assert.NotContains(t, response, "error")
}
