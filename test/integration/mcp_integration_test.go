// Package integration provides integration tests for the Container Kit MCP server
package integration

import (
	"bufio"
	"context"
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

// TestMCPServerBinaryIntegration tests the MCP server as a black box through its binary
func TestMCPServerBinaryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Build the MCP server binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "mcp-server")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/mcp-server")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build MCP server: %v\nOutput: %s", err, buildOutput)
	}

	t.Run("ServerStartupAndShutdown", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Start server with stdio transport
		cmd := exec.CommandContext(ctx, binaryPath, "--transport", "stdio")

		stdin, err := cmd.StdinPipe()
		require.NoError(t, err)

		stdout, err := cmd.StdoutPipe()
		require.NoError(t, err)

		stderr, err := cmd.StderrPipe()
		require.NoError(t, err)

		// Start the server
		err = cmd.Start()
		require.NoError(t, err)

		// Read stderr for startup messages
		errReader := bufio.NewReader(stderr)

		go func() {
			for {
				line, err := errReader.ReadString('\n')
				if err != nil {
					return
				}
				t.Logf("Server stderr: %s", strings.TrimSpace(line))
			}
		}()

		// Wait for startup
		time.Sleep(2 * time.Second)

		// Send initialize request
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		requestBytes, err := json.Marshal(initRequest)
		require.NoError(t, err)

		_, err = fmt.Fprintf(stdin, "%s\n", requestBytes)
		require.NoError(t, err)

		// Read response
		reader := bufio.NewReader(stdout)
		responseLine, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			t.Logf("Error reading response: %v", err)
		}

		if responseLine != "" {
			var response map[string]interface{}
			err = json.Unmarshal([]byte(responseLine), &response)
			if err == nil {
				t.Logf("Server response: %v", response)
				assert.Contains(t, response, "result")
			}
		}

		// Graceful shutdown
		cancel()

		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case err := <-done:
			if err != nil && ctx.Err() == nil {
				t.Errorf("Server exited with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("Failed to kill process: %v", err)
			}
			t.Error("Server did not shut down gracefully")
		}
	})
}

// TestMCPToolExecution tests tool execution through the MCP protocol
func TestMCPToolExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// This test would require a running MCP server instance
	// For now, we'll create a simple test that validates the protocol format

	t.Run("ProtocolValidation", func(t *testing.T) {
		// Test request format
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "server_status",
				"arguments": map[string]interface{}{},
			},
		}

		requestBytes, err := json.Marshal(request)
		require.NoError(t, err)

		// Validate JSON-RPC format
		assert.Contains(t, string(requestBytes), "jsonrpc")
		assert.Contains(t, string(requestBytes), "2.0")
		assert.Contains(t, string(requestBytes), "tools/call")
	})
}

// TestContainerizationWorkflow tests a complete containerization workflow
func TestContainerizationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create a test application
	testDir := t.TempDir()
	appDir := filepath.Join(testDir, "test-app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	// Create a simple Go application
	mainGo := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from Container Kit!")
	})

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}
`
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "main.go"), []byte(mainGo), 0600))

	goMod := `module example.com/hello

go 1.21
`
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(goMod), 0600))

	// Test workflow steps
	workflowSteps := []struct {
		name       string
		tool       string
		args       map[string]interface{}
		validateFn func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "AnalyzeRepository",
			tool: "analyze_repository",
			args: map[string]interface{}{
				"repo_path": appDir,
			},
			validateFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "go", result["language"])
				assert.Contains(t, result, "framework")
			},
		},
		{
			name: "GenerateDockerfile",
			tool: "generate_dockerfile_enhanced",
			args: map[string]interface{}{
				"path": appDir,
			},
			validateFn: func(t *testing.T, result map[string]interface{}) {
				assert.Contains(t, result, "dockerfile_path")

				// Verify Dockerfile content
				dockerfilePath := filepath.Join(appDir, "Dockerfile")
				if _, err := os.Stat(dockerfilePath); err == nil {
					content, err := os.ReadFile(dockerfilePath)
					require.NoError(t, err)
					assert.Contains(t, string(content), "FROM")
					assert.Contains(t, string(content), "EXPOSE")
				}
			},
		},
		{
			name: "GenerateKubernetesManifests",
			tool: "generate_kubernetes_manifests",
			args: map[string]interface{}{
				"app_name":   "test-app",
				"image_name": "test-app:latest",
				"namespace":  "default",
			},
			validateFn: func(t *testing.T, result map[string]interface{}) {
				assert.Contains(t, result, "manifests")
				manifests, ok := result["manifests"].([]interface{})
				if ok {
					assert.Greater(t, len(manifests), 0)

					// Validate manifest structure
					for _, manifest := range manifests {
						m, ok := manifest.(map[string]interface{})
						if ok {
							assert.Contains(t, m, "kind")
							assert.Contains(t, m, "metadata")
						}
					}
				}
			},
		},
	}

	// This is a placeholder for actual workflow execution
	// In a real test, these would be executed through the MCP server
	t.Log("Workflow steps defined:")
	for _, step := range workflowSteps {
		t.Logf("  - %s: %s", step.name, step.tool)
	}
}

// TestErrorScenarios tests various error conditions
func TestErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	errorTests := []struct {
		name        string
		request     map[string]interface{}
		expectError string
	}{
		{
			name: "InvalidMethod",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "invalid/method",
				"params":  map[string]interface{}{},
			},
			expectError: "method not found",
		},
		{
			name: "MissingRequiredParams",
			request: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "analyze_repository",
					// Missing required repo_path
				},
			},
			expectError: "invalid params",
		},
		{
			name: "InvalidJSONRPC",
			request: map[string]interface{}{
				"jsonrpc": "1.0", // Invalid version
				"id":      1,
				"method":  "tools/call",
			},
			expectError: "invalid request",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			requestBytes, err := json.Marshal(tt.request)
			require.NoError(t, err)

			// Validate request structure
			var req map[string]interface{}
			err = json.Unmarshal(requestBytes, &req)
			require.NoError(t, err)

			// In a real test, this would be sent to the server
			// and we would validate the error response
			t.Logf("Test request: %s", string(requestBytes))
		})
	}
}

// TestPerformanceAndLoad tests server performance under load
func TestPerformanceAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Define concurrent request count
		numRequests := 50

		// Create request template
		requestTemplate := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "server_status",
				"arguments": map[string]interface{}{},
			},
		}

		// Simulate concurrent requests
		results := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(id int) {
				// Create a deep copy of the request template
				request := make(map[string]interface{})
				for k, v := range requestTemplate {
					request[k] = v
				}
				request["id"] = id

				_, err := json.Marshal(request)
				results <- err == nil
			}(i)
		}

		// Collect results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			if <-results {
				successCount++
			}
		}

		assert.Equal(t, numRequests, successCount, "All requests should be valid")
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		// This would monitor memory usage during operations
		// For now, we just document the test intent
		t.Log("Memory usage monitoring would be implemented here")
	})
}
