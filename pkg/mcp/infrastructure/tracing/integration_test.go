package tracing

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitFromServerInfo(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	serverInfo := ServerInfo{
		ServiceName:     "test-service",
		ServiceVersion:  "v1.0.0",
		Environment:     "test",
		TraceSampleRate: 0.5,
	}

	// Test with tracing disabled (default)
	err := InitFromServerInfo(ctx, serverInfo, logger)
	assert.NoError(t, err)

	// Test shutdown
	err = Shutdown(ctx)
	assert.NoError(t, err)
}

func TestConfigFromServerInfo(t *testing.T) {
	serverInfo := ServerInfo{
		ServiceName:     "test-service",
		ServiceVersion:  "v2.1.0",
		Environment:     "production",
		TraceSampleRate: 0.1,
	}

	config := ConfigFromServerInfo(serverInfo)

	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "v2.1.0", config.ServiceVersion)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, 0.1, config.SampleRate)
	assert.False(t, config.Enabled) // Should be disabled by default
}

func TestWorkflowTracer(t *testing.T) {
	workflowID := "test-workflow-123"
	workflowName := "test-containerization"

	tracer := NewWorkflowTracer(workflowID, workflowName)

	assert.Equal(t, workflowID, tracer.workflowID)
	assert.Equal(t, workflowName, tracer.workflowName)

	ctx := context.Background()

	// Test step tracing
	err := tracer.TraceStep(ctx, "test-step", func(ctx context.Context) error {
		// Simulate work
		return nil
	})
	assert.NoError(t, err)

	// Test workflow attributes
	ctx, span := StartSpan(ctx, "test-operation")
	tracer.AddWorkflowAttributes(ctx)
	span.End()
}

func TestMiddlewareHandler(t *testing.T) {
	ctx := context.Background()

	// Create middleware
	handler := MiddlewareHandler(func(ctx context.Context) error {
		// Verify span is in context
		span := SpanFromContext(ctx)
		assert.NotNil(t, span)
		return nil
	})

	// Execute handler
	err := handler(ctx)
	assert.NoError(t, err)
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Save original getEnv function
	originalGetEnv := getEnv
	defer func() { getEnv = originalGetEnv }()

	// Mock environment variables
	envVars := map[string]string{
		"CONTAINER_KIT_OTEL_ENABLED":      "true",
		"CONTAINER_KIT_OTEL_ENDPOINT":     "http://test:4318/v1/traces",
		"CONTAINER_KIT_OTEL_HEADERS":      "api-key=secret,version=v1",
		"CONTAINER_KIT_TRACE_SAMPLE_RATE": "0.25",
	}

	getEnv = func(key string) string {
		return envVars[key]
	}

	serverInfo := ServerInfo{
		ServiceName:     "test-service",
		ServiceVersion:  "v1.0.0",
		Environment:     "test",
		TraceSampleRate: 1.0,
	}

	config := ConfigFromServerInfo(serverInfo)

	assert.True(t, config.Enabled)
	assert.Equal(t, "http://test:4318/v1/traces", config.Endpoint)
	assert.Equal(t, 0.25, config.SampleRate)
	assert.Equal(t, "secret", config.Headers["api-key"])
	assert.Equal(t, "v1", config.Headers["version"])
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{
			input:    "key1=value1,key2=value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			input:    "api-key=secret123, version=v2.0",
			expected: map[string]string{"api-key": "secret123", "version": "v2.0"},
		},
		{
			input:    "",
			expected: map[string]string{},
		},
		{
			input:    "invalid",
			expected: map[string]string{},
		},
		{
			input:    "key1=value1,invalid,key2=value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := parseHeaders(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestTracingHelperFunctions(t *testing.T) {
	ctx := context.Background()

	// Test TraceSamplingRequest
	err := TraceSamplingRequest(ctx, "test-template", func(ctx context.Context) error {
		span := SpanFromContext(ctx)
		assert.NotNil(t, span)
		return nil
	})
	assert.NoError(t, err)

	// Test TraceSamplingValidation
	valid, err := TraceSamplingValidation(ctx, "manifest", func(ctx context.Context) (bool, error) {
		span := SpanFromContext(ctx)
		assert.NotNil(t, span)
		return true, nil
	})
	assert.NoError(t, err)
	assert.True(t, valid)

	// Test TraceProgressUpdate
	err = TraceProgressUpdate(ctx, "workflow-123", "test-step", 3, 10, func(ctx context.Context) error {
		span := SpanFromContext(ctx)
		assert.NotNil(t, span)
		return nil
	})
	assert.NoError(t, err)

	// Test TraceWorkflowStep
	err = TraceWorkflowStep(ctx, "workflow-123", "deploy", func(ctx context.Context) error {
		span := SpanFromContext(ctx)
		assert.NotNil(t, span)
		return nil
	})
	assert.NoError(t, err)
}
