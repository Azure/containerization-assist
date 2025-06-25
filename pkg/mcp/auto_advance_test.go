package mcp

import (
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime/conversation"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

func TestConversationResponseAutoAdvance(t *testing.T) {
	t.Run("WithAutoAdvance", func(t *testing.T) {
		response := &conversation.ConversationResponse{
			Message: "Build complete",
			Stage:   types.StageBuild,
			Status:  conversation.ResponseStatusSuccess,
		}

		config := conversation.AutoAdvanceConfig{
			DelaySeconds:  2,
			Confidence:    0.9,
			Reason:        "High confidence next step",
			CanCancel:     true,
			DefaultAction: "proceed",
		}

		response.WithAutoAdvance(types.StagePush, config)

		if response.RequiresInput {
			t.Error("Expected RequiresInput to be false")
		}

		if response.NextStage == nil || *response.NextStage != types.StagePush {
			t.Errorf("Expected NextStage to be %s, got %v", types.StagePush, response.NextStage)
		}

		if !response.CanAutoAdvance() {
			t.Error("Expected CanAutoAdvance to return true")
		}
	})

	t.Run("WithUserInput", func(t *testing.T) {
		response := &conversation.ConversationResponse{
			Message: "Choose an option",
			Stage:   types.StageBuild,
			Status:  conversation.ResponseStatusSuccess,
		}

		response.WithUserInput()

		if !response.RequiresInput {
			t.Error("Expected RequiresInput to be true")
		}

		if response.NextStage != nil {
			t.Error("Expected NextStage to be nil")
		}

		if response.CanAutoAdvance() {
			t.Error("Expected CanAutoAdvance to return false")
		}
	})

	t.Run("ShouldAutoAdvance", func(t *testing.T) {
		response := &conversation.ConversationResponse{
			Message: "Build complete",
			Stage:   types.StageBuild,
			Status:  conversation.ResponseStatusSuccess,
		}

		config := conversation.AutoAdvanceConfig{
			Confidence: 0.9,
		}

		response.WithAutoAdvance(types.StagePush, config)

		// Test with autopilot enabled
		prefsAutopilot := types.UserPreferences{
			SkipConfirmations: true,
		}

		if !response.ShouldAutoAdvance(prefsAutopilot) {
			t.Error("Expected ShouldAutoAdvance to return true with autopilot enabled")
		}

		// Test with autopilot disabled
		prefsManual := types.UserPreferences{
			SkipConfirmations: false,
		}

		if response.ShouldAutoAdvance(prefsManual) {
			t.Error("Expected ShouldAutoAdvance to return false with autopilot disabled")
		}
	})

	t.Run("ConfidenceThreshold", func(t *testing.T) {
		response := &conversation.ConversationResponse{
			Message: "Build complete",
			Stage:   types.StageBuild,
			Status:  conversation.ResponseStatusSuccess,
		}

		// Low confidence should not auto-advance
		lowConfidenceConfig := conversation.AutoAdvanceConfig{
			Confidence: 0.5,
		}

		response.WithAutoAdvance(types.StagePush, lowConfidenceConfig)

		prefs := types.UserPreferences{
			SkipConfirmations: true,
		}

		if response.ShouldAutoAdvance(prefs) {
			t.Error("Expected ShouldAutoAdvance to return false with low confidence")
		}

		// High confidence should auto-advance
		highConfidenceConfig := conversation.AutoAdvanceConfig{
			Confidence: 0.9,
		}

		response.WithAutoAdvance(types.StagePush, highConfidenceConfig)

		if !response.ShouldAutoAdvance(prefs) {
			t.Error("Expected ShouldAutoAdvance to return true with high confidence")
		}
	})

	t.Run("AutoAdvanceMessage", func(t *testing.T) {
		response := &conversation.ConversationResponse{
			Message: "Build complete",
			Stage:   types.StageBuild,
			Status:  conversation.ResponseStatusSuccess,
		}

		config := conversation.AutoAdvanceConfig{
			DelaySeconds: 3,
			Reason:       "Test auto-advance",
			CanCancel:    true,
		}

		response.WithAutoAdvance(types.StagePush, config)

		message := response.GetAutoAdvanceMessage()

		if message == "" {
			t.Error("Expected auto-advance message to be non-empty")
		}

		expectedPhrases := []string{
			"Build complete",
			"Test auto-advance",
			"advancing in 3 seconds",
			"stop",
		}

		for _, phrase := range expectedPhrases {
			if !stringContainsText(message, phrase) {
				t.Errorf("Expected message to contain '%s', got: %s", phrase, message)
			}
		}
	})
}

// Helper function to check if a string contains a substring
func stringContainsText(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
