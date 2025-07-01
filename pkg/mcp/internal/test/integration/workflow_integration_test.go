package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// TestCompleteContainerizationWorkflow validates the complete containerization workflow
// This is the CRITICAL test for session continuity and multi-tool integration
func TestCompleteContainerizationWorkflow(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Step 1: analyze_repository - MUST return session_id
	t.Log("Step 1: Analyzing repository")
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err, "analyze_repository should succeed")

	// Extract and validate session ID
	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err, "analyze_repository must return session_id")
	require.NotEmpty(t, sessionID, "session_id must not be empty")

	t.Logf("Session ID created: %s", sessionID)

	// Validate analysis results
	err = client.ValidateToolResponse(analyzeResult, []string{"session_id", "language", "framework"})
	require.NoError(t, err, "analyze_repository should return required fields")

	// Step 2: generate_dockerfile - MUST use same session
	t.Log("Step 2: Generating Dockerfile")
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID, // CRITICAL: session continuity
		"template":   "java",
	})
	require.NoError(t, err, "generate_dockerfile should succeed")

	// CRITICAL: Validate session continuity
	resultSessionID, err := client.ExtractSessionID(dockerfileResult)
	require.NoError(t, err, "generate_dockerfile must return session_id")
	assert.Equal(t, sessionID, resultSessionID, "session_id must be preserved across tools")

	// Validate Dockerfile generation
	err = client.ValidateToolResponse(dockerfileResult, []string{"session_id", "dockerfile_path"})
	require.NoError(t, err, "generate_dockerfile should return required fields")

	// Step 3: Validate workspace persistence (files exist across tools)
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err, "Should be able to get session workspace")
	assert.NotEmpty(t, workspace, "Workspace should be provided")

	dockerfilePath := filepath.Join(workspace, "Dockerfile")
	assert.FileExists(t, dockerfilePath, "Dockerfile should exist in workspace")

	// Step 4: build_image - MUST use same session
	t.Log("Step 3: Building image")
	buildResult, err := client.CallTool(ctx, "build_image", map[string]interface{}{
		"session_id": sessionID,
		"image_name": "test-app",
		"tag":        "latest",
	})
	require.NoError(t, err, "build_image should succeed")

	// CRITICAL: Validate session continuity AND state sharing
	buildSessionID, err := client.ExtractSessionID(buildResult)
	require.NoError(t, err, "build_image must return session_id")
	assert.Equal(t, sessionID, buildSessionID, "session_id must be preserved")

	// Validate build success
	if success, exists := buildResult["success"]; exists {
		assert.True(t, success.(bool), "Build should succeed")
	}

	// Step 5: generate_manifests - complete workflow
	t.Log("Step 4: Generating Kubernetes manifests")
	manifestResult, err := client.CallTool(ctx, "generate_manifests", map[string]interface{}{
		"session_id": sessionID,
		"app_name":   "test-app",
		"port":       8080,
	})
	require.NoError(t, err, "generate_manifests should succeed")

	// Final validation
	manifestSessionID, err := client.ExtractSessionID(manifestResult)
	require.NoError(t, err, "generate_manifests must return session_id")
	assert.Equal(t, sessionID, manifestSessionID, "session_id must be preserved throughout workflow")

	// Validate complete workflow
	validateWorkflowCompletion(t, client, sessionID, manifestResult)
}

// TestWorkflowErrorRecovery tests workflow behavior when individual steps fail
func TestWorkflowErrorRecovery(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
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

	// Attempt invalid tool call
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "invalid_template",
	})

	// Workflow should handle error gracefully
	if err != nil {
		t.Logf("Expected error for invalid template: %v", err)
	}

	// Session should still be valid for recovery
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Session should still be inspectable after error")
	assert.Equal(t, sessionID, sessionState.ID, "Session should remain valid")

	// Recovery: valid call should work
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err, "Valid call should work after error recovery")

	recoverySessionID, err := client.ExtractSessionID(dockerfileResult)
	require.NoError(t, err)
	assert.Equal(t, sessionID, recoverySessionID, "Session should be preserved after recovery")
}

// TestConcurrentWorkflows validates multiple concurrent workflows don't interfere
func TestConcurrentWorkflows(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	numWorkflows := 3

	// Start multiple concurrent workflows
	sessionIDs := make([]string, numWorkflows)
	for i := 0; i < numWorkflows; i++ {
		analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": "https://github.com/spring-projects/spring-petclinic",
			"branch":   "main",
		})
		require.NoError(t, err, "Workflow %d: analyze_repository should succeed", i)

		sessionID, err := client.ExtractSessionID(analyzeResult)
		require.NoError(t, err, "Workflow %d: should get session_id", i)
		sessionIDs[i] = sessionID
	}

	// Validate all sessions are unique
	for i := 0; i < numWorkflows; i++ {
		for j := i + 1; j < numWorkflows; j++ {
			assert.NotEqual(t, sessionIDs[i], sessionIDs[j], "Sessions %d and %d should be unique", i, j)
		}
	}

	// Continue each workflow independently
	for i, sessionID := range sessionIDs {
		dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
			"session_id": sessionID,
			"template":   "java",
		})
		require.NoError(t, err, "Workflow %d: generate_dockerfile should succeed", i)

		resultSessionID, err := client.ExtractSessionID(dockerfileResult)
		require.NoError(t, err, "Workflow %d: should preserve session_id", i)
		assert.Equal(t, sessionID, resultSessionID, "Workflow %d: session continuity", i)
	}
}

// TestWorkflowWithInvalidSession tests behavior with invalid session IDs
func TestWorkflowWithInvalidSession(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test with completely invalid session ID
	_, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": "invalid-session-id",
		"template":   "java",
	})
	assert.Error(t, err, "Should fail with invalid session_id")

	// Test with empty session ID
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": "",
		"template":   "java",
	})
	assert.Error(t, err, "Should fail with empty session_id")

	// Test with missing session ID
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"template": "java",
	})
	assert.Error(t, err, "Should fail with missing session_id")
}

// TestWorkflowStateIsolation ensures different workflows don't share state
func TestWorkflowStateIsolation(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create first workflow
	analyzeResult1, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)
	sessionID1, err := client.ExtractSessionID(analyzeResult1)
	require.NoError(t, err)

	// Create second workflow
	analyzeResult2, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/expressjs/express",
		"branch":   "main",
	})
	require.NoError(t, err)
	sessionID2, err := client.ExtractSessionID(analyzeResult2)
	require.NoError(t, err)

	// Generate Dockerfiles for both
	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID1,
		"template":   "java",
	})
	require.NoError(t, err)

	_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID2,
		"template":   "node",
	})
	require.NoError(t, err)

	// Validate workspace isolation
	workspace1, err := client.GetSessionWorkspace(sessionID1)
	require.NoError(t, err)
	workspace2, err := client.GetSessionWorkspace(sessionID2)
	require.NoError(t, err)

	assert.NotEqual(t, workspace1, workspace2, "Workspaces should be isolated")

	// Check that files exist in respective workspaces
	dockerfile1 := filepath.Join(workspace1, "Dockerfile")
	dockerfile2 := filepath.Join(workspace2, "Dockerfile")

	assert.FileExists(t, dockerfile1, "Dockerfile should exist in workspace 1")
	assert.FileExists(t, dockerfile2, "Dockerfile should exist in workspace 2")

	// Validate content is different (Java vs Node)
	content1, err := os.ReadFile(dockerfile1)
	require.NoError(t, err)
	content2, err := os.ReadFile(dockerfile2)
	require.NoError(t, err)

	assert.NotEqual(t, string(content1), string(content2), "Dockerfile content should be different")
}

// Helper function to validate complete workflow
func validateWorkflowCompletion(t *testing.T, client testutil.MCPTestClient, sessionID string, manifestResult map[string]interface{}) {
	// Validate manifest generation
	err := client.ValidateToolResponse(manifestResult, []string{"session_id", "manifests"})
	assert.NoError(t, err, "generate_manifests should return required fields")

	// Validate workspace contains all expected files
	workspace, err := client.GetSessionWorkspace(sessionID)
	require.NoError(t, err)

	expectedFiles := []string{
		"Dockerfile",
		"deployment.yaml",
		"service.yaml",
	}

	for _, expectedFile := range expectedFiles {
		filePath := filepath.Join(workspace, expectedFile)
		assert.FileExists(t, filePath, "Expected file %s should exist", expectedFile)
	}

	// Validate session state
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, sessionState.ID)
	assert.NotEmpty(t, sessionState.WorkspaceDir)
	assert.NotZero(t, sessionState.CreatedAt)
	assert.NotZero(t, sessionState.UpdatedAt)
}
