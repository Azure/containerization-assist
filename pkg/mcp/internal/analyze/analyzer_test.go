package analyze

import (
	"fmt"
	"testing"
	"time"

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

			// Verify it implements the interface
			var _ mcptypes.AIAnalyzer = analyzer
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
		name     string
		envVars  map[string]string
		expected *AnalyzerConfig
	}{
		{
			name: "all valid values",
			envVars: map[string]string{
				"MCP_ENABLE_AI_ANALYZER":         "true",
				"MCP_ANALYZER_LOG_LEVEL":         "debug",
				"MCP_ANALYZER_MAX_PROMPT_LENGTH": "8192",
				"MCP_ANALYZER_CACHE_ENABLED":     "true",
				"MCP_ANALYZER_CACHE_TTL":         "600",
			},
			expected: &AnalyzerConfig{
				EnableAI:        true,
				LogLevel:        "debug",
				MaxPromptLength: 8192,
				CacheEnabled:    true,
				CacheTTLSeconds: 600,
			},
		},
		{
			name: "invalid numeric values use defaults",
			envVars: map[string]string{
				"MCP_ANALYZER_MAX_PROMPT_LENGTH": "not-a-number",
				"MCP_ANALYZER_CACHE_TTL":         "invalid",
			},
			expected: &AnalyzerConfig{
				EnableAI:        false,
				LogLevel:        "info",
				MaxPromptLength: 4096, // default
				CacheEnabled:    true,
				CacheTTLSeconds: 300, // default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config := DefaultAnalyzerConfig()
			config.LoadFromEnv()

			assert.Equal(t, tt.expected.EnableAI, config.EnableAI)
			assert.Equal(t, tt.expected.LogLevel, config.LogLevel)
			assert.Equal(t, tt.expected.MaxPromptLength, config.MaxPromptLength)
			assert.Equal(t, tt.expected.CacheEnabled, config.CacheEnabled)
			assert.Equal(t, tt.expected.CacheTTLSeconds, config.CacheTTLSeconds)
		})
	}
}
