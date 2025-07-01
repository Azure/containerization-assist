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
	server      *httptest.Server
	mcpServer   *core.Server
	tempDir     string
	cancelStart context.CancelFunc
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

	// Start the MCP server in a goroutine to initialize all components
	startCtx, cancel := context.WithCancel(ctx)
	serverStarted := make(chan error, 1)
	go func() {
		err := mcpServer.Start(startCtx)
		serverStarted <- err
	}()

	// Wait a moment for server initialization
	time.Sleep(100 * time.Millisecond)

	// Get the transport from the MCP server to access its HTTP handler
	var httpHandler http.Handler

	// Try to get the router from the transport if it's HTTP
	if transport := mcpServer.GetTransport(); transport != nil {
		if httpTransport, ok := transport.(interface{ GetRouter() http.Handler }); ok {
			httpHandler = httpTransport.GetRouter()
		}
	}

	// If we couldn't get the router, create a fallback handler
	if httpHandler == nil {
		httpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Fallback for non-HTTP transports
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			w.Write([]byte(`{"error": "HTTP transport not available"}`))
		})
	}

	// Create HTTP test server with the actual MCP handler
	httpServer := httptest.NewServer(httpHandler)

	// Cancel the server's start context since we're using httptest.Server
	cancel()

	return &TestServer{
		server:      httpServer,
		mcpServer:   mcpServer,
		tempDir:     tempDir,
		cancelStart: cancel,
	}, nil
}

// URL returns the test server URL
func (ts *TestServer) URL() string {
	return ts.server.URL
}

// Close shuts down the test server and cleans up resources
func (ts *TestServer) Close() {
	if ts.cancelStart != nil {
		ts.cancelStart()
	}
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
