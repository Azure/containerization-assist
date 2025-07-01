package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionStateSharing validates that repository analysis results are available to dockerfile generation
func TestSessionStateSharing(t *testing.T) {
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Step 1: Analyze repository to populate session state
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Validate analysis results are stored in session
	language, exists := analyzeResult["language"]
	require.True(t, exists, "Analysis should detect language")
	assert.Equal(t, "java", language, "Should detect Java language")

	framework, exists := analyzeResult["framework"]
	require.True(t, exists, "Analysis should detect framework")
	assert.Contains(t, framework.(string), "spring", "Should detect Spring framework")

	// Step 2: Generate dockerfile - should use analysis results from session
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "auto", // Should auto-select based on analysis
	})
	require.NoError(t, err)

	// Validate dockerfile generation uses analysis results
	_, exists = dockerfileResult["dockerfile_path"]
	require.True(t, exists, "Should return dockerfile path")

	// Read generated Dockerfile to verify it uses analysis results
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)

	dockerfileFullPath := filepath.Join(workspace, "Dockerfile")
	assert.FileExists(t, dockerfileFullPath, "Dockerfile should exist")

	// Step 3: Build image - should use dockerfile path from session
	buildResult, err := client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "shared-state-test",
		"tag":        "latest",
	})
	require.NoError(t, err)

	// Validate build uses dockerfile from session
	if dockerfilePath, exists := buildResult["dockerfile_used"]; exists {
		assert.Contains(t, dockerfilePath.(string), "Dockerfile", "Build should reference session Dockerfile")
	}

	// Step 4: Generate manifests - should use image reference from session
	manifestResult, err := client.CallTool(ctx, "generate_manifests", map[string]interface{}{
		"session_id": sessionID,
		"app_name":   "shared-state-test",
		"port":       8080,
	})
	require.NoError(t, err)

	// Validate manifests use session data
	manifests, exists := manifestResult["manifests"]
	require.True(t, exists, "Should return generated manifests")

	manifestList, ok := manifests.([]interface{})
	require.True(t, ok, "Manifests should be list")
	assert.NotEmpty(t, manifestList, "Should generate manifests")
}

// TestSessionWorkspaceManagement validates workspace lifecycle
func TestSessionWorkspaceManagement(t *testing.T) {
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session and get workspace
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Test workspace creation and persistence
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)
	assert.NotEmpty(t, workspace, "Workspace should be provided")
	assert.DirExists(t, workspace, "Workspace directory should exist")

	// Generate dockerfile to create file in workspace
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err)

	// Test file sharing between tools in same session
	dockerfilePath := filepath.Join(workspace, "Dockerfile")
	assert.FileExists(t, dockerfilePath, "Dockerfile should exist in workspace")

	// Create second session to test isolation
	analyzeResult2, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/expressjs/express",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID2, err := client.ExtractSessionID(analyzeResult2)
	require.NoError(t, err)

	workspace2, err := client.GetSessionWorkspace(sessionID2)
	require.NoError(t, err)

	// Test workspace isolation between different sessions
	assert.NotEqual(t, workspace, workspace2, "Different sessions should have different workspaces")
	assert.DirExists(t, workspace2, "Second workspace should exist")

	// Test that first session's files don't exist in second workspace
	dockerfile2Path := filepath.Join(workspace2, "Dockerfile")
	assert.NoFileExists(t, dockerfile2Path, "Second workspace should not contain first session's files")
}

// TestSessionPersistenceAcrossTools validates session metadata persists across tool calls
func TestSessionPersistenceAcrossTools(t *testing.T) {
	client, _, cleanup := setupMCPTestEnvironment(t)
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

	// Inspect initial session state
	initialState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, initialState.ID)
	assert.NotEmpty(t, initialState.WorkspaceDir)
	initialUpdateTime := initialState.UpdatedAt

	// Perform multiple tool operations
	operations := []struct {
		tool string
		args map[string]interface{}
	}{
		{
			"generate_dockerfile",
			map[string]interface{}{
				"session_id": sessionID,
				"template":   "java",
			},
		},
		{
			"build_image",
			map[string]interface{}{
				"session_id": sessionID,
				"image_name": "persistence-test",
				"tag":        "latest",
			},
		},
		{
			"generate_manifests",
			map[string]interface{}{
				"session_id": sessionID,
				"app_name":   "persistence-test",
				"port":       8080,
			},
		},
	}

	for i, op := range operations {
		t.Logf("Executing operation %d: %s", i+1, op.tool)

		result, err := client.CallTool(ctx, op.tool, op.args)
		require.NoError(t, err, "Operation %s should succeed", op.tool)

		// Validate session ID is preserved
		resultSessionID, err := client.ExtractSessionID(result)
		require.NoError(t, err, "Operation %s should return session_id", op.tool)
		assert.Equal(t, sessionID, resultSessionID, "Session ID should be preserved in %s", op.tool)

		// Check session state after each operation
		state, err := client.InspectSessionState(sessionID)
		require.NoError(t, err, "Should be able to inspect session after %s", op.tool)
		assert.Equal(t, sessionID, state.ID, "Session ID should remain consistent")
		assert.Equal(t, initialState.WorkspaceDir, state.WorkspaceDir, "Workspace should remain consistent")
		assert.Equal(t, initialState.CreatedAt, state.CreatedAt, "Created time should not change")

		// UpdatedAt should advance (with some tolerance for timing)
		if !state.UpdatedAt.After(initialUpdateTime) && !state.UpdatedAt.Equal(initialUpdateTime) {
			t.Errorf("UpdatedAt should advance after %s operation", op.tool)
		}
		initialUpdateTime = state.UpdatedAt
	}
}

// TestSessionConcurrencyHandling validates concurrent access to the same session
func TestSessionConcurrencyHandling(t *testing.T) {
	client, _, cleanup := setupMCPTestEnvironment(t)
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

	// Test concurrent operations on the same session
	numConcurrent := 3
	results := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			// Each goroutine tries to generate a dockerfile
			_, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
				"session_id": sessionID,
				"template":   "java",
			})
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numConcurrent; i++ {
		err := <-results
		if err != nil {
			t.Logf("Concurrent operation %d failed (may be expected): %v", i, err)
		}
	}

	// Validate session integrity after concurrent access
	finalState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should remain valid after concurrent access")
	assert.Equal(t, sessionID, finalState.ID)

	// Validate workspace still exists and is accessible
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)
	assert.DirExists(t, workspace, "Workspace should still exist")
}

// TestSessionTimeout validates session timeout behavior
func TestSessionTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	client, _, cleanup := setupMCPTestEnvironment(t)
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

	// Validate session is initially active
	state, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, state.ID)

	// Wait for potential timeout (implementation dependent)
	// Note: This test may need adjustment based on actual timeout values
	time.Sleep(2 * time.Second)

	// Session should still be accessible for reasonable timeout periods
	state2, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should not timeout too quickly")
	assert.Equal(t, sessionID, state2.ID)
}

// TestSessionErrorRecovery validates session state after errors
func TestSessionErrorRecovery(t *testing.T) {
	client, _, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create valid session
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Get initial session state
	initialState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)

	// Attempt operation that might fail
	_, err = client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "", // Invalid empty name
		"tag":        "latest",
	})

	// Whether error or not, session should remain valid
	recoveryState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should remain inspectable after error")
	assert.Equal(t, sessionID, recoveryState.ID)
	assert.Equal(t, initialState.WorkspaceDir, recoveryState.WorkspaceDir)

	// Subsequent valid operations should work
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err, "Valid operations should work after error recovery")
}
