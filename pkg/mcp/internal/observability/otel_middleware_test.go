package observability

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestOTELConfig(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("observability.NewDefaultOTELConfig", func(t *testing.T) {
		config := observability.NewDefaultOTELConfig(logger)

		assert.Equal(t, "container-kit-mcp", config.ServiceName)
		assert.Equal(t, "1.0.0", config.ServiceVersion)
		assert.Equal(t, "development", config.Environment)
		assert.False(t, config.EnableOTLP)
		assert.Equal(t, 1.0, config.TraceSampleRate)
		assert.Equal(t, 10*time.Second, config.OTLPTimeout)
		assert.True(t, config.OTLPInsecure)
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		config := observability.NewDefaultOTELConfig(logger)

		// Valid config should pass
		err := config.Validate()
		assert.NoError(t, err)

		// Invalid service name
		config.ServiceName = ""
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service_name is required")

		// Reset service name
		config.ServiceName = "test-service"

		// Invalid OTLP endpoint when OTLP is enabled
		config.EnableOTLP = true
		config.OTLPEndpoint = ""
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "otlp_endpoint is required")

		// Invalid OTLP endpoint URL
		config.OTLPEndpoint = "://invalid-url"
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid otlp_endpoint URL")

		// Invalid trace sample rate
		config.OTLPEndpoint = "http://localhost:4318/v1/traces"
		config.TraceSampleRate = 1.5
		err = config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trace_sample_rate must be between 0.0 and 1.0")

		// Valid config
		config.TraceSampleRate = 0.5
		err = config.Validate()
		assert.NoError(t, err)
	})
}

func TestOTELProvider(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("Initialize and Shutdown", func(t *testing.T) {
		config := observability.NewDefaultOTELConfig(logger)
		provider := observability.NewOTELProvider(config)

		assert.False(t, provider.IsInitialized())

		// Initialize
		ctx := context.Background()
		err := provider.Initialize(ctx)
		assert.NoError(t, err)
		assert.True(t, provider.IsInitialized())

		// Double initialization should be safe
		err = provider.Initialize(ctx)
		assert.NoError(t, err)

		// Shutdown
		err = provider.Shutdown(ctx)
		assert.NoError(t, err)
		assert.False(t, provider.IsInitialized())

		// Double shutdown should be safe
		err = provider.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("UpdateConfig", func(t *testing.T) {
		config := observability.NewDefaultOTELConfig(logger)
		provider := observability.NewOTELProvider(config)

		updates := map[string]interface{}{
			"otlp_endpoint":     "http://localhost:4318/v1/traces",
			"trace_sample_rate": 0.5,
			"environment":       "testing",
			"otlp_headers": map[string]string{
				"Authorization": "Bearer token",
			},
		}

		provider.UpdateConfig(updates)

		updatedConfig := provider.GetConfig()
		assert.Equal(t, "http://localhost:4318/v1/traces", updatedConfig.OTLPEndpoint)
		assert.True(t, updatedConfig.EnableOTLP)
		assert.Equal(t, 0.5, updatedConfig.TraceSampleRate)
		assert.Equal(t, "testing", updatedConfig.Environment)
		assert.Contains(t, updatedConfig.OTLPHeaders, "Authorization")
	})
}

func TestOTELMiddleware(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("Tool span lifecycle", func(t *testing.T) {
		middleware := observability.NewOTELMiddleware("test-service", logger)
		ctx := context.Background()

		// Start tool span
		span := middleware.StartToolSpan(ctx, "test_tool", map[string]interface{}{
			"session_id": "test-session",
			"dry_run":    true,
		})

		assert.NotNil(t, span)
		assert.NotEqual(t, ctx, span.Context()) // Should have new context with span

		// Add event
		span.AddEvent("tool_started", map[string]interface{}{
			"timestamp": time.Now().Unix(),
		})

		// Set attributes
		span.SetAttributes(map[string]interface{}{
			"tool.version": "1.0.0",
			"tool.mode":    "test",
		})

		// Finish successfully
		span.Finish(true, 1024)
	})

	t.Run("Request span lifecycle", func(t *testing.T) {
		middleware := observability.NewOTELMiddleware("test-service", logger)
		ctx := context.Background()

		// Start request span
		span := middleware.StartRequestSpan(ctx, "tools/call", map[string]interface{}{
			"request_id": "req-123",
		})

		assert.NotNil(t, span)

		// Add event
		span.AddEvent("request_parsed", nil)

		// Set attributes
		span.SetAttributes(map[string]interface{}{
			"request.size": 512,
		})

		// Finish with success
		span.Finish(200, 2048)
	})

	t.Run("Conversation span lifecycle", func(t *testing.T) {
		middleware := observability.NewOTELMiddleware("test-service", logger)
		ctx := context.Background()

		// Start conversation span
		span := middleware.StartConversationSpan(ctx, "analysis", "test-session")

		assert.NotNil(t, span)

		// Add event
		span.AddEvent("stage_started", map[string]interface{}{
			"user_input": "analyze repository",
		})

		// Finish successfully
		span.Finish(true, "dockerfile")
	})

	t.Run("Error handling", func(t *testing.T) {
		middleware := observability.NewOTELMiddleware("test-service", logger)
		ctx := context.Background()

		span := middleware.StartToolSpan(ctx, "failing_tool", nil)

		// Record error
		testErr := assert.AnError
		span.RecordError(testErr, "Tool execution failed")

		// Finish with failure
		span.Finish(false, 0)
	})
}

func TestMCPServerInstrumentation(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("InstrumentTool success", func(t *testing.T) {
		instrumentation := observability.NewMCPServerInstrumentation("test-service", logger)
		ctx := context.Background()

		expectedResult := map[string]interface{}{
			"success": true,
			"data":    "test result",
		}

		// Instrument a successful tool execution
		result, err := instrumentation.InstrumentTool(ctx, "test_tool", func(ctx context.Context) (interface{}, error) {
			// Simulate some work
			time.Sleep(1 * time.Millisecond)
			return expectedResult, nil
		})

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("InstrumentTool failure", func(t *testing.T) {
		instrumentation := observability.NewMCPServerInstrumentation("test-service", logger)
		ctx := context.Background()

		testErr := assert.AnError

		// Instrument a failing tool execution
		result, err := instrumentation.InstrumentTool(ctx, "failing_tool", func(ctx context.Context) (interface{}, error) {
			return nil, testErr
		})

		assert.Error(t, err)
		assert.Equal(t, testErr, err)
		assert.Nil(t, result)
	})
}

func TestTelemetryManagerWithOTEL(t *testing.T) {
	logger := zerolog.New(os.Stderr)

	t.Run("TelemetryManager with OTEL", func(t *testing.T) {
		otelConfig := &observability.OTELConfig{
			ServiceName:     "test-service",
			ServiceVersion:  "1.0.0",
			Environment:     "test",
			EnableOTLP:      false, // Disable OTLP for testing
			TraceSampleRate: 1.0,
			Logger:          logger,
		}

		config := observability.TelemetryConfig{
			MetricsPort:      0, // Use random port
			Logger:           logger,
			EnableAutoExport: false, // Don't start HTTP server
			OTELConfig:       otelConfig,
		}

		telemetryMgr := observability.NewTelemetryManager(config)
		assert.NotNil(t, telemetryMgr)
		assert.True(t, telemetryMgr.IsOTELEnabled())

		// Test OTEL provider access
		provider := telemetryMgr.GetOTELProvider()
		assert.NotNil(t, provider)
		assert.True(t, provider.IsInitialized())

		// Test config updates
		telemetryMgr.UpdateOTELConfig(map[string]interface{}{
			"environment": "updated-test",
		})

		// Shutdown
		ctx := context.Background()
		err := telemetryMgr.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("TelemetryManager without OTEL", func(t *testing.T) {
		config := observability.TelemetryConfig{
			MetricsPort:      0,
			Logger:           logger,
			EnableAutoExport: false,
			OTELConfig:       nil, // No OTEL config
		}

		telemetryMgr := observability.NewTelemetryManager(config)
		assert.NotNil(t, telemetryMgr)
		assert.False(t, telemetryMgr.IsOTELEnabled())

		provider := telemetryMgr.GetOTELProvider()
		assert.Nil(t, provider)

		// Shutdown should still work
		ctx := context.Background()
		err := telemetryMgr.Shutdown(ctx)
		assert.NoError(t, err)
	})
}
