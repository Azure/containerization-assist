//go:build integration_error_recovery

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

// TestWorkflowErrorRecovery tests that workflows can recover from transient errors
func (suite *ErrorRecoveryIntegrationSuite) TestWorkflowErrorRecovery() {
	suite.T().Log("Testing workflow error recovery mechanisms")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	// Initialize server
	suite.initializeServer(stdin, stdout)

	// Test recovery scenarios
	errorScenarios := []struct {
		name        string
		repoConfig  map[string]string
		expectError bool
		shouldRetry bool
	}{
		{
			name: "InvalidDockerfileSyntax",
			repoConfig: map[string]string{
				"dockerfile_error": "syntax_error",
			},
			expectError: true,
			shouldRetry: true,
		},
		{
			name: "MissingDependencies",
			repoConfig: map[string]string{
				"missing_deps": "go_mod_error",
			},
			expectError: true,
			shouldRetry: true,
		},
		{
			name: "NetworkTimeout",
			repoConfig: map[string]string{
				"network_error": "timeout",
			},
			expectError: true,
			shouldRetry: true,
		},
	}

	for _, scenario := range errorScenarios {
		suite.Run(scenario.name, func() {
			repoDir := suite.createErrorProneRepository(scenario.repoConfig)

			response := sendMCPRequest(stdin, stdout, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      time.Now().Unix(),
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "containerize_and_deploy",
					"arguments": map[string]interface{}{
						"repo_url":       "file://" + repoDir,
						"branch":         "main",
						"scan":           false,
						"deploy":         false,
						"test_mode":      true,
						"retry_on_error": scenario.shouldRetry,
					},
				},
			}, suite.T())

			// Validate error handling
			suite.validateErrorRecovery(response, scenario)
		})
	}

	suite.T().Log("✓ Workflow error recovery mechanisms verified")
}

// TestProgressiveErrorContext tests that error context accumulates properly
func (suite *ErrorRecoveryIntegrationSuite) TestProgressiveErrorContext() {
	suite.T().Log("Testing progressive error context accumulation")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	suite.initializeServer(stdin, stdout)

	// Create repository that will cause multiple sequential errors
	repoDir := suite.createMultiErrorRepository()

	response := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "containerize_and_deploy",
			"arguments": map[string]interface{}{
				"repo_url":        "file://" + repoDir,
				"branch":          "main",
				"scan":            false,
				"deploy":          false,
				"test_mode":       true,
				"max_retry_count": 3,
			},
		},
	}, suite.T())

	// Validate error context accumulation
	suite.validateProgressiveErrorContext(response)

	suite.T().Log("✓ Progressive error context accumulation verified")
}

// TestErrorEscalation tests that errors escalate appropriately after multiple failures
func (suite *ErrorRecoveryIntegrationSuite) TestErrorEscalation() {
	suite.T().Log("Testing error escalation after multiple failures")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	suite.initializeServer(stdin, stdout)

	// Create repository that will cause persistent errors
	repoDir := suite.createPersistentErrorRepository()

	response := sendMCPRequest(stdin, stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "containerize_and_deploy",
			"arguments": map[string]interface{}{
				"repo_url":        "file://" + repoDir,
				"branch":          "main",
				"scan":            false,
				"deploy":          false,
				"test_mode":       true,
				"max_retry_count": 2,
			},
		},
	}, suite.T())

	// Validate error escalation behavior
	suite.validateErrorEscalation(response)

	suite.T().Log("✓ Error escalation behavior verified")
}

// TestGracefulDegradation tests that services degrade gracefully under failure conditions
func (suite *ErrorRecoveryIntegrationSuite) TestGracefulDegradation() {
	suite.T().Log("Testing graceful degradation under failure conditions")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	stdin := server.stdin
	stdout := server.stdout

	suite.initializeServer(stdin, stdout)

	// Test degradation scenarios
	degradationTests := []struct {
		name           string
		toolName       string
		args           map[string]interface{}
		expectedResult string
	}{
		{
			name:     "PingWithDegradedService",
			toolName: "ping",
			args: map[string]interface{}{
				"message": "degradation-test",
			},
			expectedResult: "should_respond",
		},
		{
			name:     "StatusWithPartialFailure",
			toolName: "server_status",
			args: map[string]interface{}{
				"details": true,
			},
			expectedResult: "partial_info",
		},
	}

	for _, test := range degradationTests {
		suite.Run(test.name, func() {
			response := sendMCPRequest(stdin, stdout, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      time.Now().Unix(),
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name":      test.toolName,
					"arguments": test.args,
				},
			}, suite.T())

			suite.validateGracefulDegradation(response, test)
		})
	}

	suite.T().Log("✓ Graceful degradation under failure conditions verified")
}

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

// Test runner
func TestErrorRecoveryIntegration(t *testing.T) {
	suite.Run(t, new(ErrorRecoveryIntegrationSuite))
}
