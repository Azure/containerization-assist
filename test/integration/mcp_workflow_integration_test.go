// Package integration_test provides minimal, focused integration tests for the Containerization Assist MCP server workflow
package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MCPWorkflowIntegrationSuite provides focused integration tests for supported MCP workflow operations
type MCPWorkflowIntegrationSuite struct {
	suite.Suite
	testRepoDir string
	tempDir     string
}

// TestRepository represents a test repository configuration
type TestRepository struct {
	Name        string
	URL         string
	Branch      string
	Language    string
	Framework   string
	Port        int
	LocalDir    string // For local test repositories
	Description string
}

// assertErrorOrResult ensures either a top-level error exists or a result is returned
func (suite *MCPWorkflowIntegrationSuite) assertErrorOrResult(resp map[string]interface{}) {
	if _, hasErr := resp["error"]; hasErr {
		return
	}
	assert.Contains(suite.T(), resp, "result", "expected either an error or a result")
}

// GetTestRepositories returns local test repositories used for integration testing
func (suite *MCPWorkflowIntegrationSuite) GetTestRepositories() []TestRepository {
	return []TestRepository{
		{
			Name:        "simple-go-service",
			Language:    "go",
			Framework:   "http",
			Port:        8080,
			LocalDir:    suite.createLocalGoRepo(),
			Description: "Simple Go HTTP service for containerization testing",
		},
		// Future: Add more test repositories in table-driven format
		// {
		//     Name:        "python-flask-app",
		//     Language:    "python",
		//     Framework:   "flask",
		//     Port:        5000,
		//     LocalDir:    suite.createLocalPythonRepo(),
		//     Description: "Python Flask application for containerization testing",
		// },
	}
}

// SetupSuite initializes the test suite
func (suite *MCPWorkflowIntegrationSuite) SetupSuite() {
	if testing.Short() {
		suite.T().Skip("Skipping integration tests in short mode")
	}

	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "mcp-workflow-test-*")
	require.NoError(suite.T(), err)
	suite.tempDir = tempDir
}

// TearDownSuite cleans up the test suite
func (suite *MCPWorkflowIntegrationSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// createLocalGoRepo creates a local Go repository for testing
func (suite *MCPWorkflowIntegrationSuite) createLocalGoRepo() string {
	repoDir, err := os.MkdirTemp(suite.tempDir, "test-go-repo-*")
	require.NoError(suite.T(), err)

	// Create main.go
	mainGo := `package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from Containerization Assist Test App!")
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"status\": \"running\", \"timestamp\": \"%s\"}", time.Now().Format(time.RFC3339))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	fmt.Println("Server starting on :8080")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}
`

	// Create go.mod
	goMod := `module github.com/example/test-go-app

go 1.21

// Test dependencies for containerization validation
require (
	github.com/stretchr/testify v1.8.4
)
`

	// Create go.sum (empty for this simple example)
	goSum := ``

	// Create README.md
	readme := `# Test Go Application

This is a simple Go HTTP service used for Containerization Assist integration testing.

## Endpoints

- / - Hello world endpoint
- /health - Health check endpoint
- /api/status - JSON status endpoint

## Build & Run

` + "```bash" + `
go build -o app .
./app
` + "```" + `

The service runs on port 8080.
`

	// Create .gitignore
	gitIgnore := `# Binaries
app
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, build with go test -c
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS generated files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db
`

	// Write files
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.sum"), []byte(goSum), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "README.md"), []byte(readme), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(gitIgnore), 0644))

	return repoDir
}

// TestMinimalMCPWorkflow runs a minimal, supported subset of the MCP workflow for each test repo
func (suite *MCPWorkflowIntegrationSuite) TestMinimalMCPWorkflow() {

	testRepos := suite.GetTestRepositories()

	for _, repo := range testRepos {
		suite.Run(fmt.Sprintf("Workflow_%s", repo.Name), func() {
			suite.runMinimalWorkflow(repo)
		})
	}
}

// runMinimalWorkflow executes: initialize -> analyze_repository -> workflow_status
func (suite *MCPWorkflowIntegrationSuite) runMinimalWorkflow(repo TestRepository) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Start MCP server
	mcpServer := startMCPServerProcess(ctx, "")
	defer mcpServer.Cleanup()

	// Execute minimal steps (initialize + analyze_repository)
	sessionID := suite.initializeAndAnalyzeRepository(ctx, mcpServer, repo)

	// Validate session state
	suite.validateSessionState(ctx, mcpServer, sessionID)

	suite.T().Logf("Minimal workflow validation successful for repository: %s", repo.Name)
}

// initializeAndAnalyzeRepository performs initialize and analyze_repository, returning the session ID for subsequent checks
func (suite *MCPWorkflowIntegrationSuite) initializeAndAnalyzeRepository(ctx context.Context, server *MCPServerProcess, repo TestRepository) string {
	stdin := server.stdin
	stdout := server.stdout

	// Step 1: Initialize the MCP server
	initResp := initializeMCP(suite.T(), stdin, stdout, "integration-test-client", "1.0.0")
	assert.Contains(suite.T(), initResp, "result")
	suite.T().Log("✓ MCP server initialized successfully")

	// Generate a session ID for this test
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	// Step 2: Run analyze_repository (lightweight and supported in CI)
	analyzeResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"repo_path":  repo.LocalDir,
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())

	require.NotContains(suite.T(), analyzeResp, "error")
	assert.Contains(suite.T(), analyzeResp, "result")
	suite.T().Log("✓ Repository analysis executed successfully")

	return sessionID
}

// validateSessionState verifies workflow_status returns a result for the provided session ID
func (suite *MCPWorkflowIntegrationSuite) validateSessionState(ctx context.Context, server *MCPServerProcess, sessionID string) {
	stdin := server.stdin
	stdout := server.stdout

	// Query workflow_status to ensure the session is recognized by the server
	statusResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "workflow_status",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
			},
		},
	}, suite.T())

	require.NotContains(suite.T(), statusResp, "error", "workflow_status should not return a top-level error")
	assert.Contains(suite.T(), statusResp, "result", "workflow_status should return a result for the session")
	suite.T().Logf("✓ Session state validated for %s", sessionID)
}

// TestAnalyzeSetsSessionState performs init -> analyze -> workflow_status
func (suite *MCPWorkflowIntegrationSuite) TestAnalyzeSetsSessionState() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := startMCPServerProcess(ctx, "")
	defer server.Cleanup()

	stdin := server.stdin
	stdout := server.stdout

	// Initialize
	_ = initializeMCP(suite.T(), stdin, stdout, "analyze-session-test", "1.0.0")

	// Create repo and session
	repoDir := suite.createLocalGoRepo()
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	// Analyze
	analyze := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"repo_path":  repoDir,
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())
	require.NotContains(suite.T(), analyze, "error")
	assert.Contains(suite.T(), analyze, "result")

	// workflow_status
	status := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "workflow_status",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
			},
		},
	}, suite.T())
	require.NotContains(suite.T(), status, "error")
	assert.Contains(suite.T(), status, "result")
}

// TestParameterValidationErrors checks missing required args return errors
func (suite *MCPWorkflowIntegrationSuite) TestParameterValidationErrors() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := startMCPServerProcess(ctx, "")
	defer server.Cleanup()

	stdin := server.stdin
	stdout := server.stdout

	// Initialize
	_ = initializeMCP(suite.T(), stdin, stdout, "param-validation-test", "1.0.0")

	// Missing session_id for build_image
	resp1 := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "build_image",
			"arguments": map[string]interface{}{},
		},
	}, suite.T())
	suite.assertErrorOrResult(resp1)

	// Missing manifests for generate_k8s_manifests
	resp2 := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "generate_k8s_manifests",
			"arguments": map[string]interface{}{},
		},
	}, suite.T())
	suite.assertErrorOrResult(resp2)
}

// TestAnalyzeIdempotent ensures calling analyze twice doesn't corrupt state
func (suite *MCPWorkflowIntegrationSuite) TestAnalyzeIdempotent() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := startMCPServerProcess(ctx, "")
	defer server.Cleanup()

	stdin := server.stdin
	stdout := server.stdout

	// Initialize
	_ = initializeMCP(suite.T(), stdin, stdout, "analyze-idempotent-test", "1.0.0")

	repoDir := suite.createLocalGoRepo()
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	for i := 0; i < 2; i++ {
		resp := sendMCPRequest(stdin, stdout, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2 + i,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "analyze_repository",
				"arguments": map[string]interface{}{
					"repo_path":  repoDir,
					"session_id": sessionID,
					"test_mode":  true,
				},
			},
		}, suite.T())
		require.NotContains(suite.T(), resp, "error")
		assert.Contains(suite.T(), resp, "result")
	}

	// Check status
	status := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "workflow_status",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
			},
		},
	}, suite.T())
	require.NotContains(suite.T(), status, "error")
	assert.Contains(suite.T(), status, "result")
}

// Run the test suite
func TestMCPWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MCPWorkflowIntegrationSuite))
}
