package testutil

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"

	"container-kit/pkg/mcp/internal/core"
)

// TestServer wraps an HTTP test server with MCP functionality
type TestServer struct {
	server    *httptest.Server
	mcpServer *core.Server
	tempDir   string
}

// NewTestServer creates a new test server with real MCP functionality
func NewTestServer() (*TestServer, error) {
	// Create temporary workspace
	tempDir, err := os.MkdirTemp("", "mcp-test-server-*")
	if err != nil {
		return nil, err
	}

	// Initialize MCP server
	mcpServer, err := core.NewServer(core.ServerConfig{
		WorkspaceDir: tempDir,
		SessionDB:    filepath.Join(tempDir, "test-sessions.db"),
		Transport:    "http",
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	// Create HTTP test server
	httpServer := httptest.NewServer(mcpServer.HTTPHandler())

	return &TestServer{
		server:    httpServer,
		mcpServer: mcpServer,
		tempDir:   tempDir,
	}, nil
}

// URL returns the test server URL
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// Close shuts down the test server and cleans up resources
func (ts *TestServer) Close() {
	if ts.server != nil {
		ts.server.Close()
	}
	if ts.mcpServer != nil {
		ctx := context.Background()
		ts.mcpServer.Shutdown(ctx)
	}
	if ts.tempDir != "" {
		os.RemoveAll(ts.tempDir)
	}
}
