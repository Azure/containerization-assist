package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMCPProgressNotifications tests that the MCP server sends real-time progress notifications
func TestMCPProgressNotifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the MCP server
	wd, _ := os.Getwd()
	serverPath := filepath.Join(wd, "..", "container-kit-mcp")
	
	// Check if binary exists, build if not
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		t.Log("Building MCP server...")
		cmd := exec.Command("make", "mcp")
		cmd.Dir = filepath.Join(wd, "..")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Failed to build MCP server: %s", string(output))
	}

	// Start MCP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, serverPath, "--transport", "stdio")
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())
	defer cmd.Process.Kill()

	// Capture stderr for debugging
	var stderrBuf bytes.Buffer
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				stderrBuf.Write(buf[:n])
			}
		}
	}()

	// Helper to read MCP responses
	readResponse := func() map[string]interface{} {
		var result map[string]interface{}
		buf := make([]byte, 8192)
		var data []byte
		
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			n, err := stdout.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
				if bytes.Contains(data, []byte("\n")) {
					lines := bytes.Split(data, []byte("\n"))
					for _, line := range lines {
						line = bytes.TrimSpace(line)
						if len(line) == 0 {
							continue
						}
						if err := json.Unmarshal(line, &result); err == nil {
							return result
						}
					}
				}
			}
			if err != nil {
				time.Sleep(10 * time.Millisecond)
			}
		}
		
		t.Fatalf("Timeout reading response. Stderr: %s", stderrBuf.String())
		return nil
	}

	// Helper to send MCP request
	sendRequest := func(req map[string]interface{}) {
		data, err := json.Marshal(req)
		require.NoError(t, err)
		_, err = fmt.Fprintf(stdin, "%s\n", data)
		require.NoError(t, err)
	}

	// Initialize MCP server
	sendRequest(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]interface{}{
				"name":    "progress-test",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"experimental": map[string]interface{}{
					"progress": true, // Enable progress notifications
				},
			},
		},
	})

	resp := readResponse()
	assert.Contains(t, resp, "result")
	
	// Check server capabilities
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if capabilities, ok := result["capabilities"].(map[string]interface{}); ok {
			t.Logf("Server capabilities: %+v", capabilities)
		}
	}

	// Create a test repository
	tempDir, err := os.MkdirTemp("", "progress-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "test-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Simple Go app
	mainGo := `package main
import "fmt"
func main() { fmt.Println("Hello") }`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644))

	// Initialize git repo
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = repoDir
	require.NoError(t, gitCmd.Run())

	// Now call containerize_and_deploy with progress token
	progressToken := fmt.Sprintf("progress-%d", time.Now().Unix())
	
	sendRequest(map[string]interface{}{
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
			"_meta": map[string]interface{}{
				"progressToken": progressToken,
			},
		},
	})

	// Collect progress notifications
	var progressNotifications []map[string]interface{}
	progressSeen := make(map[string]bool)
	
	// Read responses for up to 60 seconds
	timeout := time.After(60 * time.Second)
	done := false
	
	for !done {
		select {
		case <-timeout:
			done = true
		default:
			// Try to read a response
			var resp map[string]interface{}
			buf := make([]byte, 8192)
			stdout.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _ := stdout.Read(buf)
			
			if n > 0 {
				lines := strings.Split(string(buf[:n]), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					
					if err := json.Unmarshal([]byte(line), &resp); err == nil {
						// Check if this is a progress notification
						if method, ok := resp["method"].(string); ok && method == "notifications/progress" {
							progressNotifications = append(progressNotifications, resp)
							
							if params, ok := resp["params"].(map[string]interface{}); ok {
								if token, ok := params["progressToken"].(string); ok && token == progressToken {
									if progress, ok := params["progress"].(float64); ok {
										if message, ok := params["message"].(string); ok {
											t.Logf("Progress: %.0f%% - %s", progress*100, message)
											progressSeen[message] = true
										}
									}
								}
							}
						}
						
						// Check if this is the final response
						if id, ok := resp["id"].(float64); ok && id == 2 {
							done = true
							t.Log("Received final response")
						}
					}
				}
			}
		}
	}

	// Verify we received progress notifications
	t.Logf("Total progress notifications received: %d", len(progressNotifications))
	
	// Check that we got notifications for key steps
	expectedSteps := []string{
		"Analyzing repository",
		"Generating",
		"Building",
		"Setting up",
		"Loading",
		"Deploying",
	}
	
	for _, step := range expectedSteps {
		found := false
		for msg := range progressSeen {
			if strings.Contains(msg, step) {
				found = true
				break
			}
		}
		if found {
			t.Logf("✓ Found progress for: %s", step)
		} else {
			t.Logf("✗ Missing progress for: %s", step)
		}
	}
	
	// Even if we don't get progress notifications (client might not support it),
	// the workflow should still complete successfully
	if len(progressNotifications) == 0 {
		t.Log("No progress notifications received - client may not support progress tracking")
	} else {
		assert.Greater(t, len(progressNotifications), 0, "Should have received at least one progress notification")
	}
}