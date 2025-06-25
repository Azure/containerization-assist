package conversation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreFlightAutoRunDecision(t *testing.T) {
	pm := &PromptManager{}

	t.Run("autopilot mode enables auto-run", func(t *testing.T) {
		state := NewConversationState("test-session", "/tmp/workspace")
		state.Context["autopilot_enabled"] = true

		shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, "analyze repo")
		assert.True(t, shouldAutoRun, "Should auto-run in autopilot mode")
	})

	t.Run("skip_confirmations flag enables auto-run", func(t *testing.T) {
		state := NewConversationState("test-session", "/tmp/workspace")
		state.Context["skip_confirmations"] = true

		shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, "analyze repo")
		assert.True(t, shouldAutoRun, "Should auto-run with skip_confirmations")
	})

	t.Run("returning user gets auto-run", func(t *testing.T) {
		state := NewConversationState("test-session", "/tmp/workspace")
		// Simulate returning user with existing context
		state.Context["previous_session"] = true
		state.RepoAnalysis = map[string]interface{}{
			"language": "go",
		}

		shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, "continue")
		assert.True(t, shouldAutoRun, "Should auto-run for returning users")
	})

	t.Run("first-time user requires confirmation", func(t *testing.T) {
		state := NewConversationState("test-session", "/tmp/workspace")
		// Simulate first-time user with empty context
		state.Context = make(map[string]interface{})
		state.RepoAnalysis = nil

		shouldAutoRun := pm.shouldAutoRunPreFlightChecks(state, "start")
		assert.False(t, shouldAutoRun, "Should not auto-run for first-time users")
	})
}

func TestShouldAutoRunPreFlightChecks(t *testing.T) {
	pm := &PromptManager{}

	testCases := []struct {
		name        string
		setupState  func(*ConversationState)
		input       string
		expectAuto  bool
		description string
	}{
		{
			name: "autopilot enabled",
			setupState: func(state *ConversationState) {
				state.Context["autopilot_enabled"] = true
			},
			input:       "analyze",
			expectAuto:  true,
			description: "Should auto-run when autopilot is enabled",
		},
		{
			name: "skip confirmations enabled",
			setupState: func(state *ConversationState) {
				state.Context["skip_confirmations"] = true
			},
			input:       "analyze",
			expectAuto:  true,
			description: "Should auto-run when skip_confirmations is enabled",
		},
		{
			name: "returning user with context",
			setupState: func(state *ConversationState) {
				state.Context["some_key"] = "some_value"
				state.RepoAnalysis = map[string]interface{}{"language": "go"}
			},
			input:       "continue",
			expectAuto:  true,
			description: "Should auto-run for returning users",
		},
		{
			name: "first time user - empty context",
			setupState: func(state *ConversationState) {
				state.Context = make(map[string]interface{})
				state.RepoAnalysis = nil
			},
			input:       "start",
			expectAuto:  false,
			description: "Should not auto-run for first-time users",
		},
		{
			name: "first time user - default state",
			setupState: func(state *ConversationState) {
				// NewConversationState initializes RepoAnalysis as empty map, not nil
				// Context is nil by default, so this should be first-time
				state.Context = nil
				// Clear the RepoAnalysis to make it truly empty
				state.RepoAnalysis = make(map[string]interface{})
			},
			input:       "begin",
			expectAuto:  false,
			description: "Should not auto-run for completely new users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := NewConversationState("test-session", "/tmp/workspace")
			tc.setupState(state)

			result := pm.shouldAutoRunPreFlightChecks(state, tc.input)
			assert.Equal(t, tc.expectAuto, result, tc.description)
		})
	}
}
