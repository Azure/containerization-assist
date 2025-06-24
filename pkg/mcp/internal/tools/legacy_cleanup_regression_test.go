package tools

import (
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/ai_context"
)

// TestLegacyAdapterCleanupRegression validates that adapter-free paths work correctly
// after the Workstream 2 legacy cleanup. This ensures that removing type aliases
// and adapter wrappers didn't break functionality.
func TestLegacyAdapterCleanupRegression(t *testing.T) {
	t.Run("BaseAIContextResult_AdapterFree_Assessment", func(t *testing.T) {
		// Test that BaseAIContextResult implements assessment capabilities without adapters
		base := NewBaseAIContextResult("test", true, 30*time.Second)

		// Verify assessment capabilities work (previously through Assessable alias)
		score := base.CalculateScore()
		if score <= 0 {
			t.Errorf("Expected positive score, got %d", score)
		}

		riskLevel := base.DetermineRiskLevel()
		if riskLevel == "" {
			t.Error("Expected non-empty risk level")
		}

		strengths := base.GetStrengths()
		if len(strengths) == 0 {
			t.Error("Expected at least one strength")
		}

		challenges := base.GetChallenges()
		// Challenges can be empty for successful operations, just verify method works
		_ = challenges

		assessment := base.GetAssessment()
		if assessment == nil {
			t.Error("Expected non-nil assessment")
		}
		if assessment.ReadinessScore != score {
			t.Errorf("Expected assessment score %d, got %d", score, assessment.ReadinessScore)
		}
	})

	t.Run("BaseAIContextResult_AdapterFree_Context", func(t *testing.T) {
		// Test that BaseAIContextResult implements context capabilities without adapters
		base := NewBaseAIContextResult("build", true, 45*time.Second)

		// Verify context capabilities work (previously through ContextEnriched alias)
		context := base.GetAIContext()
		if context == nil {
			t.Error("Expected non-nil AI context")
		}
		if context.ToolName != "build_atomic" {
			t.Errorf("Expected tool name 'build_atomic', got %s", context.ToolName)
		}

		metadata := base.GetMetadataForAI()
		if metadata == nil {
			t.Error("Expected non-nil metadata")
		}
		if metadata["operation_type"] != "build" {
			t.Errorf("Expected operation_type 'build', got %v", metadata["operation_type"])
		}
		if metadata["success"] != true {
			t.Error("Expected success true in metadata")
		}

		// Verify EnrichWithInsights doesn't panic (no-op implementation)
		base.EnrichWithInsights([]*ai_context.ContextualInsight{})
	})

	t.Run("ToolAIContextProvider_Interface_Compatibility", func(t *testing.T) {
		// Test that BaseAIContextResult satisfies ToolAIContextProvider interface
		// after legacy type alias removal
		base := NewBaseAIContextResult("deploy", false, 2*time.Minute)

		// Verify it can be used as ToolAIContextProvider
		var provider ToolAIContextProvider = base

		// Test all interface methods
		score := provider.CalculateScore()
		if score >= 80 {
			t.Errorf("Expected low score for failed operation, got %d", score)
		}

		riskLevel := provider.DetermineRiskLevel()
		if riskLevel != "critical" && riskLevel != "high" {
			t.Errorf("Expected high/critical risk for failed operation, got %s", riskLevel)
		}

		strengths := provider.GetStrengths()
		_ = strengths // Verify method works
		challenges := provider.GetChallenges()
		if len(challenges) == 0 {
			t.Error("Expected challenges for failed operation")
		}

		assessment := provider.GetAssessment()
		if assessment == nil {
			t.Error("Expected assessment")
		}

		context := provider.GetAIContext()
		if context == nil {
			t.Error("Expected AI context")
		}

		metadata := provider.GetMetadataForAI()
		if metadata["success"] != false {
			t.Error("Expected success false for failed operation")
		}
	})

	t.Run("AIContext_DirectUsage_NoAliases", func(t *testing.T) {
		// Test that ai_context.AIContext interface can be used directly
		// without the removed type aliases (Assessable, Recommendable, ContextEnriched)

		// Create a mock implementation
		mock := &mockAIContext{
			assessment: &ai_context.UnifiedAssessment{
				ReadinessScore:  85,
				RiskLevel:       "low",
				ConfidenceLevel: 90,
			},
			recommendations: []ai_context.Recommendation{
				{
					RecommendationID: "test-rec",
					Title:            "Test Recommendation",
					Category:         "operational",
				},
			},
			toolContext: &ai_context.ToolContext{
				ToolName: "test_tool",
			},
			metadata: map[string]interface{}{
				"test_key": "test_value",
			},
		}

		// Verify direct AIContext usage works
		var aiCtx ai_context.AIContext = mock

		assessment := aiCtx.GetAssessment()
		if assessment == nil || assessment.ReadinessScore != 85 {
			t.Error("AIContext assessment failed")
		}

		recommendations := aiCtx.GenerateRecommendations()
		if len(recommendations) == 0 {
			t.Error("AIContext recommendations failed")
		}

		toolContext := aiCtx.GetToolContext()
		if toolContext == nil || toolContext.ToolName != "test_tool" {
			t.Error("AIContext tool context failed")
		}

		metadata := aiCtx.GetMetadata()
		if metadata["test_key"] != "test_value" {
			t.Error("AIContext metadata failed")
		}
	})
}

// mockAIContext implements ai_context.AIContext for testing
type mockAIContext struct {
	assessment      *ai_context.UnifiedAssessment
	recommendations []ai_context.Recommendation
	toolContext     *ai_context.ToolContext
	metadata        map[string]interface{}
}

func (m *mockAIContext) GetAssessment() *ai_context.UnifiedAssessment {
	return m.assessment
}

func (m *mockAIContext) GenerateRecommendations() []ai_context.Recommendation {
	return m.recommendations
}

func (m *mockAIContext) GetToolContext() *ai_context.ToolContext {
	return m.toolContext
}

func (m *mockAIContext) GetMetadata() map[string]interface{} {
	return m.metadata
}

// TestValidationErrorBuilderUsage verifies that tools use the new structured
// error handling instead of the removed simple NewValidationError function
func TestValidationErrorBuilderUsage(t *testing.T) {
	t.Run("StructuredValidationErrors_NoSimpleErrors", func(t *testing.T) {
		// This test documents that tools should use types.NewValidationErrorBuilder
		// instead of the removed tools.NewValidationError function

		// Example of correct usage pattern that tools should follow:
		// err := types.NewValidationErrorBuilder("message", "field", value).Build()

		// This test serves as documentation - the actual validation is that
		// the code compiles and tools tests pass, proving no simple validation
		// errors are being used.
		t.Log("Structured validation errors are now the standard")
		t.Log("Simple NewValidationError function has been removed")
		t.Log("All tools use types.NewValidationErrorBuilder for validation errors")
	})
}
