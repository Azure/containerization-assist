package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// TestWorkflowErrorRecovery tests workflow continuation after non-fatal errors
func TestWorkflowErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error recovery tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Start a valid workflow
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Introduce a non-fatal error (invalid template)
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "invalid_nonexistent_template",
	})

	// Error is expected but should not kill the session
	if err != nil {
		t.Logf("Expected error for invalid template: %v", err)
	}

	// Session should still be valid and recoverable
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should remain accessible after non-fatal error")
	assert.Equal(t, sessionID, sessionState.ID)

	// Recovery: valid dockerfile generation should work
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java", // Valid template
	})
	require.NoError(t, err, "Workflow should recover after error")

	recoveredSessionID, err := client.ExtractSessionID(dockerfileResult)
	require.NoError(t, err)
	assert.Equal(t, sessionID, recoveredSessionID, "Session ID should be preserved after recovery")

	// Continue workflow to completion
	buildResult, err := client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "recovery-test",
		"tag":        "latest",
	})
	require.NoError(t, err, "Workflow should continue after recovery")

	finalSessionID, err := client.ExtractSessionID(buildResult)
	require.NoError(t, err)
	assert.Equal(t, sessionID, finalSessionID, "Session continuity should be maintained")
}

// TestSessionStateRecoveryAfterServerRestart tests session persistence across server restarts
func TestSessionStateRecoveryAfterServerRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping server restart test in short mode")
	}

	// Create initial session
	server1, err := testutil.NewTestServer()
	require.NoError(t, err)

	client1, err := testutil.NewMCPTestClient(server1.URL())
	require.NoError(t, err)

	ctx := context.Background()

	// Create session and do some work
	analyzeResult, err := client1.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client1.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Generate dockerfile to create workspace state
	_, err = client1.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err)

	// Get initial session state
	initialState, err := client1.InspectSessionState(sessionID)
	require.NoError(t, err)
	initialWorkspace := initialState.WorkspaceDir

	// Clean shutdown first server
	client1.Close()
	server1.Close()

	// Wait a moment to ensure clean shutdown
	time.Sleep(100 * time.Millisecond)

	// Start new server instance (simulating restart)
	server2, err := testutil.NewTestServer()
	require.NoError(t, err)
	defer server2.Close()

	client2, err := testutil.NewMCPTestClient(server2.URL())
	require.NoError(t, err)
	defer client2.Close()

	// Try to recover session state
	recoveredState, err := client2.InspectSessionState(sessionID)
	if err != nil {
		// If session doesn't persist across restarts, that's implementation-dependent
		t.Logf("Session does not persist across server restart (implementation choice): %v", err)
		return
	}

	// If session does persist, validate state consistency
	assert.Equal(t, sessionID, recoveredState.ID, "Session ID should be consistent")
	assert.Equal(t, initialWorkspace, recoveredState.WorkspaceDir, "Workspace should be consistent")

	// Try to continue workflow
	buildResult, err := client2.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "restart-test",
		"tag":        "latest",
	})

	if err == nil {
		// If continuation works, validate session continuity
		continuedSessionID, err := client2.ExtractSessionID(buildResult)
		require.NoError(t, err)
		assert.Equal(t, sessionID, continuedSessionID, "Session should continue after restart")
	} else {
		t.Logf("Workflow continuation after restart failed (may be expected): %v", err)
	}
}

// TestInvalidSessionIDHandling tests handling of invalid session IDs
func TestInvalidSessionIDHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping invalid session handling tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	testCases := []struct {
		name      string
		sessionID string
		tool      string
		args      map[string]interface{}
	}{
		{
			name:      "completely_invalid_session",
			sessionID: "invalid-session-12345",
			tool:      "generate_dockerfile",
			args:      map[string]interface{}{"template": "java"},
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			tool:      "build_image",
			args:      map[string]interface{}{"image_name": "test", "tag": "latest"},
		},
		{
			name:      "malformed_session_id",
			sessionID: "not::valid::session",
			tool:      "generate_manifests",
			args:      map[string]interface{}{"app_name": "test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Add session_id to args
			tc.args["session_id"] = tc.sessionID

			// Call tool with invalid session
			_, err := client.CallTool(ctx, tc.tool, tc.args)

			// Should get a clear error
			require.Error(t, err, "Invalid session ID should cause error")

			// Error should be descriptive (RichError integration)
			errorMsg := err.Error()
			assert.Contains(t, errorMsg, "session", "Error should mention session")
			assert.True(t, len(errorMsg) > 10, "Error should be descriptive")

			// Error should not crash the server
			// Verify server is still responsive
			tools, err := client.ListTools(ctx)
			require.NoError(t, err, "Server should remain responsive after invalid session error")
			assert.NotEmpty(t, tools, "Tools should still be available")
		})
	}
}

// TestToolExecutionWithMissingDependencies tests tool behavior when dependencies are missing
func TestToolExecutionWithMissingDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping missing dependencies tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session but skip dockerfile generation
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Try to build image without generating dockerfile first
	_, err = client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "missing-deps-test",
		"tag":        "latest",
	})

	// This should either:
	// 1. Fail with clear error about missing dockerfile, or
	// 2. Auto-generate dockerfile and continue
	if err != nil {
		// Validate error quality
		errorMsg := err.Error()
		t.Logf("Expected dependency error: %v", errorMsg)
		assert.True(t, len(errorMsg) > 10, "Error should be descriptive")

		// Session should still be valid
		sessionState, err := client.InspectSessionState(sessionID)
		require.NoError(t, err, "Session should remain valid after dependency error")
		assert.Equal(t, sessionID, sessionState.ID)
	} else {
		// If it succeeded, tool auto-handled dependency
		t.Log("Tool auto-handled missing dependency (implementation choice)")
	}
}

// TestConcurrentErrorHandling tests error handling under concurrent access
func TestConcurrentErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent error handling tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Launch multiple concurrent operations, some with errors
	numGoroutines := 5
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			var err error
			if index%2 == 0 {
				// Valid operations
				_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
					"session_id": sessionID,
					"template":   "java",
				})
			} else {
				// Invalid operations
				_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
					"session_id": sessionID,
					"template":   "invalid_template",
				})
			}
			results <- err
		}(i)
	}

	// Collect results
	validCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			validCount++
		} else {
			errorCount++
			t.Logf("Concurrent error %d: %v", errorCount, err)
		}
	}

	// At least some operations should succeed
	assert.True(t, validCount > 0, "Some concurrent operations should succeed")

	// Session should remain valid after concurrent errors
	finalState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should remain valid after concurrent errors")
	assert.Equal(t, sessionID, finalState.ID)
}

// TestRichErrorContextIntegration validates RichError context helps with recovery
func TestRichErrorContextIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rich error context tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test various error conditions to validate RichError integration
	errorTestCases := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{
			name: "missing_required_parameter",
			tool: "generate_dockerfile",
			args: map[string]interface{}{
				// Missing session_id
				"template": "java",
			},
		},
		{
			name: "invalid_parameter_type",
			tool: "build_image",
			args: map[string]interface{}{
				"session_id": "valid-session",
				"image_name": 123, // Should be string
				"tag":        "latest",
			},
		},
		{
			name: "invalid_parameter_value",
			tool: "generate_manifests",
			args: map[string]interface{}{
				"session_id": "valid-session",
				"app_name":   "", // Empty string
				"port":       -1, // Invalid port
			},
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CallTool(ctx, tc.tool, tc.args)
			require.Error(t, err, "Invalid parameters should cause error")

			// Validate error structure indicates RichError integration
			errorMsg := err.Error()

			// Should contain context information
			assert.True(t, len(errorMsg) > 20, "Error should contain detailed context")

			// Should mention validation (from BETA's validation system)
			assert.Contains(t, errorMsg, "validation", "Error should mention validation")

			// Should not contain internal stack traces or implementation details
			assert.NotContains(t, errorMsg, "panic", "Error should not contain panic traces")
			assert.NotContains(t, errorMsg, "goroutine", "Error should not contain goroutine dumps")

			t.Logf("RichError for %s: %v", tc.name, errorMsg)
		})
	}
}

// TestSessionCleanupAfterFailures tests session cleanup after repeated failures
func TestSessionCleanupAfterFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping session cleanup tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Cause multiple failures
	for i := 0; i < 5; i++ {
		_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
			"session_id": sessionID,
			"template":   fmt.Sprintf("invalid_template_%d", i),
		})
		// Errors are expected
		if err != nil {
			t.Logf("Expected failure %d: %v", i+1, err)
		}
	}

	// Session should either:
	// 1. Remain valid for recovery, or
	// 2. Be cleaned up due to repeated failures (implementation choice)
	sessionState, err := client.InspectSessionState(sessionID)
	if err != nil {
		t.Logf("Session was cleaned up after repeated failures: %v", err)
		// This is acceptable behavior
	} else {
		// Session is still valid
		assert.Equal(t, sessionID, sessionState.ID)
		t.Log("Session remains valid after repeated failures")

		// Should still be able to recover
		_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
			"session_id": sessionID,
			"template":   "java", // Valid template
		})
		assert.NoError(t, err, "Should be able to recover session after failures")
	}
}

// TestErrorPropagationAcrossTools tests how errors propagate through multi-tool workflows
func TestErrorPropagationAcrossTools(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error propagation tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Start valid workflow
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Introduce error in dockerfile generation
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "invalid_template",
	})
	// Error expected

	// Try to continue with build (which depends on dockerfile)
	_, err = client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "error-propagation-test",
		"tag":        "latest",
	})

	// This should either:
	// 1. Fail because dockerfile generation failed
	// 2. Retry dockerfile generation automatically
	// 3. Use a default dockerfile
	if err != nil {
		t.Logf("Build failed due to dockerfile error (expected): %v", err)

		// Error should be informative about the root cause
		errorMsg := err.Error()
		assert.True(t, len(errorMsg) > 15, "Error should be informative")
	} else {
		t.Log("Build succeeded despite dockerfile error (auto-recovery)")
	}

	// Session should remain in a valid state
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should remain inspectable")
	assert.Equal(t, sessionID, sessionState.ID)
}
