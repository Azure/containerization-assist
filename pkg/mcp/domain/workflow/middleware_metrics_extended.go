// Package workflow provides extended metrics collection middleware
package workflow

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ExtendedMetricsMiddleware provides comprehensive metrics collection for workflows
// This middleware captures detailed metrics about workflow execution, performance,
// errors, and resource usage for monitoring and optimization.
func ExtendedMetricsMiddleware(collector ExtendedMetricsCollector) StepMiddleware {
	return func(next StepHandler) StepHandler {
		return func(ctx context.Context, step Step, state *WorkflowState) error {
			if collector == nil {
				return next(ctx, step, state)
			}

			stepName := step.Name()
			queueStartTime := time.Now()

			// Record step queue time (time waiting to execute)
			if queueTime, ok := ctx.Value("queue_time").(time.Time); ok {
				collector.RecordStepQueueTime(stepName, time.Since(queueTime))
			}

			// Record processing start
			processStartTime := time.Now()
			collector.RecordStepProcessingTime(stepName, time.Since(queueStartTime))

			// Execute the step
			err := next(ctx, step, state)
			duration := time.Since(processStartTime)

			// Record basic step metrics
			if err != nil {
				collector.RecordStepDuration(stepName, duration)
				collector.RecordStepFailure(stepName)

				// Categorize and record error
				category := categorizeError(err)
				collector.RecordErrorByCategory(category, stepName)

				// Check if this is a retry
				if retryCount, ok := ctx.Value("retry_count").(int); ok && retryCount > 0 {
					collector.RecordWorkflowRetry(state.WorkflowID, stepName, retryCount)
				}
			} else {
				collector.RecordStepDuration(stepName, duration)
				collector.RecordStepSuccess(stepName)

				// Record step-specific metrics based on step type
				recordStepSpecificMetrics(collector, stepName, state, duration)
			}

			// Record workflow-level metrics on first and last steps
			if state.CurrentStep == 1 && err == nil {
				collector.RecordWorkflowStart(state.WorkflowID)
			} else if state.CurrentStep == state.TotalSteps || err != nil {
				workflowDuration := time.Since(getWorkflowStartTime(ctx))
				collector.RecordWorkflowEnd(state.WorkflowID, err == nil, workflowDuration)

				// Record business metrics
				if err == nil && state.Args != nil {
					collector.RecordDeploymentSuccess(state.Args.RepoURL, state.Args.Branch)
				} else if err != nil && state.Args != nil {
					collector.RecordDeploymentFailure(state.Args.RepoURL, state.Args.Branch, err.Error())
				}
			}

			return err
		}
	}
}

// WorkflowMetricsMiddleware wraps an entire workflow with metrics collection
func WorkflowMetricsMiddleware(collector WorkflowMetricsCollector) func(WorkflowOrchestrator) WorkflowOrchestrator {
	return func(next WorkflowOrchestrator) WorkflowOrchestrator {
		return &metricsOrchestrator{
			next:      next,
			collector: collector,
		}
	}
}

// metricsOrchestrator wraps a workflow orchestrator with metrics collection
type metricsOrchestrator struct {
	next      WorkflowOrchestrator
	collector WorkflowMetricsCollector
}

// Execute implements WorkflowOrchestrator with metrics collection
func (m *metricsOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
	if m.collector == nil {
		return m.next.Execute(ctx, req, args)
	}

	// Add workflow start time to context
	startTime := time.Now()
	ctx = context.WithValue(ctx, "workflow_start_time", startTime)
	workflowID := GenerateWorkflowID("metrics")
	ctx = context.WithValue(ctx, WorkflowIDKey, workflowID)

	// Record workflow start
	m.collector.RecordWorkflowStart(workflowID)

	// Execute workflow
	result, err := m.next.Execute(ctx, req, args)

	// Record workflow end
	duration := time.Since(startTime)
	m.collector.RecordWorkflowEnd(workflowID, err == nil, duration)

	// Record deployment outcome
	if err == nil {
		m.collector.RecordDeploymentSuccess(args.RepoURL, args.Branch)
	} else {
		m.collector.RecordDeploymentFailure(args.RepoURL, args.Branch, categorizeError(err))
	}

	// Check thresholds and generate alerts
	alerts := m.collector.CheckThresholds()
	for _, alert := range alerts {
		// Log alerts or send to monitoring system
		if state, ok := ctx.Value("workflow_state").(*WorkflowState); ok && state.Logger != nil {
			state.Logger.Warn("Metric threshold exceeded",
				"metric", alert.Metric,
				"current", alert.Current,
				"threshold", alert.Threshold,
				"message", alert.Message)
		}
	}

	return result, err
}

// Helper functions

func categorizeError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Network errors
	if contains(errMsg, "timeout", "connection refused", "no such host", "network unreachable") {
		return "network"
	}

	// Authentication errors
	if contains(errMsg, "unauthorized", "forbidden", "authentication", "permission denied") {
		return "authentication"
	}

	// Resource errors
	if contains(errMsg, "not found", "404", "no such") {
		return "not_found"
	}

	// Build errors
	if contains(errMsg, "build failed", "compilation error", "syntax error") {
		return "build"
	}

	// Registry errors
	if contains(errMsg, "registry", "push failed", "pull failed") {
		return "registry"
	}

	// Kubernetes errors
	if contains(errMsg, "deployment failed", "pod", "service", "ingress") {
		return "kubernetes"
	}

	// Validation errors
	if contains(errMsg, "invalid", "validation", "malformed") {
		return "validation"
	}

	// Rate limiting
	if contains(errMsg, "rate limit", "too many requests", "429") {
		return "rate_limit"
	}

	return "unknown"
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) && containsIgnoreCase(s, substr) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func getWorkflowStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value("workflow_start_time").(time.Time); ok {
		return startTime
	}
	return time.Now()
}

func recordStepSpecificMetrics(collector ExtendedMetricsCollector, stepName string, state *WorkflowState, duration time.Duration) {
	switch stepName {
	case "build":
		if state.BuildResult != nil {
			collector.RecordDockerBuildTime(duration, state.BuildResult.ImageSize)
		}

	case "push":
		if state.BuildResult != nil {
			// Extract registry URL from image ref
			registryURL := extractRegistryURL(state.BuildResult.ImageRef)
			collector.RecordImagePushTime(duration, registryURL)
		}

	case "deploy":
		if state.K8sResult != nil {
			collector.RecordKubernetesDeployTime(duration, state.K8sResult.Namespace)
		}

	case "scan":
		if state.ScanReport != nil {
			// Extract vulnerability counts from scan report
			critical, high, medium, low := extractVulnerabilityCounts(state.ScanReport)
			collector.RecordScanVulnerabilities(critical, high, medium, low)
		}
	}
}

func extractRegistryURL(imageRef string) string {
	// Extract registry URL from image reference
	// e.g., "registry.io/namespace/image:tag" -> "registry.io"
	if imageRef == "" {
		return "unknown"
	}

	// Find first slash
	for i, c := range imageRef {
		if c == '/' {
			return imageRef[:i]
		}
	}

	return "docker.io" // Default registry
}

func extractVulnerabilityCounts(scanReport map[string]interface{}) (critical, high, medium, low int) {
	// Extract vulnerability counts from scan report
	if vulns, ok := scanReport["vulnerabilities"].(map[string]interface{}); ok {
		if c, ok := vulns["critical"].(int); ok {
			critical = c
		}
		if h, ok := vulns["high"].(int); ok {
			high = h
		}
		if m, ok := vulns["medium"].(int); ok {
			medium = m
		}
		if l, ok := vulns["low"].(int); ok {
			low = l
		}
	}

	// Alternative format
	if summary, ok := scanReport["summary"].(map[string]interface{}); ok {
		if c, ok := summary["CRITICAL"].(float64); ok {
			critical = int(c)
		}
		if h, ok := summary["HIGH"].(float64); ok {
			high = int(h)
		}
		if m, ok := summary["MEDIUM"].(float64); ok {
			medium = int(m)
		}
		if l, ok := summary["LOW"].(float64); ok {
			low = int(l)
		}
	}

	return
}

// MetricsContext adds metrics-related values to context
func MetricsContext(ctx context.Context, workflowID string) context.Context {
	ctx = context.WithValue(ctx, WorkflowIDKey, workflowID)
	ctx = context.WithValue(ctx, "workflow_start_time", time.Now())
	return ctx
}

// WithRetryCount adds retry count to context for metrics tracking
func WithRetryCount(ctx context.Context, count int) context.Context {
	return context.WithValue(ctx, "retry_count", count)
}

// WithQueueTime adds queue time to context for metrics tracking
func WithQueueTime(ctx context.Context, queueTime time.Time) context.Context {
	return context.WithValue(ctx, "queue_time", queueTime)
}
