package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ServiceName != "container-kit" {
		t.Errorf("Expected service name 'container-kit', got '%s'", config.ServiceName)
	}

	if config.TracingEnabled != true {
		t.Error("Expected tracing to be enabled by default")
	}

	if config.MetricsEnabled != true {
		t.Error("Expected metrics to be enabled by default")
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				ServiceName:     "test-service",
				ServiceVersion:  "1.0.0",
				TraceSampleRate: 0.5,
				MetricsInterval: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			config: &Config{
				ServiceName:     "",
				ServiceVersion:  "1.0.0",
				TraceSampleRate: 0.5,
				MetricsInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid sample rate",
			config: &Config{
				ServiceName:     "test-service",
				ServiceVersion:  "1.0.0",
				TraceSampleRate: 1.5,
				MetricsInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid metrics interval",
			config: &Config{
				ServiceName:     "test-service",
				ServiceVersion:  "1.0.0",
				TraceSampleRate: 0.5,
				MetricsInterval: -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTelemetryManager(t *testing.T) {
	config := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  true,
		TraceSampleRate: 1.0,
		MetricsInterval: 1 * time.Second,
	}

	tm := NewManager(config)
	if tm == nil {
		t.Fatal("NewManager returned nil")
	}

	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	// Test that managers are available
	if tm.Tracing() == nil {
		t.Error("Tracing manager is nil")
	}

	if tm.Metrics() == nil {
		t.Error("Metrics manager is nil")
	}

	if tm.Config() != config {
		t.Error("Config mismatch")
	}
}

func TestInstrumentToolExecution(t *testing.T) {
	config := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false, // Disable metrics for simpler test
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	// Test successful tool execution
	executed := false
	err = tm.InstrumentToolExecution(ctx, "test-tool", func(ctx context.Context) error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("InstrumentToolExecution returned error: %v", err)
	}

	if !executed {
		t.Error("Tool function was not executed")
	}

	// Test tool execution with error
	testErr := &testError{"test error"}
	err = tm.InstrumentToolExecution(ctx, "failing-tool", func(ctx context.Context) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
}

func TestInstrumentPipelineStage(t *testing.T) {
	config := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	executed := false
	err = tm.InstrumentPipelineStage(ctx, "test-pipeline", "test-stage", func(ctx context.Context) error {
		executed = true

		// Verify we can add attributes and events
		tm.AddContextualAttributes(ctx, attribute.String("test.key", "test.value"))
		tm.RecordEvent(ctx, "test.event")

		return nil
	})

	if err != nil {
		t.Errorf("InstrumentPipelineStage returned error: %v", err)
	}

	if !executed {
		t.Error("Pipeline stage function was not executed")
	}
}

func TestHTTPInstrumentation(t *testing.T) {
	config := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	executed := false
	statusCode, err := tm.InstrumentHTTPRequest(ctx, "GET", "/test", func(ctx context.Context) (int, error) {
		executed = true
		return 200, nil
	})

	if err != nil {
		t.Errorf("InstrumentHTTPRequest returned error: %v", err)
	}

	if statusCode != 200 {
		t.Errorf("Expected status code 200, got %d", statusCode)
	}

	if !executed {
		t.Error("HTTP handler function was not executed")
	}
}

func TestTracingEnabledDisabled(t *testing.T) {
	// Test with tracing enabled
	enabledConfig := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tmEnabled := NewManager(enabledConfig)
	ctx := context.Background()
	err := tmEnabled.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tmEnabled.Shutdown(ctx)

	if !tmEnabled.IsTracingEnabled() {
		t.Error("Expected tracing to be enabled")
	}

	// Test with tracing disabled
	disabledConfig := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  false,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tmDisabled := NewManager(disabledConfig)
	err = tmDisabled.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tmDisabled.Shutdown(ctx)

	if tmDisabled.IsTracingEnabled() {
		t.Error("Expected tracing to be disabled")
	}
}

func TestTraceAndSpanIDs(t *testing.T) {
	config := &Config{
		ServiceName:     "test-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	// Test within a span
	err = tm.InstrumentToolExecution(ctx, "test-tool", func(ctx context.Context) error {
		traceID := tm.GetTraceID(ctx)
		spanID := tm.GetSpanID(ctx)

		// IDs should be non-empty within an active span
		if traceID == "" {
			t.Error("Expected non-empty trace ID")
		}

		if spanID == "" {
			t.Error("Expected non-empty span ID")
		}

		return nil
	})

	if err != nil {
		t.Errorf("Tool execution failed: %v", err)
	}
}

func BenchmarkInstrumentToolExecution(b *testing.B) {
	config := &Config{
		ServiceName:     "bench-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  true,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		b.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tm.InstrumentToolExecution(ctx, "bench-tool", func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			b.Errorf("Tool execution failed: %v", err)
		}
	}
}

func BenchmarkInstrumentToolExecutionNoTracing(b *testing.B) {
	config := &Config{
		ServiceName:     "bench-service",
		ServiceVersion:  "test",
		Environment:     "test",
		TracingEnabled:  false,
		MetricsEnabled:  false,
		TraceSampleRate: 1.0,
		MetricsInterval: 10 * time.Second,
	}

	tm := NewManager(config)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	if err != nil {
		b.Fatalf("Failed to initialize telemetry manager: %v", err)
	}
	defer tm.Shutdown(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tm.InstrumentToolExecution(ctx, "bench-tool", func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			b.Errorf("Tool execution failed: %v", err)
		}
	}
}

// Helper types for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
