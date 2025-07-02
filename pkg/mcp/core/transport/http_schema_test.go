package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer implements a minimal server interface for testing
type mockServer struct {
	registry *mockRegistry
}

func (m *mockServer) GetToolRegistry() interface {
	GetToolSchema(string) (map[string]interface{}, error)
} {
	return m.registry
}

// mockRegistry implements a minimal registry interface for testing
type mockRegistry struct {
	schemas map[string]map[string]interface{}
}

func (m *mockRegistry) GetToolSchema(name string) (map[string]interface{}, error) {
	if schema, ok := m.schemas[name]; ok {
		return schema, nil
	}
	return nil, fmt.Errorf("tool %s not found", name)
}

func TestHTTPTransport_PreservesDescriptions(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	t.Run("preserves description from HTTP transport when registry has empty description", func(t *testing.T) {
		// Create HTTP transport
		transport := NewHTTPTransport(HTTPTransportConfig{
			Port:   0,
			Logger: logger,
		})

		// Create mock server with registry that has empty descriptions
		mockRegistry := &mockRegistry{
			schemas: map[string]map[string]interface{}{
				"test_tool": {
					"name":        "test_tool",
					"description": "", // Empty description in registry
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"input": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		}

		mockServer := &mockServer{registry: mockRegistry}
		transport.mcpServer = mockServer

		// Register tool with description in HTTP transport
		expectedDescription := "This is a test tool with a description"
		handler := ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		})
		err := transport.RegisterTool("test_tool", expectedDescription, handler)
		require.NoError(t, err)

		// Test handleGetAllToolSchemas endpoint
		req := httptest.NewRequest("GET", "/api/v1/tools/schemas", nil)
		w := httptest.NewRecorder()

		transport.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		schemas, ok := response["schemas"].(map[string]interface{})
		require.True(t, ok)

		testToolSchema, ok := schemas["test_tool"].(map[string]interface{})
		require.True(t, ok)

		// Verify description was preserved from HTTP transport
		assert.Equal(t, expectedDescription, testToolSchema["description"])
	})

	t.Run("uses registry description when available", func(t *testing.T) {
		// Create HTTP transport
		transport := NewHTTPTransport(HTTPTransportConfig{
			Port:   0,
			Logger: logger,
		})

		// Create mock server with registry that has descriptions
		registryDescription := "Registry description"
		mockRegistry := &mockRegistry{
			schemas: map[string]map[string]interface{}{
				"test_tool": {
					"name":        "test_tool",
					"description": registryDescription,
					"parameters": map[string]interface{}{
						"type": "object",
					},
				},
			},
		}

		mockServer := &mockServer{registry: mockRegistry}
		transport.mcpServer = mockServer

		// Register tool with different description in HTTP transport
		handler := ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		})
		err := transport.RegisterTool("test_tool", "HTTP transport description", handler)
		require.NoError(t, err)

		// Test handleGetAllToolSchemas endpoint
		req := httptest.NewRequest("GET", "/api/v1/tools/schemas", nil)
		w := httptest.NewRecorder()

		transport.router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		schemas, ok := response["schemas"].(map[string]interface{})
		require.True(t, ok)

		testToolSchema, ok := schemas["test_tool"].(map[string]interface{})
		require.True(t, ok)

		// Verify registry description is used when available
		assert.Equal(t, registryDescription, testToolSchema["description"])
	})

	t.Run("getToolMetadata preserves description", func(t *testing.T) {
		// Create HTTP transport
		transport := NewHTTPTransport(HTTPTransportConfig{
			Port:   0,
			Logger: logger,
		})

		// Create mock server with registry that has empty descriptions
		mockRegistry := &mockRegistry{
			schemas: map[string]map[string]interface{}{
				"test_tool": {
					"name":        "test_tool",
					"description": "", // Empty description
					"category":    "test",
				},
			},
		}

		mockServer := &mockServer{registry: mockRegistry}
		transport.mcpServer = mockServer

		// Register tool with description
		expectedDescription := "Tool description from HTTP transport"
		handler := ToolHandler(func(ctx context.Context, args interface{}) (interface{}, error) {
			return map[string]interface{}{"result": "success"}, nil
		})
		err := transport.RegisterTool("test_tool", expectedDescription, handler)
		require.NoError(t, err)

		// Call getToolMetadata
		metadata := transport.getToolMetadata("test_tool")
		require.NotNil(t, metadata)

		// Verify description was added
		assert.Equal(t, expectedDescription, metadata.Description)
		assert.Equal(t, "test", metadata.Category) // Other fields preserved
	})
}
