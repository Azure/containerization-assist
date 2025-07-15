package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// MCPServerProcess holds the server process and its pipes
type MCPServerProcess struct {
	cmd          *exec.Cmd
	stdin        *os.File
	stdout       *os.File
	stderr       *os.File
	workspaceDir string
}

// Cleanup terminates the server process and cleans up workspace
func (p *MCPServerProcess) Cleanup() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
	if p.workspaceDir != "" {
		os.RemoveAll(p.workspaceDir)
	}
}

// startMCPServerProcess starts an MCP server process for testing
func startMCPServerProcess(ctx context.Context, testWorkspaceDir string) *MCPServerProcess {
	// Build the server binary path
	serverBinaryPath := "/tmp/mcp-server"

	// Build the server if it doesn't exist
	if _, err := os.Stat(serverBinaryPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", serverBinaryPath, "../../cmd/mcp-server")
		if err := buildCmd.Run(); err != nil {
			// Try alternative build path
			buildCmd = exec.Command("go", "build", "-o", serverBinaryPath, "./cmd/mcp-server")
			if err := buildCmd.Run(); err != nil {
				panic("Failed to build MCP server: " + err.Error())
			}
		}
	}

	// Create unique workspace and store paths for this test instance
	if testWorkspaceDir == "" {
		testWorkspaceDir = "/tmp/container-kit-test-workspace"
	}

	// Ensure unique paths by appending process PID and timestamp
	uniqueSuffix := fmt.Sprintf("-%d-%d", os.Getpid(), time.Now().Unix())
	workspaceDir := testWorkspaceDir + uniqueSuffix
	storePath := testWorkspaceDir + uniqueSuffix + "/sessions.db"

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		panic("Failed to create test workspace: " + err.Error())
	}

	cmd := exec.CommandContext(ctx, serverBinaryPath,
		"--transport", "stdio",
		"--workspace-dir", workspaceDir,
		"--store-path", storePath)

	// Get all pipes before starting
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic("Failed to create stdin pipe: " + err.Error())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic("Failed to create stdout pipe: " + err.Error())
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic("Failed to create stderr pipe: " + err.Error())
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		panic("Failed to start MCP server: " + err.Error())
	}

	// Convert to *os.File for compatibility
	stdinFile := stdin.(*os.File)
	stdoutFile := stdout.(*os.File)
	stderrFile := stderr.(*os.File)

	return &MCPServerProcess{
		cmd:          cmd,
		stdin:        stdinFile,
		stdout:       stdoutFile,
		stderr:       stderrFile,
		workspaceDir: workspaceDir,
	}
}

// sendMCPRequest sends an MCP request and returns the response
func sendMCPRequest(stdin *os.File, stdout *os.File, request map[string]interface{}, t *testing.T) map[string]interface{} {
	// Marshal request
	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Send request
	requestLine := string(requestData) + "\n"
	if _, err := stdin.Write([]byte(requestLine)); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Read response with timeout
	responseData := make([]byte, 32768)

	// Set read timeout - allow enough time for full workflow including K8s deployment validation
	deadline := time.Now().Add(90 * time.Second)
	stdout.SetReadDeadline(deadline)

	n, err := stdout.Read(responseData)
	if err != nil {
		if err == io.EOF {
			t.Log("Server closed connection")
			return nil
		}
		t.Fatalf("Failed to read response: %v", err)
	}

	// Parse response
	var response map[string]interface{}
	responseStr := strings.TrimSpace(string(responseData[:n]))
	if responseStr == "" {
		t.Log("Empty response received")
		return nil
	}

	err = json.Unmarshal([]byte(responseStr), &response)
	if err != nil {
		t.Logf("Failed to parse response: %v, raw: %s", err, responseStr)
		return nil
	}

	return response
}
