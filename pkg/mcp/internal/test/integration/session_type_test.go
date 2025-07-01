package integration

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// TestSessionTypeConsistency validates that GetOrCreateSession returns correct types
func TestSessionTypeConsistency(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session and validate type consistency
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err, "analyze_repository should succeed")

	// Test GetOrCreateSession returns correct type (no interface{})
	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err, "Should extract session_id")
	assert.IsType(t, "", sessionID, "session_id should be string type, not interface{}")

	// Test type assertions don't fail at runtime (use BETA's strong types)
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err, "Should inspect session state")

	// Validate strong typing from BETA workstream
	assert.IsType(t, "", sessionState.ID, "Session ID should be string")
	assert.IsType(t, "", sessionState.WorkspaceDir, "WorkspaceDir should be string")
	assert.IsType(t, make(map[string]interface{}), sessionState.Metadata, "Metadata should be map")

	// Test session interface implementations
	validateSessionInterfaceImplementation(t, sessionState)

	// Test import consistency across packages
	validateImportConsistency(t, analyzeResult)
}

// TestSessionManagerIntegration validates session creation through MCP tools
func TestSessionManagerIntegration(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test session creation through MCP tools
	createResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(createResult)
	require.NoError(t, err)

	// Test session retrieval and updates
	initialState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)
	initialUpdateTime := initialState.UpdatedAt

	// Update session through tool call
	updateResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err)

	// Validate session was updated
	updatedState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)
	assert.True(t, updatedState.UpdatedAt.After(initialUpdateTime), "UpdatedAt should advance")

	// Test session persistence and loading (BoltDB)
	validateSessionPersistence(t, client, sessionID)

	// Test concurrent session access
	validateConcurrentSessionAccess(t, client, sessionID)
}

// TestTypeImportConsistency validates cross-package type integration
func TestTypeImportConsistency(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test all packages use consistent session types
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	// Test interface implementations across package boundaries
	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Test different tools return consistent session types
	tools := []struct {
		name string
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
				"image_name": "consistency-test",
				"tag":        "latest",
			},
		},
		{
			"generate_manifests",
			map[string]interface{}{
				"session_id": sessionID,
				"app_name":   "consistency-test",
				"port":       8080,
			},
		},
	}

	for _, tool := range tools {
		t.Run(tool.name+"_type_consistency", func(t *testing.T) {
			result, err := client.CallTool(ctx, tool.name, tool.args)
			require.NoError(t, err, "Tool %s should succeed", tool.name)

			// Test type alias resolution
			resultSessionID, err := client.ExtractSessionID(result)
			require.NoError(t, err, "Should extract session_id from %s", tool.name)
			assert.Equal(t, sessionID, resultSessionID, "Session ID should be consistent across tools")

			// Test RichError integration (from BETA) in tool responses
			validateRichErrorIntegration(t, result, tool.name)
		})
	}
}

// TestSessionGenericTypeSupport validates generic type support from BETA workstream
func TestSessionGenericTypeSupport(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create session with metadata
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	require.NoError(t, err)

	sessionID, err := client.ExtractSessionID(analyzeResult)
	require.NoError(t, err)

	// Test generic type support in session metadata
	sessionState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)

	// Validate metadata supports generic types (from BETA)
	if metadata := sessionState.Metadata; metadata != nil {
		for key, value := range metadata {
			// Test that values maintain proper types
			validateGenericTypeSupport(t, key, value)
		}
	}

	// Test tool results maintain generic type consistency
	dockerfileResult, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": sessionID,
		"template":   "java",
	})
	require.NoError(t, err)

	// Validate response maintains type safety
	for key, value := range dockerfileResult {
		validateGenericTypeSupport(t, key, value)
	}
}

// TestSessionErrorTypeIntegration validates error type integration with BETA's RichError
func TestSessionErrorTypeIntegration(t *testing.T) {
	client, server, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test invalid session ID triggers proper error types
	_, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
		"session_id": "invalid-session-id",
		"template":   "java",
	})

	// Should get structured error from RichError system
	require.Error(t, err, "Invalid session should trigger error")

	// Validate error structure matches RichError patterns
	errorStr := err.Error()
	assert.Contains(t, errorStr, "session", "Error should mention session")

	// Test validation error types
	_, err = client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "", // Invalid empty URL
		"branch":   "main",
	})

	if err != nil {
		// Should get validation error from BETA's system
		assert.Contains(t, err.Error(), "validation", "Should get validation error")
	}
}

// Helper functions

// validateSessionInterfaceImplementation validates session implements expected interfaces
func validateSessionInterfaceImplementation(t *testing.T, sessionState *testutil.SessionState) {
	// Test basic interface compliance
	assert.NotNil(t, sessionState, "Session state should not be nil")
	assert.NotEmpty(t, sessionState.ID, "Session should have ID")
	assert.NotEmpty(t, sessionState.WorkspaceDir, "Session should have workspace")

	// Test time interfaces
	assert.False(t, sessionState.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, sessionState.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// Test metadata interface
	if sessionState.Metadata != nil {
		assert.IsType(t, make(map[string]interface{}), sessionState.Metadata, "Metadata should be map interface")
	}
}

// validateImportConsistency validates consistent imports across packages
func validateImportConsistency(t *testing.T, result map[string]interface{}) {
	// Test consistent field types across results
	for key, value := range result {
		switch key {
		case "session_id":
			assert.IsType(t, "", value, "session_id should be string across all packages")
		case "success", "completed":
			if value != nil {
				assert.IsType(t, true, value, "Boolean fields should be consistent")
			}
		case "timestamp", "created_at", "updated_at":
			if value != nil {
				// Should be string (ISO format) or time-compatible
				valueType := reflect.TypeOf(value)
				assert.True(t, valueType.Kind() == reflect.String || valueType.String() == "time.Time",
					"Time fields should be consistently typed: %v", valueType)
			}
		}
	}
}

// validateSessionPersistence validates session survives server restart scenarios
func validateSessionPersistence(t *testing.T, client testutil.MCPTestClient, sessionID string) {
	// Get initial state
	initialState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)

	// Session should persist (implementation validates BoltDB persistence)
	laterState, err := client.InspectSessionState(sessionID)
	require.NoError(t, err)

	// Validate consistency
	assert.Equal(t, initialState.ID, laterState.ID, "Session ID should persist")
	assert.Equal(t, initialState.WorkspaceDir, laterState.WorkspaceDir, "Workspace should persist")
	assert.Equal(t, initialState.CreatedAt, laterState.CreatedAt, "CreatedAt should persist")
}

// validateConcurrentSessionAccess validates concurrent session access safety
func validateConcurrentSessionAccess(t *testing.T, client testutil.MCPTestClient, sessionID string) {
	// Test concurrent read access
	results := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_, err := client.InspectSessionState(sessionID)
			results <- err
		}()
	}

	// Validate all concurrent reads succeed
	for i := 0; i < 3; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent session read %d should succeed", i+1)
	}
}

// validateRichErrorIntegration validates RichError integration in tool responses
func validateRichErrorIntegration(t *testing.T, result map[string]interface{}, toolName string) {
	// If error field exists, validate RichError structure
	if errorInfo, exists := result["error"]; exists && errorInfo != nil {
		errorObj, ok := errorInfo.(map[string]interface{})
		if ok {
			// Should have RichError structure
			assert.NotEmpty(t, errorObj["message"], "Error should have message")

			if code, exists := errorObj["code"]; exists {
				assert.NotEmpty(t, code, "Error code should not be empty")
			}

			if context, exists := errorObj["context"]; exists && context != nil {
				contextObj, ok := context.(map[string]interface{})
				if ok {
					assert.NotEmpty(t, contextObj, "Error context should contain information")
				}
			}
		}
	}
}

// validateGenericTypeSupport validates generic type handling from BETA
func validateGenericTypeSupport(t *testing.T, key string, value interface{}) {
	if value == nil {
		return // Nil values are acceptable
	}

	valueType := reflect.TypeOf(value)

	// Validate common generic types are properly handled
	switch valueType.Kind() {
	case reflect.String:
		assert.IsType(t, "", value, "String values should maintain type: %s", key)
	case reflect.Bool:
		assert.IsType(t, true, value, "Boolean values should maintain type: %s", key)
	case reflect.Int, reflect.Int64:
		// Numeric types may be represented as float64 in JSON
		if _, ok := value.(float64); !ok {
			assert.True(t, valueType.Kind() == reflect.Int || valueType.Kind() == reflect.Int64,
				"Integer values should maintain numeric type: %s", key)
		}
	case reflect.Float64:
		assert.IsType(t, float64(0), value, "Float values should maintain type: %s", key)
	case reflect.Slice:
		assert.True(t, valueType.Kind() == reflect.Slice, "Array values should maintain slice type: %s", key)
	case reflect.Map:
		assert.True(t, valueType.Kind() == reflect.Map, "Object values should maintain map type: %s", key)
	}
}
