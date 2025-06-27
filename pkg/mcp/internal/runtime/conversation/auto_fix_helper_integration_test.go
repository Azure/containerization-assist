package conversation

import (
	"testing"

	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestAutoFixHelperIntegration(t *testing.T) {
	t.Run("AttemptAutoFix with nil handler returns false", func(t *testing.T) {
		helper := NewAutoFixHelper(nil)

		response := &ConversationResponse{
			SessionID: "test-session-123",
			Status:    ResponseStatusError,
			Message:   "Initial error message",
		}
		state := &ConversationState{
			SessionState: sessiontypes.NewSessionState("test-session-123", "/tmp"),
		}

		result := helper.AttemptAutoFix(nil, response, types.StageBuild, nil, state)
		assert.False(t, result, "Should return false when handler is nil")
		assert.Equal(t, ResponseStatusError, response.Status, "Status should remain unchanged")
		assert.Equal(t, "Initial error message", response.Message, "Message should remain unchanged")
	})

	t.Run("Integration test for helper method coverage", func(t *testing.T) {
		helper := NewAutoFixHelper(nil)

		// Test all getSuccessOptions branches
		stages := []struct {
			stage         types.ConversationStage
			expectedLabel string
			expectedCount int
		}{
			{types.StageBuild, "Continue to next stage", 2},
			{types.StagePush, "Continue to manifest generation", 2},
			{types.StageManifests, "Continue to deployment", 2},
			{types.StageDeployment, "Continue to completion", 2},
			{types.StageWelcome, "Continue", 2},
			{types.StagePreFlight, "Continue", 2},
			{types.StageInit, "Continue", 2},
			{types.StageAnalysis, "Continue", 2},
			{types.StageDockerfile, "Continue", 2},
			{types.StageCompleted, "Continue", 2},
		}

		for _, tc := range stages {
			options := helper.getSuccessOptions(tc.stage)
			assert.Len(t, options, tc.expectedCount, "Should have correct number of options for stage %s", tc.stage)
			assert.Equal(t, tc.expectedLabel, options[0].Label, "First option label should match for stage %s", tc.stage)
			assert.True(t, options[0].Recommended, "First option should be recommended for stage %s", tc.stage)

			// Check second option label - varies by stage
			var expectedSecondLabel string
			switch tc.stage {
			case types.StageManifests:
				expectedSecondLabel = "Review generated manifests"
			case types.StageDeployment:
				expectedSecondLabel = "Review deployment status"
			default:
				expectedSecondLabel = "Review changes"
			}
			assert.Equal(t, expectedSecondLabel, options[1].Label, "Second option should be review for stage %s", tc.stage)
			assert.False(t, options[1].Recommended, "Second option should not be recommended for stage %s", tc.stage)
		}
	})

	t.Run("Integration test for display name coverage", func(t *testing.T) {
		stages := []struct {
			stage           types.ConversationStage
			expectedDisplay string
		}{
			{types.StageBuild, "Build"},
			{types.StagePush, "Push"},
			{types.StageManifests, "Manifest generation"},
			{types.StageDeployment, "Deployment"},
			{types.StageWelcome, "Operation"},
			{types.StagePreFlight, "Operation"},
			{types.StageInit, "Operation"},
			{types.StageAnalysis, "Operation"},
			{types.StageDockerfile, "Operation"},
			{types.StageCompleted, "Operation"},
		}

		for _, tc := range stages {
			displayName := getStageDisplayName(tc.stage)
			assert.Equal(t, tc.expectedDisplay, displayName, "Display name should match for stage %s", tc.stage)
		}
	})

	t.Run("Integration test for error prefix coverage", func(t *testing.T) {
		stages := []struct {
			stage          types.ConversationStage
			expectedPrefix string
		}{
			{types.StageBuild, "Build"},
			{types.StagePush, "Failed to push Docker image"},
			{types.StageManifests, "Failed to generate Kubernetes manifests"},
			{types.StageDeployment, "Deployment"},
			{types.StageWelcome, "Operation"},
			{types.StagePreFlight, "Operation"},
			{types.StageInit, "Operation"},
			{types.StageAnalysis, "Operation"},
			{types.StageDockerfile, "Operation"},
			{types.StageCompleted, "Operation"},
		}

		for _, tc := range stages {
			errorPrefix := getStageErrorPrefix(tc.stage)
			assert.Equal(t, tc.expectedPrefix, errorPrefix, "Error prefix should match for stage %s", tc.stage)
		}
	})

	t.Run("Integration test for full helper workflow", func(t *testing.T) {
		helper := NewAutoFixHelper(nil)

		// Test that the helper correctly handles all stages
		response := &ConversationResponse{
			SessionID: "test-session",
			Status:    ResponseStatusError,
			Message:   "Test error",
		}
		state := &ConversationState{
			SessionState: sessiontypes.NewSessionState("test-session", "/tmp"),
		}

		// Test with nil handler - should return false and not modify response
		originalStatus := response.Status
		originalMessage := response.Message

		result := helper.AttemptAutoFix(nil, response, types.StageBuild, nil, state)

		assert.False(t, result, "Should return false with nil handler")
		assert.Equal(t, originalStatus, response.Status, "Status should remain unchanged")
		assert.Equal(t, originalMessage, response.Message, "Message should remain unchanged")
		assert.Nil(t, response.Options, "Options should remain nil")
	})
}
