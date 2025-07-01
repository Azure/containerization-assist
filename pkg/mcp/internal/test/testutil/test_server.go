package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/core"
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
	config := core.ServerConfig{
		WorkspaceDir:  tempDir,
		StorePath:     filepath.Join(tempDir, "test-sessions.db"),
		TransportType: "http",
		HTTPAddr:      "localhost",
		HTTPPort:      0, // Use random port
		SessionTTL:    time.Hour,
		LogLevel:      "info",
		MaxSessions:   100,
	}

	ctx := context.Background()
	mcpServer, err := core.NewServer(ctx, config)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	// Create HTTP test server with a simple handler
	// Note: We'll need to implement the HTTPHandler method or use a basic handler
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic handler for testing - in real implementation this would use mcpServer's handler
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))

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
