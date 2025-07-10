package conversation

import (
	"context"
	"testing"

	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/stretchr/testify/assert"
)

func TestAutoFixHelper(t *testing.T) {
	t.Parallel()
	t.Run("NewAutoFixHelper", func(t *testing.T) {
		t.Parallel()
		helper := NewAutoFixHelper(nil)
		assert.NotNil(t, helper, "Helper should not be nil")
		// Cannot access unexported field conversationHandler
	})

	t.Run("AttemptAutoFix with nil handler", func(t *testing.T) {
		t.Parallel()
		helper := NewAutoFixHelper(nil)
		response := &ConversationResponse{
			SessionID: "test-session",
			Status:    ResponseStatusError,
		}
		state := &ConversationState{}

		result := helper.AttemptAutoFix(context.Background(), response, domaintypes.StageBuild, nil, state)
		assert.False(t, result, "Should return false when handler is nil")
		assert.Equal(t, ResponseStatusError, response.Status, "Status should remain unchanged")
	})

	t.Run("getSuccessOptions for different stages", func(t *testing.T) {
		t.Parallel()
		helper := &AutoFixHelper{}

		testCases := []struct {
			stage    domaintypes.ConversationStage
			expected string
		}{
			{domaintypes.StageBuild, "Continue to next stage"},
			{domaintypes.StagePush, "Continue to manifest generation"},
			{domaintypes.StageManifests, "Continue to deployment"},
			{domaintypes.StageDeployment, "Continue to completion"},
			{domaintypes.StageWelcome, "Continue"},
		}

		for _, tc := range testCases {
			options := helper.getSuccessOptions(tc.stage)
			assert.NotEmpty(t, options, "Options should not be empty for stage %s", tc.stage)
			assert.Equal(t, tc.expected, options[0].Label, "First option label should match for stage %s", tc.stage)
			assert.True(t, options[0].Recommended, "First option should be recommended")
		}
	})

	t.Run("getStageDisplayName", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			stage    domaintypes.ConversationStage
			expected string
		}{
			{convertFromTypesStage(domaintypes.StageBuild), "Build"},
			{convertFromTypesStage(domaintypes.StagePush), "Push"},
			{convertFromTypesStage(domaintypes.StageManifests), "Manifest generation"},
			{convertFromTypesStage(domaintypes.StageDeployment), "Deployment"},
			{convertFromTypesStage(domaintypes.StageWelcome), "Operation"},
		}

		for _, tc := range testCases {
			name := getStageDisplayName(tc.stage)
			assert.Equal(t, tc.expected, name, "Display name should match for stage %s", tc.stage)
		}
	})

	t.Run("getStageErrorPrefix", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			stage    domaintypes.ConversationStage
			expected string
		}{
			{convertFromTypesStage(domaintypes.StageBuild), "Build"},
			{convertFromTypesStage(domaintypes.StagePush), "Failed to push Docker image"},
			{convertFromTypesStage(domaintypes.StageManifests), "Failed to generate Kubernetes manifests"},
			{convertFromTypesStage(domaintypes.StageDeployment), "Deployment"},
			{convertFromTypesStage(domaintypes.StageWelcome), "Operation"},
		}

		for _, tc := range testCases {
			prefix := getStageErrorPrefix(tc.stage)
			assert.Equal(t, tc.expected, prefix, "Error prefix should match for stage %s", tc.stage)
		}
	})

	t.Run("getSuccessOptions coverage for all branches", func(t *testing.T) {
		t.Parallel()
		helper := &AutoFixHelper{}
		allStages := []domaintypes.ConversationStage{
			convertFromTypesStage(domaintypes.StageBuild),
			convertFromTypesStage(domaintypes.StagePush),
			convertFromTypesStage(domaintypes.StageManifests),
			convertFromTypesStage(domaintypes.StageDeployment),
			convertFromTypesStage(domaintypes.StageWelcome),
			convertFromTypesStage(domaintypes.StagePreFlight),
		}

		for _, stage := range allStages {
			options := helper.getSuccessOptions(stage)
			assert.NotEmpty(t, options, "Options should not be empty for stage %s", stage)
			assert.Len(t, options, 2, "Should always have 2 options")
			assert.True(t, options[0].Recommended, "First option should be recommended")
			assert.False(t, options[1].Recommended, "Second option should not be recommended")
		}
	})

	t.Run("getStageDisplayName coverage for all branches", func(t *testing.T) {
		t.Parallel()

		allStages := []domaintypes.ConversationStage{
			convertFromTypesStage(domaintypes.StageBuild),
			convertFromTypesStage(domaintypes.StagePush),
			convertFromTypesStage(domaintypes.StageManifests),
			convertFromTypesStage(domaintypes.StageDeployment),
			convertFromTypesStage(domaintypes.StageWelcome),
			convertFromTypesStage(domaintypes.StagePreFlight),
		}

		for _, stage := range allStages {
			name := getStageDisplayName(stage)
			assert.NotEmpty(t, name, "Display name should not be empty for stage %s", stage)
		}
	})

	t.Run("getStageErrorPrefix coverage for all branches", func(t *testing.T) {
		t.Parallel()

		allStages := []domaintypes.ConversationStage{
			convertFromTypesStage(domaintypes.StageBuild),
			convertFromTypesStage(domaintypes.StagePush),
			convertFromTypesStage(domaintypes.StageManifests),
			convertFromTypesStage(domaintypes.StageDeployment),
			convertFromTypesStage(domaintypes.StageWelcome),
			convertFromTypesStage(domaintypes.StagePreFlight),
		}

		for _, stage := range allStages {
			prefix := getStageErrorPrefix(stage)
			assert.NotEmpty(t, prefix, "Error prefix should not be empty for stage %s", stage)
		}
	})
}
