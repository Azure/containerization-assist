package conversation

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/application/internal/types"
)

func TestConversationResponseAutoAdvance(t *testing.T) {
	t.Run("WithAutoAdvance", func(t *testing.T) {
		response := &ConversationResponse{
			Message: "Build complete",
			Stage:   convertFromTypesStage(types.StageBuild),
			Status:  ResponseStatusSuccess,
		}

		config := AutoAdvanceConfig{
			DelaySeconds:  2,
			Confidence:    0.9,
			Reason:        "High confidence next step",
			CanCancel:     true,
			DefaultAction: "proceed",
		}

		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), config)

		if response.RequiresInput {
			t.Error("Expected RequiresInput to be false")
		}

		if response.NextStage == nil || *response.NextStage != convertFromTypesStage(types.StagePush) {
			t.Errorf("Expected NextStage to be %s, got %v", convertFromTypesStage(types.StagePush), response.NextStage)
		}

		if !response.CanAutoAdvance() {
			t.Error("Expected CanAutoAdvance to return true")
		}
	})

	t.Run("WithUserInput", func(t *testing.T) {
		response := &ConversationResponse{
			Message: "Choose an option",
			Stage:   convertFromTypesStage(types.StageBuild),
			Status:  ResponseStatusSuccess,
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
		response := &ConversationResponse{
			Message: "Build complete",
			Stage:   convertFromTypesStage(types.StageBuild),
			Status:  ResponseStatusSuccess,
		}

		config := AutoAdvanceConfig{
			Confidence: 0.9,
		}

		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), config)
		prefsAutopilot := types.UserPreferences{
			SkipConfirmations: true,
		}

		if !response.ShouldAutoAdvance(prefsAutopilot) {
			t.Error("Expected ShouldAutoAdvance to return true with autopilot enabled")
		}
		prefsManual := types.UserPreferences{
			SkipConfirmations: false,
		}

		if response.ShouldAutoAdvance(prefsManual) {
			t.Error("Expected ShouldAutoAdvance to return false with autopilot disabled")
		}
	})

	t.Run("ConfidenceThreshold", func(t *testing.T) {
		response := &ConversationResponse{
			Message: "Build complete",
			Stage:   convertFromTypesStage(types.StageBuild),
			Status:  ResponseStatusSuccess,
		}
		lowConfidenceConfig := AutoAdvanceConfig{
			Confidence: 0.5,
		}

		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), lowConfidenceConfig)

		prefs := types.UserPreferences{
			SkipConfirmations: true,
		}

		if response.ShouldAutoAdvance(prefs) {
			t.Error("Expected ShouldAutoAdvance to return false with low confidence")
		}
		highConfidenceConfig := AutoAdvanceConfig{
			Confidence: 0.9,
		}

		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), highConfidenceConfig)

		if !response.ShouldAutoAdvance(prefs) {
			t.Error("Expected ShouldAutoAdvance to return true with high confidence")
		}
	})

	t.Run("AutoAdvanceMessage", func(t *testing.T) {
		response := &ConversationResponse{
			Message: "Build complete",
			Stage:   convertFromTypesStage(types.StageBuild),
			Status:  ResponseStatusSuccess,
		}

		config := AutoAdvanceConfig{
			DelaySeconds: 3,
			Reason:       "Test auto-advance",
			CanCancel:    true,
		}

		response.WithAutoAdvance(convertFromTypesStage(types.StagePush), config)

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
func stringContainsText(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
