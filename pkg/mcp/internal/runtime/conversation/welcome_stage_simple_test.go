package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestHandleWelcomeStage(t *testing.T) {
	// Create a minimal prompt manager for testing
	pm := &PromptManager{}

	// Create a conversation state
	state := NewConversationState("test-session", "/tmp/workspace")
	state.CurrentStage = types.StageWelcome

	ctx := context.Background()

	t.Run("initial welcome with empty input", func(t *testing.T) {
		response := pm.handleWelcomeStage(ctx, state, "")

		assert.NotNil(t, response)
		assert.Equal(t, types.StageWelcome, response.Stage)
		assert.Equal(t, ResponseStatusWaitingInput, response.Status)

		// Check welcome message
		assert.Contains(t, response.Message, "Welcome to Container Kit")
		assert.Contains(t, response.Message, "How would you like to proceed?")

		// Check options
		assert.Len(t, response.Options, 2)
		assert.Equal(t, "interactive", response.Options[0].ID)
		assert.Equal(t, "autopilot", response.Options[1].ID)
	})

	t.Run("select interactive mode", func(t *testing.T) {
		// Reset state
		state = NewConversationState("test-session", "/tmp/workspace")
		state.CurrentStage = types.StageWelcome

		response := pm.handleWelcomeStage(ctx, state, "interactive")

		assert.NotNil(t, response)
		assert.Equal(t, types.StageInit, response.Stage)
		assert.Contains(t, response.Message, "guide you through each step")
		assert.Contains(t, response.Message, "repository URL or local path")
	})

	t.Run("select autopilot mode", func(t *testing.T) {
		// Reset state
		state = NewConversationState("test-session", "/tmp/workspace")
		state.CurrentStage = types.StageWelcome

		response := pm.handleWelcomeStage(ctx, state, "autopilot")

		assert.NotNil(t, response)
		assert.Equal(t, types.StageInit, response.Stage)
		assert.Contains(t, response.Message, "Autopilot mode enabled")

		// Check autopilot was enabled
		autopilot, ok := state.Context["autopilot_enabled"].(bool)
		assert.True(t, ok)
		assert.True(t, autopilot)

		skip, ok := state.Context["skip_confirmations"].(bool)
		assert.True(t, ok)
		assert.True(t, skip)
	})

	t.Run("numeric input handling", func(t *testing.T) {
		// Test "1" for interactive
		state = NewConversationState("test-session", "/tmp/workspace")
		state.CurrentStage = types.StageWelcome

		response := pm.handleWelcomeStage(ctx, state, "1")
		assert.Equal(t, types.StageInit, response.Stage)
		assert.Contains(t, response.Message, "guide you through each step")

		// Test "2" for autopilot
		state = NewConversationState("test-session", "/tmp/workspace")
		state.CurrentStage = types.StageWelcome

		response = pm.handleWelcomeStage(ctx, state, "2")
		assert.Equal(t, types.StageInit, response.Stage)
		assert.Contains(t, response.Message, "Autopilot mode enabled")
	})

	t.Run("invalid input re-prompts", func(t *testing.T) {
		state = NewConversationState("test-session", "/tmp/workspace")
		state.CurrentStage = types.StageWelcome

		response := pm.handleWelcomeStage(ctx, state, "invalid option")

		assert.NotNil(t, response)
		assert.Equal(t, types.StageWelcome, response.Stage)
		assert.Contains(t, response.Message, "Please choose how you'd like to proceed")
		assert.Len(t, response.Options, 2)
	})
}

func TestWelcomePromptTemplate(t *testing.T) {
	// Verify the welcome.md template content
	welcomeTemplate := `Welcome the user to Container Kit and explain what you'll help them accomplish.

**Your Role:** Greet the user warmly and set expectations for the containerization workflow.

**Key Points:**
- Warmly welcome the user to Container Kit
- Explain that you'll help containerize their application step by step
- Mention the workflow: analyze code → generate Dockerfile → build image → create Kubernetes manifests → deploy
- Offer choice between Interactive Mode (step-by-step with confirmations) and Autopilot Mode (automated workflow)
- Be encouraging and professional

**Interactive Mode:**
- You'll guide them through each step
- They can review and approve each action
- Full control over the process

**Autopilot Mode:**
- Automated workflow with minimal interruptions
- You'll make sensible defaults
- They can still intervene if needed

**What to do:** Provide a friendly welcome message and offer the choice between Interactive and Autopilot modes.`

	// Verify key elements are present
	assert.Contains(t, welcomeTemplate, "Welcome the user to Container Kit")
	assert.Contains(t, welcomeTemplate, "Interactive Mode")
	assert.Contains(t, welcomeTemplate, "Autopilot Mode")
	assert.Contains(t, welcomeTemplate, "analyze code → generate Dockerfile → build image → create Kubernetes manifests → deploy")
}

func TestEnableDisableAutopilot(t *testing.T) {
	pm := &PromptManager{}
	state := NewConversationState("test-session", "/tmp/workspace")

	// Test enableAutopilot
	pm.enableAutopilot(state)

	autopilot, ok := state.Context["autopilot_enabled"].(bool)
	assert.True(t, ok)
	assert.True(t, autopilot)

	// Test disableAutopilot
	pm.disableAutopilot(state)

	autopilot, ok = state.Context["autopilot_enabled"].(bool)
	assert.True(t, ok)
	assert.False(t, autopilot)
}

func TestHasAutopilotEnabled(t *testing.T) {
	pm := &PromptManager{}
	state := NewConversationState("test-session", "/tmp/workspace")

	// Initially should be false
	assert.False(t, pm.hasAutopilotEnabled(state))

	// Enable via context
	state.Context["autopilot_enabled"] = true
	assert.True(t, pm.hasAutopilotEnabled(state))

	// Test skip_confirmations
	state.Context["autopilot_enabled"] = false
	state.Context["skip_confirmations"] = true
	assert.True(t, pm.hasAutopilotEnabled(state))
}

func TestAutopilotCommands(t *testing.T) {
	pm := &PromptManager{}
	state := NewConversationState("test-session", "/tmp/workspace")
	state.CurrentStage = types.StageAnalysis

	testCases := []struct {
		name     string
		input    string
		contains string
		enabled  bool
	}{
		{
			name:     "enable autopilot",
			input:    "enable autopilot",
			contains: "Autopilot mode enabled",
			enabled:  true,
		},
		{
			name:     "autopilot on",
			input:    "autopilot on",
			contains: "Autopilot mode enabled",
			enabled:  true,
		},
		{
			name:     "disable autopilot",
			input:    "disable autopilot",
			contains: "Autopilot mode disabled",
			enabled:  false,
		},
		{
			name:     "autopilot off",
			input:    "autopilot off",
			contains: "Autopilot mode disabled",
			enabled:  false,
		},
		{
			name:     "stop command",
			input:    "stop",
			contains: "Autopilot paused",
			enabled:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset state
			state.Context = make(map[string]interface{})
			if strings.Contains(tc.name, "stop") {
				// For stop test, enable autopilot first
				state.Context["autopilot_enabled"] = true
			}

			response := pm.handleAutopilotCommands(tc.input, state)

			assert.NotNil(t, response)
			assert.Contains(t, response.Message, tc.contains)

			// Verify autopilot state
			autopilot, ok := state.Context["autopilot_enabled"].(bool)
			if tc.enabled {
				assert.True(t, ok)
				assert.True(t, autopilot)
			} else {
				assert.True(t, ok)
				assert.False(t, autopilot)
			}
		})
	}
}
