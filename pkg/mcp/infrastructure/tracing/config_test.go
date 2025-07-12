package tracing

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
	assert.Equal(t, "container-kit-mcp", config.ServiceName)
	assert.Equal(t, "dev", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, 1.0, config.SampleRate)
	assert.Equal(t, 30*time.Second, config.ExportTimeout)
	assert.NotNil(t, config.Headers)
}

func TestInitializeTracingDisabled(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = false

	err := InitializeTracing(ctx, config)
	assert.NoError(t, err)

	// Should set a no-op tracer provider
	provider := otel.GetTracerProvider()
	tracer := provider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	assert.False(t, span.IsRecording())
}

func TestGetTracer(t *testing.T) {
	tracer := GetTracer("test-component")
	assert.NotNil(t, tracer)

	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()

	newCtx, span := StartSpan(ctx, "test-operation")
	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)

	// Verify span is in context
	extractedSpan := trace.SpanFromContext(newCtx)
	assert.Equal(t, span, extractedSpan)
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()

	// No span in context
	span := SpanFromContext(ctx)
	assert.NotNil(t, span)
	assert.False(t, span.IsRecording())

	// With span in context
	ctxWithSpan, newSpan := StartSpan(ctx, "test")
	extractedSpan := SpanFromContext(ctxWithSpan)
	assert.Equal(t, newSpan, extractedSpan)
}

func TestShutdown(t *testing.T) {
	ctx := context.Background()

	// Test shutdown with no provider
	err := Shutdown(ctx)
	assert.NoError(t, err)

	// Test shutdown after initialization
	config := DefaultConfig()
	config.Enabled = false
	InitializeTracing(ctx, config)

	err = Shutdown(ctx)
	assert.NoError(t, err)
}
