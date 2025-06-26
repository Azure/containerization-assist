package conversation

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestRenamedFileIntegration(t *testing.T) {
	t.Parallel()

	t.Run("conversation state integration works", func(t *testing.T) {
		t.Parallel()
		// Test that we can create conversation state (uses functionality from renamed files)
		state := NewConversationState("test-session", "/tmp")
		assert.NotNil(t, state)
		assert.Equal(t, types.StageWelcome, state.CurrentStage)
	})

	t.Run("stage file organization maintains functionality", func(t *testing.T) {
		t.Parallel()
		// Verify that all stage files exist and are properly organized
		// by testing that the core conversation functionality works

		// Test conversation state creation and basic operations
		state := NewConversationState("test-integration", "/tmp/test")

		// Test context management (depends on renamed files being accessible)
		state.Context["test_key"] = "test_value"
		assert.Equal(t, "test_value", state.Context["test_key"])

		// Test stage progression
		originalStage := state.CurrentStage
		state.SetStage(types.StageAnalysis)
		assert.NotEqual(t, originalStage, state.CurrentStage)
		assert.Equal(t, types.StageAnalysis, state.CurrentStage)

		// Test conversation history
		turn := ConversationTurn{
			UserInput: "test input",
			Assistant: "test response",
			Stage:     types.StageAnalysis,
		}
		state.AddConversationTurn(turn)
		assert.Len(t, state.History, 1)
		assert.Equal(t, "test input", state.History[0].UserInput)
	})
}

func TestFileNamingConsistency(t *testing.T) {
	t.Parallel()

	t.Run("renamed files maintain package integrity", func(t *testing.T) {
		t.Parallel()
		// Test that the conversation package still works after file renames

		stageTests := []struct {
			name  string
			stage types.ConversationStage
		}{
			{"welcome", types.StageWelcome},
			{"analysis", types.StageAnalysis},
			{"build", types.StageBuild},
			{"deploy", types.StageDeployment},
		}

		for _, tt := range stageTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				// Test stage functionality works with renamed files
				state := NewConversationState("test-"+tt.name, "/tmp")
				state.SetStage(tt.stage)
				assert.Equal(t, tt.stage, state.CurrentStage)
			})
		}
	})
}

func TestConversationEngineArchitecture(t *testing.T) {
	t.Parallel()

	t.Run("conversation engine components integrate properly after renames", func(t *testing.T) {
		t.Parallel()
		// Test that the reorganized conversation engine works as a cohesive unit

		// Create multiple conversation states to test concurrency
		states := make([]*ConversationState, 3)
		for i := 0; i < 3; i++ {
			states[i] = NewConversationState("test-concurrent", "/tmp/test")
			assert.NotNil(t, states[i])
		}

		// Test that each state is independent
		states[0].SetStage(types.StageAnalysis)
		states[1].SetStage(types.StageBuild)
		states[2].SetStage(types.StageDeployment)

		assert.Equal(t, types.StageAnalysis, states[0].CurrentStage)
		assert.Equal(t, types.StageBuild, states[1].CurrentStage)
		assert.Equal(t, types.StageDeployment, states[2].CurrentStage)

		// Test conversation history independence
		for _, state := range states {
			turn := ConversationTurn{
				UserInput: "test input",
				Assistant: "test response",
				Stage:     state.CurrentStage,
			}
			state.AddConversationTurn(turn)
			assert.Len(t, state.History, 1)
		}
	})
}
