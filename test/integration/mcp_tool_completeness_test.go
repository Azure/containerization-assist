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

// MCPToolCompletenessTestSuite validates that all MCP tools are properly implemented
type MCPToolCompletenessTestSuite struct {
	suite.Suite
	tmpDir string
}

func (suite *MCPToolCompletenessTestSuite) SetupSuite() {
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "mcp-tool-completeness-test-")
	require.NoError(suite.T(), err)
}

func (suite *MCPToolCompletenessTestSuite) TearDownSuite() {
	if suite.tmpDir != "" {
		os.RemoveAll(suite.tmpDir)
	}
}

// TestAllToolsImplemented validates that all essential MCP tools are implemented and working
func (suite *MCPToolCompletenessTestSuite) TestAllToolsImplemented() {
	suite.T().Log("Testing all MCP tools are properly implemented")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start MCP server
	mcpServer := suite.startMCPServer(ctx)
	defer mcpServer.Cleanup()

	time.Sleep(2 * time.Second)

	stdin := mcpServer.stdin
	stdout := mcpServer.stdout

	// Initialize server
	initResp := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    "tool-completeness-test",
				"version": "1.0.0",
			},
		},
	})
	assert.Contains(suite.T(), initResp, "result", "initialize should return a result")

	// Minimal validation: tools/list works and returns non-empty toolset
	_, toolsArr := suite.listAndDiscoverTools(stdin, stdout, 2)
	assert.GreaterOrEqual(suite.T(), len(toolsArr), 1, "tools/list should return at least one tool")
	suite.T().Log("✓ tools/list is available")

	// Create a test repo and session for exercising tools
	repoDir := suite.createTestRepository()
	sessionID := fmt.Sprintf("tools-%d", time.Now().UnixNano())

	// 1) analyze_repository
	respAnalyze := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"repo_path":  repoDir,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respAnalyze, "result", "analyze_repository should return a result")

	// 2) verify_dockerfile
	dockerfileContent := "FROM alpine:3.19\nCMD [\"sh\", \"-c\", \"sleep 1d\"]"
	respDockerfile := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "verify_dockerfile",
			"arguments": map[string]interface{}{
				"session_id":         sessionID,
				"dockerfile_content": dockerfileContent,
				"test_mode":          true,
			},
		},
	})
	assert.Contains(suite.T(), respDockerfile, "result", "verify_dockerfile should return a result")

	// 3) build_image
	respBuild := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "build_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respBuild, "result", "build_image should return a result")

	// 4) scan_image
	respScan := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "scan_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respScan, "result", "scan_image should return a result")

	// 5) tag_image
	respTag := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "tag_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"tag":        "latest",
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respTag, "result", "tag_image should return a result")

	// 6) push_image
	respPush := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "push_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"registry":   "localhost:5001",
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respPush, "result", "push_image should return a result")

	// 7) verify_k8s_manifests (include ingress)
	manifests := BasicK8sManifestsWithIngress()
	respManifests := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      9,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "verify_k8s_manifests",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"manifests":  manifests,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respManifests, "result", "verify_k8s_manifests should return a result")

	// 8) prepare_cluster
	respPrepare := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "prepare_cluster",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respPrepare, "result", "prepare_cluster should return a result")

	// 9) deploy_application
	respDeploy := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "deploy_application",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respDeploy, "result", "deploy_application should return a result")

	// 10) verify_deployment
	respVerify := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "verify_deployment",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	})
	assert.Contains(suite.T(), respVerify, "result", "verify_deployment should return a result")

	// SAD PATH: call an unknown tool to ensure proper error response
	respUnknown := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      13,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "totally_nonexistent_tool",
			"arguments": map[string]interface{}{},
		},
	})
	assert.Contains(suite.T(), respUnknown, "error", "unknown tool should return an error")

	suite.T().Log("✓ All tools were able to be called in test_mode")
}

// TestToolDiscoverability validates that all tools can be discovered via tools/list
func (suite *MCPToolCompletenessTestSuite) TestToolDiscoverability() {
	suite.T().Log("Testing tool discoverability via tools/list")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start MCP server
	mcpServer := suite.startMCPServer(ctx)
	defer mcpServer.Cleanup()

	time.Sleep(2 * time.Second)

	stdin := mcpServer.stdin
	stdout := mcpServer.stdout

	// Initialize server
	initResp := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    "tool-discovery-test",
				"version": "1.0.0",
			},
		},
	})
	assert.Contains(suite.T(), initResp, "result", "initialize should return a result")

	// List and discover tools using helper
	discoveredTools, toolsArray := suite.listAndDiscoverTools(stdin, stdout, 2)
	suite.T().Logf("Found %d tools via tools/list", len(toolsArray))

	// Verify current workflow tools are discoverable
	expectedTools := []string{
		"analyze_repository",
		"verify_dockerfile",
		"build_image",
		"scan_image",
		"tag_image",
		"push_image",
		"verify_k8s_manifests",
		"prepare_cluster",
		"deploy_application",
		"verify_deployment",
		"start_workflow",
		"workflow_status",
		"list_tools",
	}

	for _, expectedTool := range expectedTools {
		assert.True(suite.T(), discoveredTools[expectedTool],
			"Tool %s should be discoverable via tools/list", expectedTool)
	}

	suite.T().Log("✓ All essential tools are discoverable")
}

// Helper methods (reused from workflow integration test)

func (suite *MCPToolCompletenessTestSuite) startMCPServer(ctx context.Context) *MCPServerProcess {
	return startMCPServerProcess(ctx, suite.tmpDir)
}

func (suite *MCPToolCompletenessTestSuite) sendMCPRequest(stdin *os.File, stdout *os.File, request map[string]interface{}) map[string]interface{} {
	return sendMCPRequest(stdin, stdout, request, suite.T())
}

func (suite *MCPToolCompletenessTestSuite) createTestRepository() string {
	// Create a simple Go repository for testing
	repoDir := filepath.Join(suite.tmpDir, "test-go-repo")
	err := os.MkdirAll(repoDir, 0755)
	require.NoError(suite.T(), err)

	// Create go.mod
	goMod := `module test-app

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
)
`
	err = os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(suite.T(), err)

	// Create main.go
	mainGo := `package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	log.Println("Server starting on :8080")
	r.Run(":8080")
}
`
	err = os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644)
	require.NoError(suite.T(), err)

	return repoDir
}

// extractToolResult and validateToolResponse functions removed as dead code

// listAndDiscoverTools calls tools/list and returns a set of tool names and the raw tools array
func (suite *MCPToolCompletenessTestSuite) listAndDiscoverTools(stdin *os.File, stdout *os.File, requestID int) (map[string]bool, []interface{}) {
	resp := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})

	// Basic structure assertions
	assert.Contains(suite.T(), resp, "result", "tools/list should return a result")
	result, _ := resp["result"].(map[string]interface{})
	toolsRaw, ok := result["tools"]
	if !ok {
		return map[string]bool{}, []interface{}{}
	}
	toolsArray, _ := toolsRaw.([]interface{})

	// Extract tool names
	discovered := make(map[string]bool)
	for _, tr := range toolsArray {
		if tool, ok := tr.(map[string]interface{}); ok {
			if name, ok := tool["name"].(string); ok {
				discovered[name] = true
			}
		}
	}
	return discovered, toolsArray
}

// Test runner
func TestMCPToolCompletenessIntegration(t *testing.T) {
	suite.Run(t, new(MCPToolCompletenessTestSuite))
}
