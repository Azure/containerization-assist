package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingIntegration provides integration patterns for distributed tracing
type TracingIntegration struct {
	manager *TracingManager
}

// NewTracingIntegration creates a new tracing integration helper
func NewTracingIntegration(manager *TracingManager) *TracingIntegration {
	return &TracingIntegration{
		manager: manager,
	}
}

// ToolExecutionTracer traces tool execution with detailed insights
type ToolExecutionTracer struct {
	integration *TracingIntegration
}

// TraceToolExecution traces a complete tool execution lifecycle
func (ti *TracingIntegration) TraceToolExecution(ctx context.Context, toolName string, fn func(context.Context) error) error {
	// Start tool span
	ctx, span := ti.manager.StartToolSpan(ctx, toolName, "execute")
	defer span.End()

	// Add tool metadata
	span.SetAttributes(
		attribute.String("tool.category", categorizeToolName(toolName)),
		attribute.String("tool.version", "1.0.0"), // Would get from registry
		attribute.Int64("tool.execution.start_time", time.Now().UnixNano()),
	)

	// Pre-execution phase
	ctx, preSpan := ti.manager.StartSpan(ctx, fmt.Sprintf("%s.pre_execution", toolName))
	ti.manager.AddEvent(ctx, "validation_start")
	// Simulate validation
	time.Sleep(10 * time.Millisecond)
	ti.manager.AddEvent(ctx, "validation_complete")
	preSpan.End()

	// Main execution phase
	ctx, execSpan := ti.manager.StartSpan(ctx, fmt.Sprintf("%s.execution", toolName))

	// Execute the tool
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Record execution metrics
	execSpan.SetAttributes(
		attribute.Float64("tool.execution.duration_ms", duration.Seconds()*1000),
		attribute.Bool("tool.execution.success", err == nil),
	)

	if err != nil {
		ti.manager.RecordError(ctx, err)
	}
	execSpan.End()

	// Post-execution phase
	ctx, postSpan := ti.manager.StartSpan(ctx, fmt.Sprintf("%s.post_execution", toolName))
	ti.manager.AddEvent(ctx, "cleanup_start")
	// Simulate cleanup
	time.Sleep(5 * time.Millisecond)
	ti.manager.AddEvent(ctx, "cleanup_complete")
	postSpan.End()

	// Set final span attributes
	span.SetAttributes(
		attribute.Float64("tool.total_duration_ms", time.Since(start).Seconds()*1000),
		attribute.Bool("tool.success", err == nil),
	)

	return err
}

// TraceWorkflow traces a multi-step workflow
func (ti *TracingIntegration) TraceWorkflow(ctx context.Context, workflowName string, steps []WorkflowStep) error {
	// Start workflow span
	ctx, span := ti.manager.StartSpan(ctx, fmt.Sprintf("workflow.%s", workflowName),
		trace.WithAttributes(
			attribute.String("workflow.name", workflowName),
			attribute.Int("workflow.total_steps", len(steps)),
		),
	)
	defer span.End()

	// Execute each step
	for i, step := range steps {
		// Start step span
		stepCtx, stepSpan := ti.manager.StartSpan(ctx, fmt.Sprintf("%s.step_%d_%s", workflowName, i+1, step.Name),
			trace.WithAttributes(
				attribute.Int("workflow.step.number", i+1),
				attribute.String("workflow.step.name", step.Name),
				attribute.String("workflow.step.type", step.Type),
			),
		)

		// Execute step
		err := ti.executeWorkflowStep(stepCtx, step)

		if err != nil {
			ti.manager.RecordError(stepCtx, err)
			stepSpan.SetAttributes(attribute.Bool("workflow.step.success", false))
			stepSpan.End()

			// Decide whether to continue or abort
			if !step.ContinueOnError {
				span.SetAttributes(
					attribute.Bool("workflow.completed", false),
					attribute.Int("workflow.failed_at_step", i+1),
				)
				return fmt.Errorf("workflow failed at step %d (%s): %w", i+1, step.Name, err)
			}
		} else {
			stepSpan.SetAttributes(attribute.Bool("workflow.step.success", true))
		}

		stepSpan.End()
	}

	span.SetAttributes(attribute.Bool("workflow.completed", true))
	return nil
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name            string
	Type            string
	Handler         func(context.Context) error
	ContinueOnError bool
	Timeout         time.Duration
}

func (ti *TracingIntegration) executeWorkflowStep(ctx context.Context, step WorkflowStep) error {
	// Apply timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Record step start
	ti.manager.AddEvent(ctx, "step_started",
		attribute.String("step.name", step.Name),
		attribute.String("step.type", step.Type),
	)

	// Execute step
	err := step.Handler(ctx)

	// Record step completion
	ti.manager.AddEvent(ctx, "step_completed",
		attribute.Bool("step.success", err == nil),
	)

	return err
}

// TraceDatabaseOperation traces database operations with query details
func (ti *TracingIntegration) TraceDatabaseOperation(ctx context.Context, dbType, operation string, queryFn func(context.Context) error) error {
	// Start database span
	ctx, span := ti.manager.StartDatabaseSpan(ctx, dbType, operation, "")
	defer span.End()

	// Add database attributes
	span.SetAttributes(
		attribute.String("db.connection_string", "masked"), // Never log actual connection strings
		attribute.String("db.user", "app_user"),
	)

	// Execute query
	start := time.Now()
	err := queryFn(ctx)
	duration := time.Since(start)

	// Record query metrics
	span.SetAttributes(
		attribute.Float64("db.query.duration_ms", duration.Seconds()*1000),
		attribute.Bool("db.query.success", err == nil),
	)

	if err != nil {
		ti.manager.RecordError(ctx, err)
	}

	return err
}

// TraceAsyncOperation traces asynchronous operations
func (ti *TracingIntegration) TraceAsyncOperation(ctx context.Context, operationName string, asyncFn func(context.Context) chan error) error {
	// Start async operation span
	ctx, span := ti.manager.StartSpan(ctx, fmt.Sprintf("async.%s", operationName),
		trace.WithAttributes(
			attribute.String("operation.type", "async"),
			attribute.String("operation.name", operationName),
		),
	)
	defer span.End()

	// Create trace context for async operation
	traceCtx := ti.manager.GetTraceContext(ctx)
	span.SetAttributes(
		attribute.String("async.trace_id", traceCtx.TraceID),
		attribute.String("async.parent_span_id", traceCtx.SpanID),
	)

	// Start async operation
	ti.manager.AddEvent(ctx, "async_operation_started")
	errChan := asyncFn(ctx)

	// Wait for completion
	select {
	case err := <-errChan:
		if err != nil {
			ti.manager.RecordError(ctx, err)
			span.SetAttributes(attribute.Bool("async.success", false))
			return err
		}
		span.SetAttributes(attribute.Bool("async.success", true))
		ti.manager.AddEvent(ctx, "async_operation_completed")
		return nil

	case <-ctx.Done():
		err := ctx.Err()
		ti.manager.RecordError(ctx, err)
		span.SetAttributes(
			attribute.Bool("async.success", false),
			attribute.Bool("async.cancelled", true),
		)
		return err
	}
}

// TraceBatch traces batch operations with per-item tracking
func (ti *TracingIntegration) TraceBatch(ctx context.Context, batchName string, items []interface{}, processFn func(context.Context, interface{}) error) error {
	// Start batch span
	ctx, span := ti.manager.StartSpan(ctx, fmt.Sprintf("batch.%s", batchName),
		trace.WithAttributes(
			attribute.String("batch.name", batchName),
			attribute.Int("batch.size", len(items)),
		),
	)
	defer span.End()

	successCount := 0
	errorCount := 0

	// Process each item
	for i, item := range items {
		// Start item span
		itemCtx, itemSpan := ti.manager.StartSpan(ctx, fmt.Sprintf("%s.item_%d", batchName, i),
			trace.WithAttributes(
				attribute.Int("batch.item.index", i),
			),
		)

		// Process item
		err := processFn(itemCtx, item)

		if err != nil {
			ti.manager.RecordError(itemCtx, err)
			itemSpan.SetAttributes(attribute.Bool("batch.item.success", false))
			errorCount++
		} else {
			itemSpan.SetAttributes(attribute.Bool("batch.item.success", true))
			successCount++
		}

		itemSpan.End()
	}

	// Set batch summary
	span.SetAttributes(
		attribute.Int("batch.success_count", successCount),
		attribute.Int("batch.error_count", errorCount),
		attribute.Float64("batch.success_rate", float64(successCount)/float64(len(items))*100),
	)

	if errorCount > 0 {
		return fmt.Errorf("batch processing completed with %d errors out of %d items", errorCount, len(items))
	}

	return nil
}

// TraceCache traces cache operations
func (ti *TracingIntegration) TraceCache(ctx context.Context, operation, key string, cacheFn func(context.Context) (interface{}, error)) (interface{}, error) {
	// Start cache span
	ctx, span := ti.manager.StartSpan(ctx, fmt.Sprintf("cache.%s", operation),
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
		),
	)
	defer span.End()

	// Execute cache operation
	start := time.Now()
	result, err := cacheFn(ctx)
	duration := time.Since(start)

	// Determine cache hit/miss
	cacheHit := err == nil && result != nil
	span.SetAttributes(
		attribute.Bool("cache.hit", cacheHit),
		attribute.Float64("cache.operation.duration_ms", duration.Seconds()*1000),
	)

	if err != nil {
		ti.manager.RecordError(ctx, err)
	}

	// Add cache-specific events
	if cacheHit {
		ti.manager.AddEvent(ctx, "cache_hit", attribute.String("cache.key", key))
	} else {
		ti.manager.AddEvent(ctx, "cache_miss", attribute.String("cache.key", key))
	}

	return result, err
}

// TraceHTTPClient traces outbound HTTP requests
func (ti *TracingIntegration) TraceHTTPClient(ctx context.Context, method, url string, doRequest func(context.Context) (*http.Response, error)) (*http.Response, error) {
	// Start HTTP client span
	ctx, span := ti.manager.StartSpan(ctx, fmt.Sprintf("http.client.%s", method),
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", url),
			attribute.String("http.flavor", "1.1"),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	// Execute request
	start := time.Now()
	resp, err := doRequest(ctx)
	duration := time.Since(start)

	// Record request metrics
	span.SetAttributes(
		attribute.Float64("http.request.duration_ms", duration.Seconds()*1000),
	)

	if err != nil {
		ti.manager.RecordError(ctx, err)
		span.SetAttributes(attribute.Bool("http.request.success", false))
		return nil, err
	}

	// Record response details
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.Bool("http.request.success", resp.StatusCode < 400),
		attribute.Int64("http.response.size", resp.ContentLength),
	)

	return resp, nil
}

// Helper functions

func categorizeToolName(toolName string) string {
	// Categorize tools based on name patterns
	switch {
	case commonUtils.ContainsAny(toolName, []string{"build", "compile", "package"}):
		return "build"
	case commonUtils.ContainsAny(toolName, []string{"test", "validate", "check"}):
		return "validation"
	case commonUtils.ContainsAny(toolName, []string{"deploy", "release", "publish"}):
		return "deployment"
	case commonUtils.ContainsAny(toolName, []string{"monitor", "metric", "log"}):
		return "observability"
	default:
		return "general"
	}
}

// TracingExamples provides example usage patterns
type TracingExamples struct {
	integration *TracingIntegration
}

// ExampleComplexWorkflow shows how to trace a complex multi-tool workflow
func (te *TracingExamples) ExampleComplexWorkflow(ctx context.Context) error {
	workflow := []WorkflowStep{
		{
			Name: "validate_input",
			Type: "validation",
			Handler: func(ctx context.Context) error {
				// Validation logic
				te.integration.manager.AddEvent(ctx, "validating_configuration")
				return nil
			},
			Timeout: 30 * time.Second,
		},
		{
			Name: "build_artifact",
			Type: "build",
			Handler: func(ctx context.Context) error {
				// Build logic with nested tracing
				return te.integration.TraceToolExecution(ctx, "docker_build", func(ctx context.Context) error {
					te.integration.manager.AddEvent(ctx, "building_docker_image")
					return nil
				})
			},
			Timeout: 5 * time.Minute,
		},
		{
			Name: "run_tests",
			Type: "test",
			Handler: func(ctx context.Context) error {
				// Test execution with batch tracing
				tests := []interface{}{"unit", "integration", "e2e"}
				return te.integration.TraceBatch(ctx, "test_suite", tests, func(ctx context.Context, test interface{}) error {
					te.integration.manager.AddEvent(ctx, fmt.Sprintf("running_%s_tests", test))
					return nil
				})
			},
			ContinueOnError: true, // Continue even if tests fail
			Timeout:         10 * time.Minute,
		},
		{
			Name: "deploy",
			Type: "deployment",
			Handler: func(ctx context.Context) error {
				// Deployment with async tracking
				return te.integration.TraceAsyncOperation(ctx, "k8s_deployment", func(ctx context.Context) chan error {
					errChan := make(chan error, 1)
					go func() {
						// Simulate async deployment
						time.Sleep(2 * time.Second)
						te.integration.manager.AddEvent(ctx, "deployment_completed")
						errChan <- nil
					}()
					return errChan
				})
			},
			Timeout: 15 * time.Minute,
		},
	}

	return te.integration.TraceWorkflow(ctx, "ci_cd_pipeline", workflow)
}
