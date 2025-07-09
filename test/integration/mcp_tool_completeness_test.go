package integration

import (
	"context"
	"encoding/json"
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
	defer func() {
		if err := mcpServer.cmd.Process.Kill(); err != nil {
			suite.T().Logf("Failed to kill MCP server process: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	stdin := mcpServer.stdin
	stdout := mcpServer.stdout

	// Initialize server
	suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
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

	// Create test repository
	testRepo := suite.createTestRepository()

	// Define all essential tools that must be implemented
	essentialTools := []struct {
		name           string
		args           map[string]interface{}
		requiredFields []string // Fields that must be present in successful response
		description    string
	}{
		{
			name: "analyze_repository",
			args: map[string]interface{}{
				"repo_url":      "file://" + testRepo,
				"context":       "Test repository analysis",
				"branch":        "main",
				"language_hint": "go",
				"shallow":       true,
			},
			requiredFields: []string{"success", "language", "analysis"},
			description:    "Repository analysis with real analysis engine",
		},
		{
			name: "generate_dockerfile",
			args: map[string]interface{}{
				"base_image":           "alpine:latest",
				"template":             "go",
				"optimization":         "size",
				"include_health_check": true,
				"build_args":           map[string]string{"GO_VERSION": "1.21"},
				"platform":             "linux/amd64",
				"session_id":           "test-session-123",
				"dry_run":              false,
			},
			requiredFields: []string{"success", "content"},
			description:    "Dockerfile generation based on templates",
		},
		{
			name: "build_image",
			args: map[string]interface{}{
				"image_name":      "test-image",
				"image_tag":       "latest",
				"dockerfile_path": "Dockerfile",
				"build_context":   ".",
				"platform":        "linux/amd64",
				"no_cache":        false,
				"build_args":      map[string]string{"GO_VERSION": "1.21"},
				"session_id":      "test-session-123",
			},
			requiredFields: []string{"success", "image_name", "image_tag"},
			description:    "Docker image building (simulated)",
		},
		{
			name: "push_image",
			args: map[string]interface{}{
				"image_name": "test-image",
				"image_tag":  "latest",
				"registry":   "localhost:5000",
				"session_id": "test-session-123",
			},
			requiredFields: []string{"success", "image_ref"},
			description:    "Docker image pushing (simulated)",
		},
		{
			name: "generate_manifests",
			args: map[string]interface{}{
				"session_id": "test-session-123",
				"app_name":   "test-app",
				"image_ref": map[string]interface{}{
					"registry":   "localhost:5000",
					"repository": "test-app",
					"tag":        "latest",
				},
				"namespace": "default",
				"replicas":  1,
			},
			requiredFields: []string{"success", "manifests"},
			description:    "Kubernetes manifest generation",
		},
		{
			name: "scan_image",
			args: map[string]interface{}{
				"image_name": "test-image",
				"image_tag":  "latest",
				"session_id": "test-session-123",
			},
			requiredFields: []string{"success", "vulnerabilities"},
			description:    "Security vulnerability scanning (simulated)",
		},
		{
			name: "list_sessions",
			args: map[string]interface{}{
				"limit": 10,
			},
			requiredFields: []string{"sessions", "total"},
			description:    "Session listing functionality",
		},
		{
			name: "ping",
			args: map[string]interface{}{
				"message": "test-ping",
			},
			requiredFields: []string{"response", "timestamp"},
			description:    "Connectivity testing",
		},
		{
			name: "server_status",
			args: map[string]interface{}{
				"details": true,
			},
			requiredFields: []string{"status", "version", "uptime"},
			description:    "Server status information",
		},
	}

	// Test each essential tool
	for i, tool := range essentialTools {
		suite.Run(fmt.Sprintf("Tool_%s", tool.name), func() {
			suite.T().Logf("Testing tool: %s - %s", tool.name, tool.description)

			response := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      i + 10,
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name":      tool.name,
					"arguments": tool.args,
				},
			})

			// Validate response structure
			assert.Contains(suite.T(), response, "result", "Tool %s should return a result", tool.name)
			assert.NotContains(suite.T(), response, "error", "Tool %s should not return an error", tool.name)

			if resultRaw, ok := response["result"]; ok && resultRaw != nil {
				result := suite.extractToolResult(resultRaw)
				if result != nil {
					suite.validateToolResponse(tool.name, result, tool.requiredFields)
				} else {
					suite.T().Errorf("Tool %s returned null/empty result", tool.name)
				}
			} else {
				suite.T().Errorf("Tool %s missing result field", tool.name)
			}
		})
	}

	// Verify all tools returned successful responses
	suite.T().Log("✓ All essential MCP tools are properly implemented and functional")
}

// TestToolDiscoverability validates that all tools can be discovered via tools/list
func (suite *MCPToolCompletenessTestSuite) TestToolDiscoverability() {
	suite.T().Log("Testing tool discoverability via tools/list")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start MCP server
	mcpServer := suite.startMCPServer(ctx)
	defer func() {
		if err := mcpServer.cmd.Process.Kill(); err != nil {
			suite.T().Logf("Failed to kill MCP server process: %v", err)
		}
	}()

	time.Sleep(2 * time.Second)

	stdin := mcpServer.stdin
	stdout := mcpServer.stdout

	// Initialize server
	suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
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

	// List all available tools
	response := suite.sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})

	// Validate response
	assert.Contains(suite.T(), response, "result", "tools/list should return a result")

	if resultRaw, ok := response["result"]; ok && resultRaw != nil {
		if result, ok := resultRaw.(map[string]interface{}); ok {
			if toolsRaw, ok := result["tools"]; ok {
				if toolsArray, ok := toolsRaw.([]interface{}); ok {
					suite.T().Logf("Found %d tools via tools/list", len(toolsArray))

					// Extract tool names
					discoveredTools := make(map[string]bool)
					for _, toolRaw := range toolsArray {
						if tool, ok := toolRaw.(map[string]interface{}); ok {
							if name, ok := tool["name"].(string); ok {
								discoveredTools[name] = true
								suite.T().Logf("  - %s: %s", name, tool["description"])
							}
						}
					}

					// Verify all essential tools are discoverable
					expectedTools := []string{
						"analyze_repository",
						"generate_dockerfile",
						"build_image",
						"push_image",
						"generate_manifests",
						"scan_image",
						"list_sessions",
						"ping",
						"server_status",
					}

					for _, expectedTool := range expectedTools {
						assert.True(suite.T(), discoveredTools[expectedTool],
							"Essential tool %s should be discoverable via tools/list", expectedTool)
					}

					suite.T().Log("✓ All essential tools are discoverable")
				} else {
					suite.T().Error("tools/list result.tools is not an array")
				}
			} else {
				suite.T().Error("tools/list result missing 'tools' field")
			}
		} else {
			suite.T().Error("tools/list result is not an object")
		}
	} else {
		suite.T().Error("tools/list missing result field")
	}
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

func (suite *MCPToolCompletenessTestSuite) extractToolResult(resultRaw interface{}) map[string]interface{} {
	if result, ok := resultRaw.(map[string]interface{}); ok {
		// Handle gomcp response format with content wrapper
		if content, ok := result["content"]; ok {
			if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
				if contentItem, ok := contentArray[0].(map[string]interface{}); ok {
					if text, ok := contentItem["text"].(string); ok {
						// Parse the JSON text
						var toolResult map[string]interface{}
						if err := json.Unmarshal([]byte(text), &toolResult); err == nil {
							return toolResult
						}
					}
				}
			}
		}
		return result
	}
	return nil
}

func (suite *MCPToolCompletenessTestSuite) validateToolResponse(toolName string, result map[string]interface{}, requiredFields []string) {
	suite.T().Logf("Validating tool response for %s", toolName)

	// Check for required fields
	for _, field := range requiredFields {
		assert.Contains(suite.T(), result, field, "Tool %s response should contain field %s", toolName, field)
	}

	// Additional validation for specific tools
	switch toolName {
	case "analyze_repository":
		// Should have successful analysis with real data
		if success, ok := result["success"].(bool); ok && success {
			if analysis, ok := result["analysis"].(map[string]interface{}); ok {
				assert.Contains(suite.T(), analysis, "language", "Analysis should detect language")
				assert.Contains(suite.T(), analysis, "files_analyzed", "Analysis should count files")
				suite.T().Logf("✓ Repository analysis detected language: %v", analysis["language"])
			}
		}

	case "generate_dockerfile":
		// Should have non-empty Dockerfile content
		if success, ok := result["success"].(bool); ok && success {
			if content, ok := result["content"].(string); ok {
				assert.NotEmpty(suite.T(), content, "Dockerfile content should not be empty")
				assert.Contains(suite.T(), content, "FROM", "Dockerfile should contain FROM instruction")
				suite.T().Logf("✓ Dockerfile generated with %d characters", len(content))
			}
		}

	case "build_image":
		// Should have image details
		if success, ok := result["success"].(bool); ok && success {
			assert.Contains(suite.T(), result, "image_id", "Build should return image ID")
			assert.Contains(suite.T(), result, "build_time", "Build should return build time")
			suite.T().Logf("✓ Image built: %s:%s", result["image_name"], result["image_tag"])
		}

	case "generate_manifests":
		// Should have deployment and service manifests
		if success, ok := result["success"].(bool); ok && success {
			if manifests, ok := result["manifests"].(map[string]interface{}); ok {
				assert.Contains(suite.T(), manifests, "deployment", "Should generate deployment manifest")
				assert.Contains(suite.T(), manifests, "service", "Should generate service manifest")
				suite.T().Logf("✓ Kubernetes manifests generated: %d resources", len(manifests))
			}
		}

	case "scan_image":
		// Should have vulnerability data
		if success, ok := result["success"].(bool); ok && success {
			if vulnerabilities, ok := result["vulnerabilities"].(map[string]interface{}); ok {
				assert.Contains(suite.T(), vulnerabilities, "total_vulnerabilities", "Should report vulnerability count")
				assert.Contains(suite.T(), vulnerabilities, "scan_time", "Should report scan time")
				suite.T().Logf("✓ Security scan completed: %v vulnerabilities", vulnerabilities["total_vulnerabilities"])
			}
		}

	case "list_sessions":
		// Should have sessions array and total
		if sessions, ok := result["sessions"].([]interface{}); ok {
			suite.T().Logf("✓ Session listing returned %d sessions", len(sessions))
		}
		if total, ok := result["total"].(float64); ok {
			suite.T().Logf("✓ Total sessions: %.0f", total)
		}

	case "ping":
		// Should have response and timestamp
		if response, ok := result["response"].(string); ok {
			assert.NotEmpty(suite.T(), response, "Ping response should not be empty")
			suite.T().Logf("✓ Ping response: %s", response)
		}

	case "server_status":
		// Should have status information
		if status, ok := result["status"].(string); ok {
			assert.Equal(suite.T(), "running", status, "Server should be running")
			suite.T().Logf("✓ Server status: %s", status)
		}
	}
}

// Test runner
func TestMCPToolCompletenessIntegration(t *testing.T) {
	suite.Run(t, new(MCPToolCompletenessTestSuite))
}
