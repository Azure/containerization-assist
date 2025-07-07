package execution

import (
	"io"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzerHelper(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("NewAnalyzerHelper", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		assert.NotNil(t, helper)
		assert.Nil(t, helper.analyzer)
		assert.Nil(t, helper.enhancedBuildAnalyzer)
	})

	t.Run("SetupBuildToolAnalyzer with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		analyzer, fixingMixin := helper.SetupBuildToolAnalyzer("test_tool")
		assert.Nil(t, analyzer)
		assert.Nil(t, fixingMixin)
	})

	t.Run("SetupAnalyzeToolAnalyzer with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		analyzer := helper.SetupAnalyzeToolAnalyzer("test_tool")
		assert.Nil(t, analyzer)
	})

	t.Run("SetupDeployToolAnalyzer with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		analyzer, fixingMixin := helper.SetupDeployToolAnalyzer("test_tool")
		assert.Nil(t, analyzer)
		assert.Nil(t, fixingMixin)
	})

	t.Run("GetEnhancedBuildAnalyzer with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		analyzer := helper.GetEnhancedBuildAnalyzer()
		assert.Nil(t, analyzer)
	})

	t.Run("ensureEnhancedAnalyzer with nil analyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)

		// Call multiple times - should not panic and should remain nil
		helper.ensureEnhancedAnalyzer()
		assert.Nil(t, helper.enhancedBuildAnalyzer)

		helper.ensureEnhancedAnalyzer()
		assert.Nil(t, helper.enhancedBuildAnalyzer)
	})

	t.Run("BuildToolInitializer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewBuildToolInitializer(helper)

		assert.NotNil(t, initializer)
		assert.Equal(t, helper, initializer.helper)
	})

	t.Run("BuildToolInitializer.SetupAnalyzer with non-supporting tool", func(_ *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewBuildToolInitializer(helper)

		// Test with a tool that doesn't support SetAnalyzer
		mockTool := struct{}{}
		// Should not panic
		initializer.SetupAnalyzer(mockTool, "test_tool")
	})

	t.Run("DeployToolInitializer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewDeployToolInitializer(helper)

		assert.NotNil(t, initializer)
		assert.Equal(t, helper, initializer.helper)
	})

	t.Run("DeployToolInitializer.SetupAnalyzer with non-supporting tool", func(_ *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewDeployToolInitializer(helper)

		// Test with a tool that doesn't support SetAnalyzer
		mockTool := struct{}{}
		// Should not panic
		initializer.SetupAnalyzer(mockTool, "test_tool")
	})

	t.Run("BuildToolInitializer.SetupAnalyzer with tool supporting SetAnalyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewBuildToolInitializer(helper)

		// Mock tool that supports SetAnalyzer but with nil analyzer helper
		mockTool := &mockToolWithSetters{}
		initializer.SetupAnalyzer(mockTool, "test_tool")

		// Should not have set anything since analyzer is nil
		assert.Nil(t, mockTool.analyzer)
		assert.Nil(t, mockTool.fixingMixin)
	})

	t.Run("DeployToolInitializer.SetupAnalyzer with tool supporting SetAnalyzer", func(t *testing.T) {
		helper := NewAnalyzerHelper(nil, logger)
		initializer := NewDeployToolInitializer(helper)

		// Mock tool that supports SetAnalyzer but with nil analyzer helper
		mockTool := &mockToolWithSetters{}
		initializer.SetupAnalyzer(mockTool, "test_tool")

		// Should not have set anything since analyzer is nil
		assert.Nil(t, mockTool.analyzer)
		assert.Nil(t, mockTool.fixingMixin)
	})
}

// mockToolWithSetters implements the SetAnalyzer and SetFixingMixin interfaces for testing
type mockToolWithSetters struct {
	analyzer    interface{}
	fixingMixin interface{}
}

func (m *mockToolWithSetters) SetAnalyzer(analyzer interface{}) {
	m.analyzer = analyzer
}

func (m *mockToolWithSetters) SetFixingMixin(mixin interface{}) {
	m.fixingMixin = mixin
}
