package sampling

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPServer for testing
type mockMCPServer struct {
	callCount int
}

func (m *mockMCPServer) SendNotificationToClient(ctx context.Context, method string, params map[string]any) error {
	m.callCount++
	return nil
}

func TestClient_NewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test default client
	client := NewClient(logger)
	assert.Equal(t, int32(2048), client.maxTokens)
	assert.Equal(t, float32(0.3), client.temperature)
	assert.Equal(t, 3, client.retryAttempts)
	assert.Equal(t, 5000, client.tokenBudget)

	// Test client with options
	client = NewClient(logger,
		WithMaxTokens(4096),
		WithTemperature(0.7),
		WithRetry(5, 10000),
	)
	assert.Equal(t, int32(4096), client.maxTokens)
	assert.Equal(t, float32(0.7), client.temperature)
	assert.Equal(t, 5, client.retryAttempts)
	assert.Equal(t, 10000, client.tokenBudget)
}

func TestClient_NewClientFromEnv(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Set environment variables
	os.Setenv("SAMPLING_MAX_TOKENS", "1024")
	os.Setenv("SAMPLING_TEMPERATURE", "0.5")
	os.Setenv("SAMPLING_RETRY_ATTEMPTS", "2")
	defer func() {
		os.Unsetenv("SAMPLING_MAX_TOKENS")
		os.Unsetenv("SAMPLING_TEMPERATURE")
		os.Unsetenv("SAMPLING_RETRY_ATTEMPTS")
	}()

	client, err := NewClientFromEnv(logger)
	require.NoError(t, err)

	assert.Equal(t, int32(1024), client.maxTokens)
	assert.Equal(t, float32(0.5), client.temperature)
	assert.Equal(t, 2, client.retryAttempts)
}

func TestClient_NewClientFromEnv_InvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Set invalid environment variable
	os.Setenv("SAMPLING_TEMPERATURE", "5.0") // Invalid: > 2.0
	defer os.Unsetenv("SAMPLING_TEMPERATURE")

	_, err := NewClientFromEnv(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "temperature must be between 0 and 2")
}

func TestClient_Sample_NoMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	req := SamplingRequest{
		Prompt: "Test prompt",
	}

	_, err := client.Sample(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestClient_Sample_WithMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	// Without proper MCP server context, this should fail
	ctx := context.Background()

	req := SamplingRequest{
		Prompt:      "Test prompt",
		MaxTokens:   100,
		Temperature: 0.1,
	}

	_, err := client.Sample(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestClient_AnalyzeError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()

	testErr := errors.New("mvn: command not found")
	_, err := client.AnalyzeError(ctx, testErr, "Docker build context")

	// Should fail without MCP server context
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestClient_CalculateBackoff(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger,
		WithConfig(Config{
			BaseBackoff: 100 * time.Millisecond,
			MaxBackoff:  5 * time.Second,
		}),
	)

	// Test exponential backoff
	backoff0 := client.calculateBackoff(0)
	backoff1 := client.calculateBackoff(1)
	backoff2 := client.calculateBackoff(2)

	assert.GreaterOrEqual(t, backoff0, 75*time.Millisecond) // With jitter
	assert.LessOrEqual(t, backoff0, 125*time.Millisecond)

	assert.Greater(t, backoff1, backoff0)
	assert.Greater(t, backoff2, backoff1)

	// Test max backoff cap
	backoff10 := client.calculateBackoff(10)
	assert.LessOrEqual(t, backoff10, 5*time.Second)
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},                   // 1 word * 1.3 = 1
		{"hello world", 2},             // 2 words * 1.3 = 2.6 -> 2
		{"hello world test", 3},        // 3 words * 1.3 = 3.9 -> 3
		{"one two three four five", 6}, // 5 words * 1.3 = 6.5 -> 6
	}

	for _, tt := range tests {
		result := estimateTokens(tt.input)
		assert.Equal(t, tt.expected, result, "Failed for input: %q", tt.input)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err       error
		retryable bool
	}{
		{errors.New("timeout"), true},
		{errors.New("rate limit exceeded"), true},
		{errors.New("temporarily unavailable"), true},
		{errors.New("connection refused"), true},
		{errors.New("broken pipe"), true},
		{errors.New("invalid request"), false},
		{errors.New("authentication failed"), false},
		{errors.New("not found"), false},
	}

	for _, tt := range tests {
		result := isRetryable(tt.err)
		assert.Equal(t, tt.retryable, result, "Failed for error: %v", tt.err)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid max tokens",
			config: Config{
				MaxTokens:      -1,
				Temperature:    0.3,
				TokenBudget:    1000,
				BaseBackoff:    100 * time.Millisecond,
				MaxBackoff:     1 * time.Second,
				RequestTimeout: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "max_tokens must be positive",
		},
		{
			name: "invalid temperature",
			config: Config{
				MaxTokens:      1000,
				Temperature:    3.0,
				TokenBudget:    1000,
				BaseBackoff:    100 * time.Millisecond,
				MaxBackoff:     1 * time.Second,
				RequestTimeout: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "temperature must be between 0 and 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	config := Config{
		MaxTokens:        1024,
		Temperature:      0.7,
		RetryAttempts:    5,
		TokenBudget:      8000,
		BaseBackoff:      50 * time.Millisecond,
		MaxBackoff:       2 * time.Second,
		StreamingEnabled: true,
		RequestTimeout:   60 * time.Second,
	}

	client := NewClient(logger, WithConfig(config))

	assert.Equal(t, config.MaxTokens, client.maxTokens)
	assert.Equal(t, config.Temperature, client.temperature)
	assert.Equal(t, config.RetryAttempts, client.retryAttempts)
	assert.Equal(t, config.TokenBudget, client.tokenBudget)
	assert.Equal(t, config.BaseBackoff, client.baseBackoff)
	assert.Equal(t, config.MaxBackoff, client.maxBackoff)
	assert.Equal(t, config.StreamingEnabled, client.streamingEnabled)
	assert.Equal(t, config.RequestTimeout, client.requestTimeout)
}

// Benchmark tests

func BenchmarkEstimateTokens(b *testing.B) {
	text := strings.Repeat("word ", 100) // 100 words

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimateTokens(text)
	}
}

func BenchmarkClient_CalculateBackoff(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.calculateBackoff(i % 10)
	}
}
