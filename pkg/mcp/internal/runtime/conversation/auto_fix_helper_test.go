package conversation

import (
	"context"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestAutoFixHelper(t *testing.T) {
	t.Parallel()
	t.Run("NewAutoFixHelper", func(t *testing.T) {
		t.Parallel()
		helper := NewAutoFixHelper(nil)
		assert.NotNil(t, helper, "Helper should not be nil")
		assert.Nil(t, helper.conversationHandler, "Handler should be nil when passed nil")
	})

	t.Run("AttemptAutoFix with nil handler", func(t *testing.T) {
		t.Parallel()
		helper := NewAutoFixHelper(nil)
		response := &ConversationResponse{
			SessionID: "test-session",
			Status:    ResponseStatusError,
		}
		state := &ConversationState{}

		result := helper.AttemptAutoFix(context.Background(), response, convertFromTypesStage(types.StageBuild), nil, state)
		assert.False(t, result, "Should return false when handler is nil")
		assert.Equal(t, ResponseStatusError, response.Status, "Status should remain unchanged")
	})

	t.Run("getSuccessOptions for different stages", func(t *testing.T) {
		t.Parallel()
		helper := &AutoFixHelper{}

		testCases := []struct {
			stage    core.ConversationStage
			expected string
		}{
			{convertFromTypesStage(types.StageBuild), "Continue to next stage"},
			{convertFromTypesStage(types.StagePush), "Continue to manifest generation"},
			{convertFromTypesStage(types.StageManifests), "Continue to deployment"},
			{convertFromTypesStage(types.StageDeployment), "Continue to completion"},
			{convertFromTypesStage(types.StageWelcome), "Continue"},
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
			stage    core.ConversationStage
			expected string
		}{
			{convertFromTypesStage(types.StageBuild), "Build"},
			{convertFromTypesStage(types.StagePush), "Push"},
			{convertFromTypesStage(types.StageManifests), "Manifest generation"},
			{convertFromTypesStage(types.StageDeployment), "Deployment"},
			{convertFromTypesStage(types.StageWelcome), "Operation"},
		}

		for _, tc := range testCases {
			name := getStageDisplayName(tc.stage)
			assert.Equal(t, tc.expected, name, "Display name should match for stage %s", tc.stage)
		}
	})

	t.Run("getStageErrorPrefix", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			stage    core.ConversationStage
			expected string
		}{
			{convertFromTypesStage(types.StageBuild), "Build"},
			{convertFromTypesStage(types.StagePush), "Failed to push Docker image"},
			{convertFromTypesStage(types.StageManifests), "Failed to generate Kubernetes manifests"},
			{convertFromTypesStage(types.StageDeployment), "Deployment"},
			{convertFromTypesStage(types.StageWelcome), "Operation"},
		}

		for _, tc := range testCases {
			prefix := getStageErrorPrefix(tc.stage)
			assert.Equal(t, tc.expected, prefix, "Error prefix should match for stage %s", tc.stage)
		}
	})

	t.Run("getSuccessOptions coverage for all branches", func(t *testing.T) {
		t.Parallel()
		helper := &AutoFixHelper{}
		allStages := []core.ConversationStage{
			convertFromTypesStage(types.StageBuild),
			convertFromTypesStage(types.StagePush),
			convertFromTypesStage(types.StageManifests),
			convertFromTypesStage(types.StageDeployment),
			convertFromTypesStage(types.StageWelcome),
			convertFromTypesStage(types.StagePreFlight),
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

		allStages := []core.ConversationStage{
			convertFromTypesStage(types.StageBuild),
			convertFromTypesStage(types.StagePush),
			convertFromTypesStage(types.StageManifests),
			convertFromTypesStage(types.StageDeployment),
			convertFromTypesStage(types.StageWelcome),
			convertFromTypesStage(types.StagePreFlight),
		}

		for _, stage := range allStages {
			name := getStageDisplayName(stage)
			assert.NotEmpty(t, name, "Display name should not be empty for stage %s", stage)
		}
	})

	t.Run("getStageErrorPrefix coverage for all branches", func(t *testing.T) {
		t.Parallel()

		allStages := []core.ConversationStage{
			convertFromTypesStage(types.StageBuild),
			convertFromTypesStage(types.StagePush),
			convertFromTypesStage(types.StageManifests),
			convertFromTypesStage(types.StageDeployment),
			convertFromTypesStage(types.StageWelcome),
			convertFromTypesStage(types.StagePreFlight),
		}

		for _, stage := range allStages {
			prefix := getStageErrorPrefix(stage)
			assert.NotEmpty(t, prefix, "Error prefix should not be empty for stage %s", stage)
		}
	})
}
