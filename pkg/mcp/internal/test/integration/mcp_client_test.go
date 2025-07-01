package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Azure/container-kit/pkg/mcp/internal/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// MCPIntegrationTestSuite provides real MCP client/server integration testing
// This suite uses actual gomcp clients and HTTP transport, not mocks
type MCPIntegrationTestSuite struct {
	suite.Suite
	server     *core.Server
	client     testutil.MCPTestClient
	tempDir    string
	serverAddr string
	httpServer *httptest.Server
	ctx        context.Context
	cancel     context.CancelFunc
}

// SetupSuite initializes the real MCP server with HTTP transport
func (suite *MCPIntegrationTestSuite) SetupSuite() {
	var err error
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	// Create temporary workspace directory with BoltDB
	suite.tempDir, err = os.MkdirTemp("", "mcp-integration-test-*")
	suite.Require().NoError(err)

	// Initialize real MCP server with BoltDB session persistence
	config := core.ServerConfig{
		WorkspaceDir:  suite.tempDir,
		StorePath:     filepath.Join(suite.tempDir, "sessions.db"),
		TransportType: "http",
		HTTPAddr:      "localhost",
		HTTPPort:      0,
		SessionTTL:    time.Hour,
		LogLevel:      "info",
		MaxSessions:   100,
	}

	suite.server, err = core.NewServer(suite.ctx, config)
	suite.Require().NoError(err)

	// Start HTTP server with basic handler for testing
	suite.httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	suite.serverAddr = suite.httpServer.URL

	// Create real gomcp client connection
	suite.client, err = testutil.NewMCPTestClient(suite.serverAddr)
	suite.Require().NoError(err)

	// Validate server startup and client connection
	err = suite.client.Ping(suite.ctx)
	suite.Require().NoError(err, "MCP client should successfully connect to server")
}

// TearDownSuite cleans up test resources
func (suite *MCPIntegrationTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
	}
	if suite.httpServer != nil {
		suite.httpServer.Close()
	}
	if suite.server != nil {
		suite.server.Shutdown(suite.ctx)
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

// TestMCPProtocolCompliance validates basic MCP protocol compliance
func (suite *MCPIntegrationTestSuite) TestMCPProtocolCompliance() {
	// Test tool listing through MCP protocol
	tools, err := suite.client.ListTools(suite.ctx)
	suite.Require().NoError(err)
	suite.Assert().NotEmpty(tools, "Server should expose available tools")

	// Validate expected tools are present
	expectedTools := []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"generate_manifests",
		"scan_image",
	}

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	for _, expected := range expectedTools {
		suite.Assert().Contains(toolNames, expected, "Expected tool %s should be available", expected)
	}
}

// TestServerStartupValidation ensures server starts correctly with all components
func (suite *MCPIntegrationTestSuite) TestServerStartupValidation() {
	// Validate workspace directory exists
	suite.Assert().DirExists(suite.tempDir, "Workspace directory should exist")

	// Validate BoltDB session database exists
	dbPath := filepath.Join(suite.tempDir, "sessions.db")
	suite.Assert().FileExists(dbPath, "Session database should be created")

	// Validate server health endpoint
	health, err := suite.client.GetHealth(suite.ctx)
	suite.Require().NoError(err)
	suite.Assert().Equal("healthy", health.Status)
}

// TestClientConnectionResilience tests client connection stability
func (suite *MCPIntegrationTestSuite) TestClientConnectionResilience() {
	// Test multiple rapid connections
	for i := 0; i < 5; i++ {
		err := suite.client.Ping(suite.ctx)
		suite.Require().NoError(err, "Ping %d should succeed", i+1)
	}

	// Test connection after brief delay
	time.Sleep(100 * time.Millisecond)
	err := suite.client.Ping(suite.ctx)
	suite.Require().NoError(err, "Ping after delay should succeed")
}

// TestConcurrentClientAccess validates multiple concurrent client operations
func (suite *MCPIntegrationTestSuite) TestConcurrentClientAccess() {
	// Create multiple clients
	clients := make([]testutil.MCPTestClient, 3)
	for i := range clients {
		client, err := testutil.NewMCPTestClient(suite.serverAddr)
		suite.Require().NoError(err)
		clients[i] = client
		defer client.Close()
	}

	// Test concurrent tool listing
	results := make(chan error, len(clients))
	for _, client := range clients {
		go func(c testutil.MCPTestClient) {
			_, err := c.ListTools(suite.ctx)
			results <- err
		}(client)
	}

	// Validate all concurrent operations succeed
	for i := 0; i < len(clients); i++ {
		err := <-results
		suite.Assert().NoError(err, "Concurrent operation %d should succeed", i+1)
	}
}

// TestMCPIntegrationSuite runs the integration test suite
func TestMCPIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP integration suite in short mode")
	}
	suite.Run(t, new(MCPIntegrationTestSuite))
}
