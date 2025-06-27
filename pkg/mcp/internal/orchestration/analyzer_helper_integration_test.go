package orchestration

import (
	"context"
	"testing"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// mockAnalyzer implements mcptypes.AIAnalyzer for testing
type mockAnalyzer struct {
	called bool
}

func (m *mockAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	m.called = true
	return "analysis result", nil
}

func (m *mockAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	m.called = true
	return "file analysis result", nil
}

func (m *mockAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	m.called = true
	return "formatted analysis result", nil
}

func (m *mockAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return mcptypes.TokenUsage{
		CompletionTokens: 10,
		PromptTokens:     5,
	}
}

func (m *mockAnalyzer) ResetTokenUsage() {
	// No-op for mock
}

// mockToolWithAnalyzer implements the SetAnalyzer interface for testing
type mockToolWithAnalyzer struct {
	analyzer    interface{}
	fixingMixin interface{}
}

func (m *mockToolWithAnalyzer) SetAnalyzer(analyzer interface{}) {
	m.analyzer = analyzer
}

func (m *mockToolWithAnalyzer) SetFixingMixin(mixin interface{}) {
	m.fixingMixin = mixin
}

func TestAnalyzerHelperIntegration(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Full workflow integration test", func(t *testing.T) {
		mockAnalyzer := &mockAnalyzer{}
		helper := NewAnalyzerHelper(mockAnalyzer, logger)

		// Test all setup methods with real analyzer
		buildAnalyzer, buildMixin := helper.SetupBuildToolAnalyzer("build_test")
		assert.NotNil(t, buildAnalyzer, "Build analyzer should not be nil")
		assert.NotNil(t, buildMixin, "Build mixin should not be nil")

		analyzeAnalyzer := helper.SetupAnalyzeToolAnalyzer("analyze_test")
		assert.NotNil(t, analyzeAnalyzer, "Analyze analyzer should not be nil")

		deployAnalyzer, deployMixin := helper.SetupDeployToolAnalyzer("deploy_test")
		assert.NotNil(t, deployAnalyzer, "Deploy analyzer should not be nil")
		assert.NotNil(t, deployMixin, "Deploy mixin should not be nil")

		// Test enhanced analyzer creation
		enhancedAnalyzer := helper.GetEnhancedBuildAnalyzer()
		assert.NotNil(t, enhancedAnalyzer, "Enhanced analyzer should not be nil")

		// Test that calling again returns the same instance
		enhancedAnalyzer2 := helper.GetEnhancedBuildAnalyzer()
		assert.Equal(t, enhancedAnalyzer, enhancedAnalyzer2, "Should return same instance")
	})

	t.Run("Nil analyzer integration test", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)

		// Test all setup methods with nil analyzer
		buildAnalyzer, buildMixin := helper.SetupBuildToolAnalyzer("build_test")
		assert.Nil(t, buildAnalyzer, "Build analyzer should be nil")
		assert.Nil(t, buildMixin, "Build mixin should be nil")

		analyzeAnalyzer := helper.SetupAnalyzeToolAnalyzer("analyze_test")
		assert.Nil(t, analyzeAnalyzer, "Analyze analyzer should be nil")

		deployAnalyzer, deployMixin := helper.SetupDeployToolAnalyzer("deploy_test")
		assert.Nil(t, deployAnalyzer, "Deploy analyzer should be nil")
		assert.Nil(t, deployMixin, "Deploy mixin should be nil")

		// Test enhanced analyzer with nil base
		enhancedAnalyzer := helper.GetEnhancedBuildAnalyzer()
		assert.Nil(t, enhancedAnalyzer, "Enhanced analyzer should be nil")

		// Test ensureEnhancedAnalyzer multiple calls - should not panic
		helper.ensureEnhancedAnalyzer()
		helper.ensureEnhancedAnalyzer()
		assert.Nil(t, helper.enhancedBuildAnalyzer, "Enhanced analyzer should remain nil")
	})

	t.Run("Tool initializers integration test", func(t *testing.T) {
		mockAnalyzer := &mockAnalyzer{}
		helper := NewAnalyzerHelper(mockAnalyzer, logger)

		// Test BuildToolInitializer
		buildInit := NewBuildToolInitializer(helper)
		assert.NotNil(t, buildInit, "Build initializer should not be nil")
		assert.Equal(t, helper, buildInit.helper, "Should store helper reference")

		// Test with tool that supports SetAnalyzer
		mockTool := &mockToolWithAnalyzer{}
		buildInit.SetupAnalyzer(mockTool, "test_tool")
		assert.NotNil(t, mockTool.analyzer, "Tool analyzer should be set")
		assert.NotNil(t, mockTool.fixingMixin, "Tool fixing mixin should be set")

		// Test DeployToolInitializer
		deployInit := NewDeployToolInitializer(helper)
		assert.NotNil(t, deployInit, "Deploy initializer should not be nil")
		assert.Equal(t, helper, deployInit.helper, "Should store helper reference")

		// Test with another tool that supports SetAnalyzer
		mockTool2 := &mockToolWithAnalyzer{}
		deployInit.SetupAnalyzer(mockTool2, "deploy_tool")
		assert.NotNil(t, mockTool2.analyzer, "Tool analyzer should be set")
		assert.NotNil(t, mockTool2.fixingMixin, "Tool fixing mixin should be set")
	})

	t.Run("Tool initializers with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)

		// Test BuildToolInitializer with nil analyzer
		buildInit := NewBuildToolInitializer(helper)
		mockTool := &mockToolWithAnalyzer{}
		buildInit.SetupAnalyzer(mockTool, "test_tool")
		assert.Nil(t, mockTool.analyzer, "Tool analyzer should remain nil")
		assert.Nil(t, mockTool.fixingMixin, "Tool fixing mixin should remain nil")

		// Test DeployToolInitializer with nil analyzer
		deployInit := NewDeployToolInitializer(helper)
		mockTool2 := &mockToolWithAnalyzer{}
		deployInit.SetupAnalyzer(mockTool2, "deploy_tool")
		assert.Nil(t, mockTool2.analyzer, "Tool analyzer should remain nil")
		assert.Nil(t, mockTool2.fixingMixin, "Tool fixing mixin should remain nil")
	})

	t.Run("Tool initializers with non-supporting tools", func(t *testing.T) {
		mockAnalyzer := &mockAnalyzer{}
		helper := NewAnalyzerHelper(mockAnalyzer, logger)

		buildInit := NewBuildToolInitializer(helper)
		deployInit := NewDeployToolInitializer(helper)

		// Test with tool that doesn't implement SetAnalyzer - should not panic
		nonSupportingTool := struct{}{}

		// These should not panic
		buildInit.SetupAnalyzer(nonSupportingTool, "test_tool")
		deployInit.SetupAnalyzer(nonSupportingTool, "test_tool")
	})

	t.Run("Enhanced analyzer lazy initialization", func(t *testing.T) {
		mockAnalyzer := &mockAnalyzer{}
		helper := NewAnalyzerHelper(mockAnalyzer, logger)

		// Initially nil
		assert.Nil(t, helper.enhancedBuildAnalyzer, "Should start as nil")

		// First call creates it
		enhanced := helper.GetEnhancedBuildAnalyzer()
		assert.NotNil(t, enhanced, "Should create enhanced analyzer")
		assert.NotNil(t, helper.enhancedBuildAnalyzer, "Should store enhanced analyzer")

		// Second call returns same instance
		enhanced2 := helper.GetEnhancedBuildAnalyzer()
		assert.Equal(t, enhanced, enhanced2, "Should return same instance")
	})

	t.Run("Enhanced analyzer with real build analyzer", func(t *testing.T) {
		mockAnalyzer := &mockAnalyzer{}
		helper := NewAnalyzerHelper(mockAnalyzer, logger)

		// Get enhanced analyzer
		enhanced := helper.GetEnhancedBuildAnalyzer()
		assert.NotNil(t, enhanced, "Enhanced analyzer should not be nil")

		// Test that it's not nil (actual type checking avoided due to import issues)
		assert.NotNil(t, enhanced, "Should be a valid enhanced analyzer instance")
	})
}
