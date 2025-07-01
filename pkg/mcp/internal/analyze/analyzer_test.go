package analyze

import (
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/config"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMTransport implements LLMTransport for testing
type MockLLMTransport struct {
	SendPromptFunc func(prompt string) (string, error)
}

func (m *MockLLMTransport) SendPrompt(prompt string) (string, error) {
	if m.SendPromptFunc != nil {
		return m.SendPromptFunc(prompt)
	}
	return "mock response", nil
}

func TestAnalyzerFactory_CreateAnalyzer(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name         string
		enableAI     bool
		transport    LLMTransport
		expectedType string
	}{
		{
			name:         "AI disabled returns StubAnalyzer",
			enableAI:     false,
			transport:    &MockLLMTransport{},
			expectedType: "*analyze.StubAnalyzer",
		},
		{
			name:         "AI enabled with transport returns CallerAnalyzer",
			enableAI:     true,
			transport:    &MockLLMTransport{},
			expectedType: "*analyze.CallerAnalyzer",
		},
		{
			name:         "AI enabled without transport returns StubAnalyzer",
			enableAI:     true,
			transport:    nil,
			expectedType: "*analyze.StubAnalyzer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewAnalyzerFactory(logger, tt.enableAI, tt.transport)
			analyzer := factory.CreateAnalyzer()

			require.NotNil(t, analyzer)

			// Check the type of analyzer returned
			actualType := fmt.Sprintf("%T", analyzer)
			assert.Equal(t, tt.expectedType, actualType)

			// Verify it implements the interface (via bridge pattern due to TokenUsage type differences)
			if typedAnalyzer, ok := analyzer.(mcptypes.AIAnalyzer); ok {
				// Can be used as mcptypes.AIAnalyzer
				_ = typedAnalyzer
			} else {
				t.Errorf("analyzer should implement mcptypes.AIAnalyzer interface")
			}
		})
	}
}

func TestAnalyzerFactory_SetAnalyzerOptions(t *testing.T) {
	logger := zerolog.Nop()
	transport := &MockLLMTransport{}

	factory := NewAnalyzerFactory(logger, true, transport)

	customOpts := CallerAnalyzerOpts{
		ToolName:       "custom_tool",
		SystemPrompt:   "Custom prompt",
		PerCallTimeout: 30 * time.Second,
	}

	factory.SetAnalyzerOptions(customOpts)

	// Create analyzer and verify it uses custom options
	analyzer := factory.CreateAnalyzer()

	// Verify it's a CallerAnalyzer
	callerAnalyzer, ok := analyzer.(*CallerAnalyzer)
	require.True(t, ok)

	// Check internal fields (note: these are private, so in real code you'd test behavior)
	assert.Equal(t, "custom_tool", callerAnalyzer.toolName)
	assert.Equal(t, 30*time.Second, callerAnalyzer.timeout)
}

func TestCreateAnalyzerFromEnv(t *testing.T) {
	logger := zerolog.Nop()

	// Test with default env (no AI enabled)
	analyzer := CreateAnalyzerFromEnv(logger)
	assert.IsType(t, &StubAnalyzer{}, analyzer)

	// Test with AI enabled env (but no transport available)
	t.Setenv("MCP_ENABLE_AI_ANALYZER", "true")
	analyzer = CreateAnalyzerFromEnv(logger)
	assert.IsType(t, &StubAnalyzer{}, analyzer)
}

func TestAnalyzerConfig_LoadFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		checkFunc func(t *testing.T, cm *config.ConfigManager)
	}{
		{
			name: "all valid values",
			envVars: map[string]string{
				"MCP_ANALYZER_ENABLE_AI":             "true",
				"MCP_ANALYZER_AI_LOG_LEVEL":          "debug",
				"MCP_ANALYZER_MAX_ANALYSIS_TIME":     "30s",
				"MCP_ANALYZER_ENABLE_FILE_DETECTION": "true",
				"MCP_ANALYZER_CACHE_RESULTS":         "true",
				"MCP_ANALYZER_CACHE_TTL":             "10m",
			},
			checkFunc: func(t *testing.T, cm *config.ConfigManager) {
				assert.True(t, cm.Analyzer.EnableAI)
				assert.Equal(t, "debug", cm.Analyzer.AIAnalyzerLogLevel)
				assert.Equal(t, 30*time.Second, cm.Analyzer.MaxAnalysisTime)
				assert.True(t, cm.Analyzer.EnableFileDetection)
				assert.True(t, cm.Analyzer.CacheResults)
				assert.Equal(t, 10*time.Minute, cm.Analyzer.CacheTTL)
			},
		},
		{
			name:    "defaults when env not set",
			envVars: map[string]string{},
			checkFunc: func(t *testing.T, cm *config.ConfigManager) {
				// Check that defaults are applied
				assert.False(t, cm.Analyzer.EnableAI)
				// Default log level is "info" when not set
				assert.Equal(t, "info", cm.Analyzer.AIAnalyzerLogLevel)
				assert.Greater(t, cm.Analyzer.MaxAnalysisTime, time.Duration(0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Create ConfigManager and load from env
			cm := config.NewConfigManager()
			err := cm.LoadConfig("")
			assert.NoError(t, err)

			// Run check function
			tt.checkFunc(t, cm)
		})
	}
}
