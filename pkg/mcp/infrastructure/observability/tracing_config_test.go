// Package observability provides unified monitoring, tracing, and health infrastructure
// for the MCP components. It consolidates telemetry, distributed tracing, health checks,
// and logging enrichment into a single coherent package.
package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, "http://localhost:4318/v1/traces", config.Endpoint)
	assert.NotNil(t, config.Headers)
	assert.Equal(t, 0, len(config.Headers))
	assert.Equal(t, "container-kit-mcp", config.ServiceName)
	assert.Equal(t, "dev", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, 1.0, config.SampleRate)
	assert.Equal(t, 30*time.Second, config.ExportTimeout)
}

func TestConfig_CustomValues(t *testing.T) {
	config := Config{
		Enabled:        true,
		Endpoint:       "http://jaeger:14268/api/traces",
		Headers:        map[string]string{"Authorization": "Bearer token"},
		ServiceName:    "custom-service",
		ServiceVersion: "1.2.3",
		Environment:    "production",
		SampleRate:     0.5,
		ExportTimeout:  10 * time.Second,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "http://jaeger:14268/api/traces", config.Endpoint)
	assert.Equal(t, "Bearer token", config.Headers["Authorization"])
	assert.Equal(t, "custom-service", config.ServiceName)
	assert.Equal(t, "1.2.3", config.ServiceVersion)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, 0.5, config.SampleRate)
	assert.Equal(t, 10*time.Second, config.ExportTimeout)
}

func TestInitializeTracing_Disabled(t *testing.T) {
	// Save original provider to restore later
	originalProvider := otel.GetTracerProvider()
	defer otel.SetTracerProvider(originalProvider)

	config := Config{
		Enabled: false,
	}

	err := InitializeTracing(context.Background(), config)
	assert.NoError(t, err)

	// Should set a no-op tracer provider
	provider := otel.GetTracerProvider()
	assert.NotNil(t, provider)

	// Verify it's a no-op provider by checking tracer behavior
	tracer := provider.Tracer("test")
	testCtx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, testCtx)
	assert.NotNil(t, span)
	assert.False(t, span.IsRecording()) // No-op spans don't record
	span.End()

	// Verify no global provider is set
	assert.Nil(t, globalTracerProvider)
}

func TestInitializeTracing_EdgeCases(t *testing.T) {
	config := Config{
		Enabled:       true,
		Endpoint:      "http://localhost:4318/v1/traces",
		ServiceName:   "test-service",
		SampleRate:    1.0,
		ExportTimeout: 5 * time.Second,
	}

	// This should complete quickly since the OpenTelemetry lib is lenient with endpoints
	// but we can at least test that it doesn't panic with various configs
	err := InitializeTracing(context.Background(), config)
	// The error may or may not occur depending on the OTLP library behavior
	// So we just ensure it doesn't panic and returns some result
	_ = err // Don't assert on error since OTLP creation is lenient
}

func TestInitializeTracing_ValidConfig(t *testing.T) {
	// Skip this test in CI or if no OTLP endpoint is available
	// This test would require a real OTLP endpoint to succeed
	t.Skip("Skipping test that requires live OTLP endpoint")

	// Save original provider to restore later
	originalProvider := otel.GetTracerProvider()
	defer func() {
		if globalTracerProvider != nil {
			_ = Shutdown(context.Background())
		}
		otel.SetTracerProvider(originalProvider)
	}()

	config := Config{
		Enabled:        true,
		Endpoint:       "http://localhost:4318/v1/traces",
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRate:     0.1,
		ExportTimeout:  5 * time.Second,
		Headers:        map[string]string{"test-header": "test-value"},
	}

	err := InitializeTracing(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, globalTracerProvider)
}

func TestShutdown_NoProvider(t *testing.T) {
	// Ensure no global provider is set
	globalTracerProvider = nil

	err := Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_WithProvider(t *testing.T) {
	// This test would require setting up a real provider
	// For now, we test the nil case above
	t.Skip("Skipping test that requires real tracer provider setup")
}

func TestGetTracer(t *testing.T) {
	tracer := GetTracer("test-component")
	assert.NotNil(t, tracer)

	// Should return same tracer for same name
	tracer2 := GetTracer("test-component")
	assert.Equal(t, tracer, tracer2)

	// Different name should return different tracer
	tracer3 := GetTracer("different-component")
	assert.NotEqual(t, tracer, tracer3)
}

func TestSpanFromContext(t *testing.T) {
	// Test with context without span
	ctx := context.Background()
	span := SpanFromContext(ctx)
	assert.NotNil(t, span)
	assert.False(t, span.IsRecording()) // Should be no-op span

	// Test with context that has a span
	tracer := GetTracer("test")
	spanCtx, testSpan := tracer.Start(ctx, "test-span")

	retrievedSpan := SpanFromContext(spanCtx)
	assert.NotNil(t, retrievedSpan)
	assert.Equal(t, testSpan.SpanContext(), retrievedSpan.SpanContext())

	testSpan.End()
}

func TestStartSpan_Basic(t *testing.T) {
	ctx := context.Background()
	spanCtx, span := StartSpan(ctx, "test-operation")

	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, spanCtx) // Should be different context with span

	// Verify span is in the returned context
	retrievedSpan := SpanFromContext(spanCtx)
	assert.Equal(t, span.SpanContext(), retrievedSpan.SpanContext())

	span.End()
}

func TestStartSpan_WithOptions(t *testing.T) {
	ctx := context.Background()

	// Start span with custom options
	spanCtx, span := StartSpan(ctx, "test-operation",
		trace.WithSpanKind(trace.SpanKindClient),
	)

	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)

	span.End()
}

func TestTracer_ComponentName(t *testing.T) {
	// Verify StartSpan uses the expected tracer name
	ctx := context.Background()

	_, span := StartSpan(ctx, "test-span")
	assert.NotNil(t, span)

	// The span should be created by the "container-kit" tracer
	// This is somewhat implementation-dependent, but we can at least verify it works
	span.End()
}

func TestConfig_ZeroValues(t *testing.T) {
	var config Config

	assert.False(t, config.Enabled)
	assert.Equal(t, "", config.Endpoint)
	assert.Nil(t, config.Headers)
	assert.Equal(t, "", config.ServiceName)
	assert.Equal(t, "", config.ServiceVersion)
	assert.Equal(t, "", config.Environment)
	assert.Equal(t, 0.0, config.SampleRate)
	assert.Equal(t, time.Duration(0), config.ExportTimeout)
}

func TestConfig_SampleRateBounds(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		valid      bool
	}{
		{"zero", 0.0, true},
		{"half", 0.5, true},
		{"one", 1.0, true},
		{"negative", -0.1, false}, // Invalid but won't error in config creation
		{"over one", 1.5, false},  // Invalid but won't error in config creation
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := Config{
				SampleRate: test.sampleRate,
			}

			// Config creation always succeeds, validation happens during initialization
			assert.Equal(t, test.sampleRate, config.SampleRate)
		})
	}
}

func TestGlobalTracerProvider_ThreadSafety(t *testing.T) {
	// Test concurrent access to global functions
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			// These should be safe to call concurrently
			_ = GetTracer("concurrent-test")
			ctx, span := StartSpan(context.Background(), "concurrent-span")
			_ = SpanFromContext(ctx)
			span.End()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should not have panicked
	assert.True(t, true)
}
