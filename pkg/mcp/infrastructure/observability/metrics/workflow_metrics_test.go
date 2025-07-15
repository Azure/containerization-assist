// Package metrics provides tests for workflow metrics collection
package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

func TestWorkflowMetricsCollector_BasicMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record workflow start
	collector.RecordWorkflowStart("workflow-1")
	assert.Equal(t, 1, collector.GetCurrentWorkflowCount())

	// Record step success
	collector.RecordStepDuration("analyze", 100*time.Millisecond)
	collector.RecordStepSuccess("analyze")

	// Record step failure
	collector.RecordStepFailure("build")
	collector.RecordErrorByCategory("build", "build")

	// Record workflow end
	collector.RecordWorkflowEnd("workflow-1", false, 5*time.Second)
	assert.Equal(t, 0, collector.GetCurrentWorkflowCount())

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, int64(1), snapshot.TotalWorkflows)
	assert.Equal(t, int64(0), snapshot.SuccessfulWorkflows)
	assert.Equal(t, int64(1), snapshot.FailedWorkflows)
	assert.Equal(t, int64(1), snapshot.ErrorsByCategory["build"])
}

func TestWorkflowMetricsCollector_DockerMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record Docker build metrics
	collector.RecordDockerBuildTime(45*time.Second, 100*1024*1024) // 100MB
	collector.RecordDockerBuildTime(60*time.Second, 150*1024*1024) // 150MB

	// Record image push metrics
	collector.RecordImagePushTime(30*time.Second, "docker.io")
	collector.RecordImagePushTime(45*time.Second, "gcr.io")

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, 52*time.Second+500*time.Millisecond, snapshot.AvgDockerBuildTime)
	assert.Equal(t, int64(125*1024*1024), snapshot.AvgImageSize)
}

func TestWorkflowMetricsCollector_KubernetesMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record deployment metrics
	collector.RecordKubernetesDeployTime(2*time.Minute, "default")
	collector.RecordKubernetesDeployTime(3*time.Minute, "production")

	// Record deployment outcomes
	collector.RecordDeploymentSuccess("repo1", "main")
	collector.RecordDeploymentSuccess("repo2", "main")
	collector.RecordDeploymentFailure("repo3", "main", "timeout")

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, 2*time.Minute+30*time.Second, snapshot.AvgDeployTime)
	assert.InDelta(t, 0.666, snapshot.DeploymentSuccessRate, 0.01)
	assert.Equal(t, int64(1), snapshot.TopFailureReasons["timeout"])
}

func TestWorkflowMetricsCollector_SecurityMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record vulnerability scan results
	collector.RecordScanVulnerabilities(2, 5, 10, 20)
	collector.RecordScanVulnerabilities(1, 3, 8, 15)

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	require.NotNil(t, snapshot.SecurityMetrics)
	assert.Equal(t, int64(2), snapshot.SecurityMetrics.TotalScans)
	assert.Equal(t, int64(3), snapshot.SecurityMetrics.CriticalVulns)
	assert.Equal(t, int64(8), snapshot.SecurityMetrics.HighVulns)
	assert.Equal(t, int64(18), snapshot.SecurityMetrics.MediumVulns)
	assert.Equal(t, int64(35), snapshot.SecurityMetrics.LowVulns)
}

func TestWorkflowMetricsCollector_CacheMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record cache operations
	collector.RecordCacheHit("dockerfile")
	collector.RecordCacheHit("dockerfile")
	collector.RecordCacheMiss("dockerfile")
	collector.RecordCacheHit("manifest")
	collector.RecordCacheMiss("manifest")
	collector.RecordCacheMiss("manifest")

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.InDelta(t, 0.5, snapshot.CacheHitRate, 0.01) // 3 hits / 6 total
}

func TestWorkflowMetricsCollector_AIMLMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record LLM token usage
	collector.RecordLLMTokenUsage(500, 1000)
	collector.RecordLLMTokenUsage(300, 800)

	// Record pattern recognition accuracy
	collector.RecordPatternRecognitionAccuracy("network", "network", true)
	collector.RecordPatternRecognitionAccuracy("build", "network", false)
	collector.RecordPatternRecognitionAccuracy("registry", "registry", true)

	// Record step enhancement impact
	collector.RecordStepEnhancementImpact("build", 15.5)
	collector.RecordStepEnhancementImpact("deploy", 20.0)

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, int64(2600), snapshot.TotalTokensUsed)
	assert.InDelta(t, 0.666, snapshot.PatternAccuracy, 0.01)
	assert.InDelta(t, 17.75, snapshot.AvgEnhancementImpact, 0.01)
}

func TestWorkflowMetricsCollector_ErrorTracking(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record various errors
	collector.RecordErrorByCategory("network", "analyze")
	collector.RecordErrorByCategory("build", "build")
	collector.RecordErrorByCategory("network", "push")
	collector.RecordErrorByCategory("kubernetes", "deploy")

	// Get recent errors
	recentErrors := collector.GetRecentErrors(3)
	assert.Len(t, recentErrors, 3)
	assert.Equal(t, "kubernetes", recentErrors[0].Category)
	assert.Equal(t, "deploy", recentErrors[0].StepName)
}

func TestWorkflowMetricsCollector_RetryMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record retries
	collector.RecordWorkflowRetry("workflow-1", "build", 1)
	collector.RecordWorkflowRetry("workflow-1", "build", 2)
	collector.RecordWorkflowRetry("workflow-2", "push", 1)

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, int64(3), snapshot.TotalRetries)
}

func TestWorkflowMetricsCollector_HealthStatus(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Simulate healthy workflow
	collector.RecordWorkflowStart("workflow-1")
	collector.RecordWorkflowEnd("workflow-1", true, 1*time.Minute)
	collector.RecordWorkflowStart("workflow-2")
	collector.RecordWorkflowEnd("workflow-2", true, 2*time.Minute)

	health := collector.GetHealthStatus()
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "healthy", health.WorkflowHealth.Status)
	assert.Equal(t, 0.0, health.WorkflowHealth.ErrorRate)

	// Add more successful workflows to test degraded state
	for i := 0; i < 6; i++ {
		collector.RecordWorkflowStart("workflow-success")
		collector.RecordWorkflowEnd("workflow-success", true, 1*time.Minute)
	}

	// Add two failures - now we have 8 success, 2 failures = 0.2 error rate
	collector.RecordWorkflowStart("workflow-3")
	collector.RecordWorkflowEnd("workflow-3", false, 3*time.Minute)
	collector.RecordWorkflowStart("workflow-4")
	collector.RecordWorkflowEnd("workflow-4", false, 2*time.Minute)

	health = collector.GetHealthStatus()
	// Debug: print actual values
	snapshot := collector.GetMetricsSnapshot()
	t.Logf("Total workflows: %d, Failed: %d, Success: %d",
		snapshot.TotalWorkflows, snapshot.FailedWorkflows, snapshot.SuccessfulWorkflows)
	t.Logf("Error rate: %f", health.WorkflowHealth.ErrorRate)

	// With 2/10 error rate (0.2), this is > 0.1 so it's degraded
	assert.Equal(t, "degraded", health.Status)
	assert.InDelta(t, 0.2, health.WorkflowHealth.ErrorRate, 0.01)

	// Simulate unhealthy state
	for i := 0; i < 6; i++ {
		collector.RecordWorkflowStart("workflow-fail")
		collector.RecordWorkflowEnd("workflow-fail", false, 1*time.Minute)
	}

	health = collector.GetHealthStatus()
	assert.Equal(t, "unhealthy", health.Status)
	assert.Equal(t, "unhealthy", health.WorkflowHealth.Status)
}

func TestWorkflowMetricsCollector_Thresholds(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Set thresholds
	collector.SetThreshold("error_rate", 0.2)
	collector.SetThreshold("avg_duration", 120.0) // 2 minutes

	// Record metrics that exceed thresholds
	collector.RecordWorkflowStart("workflow-1")
	collector.RecordWorkflowEnd("workflow-1", false, 3*time.Minute)
	collector.RecordWorkflowStart("workflow-2")
	collector.RecordWorkflowEnd("workflow-2", true, 4*time.Minute)

	// Check thresholds
	alerts := collector.CheckThresholds()
	assert.Len(t, alerts, 2)

	// Verify error rate alert
	errorAlert := alerts[0]
	assert.Equal(t, "error_rate", errorAlert.Metric)
	assert.Equal(t, 0.2, errorAlert.Threshold)
	assert.InDelta(t, 0.5, errorAlert.Current, 0.01)
	assert.Equal(t, "critical", errorAlert.Severity)

	// Verify duration alert
	durationAlert := alerts[1]
	assert.Equal(t, "avg_duration", durationAlert.Metric)
	assert.Equal(t, 120.0, durationAlert.Threshold)
	assert.InDelta(t, 210.0, durationAlert.Current, 0.01)
	assert.Equal(t, "warning", durationAlert.Severity)
}

func TestWorkflowMetricsCollector_StepMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record step executions
	steps := []struct {
		name     string
		duration time.Duration
		success  bool
	}{
		{"analyze", 5 * time.Second, true},
		{"analyze", 3 * time.Second, true},
		{"analyze", 4 * time.Second, true},
		{"build", 30 * time.Second, true},
		{"build", 45 * time.Second, false},
		{"build", 60 * time.Second, true},
	}

	for _, step := range steps {
		if step.success {
			collector.RecordStepDuration(step.name, step.duration)
			collector.RecordStepSuccess(step.name)
		} else {
			collector.RecordStepFailure(step.name)
		}
	}

	// Get metrics snapshot
	snapshot := collector.GetMetricsSnapshot()

	// Verify analyze metrics
	analyzeMetrics := snapshot.StepMetrics["analyze"]
	require.NotNil(t, analyzeMetrics)
	assert.Equal(t, int64(3), analyzeMetrics.ExecutionCount)
	assert.Equal(t, int64(3), analyzeMetrics.SuccessCount)
	assert.Equal(t, int64(0), analyzeMetrics.FailureCount)
	assert.Equal(t, 4*time.Second, analyzeMetrics.AvgDuration)
	assert.Equal(t, 3*time.Second, analyzeMetrics.MinDuration)
	assert.Equal(t, 5*time.Second, analyzeMetrics.MaxDuration)

	// Verify build metrics
	buildMetrics := snapshot.StepMetrics["build"]
	require.NotNil(t, buildMetrics)
	assert.Equal(t, int64(3), buildMetrics.ExecutionCount)
	assert.Equal(t, int64(2), buildMetrics.SuccessCount)
	assert.Equal(t, int64(1), buildMetrics.FailureCount)
	assert.Equal(t, 45*time.Second, buildMetrics.AvgDuration)
}

func TestWorkflowMetricsCollector_AdaptationMetrics(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Record adaptation applications
	collector.RecordAdaptationApplied(workflow.AdaptationRetryStrategy, true)
	collector.RecordAdaptationApplied(workflow.AdaptationRetryStrategy, true)
	collector.RecordAdaptationApplied(workflow.AdaptationTimeout, true)
	collector.RecordAdaptationApplied(workflow.AdaptationTimeout, false)
	collector.RecordAdaptationApplied(workflow.AdaptationResourceAllocation, true)

	// Verify metrics were recorded (would need to check Prometheus metrics in real test)
	// For now, just verify the method doesn't panic
	assert.NotPanics(t, func() {
		collector.RecordAdaptationApplied(workflow.AdaptationParameterTuning, false)
	})
}

func TestExtendedMetricsMiddleware(t *testing.T) {
	collector := NewWorkflowMetricsCollector("test_" + t.Name())

	// Create a test step
	var executedStep workflow.Step
	handler := workflow.StepHandler(func(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) error {
		executedStep = step
		return nil
	})

	// Apply middleware
	middleware := workflow.ExtendedMetricsMiddleware(collector)
	wrappedHandler := middleware(handler)

	// Create test state
	state := &workflow.WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 1,
		TotalSteps:  5,
		Args: &workflow.ContainerizeAndDeployArgs{
			RepoURL: "https://github.com/test/repo",
			Branch:  "main",
		},
	}

	// Create test step
	testStep := &mockStep{name: "test-step"}

	// Execute with metrics
	ctx := context.Background()
	ctx = context.WithValue(ctx, "workflow_start_time", time.Now())

	err := wrappedHandler(ctx, testStep, state)

	// Verify execution
	assert.NoError(t, err)
	assert.Equal(t, testStep, executedStep)

	// Verify metrics were recorded
	snapshot := collector.GetMetricsSnapshot()
	stepMetrics := snapshot.StepMetrics["test-step"]
	require.NotNil(t, stepMetrics)
	assert.Equal(t, int64(1), stepMetrics.ExecutionCount)
	assert.Equal(t, int64(1), stepMetrics.SuccessCount)
}

func TestExtendedMetricsMiddleware_WithError(t *testing.T) {
	// Use a completely separate collector to avoid interference
	collector := NewWorkflowMetricsCollector("error_test_" + t.Name())

	// Create a failing handler
	testError := errors.New("network timeout occurred")
	handler := workflow.StepHandler(func(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) error {
		return testError
	})

	// Apply middleware
	middleware := workflow.ExtendedMetricsMiddleware(collector)
	wrappedHandler := middleware(handler)

	// Create test state
	state := &workflow.WorkflowState{
		WorkflowID:  "test-workflow",
		CurrentStep: 5,
		TotalSteps:  5,
		Args: &workflow.ContainerizeAndDeployArgs{
			RepoURL: "https://github.com/test/repo",
			Branch:  "main",
		},
	}

	// Create test step
	testStep := &mockStep{name: "deploy"}

	// Execute with metrics
	ctx := context.Background()
	ctx = context.WithValue(ctx, "workflow_start_time", time.Now())
	ctx = context.WithValue(ctx, "retry_count", 2)

	err := wrappedHandler(ctx, testStep, state)

	// Verify error
	assert.Error(t, err)
	assert.Equal(t, testError, err)

	// Verify metrics were recorded
	snapshot := collector.GetMetricsSnapshot()
	assert.Equal(t, int64(1), snapshot.ErrorsByCategory["network"])
	assert.Equal(t, int64(1), snapshot.TotalRetries)    // One retry recorded for this step
	assert.Equal(t, int64(1), snapshot.FailedWorkflows) // One workflow failed
	assert.Equal(t, int64(1), snapshot.TopFailureReasons["network timeout occurred"])

	// Verify step failure was recorded
	stepMetrics := snapshot.StepMetrics["deploy"]
	require.NotNil(t, stepMetrics)
	assert.Equal(t, int64(1), stepMetrics.ExecutionCount)
	assert.Equal(t, int64(0), stepMetrics.SuccessCount)
	assert.Equal(t, int64(1), stepMetrics.FailureCount)
}

// mockStep is a simple step implementation for testing
type mockStep struct {
	name string
}

func (s *mockStep) Name() string { return s.name }
func (s *mockStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	return nil
}
func (s *mockStep) MaxRetries() int { return 3 }
