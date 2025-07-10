package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"go.opentelemetry.io/otel/attribute"
)

// This file demonstrates how to integrate telemetry into Container Kit components
// It provides examples for common usage patterns

// ExampleToolExecution shows how to instrument a tool execution
func ExampleToolExecution(tm *Manager) {
	ctx := context.Background()

	// Example: Instrument a tool execution
	err := tm.InstrumentToolExecution(ctx, "analyze", func(ctx context.Context) error {
		// Add contextual information
		tm.AddContextualAttributes(ctx,
			attribute.String("tool.version", "1.0.0"),
			attribute.String("repository.path", "/workspace/myapp"),
			attribute.String("repository.language", "go"),
		)

		// Record significant events
		tm.RecordEvent(ctx, "tool.analysis.started",
			attribute.String("analysis.type", "dockerfile_generation"),
		)

		// Simulate tool work
		time.Sleep(100 * time.Millisecond)

		// Record completion
		tm.RecordEvent(ctx, "tool.analysis.completed",
			attribute.Int("dockerfile.lines", 25),
			attribute.Bool("dockerfile.multistage", true),
		)

		return nil
	})

	if err != nil {
		fmt.Printf("Tool execution failed: %v\n", err)
	}
}

// ExamplePipelineExecution shows how to instrument a pipeline
func ExamplePipelineExecution(tm *Manager) {
	ctx := context.Background()
	pipelineName := "container-build"

	// Start pipeline-level span
	ctx, span := tm.Tracing().StartSpan(ctx, fmt.Sprintf("pipeline.%s", pipelineName))
	defer span.End()

	tm.AddContextualAttributes(ctx,
		attribute.String("pipeline.name", pipelineName),
		attribute.String("pipeline.type", "containerization"),
	)

	stages := []string{"analyze", "build", "scan", "deploy"}
	start := time.Now()

	for i, stage := range stages {
		err := tm.InstrumentPipelineStage(ctx, pipelineName, stage, func(ctx context.Context) error {
			// Add stage-specific attributes
			tm.AddContextualAttributes(ctx,
				attribute.Int("pipeline.stage.index", i),
				attribute.Int("pipeline.stage.total", len(stages)),
			)

			// Simulate stage work
			time.Sleep(50 * time.Millisecond)

			// Record stage events
			tm.RecordEvent(ctx, fmt.Sprintf("stage.%s.completed", stage),
				attribute.String("stage.result", "success"),
			)

			return nil
		})

		if err != nil {
			tm.Tracing().RecordError(span, err)
			break
		}
	}

	// Record pipeline metrics
	duration := time.Since(start)
	tm.Metrics().RecordPipelineExecution(ctx, pipelineName, duration, len(stages), nil)
}

// ExampleHTTPHandler shows how to instrument HTTP requests
func ExampleHTTPHandler(tm *Manager) {
	// Example HTTP handler with telemetry
	handler := func(ctx context.Context, method, path string) (int, error) {
		statusCode, err := tm.InstrumentHTTPRequest(ctx, method, path, func(ctx context.Context) (int, error) {
			// Add request-specific attributes
			tm.AddContextualAttributes(ctx,
				attribute.String("http.user_agent", "container-kit-client/1.0"),
				attribute.String("http.remote_addr", "192.168.1.100"),
			)

			// Simulate request processing
			time.Sleep(25 * time.Millisecond)

			// Return success
			return 200, nil
		})

		return statusCode, err
	}

	// Simulate some HTTP requests
	ctx := context.Background()
	_, _ = handler(ctx, "POST", "/api/tools/analyze")
	_, _ = handler(ctx, "GET", "/api/sessions")
	_, _ = handler(ctx, "POST", "/api/pipelines/execute")
}

// ExampleSessionInstrumentation shows how to instrument session operations
func ExampleSessionInstrumentation(tm *Manager) {
	ctx := context.Background()

	// Create session with telemetry
	sessionID := "session-123"
	start := time.Now()

	ctx, span := tm.Tracing().StartSpan(ctx, "session.create")
	defer span.End()

	tm.AddContextualAttributes(ctx,
		attribute.String("session.id", sessionID),
		attribute.String("session.type", "containerization"),
		attribute.String("user.id", "user-456"),
	)

	// Record session creation
	tm.Metrics().RecordSessionCreation(ctx, "containerization")

	// Simulate session work
	time.Sleep(200 * time.Millisecond)

	// Record session completion
	duration := time.Since(start)
	tm.Metrics().RecordSessionDuration(ctx, "containerization", duration)

	tm.RecordEvent(ctx, "session.completed",
		attribute.String("session.result", "success"),
		attribute.Int("session.tools_executed", 3),
	)
}

// ExampleErrorHandling shows how to handle errors with telemetry
func ExampleErrorHandling(tm *Manager) {
	ctx := context.Background()

	err := tm.InstrumentToolExecution(ctx, "problematic-tool", func(ctx context.Context) error {
		// Add context before potential failure
		tm.AddContextualAttributes(ctx,
			attribute.String("tool.version", "1.0.0"),
			attribute.String("input.file", "Dockerfile"),
		)

		// Record attempt
		tm.RecordEvent(ctx, "tool.validation.started")

		// Simulate an error condition
		if true { // Simulate error condition
			// Record the specific failure point
			tm.RecordEvent(ctx, "tool.validation.failed",
				attribute.String("failure.reason", "invalid_syntax"),
				attribute.String("failure.line", "15"),
			)

			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Message("dockerfile validation failed: invalid syntax on line 15").
				WithLocation().
				Build()
		}

		return nil
	})

	if err != nil {
		// Error is already recorded in span by InstrumentToolExecution
		fmt.Printf("Tool failed as expected: %v\n", err)
	}
}

// ExampleCustomMetrics shows how to create and use custom metrics
func ExampleCustomMetrics(tm *Manager) {
	ctx := context.Background()

	// Record various metrics during operation
	tm.Metrics().RecordToolExecution(ctx, "custom-tool", 150*time.Millisecond, nil)
	tm.Metrics().RecordToolExecution(ctx, "custom-tool", 200*time.Millisecond, errors.NewError().
		Code(errors.CodeOperationFailed).
		Type(errors.ErrTypeOperation).
		Severity(errors.SeverityMedium).
		Message("failed").
		WithLocation().
		Build())

	tm.Metrics().RecordHTTPRequest(ctx, "GET", "/health", 200, 5*time.Millisecond)
	tm.Metrics().RecordHTTPRequest(ctx, "POST", "/api/tools", 201, 120*time.Millisecond)

	tm.Metrics().RecordGCDuration(ctx, 2*time.Millisecond)
}

// ExampleDistributedTracing shows how to handle distributed tracing
func ExampleDistributedTracing(tm *Manager) {
	// Simulate a request that spans multiple services
	ctx := context.Background()

	// Root span for the entire operation
	ctx, rootSpan := tm.Tracing().StartSpan(ctx, "api.container.build")
	defer rootSpan.End()

	// Get trace ID for logging correlation
	traceID := tm.GetTraceID(ctx)
	spanID := tm.GetSpanID(ctx)

	fmt.Printf("Processing request with trace_id=%s span_id=%s\n", traceID, spanID)

	// Child span for validation
	ctx, validationSpan := tm.Tracing().StartSpan(ctx, "validation.dockerfile")
	tm.AddContextualAttributes(ctx,
		attribute.String("validation.type", "dockerfile"),
		attribute.String("file.path", "/workspace/Dockerfile"),
	)
	time.Sleep(50 * time.Millisecond)
	validationSpan.End()

	// Child span for build process
	ctx, buildSpan := tm.Tracing().StartSpan(ctx, "build.docker")
	tm.AddContextualAttributes(ctx,
		attribute.String("build.context", "/workspace"),
		attribute.String("build.target", "production"),
	)
	time.Sleep(150 * time.Millisecond)
	buildSpan.End()

	// Child span for registry push
	ctx, pushSpan := tm.Tracing().StartSpan(ctx, "registry.push")
	tm.AddContextualAttributes(ctx,
		attribute.String("registry.url", "myregistry.azurecr.io"),
		attribute.String("image.tag", "myapp:latest"),
	)
	time.Sleep(100 * time.Millisecond)
	pushSpan.End()
}

// ExampleTelemetryConfiguration shows different configuration scenarios
func ExampleTelemetryConfiguration() {
	// Development configuration
	devConfig := &Config{
		ServiceName:     "container-kit-dev",
		ServiceVersion:  "dev",
		Environment:     "development",
		TracingEnabled:  true,
		TracingEndpoint: "",  // Will use stdout exporter
		TraceSampleRate: 1.0, // Sample all traces in dev
		MetricsEnabled:  true,
		MetricsInterval: 5 * time.Second,
	}

	// Production configuration
	prodConfig := &Config{
		ServiceName:     "container-kit",
		ServiceVersion:  "1.0.0",
		Environment:     "production",
		TracingEnabled:  true,
		TracingEndpoint: "http://jaeger-collector:14268/api/traces",
		TraceSampleRate: 0.1, // Sample 10% of traces in prod
		MetricsEnabled:  true,
		MetricsInterval: 15 * time.Second,
		ResourceAttributes: map[string]string{
			"deployment.region": "us-west-2",
			"deployment.zone":   "us-west-2a",
			"cluster.name":      "prod-cluster",
		},
	}

	fmt.Printf("Dev config: %+v\n", devConfig)
	fmt.Printf("Prod config: %+v\n", prodConfig)
}

// ExampleFullIntegration shows a complete integration example
func ExampleFullIntegration() {
	// Initialize telemetry
	config := DefaultConfig()
	tm := NewManager(config)

	ctx := context.Background()
	if err := tm.Initialize(ctx); err != nil {
		panic(fmt.Sprintf("Failed to initialize telemetry: %v", err))
	}
	defer tm.Shutdown(ctx)

	// Simulate application startup
	ctx, span := tm.Tracing().StartSpan(ctx, "app.startup")
	tm.RecordEvent(ctx, "app.startup.started")

	// Initialize components with telemetry
	initializeComponents(ctx, tm)

	// Process some requests
	processRequests(ctx, tm)

	tm.RecordEvent(ctx, "app.startup.completed")
	span.End()

	fmt.Println("Application run completed with full telemetry")
}

func initializeComponents(ctx context.Context, tm *Manager) {
	ctx, span := tm.Tracing().StartSpan(ctx, "components.initialize")
	defer span.End()

	components := []string{"registry", "sessions", "pipeline", "transport"}
	for _, component := range components {
		ctx, componentSpan := tm.Tracing().StartSpan(ctx, fmt.Sprintf("component.%s.init", component))

		// Simulate component initialization
		time.Sleep(20 * time.Millisecond)

		tm.RecordEvent(ctx, "component.initialized",
			attribute.String("component.name", component),
		)
		componentSpan.End()
	}
}

func processRequests(ctx context.Context, tm *Manager) {
	requests := []struct {
		tool   string
		params map[string]string
	}{
		{"analyze", map[string]string{"repo": "/workspace/app1", "lang": "go"}},
		{"build", map[string]string{"dockerfile": "Dockerfile", "tag": "app1:latest"}},
		{"scan", map[string]string{"image": "app1:latest", "severity": "HIGH"}},
	}

	for _, req := range requests {
		err := tm.InstrumentToolExecution(ctx, req.tool, func(ctx context.Context) error {
			for key, value := range req.params {
				tm.AddContextualAttributes(ctx, attribute.String(key, value))
			}

			// Simulate tool execution
			time.Sleep(80 * time.Millisecond)
			return nil
		})

		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
		}
	}
}
