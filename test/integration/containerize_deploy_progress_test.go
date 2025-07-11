package integration

import (
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

// TestContainerizeAndDeployProgress tests the progress indicators in the containerize_and_deploy workflow
func TestContainerizeAndDeployProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "progress-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Use the pre-built MCP server binary
	wd, _ := os.Getwd()
	serverBinaryPath := filepath.Join(wd, "..", "..", "container-kit-mcp")

	// Check if binary exists
	if _, err := os.Stat(serverBinaryPath); os.IsNotExist(err) {
		// Try to build it
		t.Log("Binary not found, building MCP server...")
		buildCmd := exec.Command("make", "mcp")
		buildCmd.Dir = filepath.Join(wd, "..", "..")
		buildOutput, err := buildCmd.CombinedOutput()
		require.NoError(t, err, "Failed to build MCP server: %s", string(buildOutput))
	}

	// Create a simple test repository
	repoDir := createTestRepo(t, tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start MCP server
	cmd := exec.CommandContext(ctx, serverBinaryPath, "--transport", "stdio")

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())
	defer cmd.Process.Kill()

	// Log server stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				t.Logf("MCP Server STDERR: %s", strings.TrimSpace(string(buf[:n])))
			}
		}
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Initialize MCP server
	initResp := sendRequest(t, stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    "progress-test-client",
				"version": "1.0.0",
			},
		},
	})
	assert.Contains(t, initResp, "result")

	// Execute containerize_and_deploy workflow
	workflowResp := sendRequest(t, stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "containerize_and_deploy",
			"arguments": map[string]interface{}{
				"repo_url": "file://" + repoDir,
				"branch":   "main",
				"scan":     false,
			},
		},
	})

	// Check response structure
	assert.Contains(t, workflowResp, "result")
	result := workflowResp["result"].(map[string]interface{})

	// Log the full response for debugging
	t.Logf("Full workflow response: %+v", result)

	// Check for content wrapper (MCP format)
	var workflowResult map[string]interface{}
	var isError bool

	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if contentItem, ok := content[0].(map[string]interface{}); ok {
			if textStr, ok := contentItem["text"].(string); ok {
				// Try to parse as JSON first
				if err := json.Unmarshal([]byte(textStr), &workflowResult); err != nil {
					// If not JSON, it might be an error message
					t.Logf("Response text (not JSON): %s", textStr)
					if strings.Contains(textStr, "workflow failed") {
						isError = true
					}
				}
			}
		}
	} else {
		// Direct result format
		workflowResult = result
	}

	// Check if this is an error response
	if errFlag, ok := result["isError"].(bool); ok && errFlag {
		isError = true
		t.Log("Workflow returned an error response")
	}

	// For now, just log if it's an error
	if isError {
		t.Log("Workflow execution resulted in an error, but we can still check for partial progress")
	}

	// Verify workflow result structure
	if workflowResult != nil {
		// Check for success
		if success, ok := workflowResult["success"].(bool); ok {
			assert.True(t, success, "Workflow should succeed")
		}

		// Check for steps with progress
		if steps, ok := workflowResult["steps"].([]interface{}); ok {
			t.Logf("Found %d workflow steps", len(steps))

			// Verify each step has progress information
			progressFound := false
			for i, stepRaw := range steps {
				if step, ok := stepRaw.(map[string]interface{}); ok {
					t.Logf("Step %d: %+v", i+1, step)

					// Check for progress field
					if progress, ok := step["progress"].(string); ok && progress != "" {
						progressFound = true
						t.Logf("  Progress: %s", progress)

						// Verify progress format (e.g., "3/10")
						parts := strings.Split(progress, "/")
						assert.Equal(t, 2, len(parts), "Progress should be in format 'current/total'")
					}

					// Check for message field
					if message, ok := step["message"].(string); ok && message != "" {
						t.Logf("  Message: %s", message)

						// Message should contain percentage
						assert.Contains(t, message, "%", "Progress message should contain percentage")
					}

					// Check step name and status
					if name, ok := step["name"].(string); ok {
						t.Logf("  Name: %s", name)
					}
					if status, ok := step["status"].(string); ok {
						t.Logf("  Status: %s", status)
					}
				}
			}

			assert.True(t, progressFound, "At least one step should have progress information")
			assert.Greater(t, len(steps), 0, "Workflow should have at least one step")
		} else {
			t.Errorf("No steps found in workflow result: %+v", workflowResult)
		}
	} else {
		t.Errorf("Could not parse workflow result from response: %+v", result)
	}
}

func createTestRepo(t *testing.T, tempDir string) string {
	repoDir := filepath.Join(tempDir, "test-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Create a simple Go application
	mainGo := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from test app")
	})
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}
`

	goMod := `module testapp

go 1.21
`

	// Create empty go.sum since the Dockerfile expects it
	goSum := ``

	// Write files
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.sum"), []byte(goSum), 0644))

	// Initialize git repo
	gitInit := exec.Command("git", "init")
	gitInit.Dir = repoDir
	require.NoError(t, gitInit.Run())

	gitConfig := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig.Dir = repoDir
	require.NoError(t, gitConfig.Run())

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = repoDir
	require.NoError(t, gitConfig2.Run())

	gitConfig3 := exec.Command("git", "config", "commit.gpgsign", "false")
	gitConfig3.Dir = repoDir
	gitConfig3.Run() // Ignore error if config doesn't exist

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = repoDir
	output, err := gitAdd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(output))

	gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
	gitCommit.Dir = repoDir
	output, err = gitCommit.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(output))

	return repoDir
}

func sendRequest(t *testing.T, stdin io.WriteCloser, stdout io.ReadCloser, request map[string]interface{}) map[string]interface{} {
	// Serialize request
	requestBytes, err := json.Marshal(request)
	require.NoError(t, err)

	// Send request
	_, err = fmt.Fprintf(stdin, "%s\n", requestBytes)
	require.NoError(t, err)

	// Read response
	var responseData []byte
	buf := make([]byte, 8192)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		n, err := stdout.Read(buf)
		if n > 0 {
			responseData = append(responseData, buf[:n]...)
			// Check if we have a complete JSON response
			responseStr := string(responseData)
			if strings.Contains(responseStr, "\n") {
				lines := strings.Split(responseStr, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && strings.HasPrefix(line, "{") {
						var response map[string]interface{}
						if err := json.Unmarshal([]byte(line), &response); err == nil {
							if _, hasResult := response["result"]; hasResult {
								return response
							}
							if _, hasError := response["error"]; hasError {
								return response
							}
						}
					}
				}
			}
		}
		if err != nil && err != io.EOF {
			continue
		}
	}

	t.Fatalf("Timeout waiting for response. Received data: %s", string(responseData))
	return nil
}
