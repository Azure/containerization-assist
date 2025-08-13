//go:build integration_error_recovery

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ErrorRecoveryIntegrationSuite tests error handling and recovery mechanisms
type ErrorRecoveryIntegrationSuite struct {
	suite.Suite
	tmpDir string
}

func (suite *ErrorRecoveryIntegrationSuite) SetupSuite() {
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "error-recovery-test-")
	require.NoError(suite.T(), err)
}

func (suite *ErrorRecoveryIntegrationSuite) TearDownSuite() {
	if suite.tmpDir != "" {
		os.RemoveAll(suite.tmpDir)
	}
}

// TestWorkflowErrorRecovery tests recovery using the 10-step tools with redirect behavior
func (suite *ErrorRecoveryIntegrationSuite) TestWorkflowErrorRecovery() {
	suite.T().Log("Testing workflow error recovery with step tools")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	// Initialize server
	suite.initializeServer(stdin, stdout)

	// Create minimal repo
	repoDir := suite.createErrorProneRepository(map[string]string{})

	// Use a unique session id
	sessionID := fmt.Sprintf("recovery-%d", time.Now().UnixNano())

	// 1) analyze_repository (test_mode true to avoid AI sampling)
	analyzeResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"repo_path":  repoDir,
				"test_mode":  true,
			},
		},
	}, suite.T())
	assert.Contains(suite.T(), analyzeResp, "result")

	// SAD PATH: build_image WITHOUT dockerfile_result -> expect redirect to generate_dockerfile
	suite.T().Log("SAD PATH: build_image without Dockerfile should redirect to generate_dockerfile")
	buildFail := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "build_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				// test_mode can be true; failure is due to invalid state before build
				"test_mode": true,
			},
		},
	}, suite.T())
	// Extract text content and assert redirect hint
	redirectText := suite.extractFirstContentText(buildFail)
	require.NotEmpty(suite.T(), redirectText)
	assert.Contains(suite.T(), redirectText, "Tool build_image failed")
	assert.Contains(suite.T(), redirectText, "Call tool \"generate_dockerfile\"")

	// Recovery step: generate_dockerfile with provided content (valid minimal Dockerfile)
	dockerfileContent := strings.TrimSpace(`FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o app
EXPOSE 8080
CMD ["./app"]`)

	genResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "generate_dockerfile",
			"arguments": map[string]interface{}{
				"session_id":         sessionID,
				"dockerfile_content": dockerfileContent,
				"test_mode":          true,
			},
		},
	}, suite.T())
	assert.Contains(suite.T(), genResp, "result")

	// HAPPY PATH: build_image again (now should succeed in test_mode)
	suite.T().Log("HAPPY PATH: after generating Dockerfile, build_image succeeds and suggests next step")
	buildOk := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "build_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())
	okText := suite.extractFirstContentText(buildOk)
	require.NotEmpty(suite.T(), okText)
	assert.Contains(suite.T(), okText, "build_image completed successfully")
	assert.Contains(suite.T(), okText, "**Next Step:** scan_image")

	suite.T().Log("✓ Redirect and recovery for build_image verified")
}

// TestDeployRedirectsToManifestsOnMissingPath ensures deploy errors redirect to generate_k8s_manifests and then succeed after providing manifests
func (suite *ErrorRecoveryIntegrationSuite) TestDeployRedirectsToManifestsOnMissingPath() {
	suite.T().Log("Testing deploy failure redirect to generate_k8s_manifests")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	suite.initializeServer(stdin, stdout)

	// Setup repo and session
	repoDir := suite.createMultiErrorRepository() // content not important here
	sessionID := fmt.Sprintf("deploy-%d", time.Now().UnixNano())

	// analyze_repository
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"repo_path":  repoDir,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// generate_dockerfile
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "generate_dockerfile",
			"arguments": map[string]interface{}{
				"session_id":         sessionID,
				"dockerfile_content": "FROM alpine:3.19\nCMD [\"sh\", \"-c\", \"sleep 1d\"]",
				"test_mode":          true,
			},
		},
	}, suite.T())

	// build_image (test mode success)
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "build_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// tag_image
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "tag_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"tag":        "latest",
				"test_mode":  true,
			},
		},
	}, suite.T())

	// push_image (prepares local ref)
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "push_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"registry":   "localhost:5001",
			},
		},
	}, suite.T())

	// SAD PATH: deploy_application BEFORE manifests to force error and redirect
	suite.T().Log("SAD PATH: deploy without manifests should redirect to generate_k8s_manifests")
	deployFail := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "deploy_application",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())
	red := suite.extractFirstContentText(deployFail)
	require.NotEmpty(suite.T(), red)
	assert.Contains(suite.T(), red, "Tool deploy_application failed")
	assert.Contains(suite.T(), red, "Call tool \"generate_k8s_manifests\"")

	// Recovery step: generate_k8s_manifests by providing content so manifest_path is set
	manifests := BasicK8sManifestsWithIngress()

	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "generate_k8s_manifests",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"manifests":  manifests,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// HAPPY PATH: deploy_application again (should succeed in test_mode)
	suite.T().Log("HAPPY PATH: after generating manifests, deploy_application succeeds")
	deployOk := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "deploy_application",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())
	ok := suite.extractFirstContentText(deployOk)
	require.NotEmpty(suite.T(), ok)
	assert.Contains(suite.T(), ok, "Deployment")

	suite.T().Log("✓ Redirect and recovery for deploy_application verified")
}

// TestWorkflowStatusErrorAccumulation verifies that failures are accumulated in workflow status
func (suite *ErrorRecoveryIntegrationSuite) TestWorkflowStatusErrorAccumulation() {
	suite.T().Log("Testing workflow_status accumulates failed steps across attempts")
	suite.T().Log("SAD PATH ONLY: intentionally trigger multiple failures and verify accumulation via workflow_status")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	suite.initializeServer(stdin, stdout)

	// Create a simple repo and session
	repoDir := suite.createErrorProneRepository(map[string]string{})
	sessionID := fmt.Sprintf("status-%d", time.Now().UnixNano())

	// analyze_repository
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "analyze_repository",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"repo_path":  repoDir,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// Intentionally cause two failures to accumulate:
	// 1) build_image before generating a Dockerfile
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "build_image",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// 2) deploy_application before generating manifests
	_ = sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "deploy_application",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
				"test_mode":  true,
			},
		},
	}, suite.T())

	// Query workflow_status to verify error accumulation for this session
	statusResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "workflow_status",
			"arguments": map[string]interface{}{
				"session_id": sessionID,
			},
		},
	}, suite.T())

	// Prefer structured check if server returns JSON; otherwise fall back to text contains
	var failedSteps []string
	if resultRaw, ok := statusResp["result"]; ok {
		if parsed := suite.extractToolResult(resultRaw); parsed != nil {
			if fs, ok := parsed["failed_steps"].([]interface{}); ok {
				for _, s := range fs {
					if name, ok := s.(string); ok {
						failedSteps = append(failedSteps, name)
					}
				}
			}
		}
	}

	if len(failedSteps) > 0 {
		assert.GreaterOrEqual(suite.T(), len(failedSteps), 2, "expected at least two failed steps recorded")
		assert.Contains(suite.T(), failedSteps, "build_image")
		assert.Contains(suite.T(), failedSteps, "deploy_application")
	} else {
		// Fallback to human-readable content assertions
		text := suite.extractFirstContentText(statusResp)
		require.NotEmpty(suite.T(), text)
		assert.Contains(suite.T(), text, "build_image")
		assert.Contains(suite.T(), text, "deploy_application")
	}

	suite.T().Log("✓ workflow_status reflects accumulated failures for the session")
}

//

// Helper methods

func (suite *ErrorRecoveryIntegrationSuite) startMCPServer(ctx context.Context) *MCPServerProcess {
	return startMCPServerProcess(ctx, suite.tmpDir)
}

func (suite *ErrorRecoveryIntegrationSuite) initializeServer(stdin, stdout *os.File) {
	initResp := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    "error-recovery-test",
				"version": "1.0.0",
			},
		},
	}, suite.T())

	require.Contains(suite.T(), initResp, "result")
}

func (suite *ErrorRecoveryIntegrationSuite) createErrorProneRepository(config map[string]string) string {
	repoDir := filepath.Join(suite.tmpDir, fmt.Sprintf("error-repo-%d", time.Now().Unix()))
	require.NoError(suite.T(), os.MkdirAll(repoDir, 0755))

	var mainGo string
	var goMod string

	// Create different types of errors based on config
	if errorType, exists := config["dockerfile_error"]; exists && errorType == "syntax_error" {
		// Create code that will generate a Dockerfile with syntax errors
		mainGo = `package main

import "net/http"

func main() {
	// This will cause dockerfile generation issues
	http.ListenAndServe(":8080", nil)
}
`
		goMod = `module error-prone-app

go 1.21

require (
	invalid-dependency-name v1.0.0 // This will cause module resolution errors
)
`
	} else if errorType, exists := config["missing_deps"]; exists && errorType == "go_mod_error" {
		mainGo = `package main

import (
	"nonexistent/package" // This import will cause build errors
)

func main() {
	package.DoSomething()
}
`
		goMod = `module missing-deps-app

go 1.21
`
	} else {
		// Default working app
		mainGo = `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World!")
	})
	http.ListenAndServe(":8080", nil)
}
`
		goMod = `module test-app

go 1.21
`
	}

	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))

	return repoDir
}

func (suite *ErrorRecoveryIntegrationSuite) createMultiErrorRepository() string {
	repoDir := filepath.Join(suite.tmpDir, "multi-error-repo")
	require.NoError(suite.T(), os.MkdirAll(repoDir, 0755))

	// Create an app with multiple potential issues
	mainGo := `package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Multi-error test app")
	})
	
	fmt.Printf("Server starting on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
`

	// Include problematic dependencies that might cause build issues
	goMod := `module multi-error-app

go 1.21

require (
	github.com/nonexistent/dependency v1.0.0
)
`

	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))

	return repoDir
}

func (suite *ErrorRecoveryIntegrationSuite) createPersistentErrorRepository() string {
	repoDir := filepath.Join(suite.tmpDir, "persistent-error-repo")
	require.NoError(suite.T(), os.MkdirAll(repoDir, 0755))

	// Create a completely broken app that cannot be containerized
	mainGo := `package main

// Missing imports and syntax errors that cannot be auto-fixed
func main( {
	missing_function_call()
	undefined_variable = "test"
}
`

	goMod := `module persistent-error-app

go 1.21

require (
	completely.invalid/module/name v999.999.999
)
`

	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))

	return repoDir
}

func (suite *ErrorRecoveryIntegrationSuite) validateErrorRecovery(response map[string]interface{}, scenario struct {
	name        string
	repoConfig  map[string]string
	expectError bool
	shouldRetry bool
}) {
	assert.Contains(suite.T(), response, "result", "Response should contain result for scenario %s", scenario.name)

	// Extract result
	if resultRaw, ok := response["result"]; ok && resultRaw != nil {
		result := suite.extractToolResult(resultRaw)
		if result != nil {
			// Check if errors were encountered and handled
			if steps, ok := result["steps"].([]interface{}); ok {
				errorStepsFound := false
				recoveryAttemptsFound := false

				for _, step := range steps {
					if stepMap, ok := step.(map[string]interface{}); ok {
						if status, ok := stepMap["status"].(string); ok {
							if status == "error" || status == "retry" {
								errorStepsFound = true
								suite.T().Logf("Found error step: %s", stepMap["name"])
							}
							if status == "retry" || status == "recovered" {
								recoveryAttemptsFound = true
								suite.T().Logf("Found recovery attempt: %s", stepMap["name"])
							}
						}
					}
				}

				if scenario.expectError {
					assert.True(suite.T(), errorStepsFound, "Should have found error steps for scenario %s", scenario.name)
				}

				if scenario.shouldRetry {
					assert.True(suite.T(), recoveryAttemptsFound, "Should have found recovery attempts for scenario %s", scenario.name)
				}
			}
		}
	}
}

func (suite *ErrorRecoveryIntegrationSuite) validateProgressiveErrorContext(response map[string]interface{}) {
	assert.Contains(suite.T(), response, "result")

	if resultRaw, ok := response["result"]; ok && resultRaw != nil {
		result := suite.extractToolResult(resultRaw)
		if result != nil {
			// Check if error context is accumulated
			if errorContext, ok := result["error_context"].(map[string]interface{}); ok {
				assert.Contains(suite.T(), errorContext, "errors", "Error context should contain error history")
				if errors, ok := errorContext["errors"].([]interface{}); ok {
					assert.Greater(suite.T(), len(errors), 0, "Should have accumulated errors")
					suite.T().Logf("Error context accumulated %d errors", len(errors))
				}
			}
		}
	}
}

func (suite *ErrorRecoveryIntegrationSuite) validateErrorEscalation(response map[string]interface{}) {
	assert.Contains(suite.T(), response, "result")

	if resultRaw, ok := response["result"]; ok && resultRaw != nil {
		result := suite.extractToolResult(resultRaw)
		if result != nil {
			// Check if escalation occurred
			if escalated, ok := result["escalated"].(bool); ok {
				assert.True(suite.T(), escalated, "Errors should have escalated after multiple failures")
			}

			// Check final status
			if success, ok := result["success"].(bool); ok {
				assert.False(suite.T(), success, "Workflow should have failed due to persistent errors")
			}
		}
	}
}

func (suite *ErrorRecoveryIntegrationSuite) validateGracefulDegradation(response map[string]interface{}, test struct {
	name           string
	toolName       string
	args           map[string]interface{}
	expectedResult string
}) {
	assert.Contains(suite.T(), response, "result", "Tool %s should respond even under degraded conditions", test.toolName)

	// Tools should respond even if some functionality is degraded
	if resultRaw, ok := response["result"]; ok && resultRaw != nil {
		result := suite.extractToolResult(resultRaw)
		if result != nil {
			switch test.expectedResult {
			case "should_respond":
				// Ping should always respond
				assert.Contains(suite.T(), result, "response", "Ping should respond even under degraded conditions")
			case "partial_info":
				// Status should provide at least basic info
				assert.Contains(suite.T(), result, "status", "Status should provide basic info even under degraded conditions")
			}
		}
	}
}

func (suite *ErrorRecoveryIntegrationSuite) extractToolResult(resultRaw interface{}) map[string]interface{} {
	if result, ok := resultRaw.(map[string]interface{}); ok {
		if content, ok := result["content"]; ok {
			if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
				if contentItem, ok := contentArray[0].(map[string]interface{}); ok {
					if text, ok := contentItem["text"].(string); ok {
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

// extractFirstContentText fetches the first text field from MCP result
func (suite *ErrorRecoveryIntegrationSuite) extractFirstContentText(resp map[string]interface{}) string {
	if resp == nil {
		return ""
	}
	res, ok := resp["result"].(map[string]interface{})
	if !ok {
		return ""
	}
	contentArr, ok := res["content"].([]interface{})
	if !ok || len(contentArr) == 0 {
		return ""
	}
	first, ok := contentArr[0].(map[string]interface{})
	if !ok {
		return ""
	}
	text, _ := first["text"].(string)
	return text
}

// Test runner
func TestErrorRecoveryIntegration(t *testing.T) {
	suite.Run(t, new(ErrorRecoveryIntegrationSuite))
}
