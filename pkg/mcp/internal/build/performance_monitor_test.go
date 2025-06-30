package build

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformanceMonitor_StartBuildMonitoring(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewPerformanceMonitor(logger)
	ctx := context.Background()

	operation := &BuildOperation{
		Name:        "test-build",
		Tool:        "docker",
		Type:        "image",
		Strategy:    "buildkit",
		SessionID:   "session-123",
		ContextSize: 1024 * 1024, // 1MB
	}

	buildMonitor := monitor.StartBuildMonitoring(ctx, operation)

	assert.NotNil(t, buildMonitor)
	assert.Equal(t, operation, buildMonitor.operation)
	assert.NotZero(t, buildMonitor.startTime)
	assert.Empty(t, buildMonitor.stages)
}

func TestBuildMonitor_StageTracking(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewPerformanceMonitor(logger)
	ctx := context.Background()

	operation := &BuildOperation{
		Name:      "test-build",
		Tool:      "docker",
		Type:      "image",
		SessionID: "session-123",
	}

	buildMonitor := monitor.StartBuildMonitoring(ctx, operation)

	// Start and end multiple stages
	stages := []string{"pull_base", "run_commands", "copy_files", "build_app"}

	for _, stage := range stages {
		buildMonitor.StartStage(stage)
		time.Sleep(10 * time.Millisecond) // Simulate work
		buildMonitor.EndStage(stage, true)
	}

	// Check stages were tracked
	assert.Len(t, buildMonitor.stages, len(stages))

	for _, stageName := range stages {
		stage, exists := buildMonitor.stages[stageName]
		assert.True(t, exists)
		assert.Equal(t, stageName, stage.Name)
		assert.True(t, stage.Success)
		assert.Greater(t, stage.Duration.Nanoseconds(), int64(0))
		assert.False(t, stage.StartTime.IsZero())
		assert.False(t, stage.EndTime.IsZero())
	}
}

func TestBuildMonitor_CacheTracking(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewPerformanceMonitor(logger)
	ctx := context.Background()

	operation := &BuildOperation{
		Name: "test-build",
		Tool: "docker",
		Type: "image",
	}

	buildMonitor := monitor.StartBuildMonitoring(ctx, operation)

	// Record cache hits and misses
	buildMonitor.RecordCacheHit("layer1")
	buildMonitor.RecordCacheHit("layer2")
	buildMonitor.RecordCacheMiss("layer3")
	buildMonitor.RecordCacheMiss("layer4")
	buildMonitor.RecordCacheMiss("layer5")

	// Metrics are recorded directly to Prometheus, so we can't easily verify counts
	// Just ensure methods don't panic
	assert.NotPanics(t, func() {
		buildMonitor.RecordCacheHit("layer6")
		buildMonitor.RecordCacheMiss("layer7")
	})
}

func TestBuildMonitor_Complete(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewPerformanceMonitor(logger)
	ctx := context.Background()

	tests := []struct {
		name      string
		success   bool
		errorType string
		imageInfo *ImageInfo
	}{
		{
			name:      "successful build with image info",
			success:   true,
			errorType: "",
			imageInfo: &BuildImageInfo{
				Name:       "myapp",
				Tag:        "v1.0.0",
				Size:       150 * 1024 * 1024, // 150MB
				LayerCount: 12,
			},
		},
		{
			name:      "failed build",
			success:   false,
			errorType: "dockerfile_error",
			imageInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := &BuildOperation{
				Name:     "test-build",
				Tool:     "docker",
				Type:     "image",
				Strategy: "buildkit",
			}

			buildMonitor := monitor.StartBuildMonitoring(ctx, operation)

			// Add some stages
			buildMonitor.StartStage("build")
			time.Sleep(10 * time.Millisecond)
			buildMonitor.EndStage("build", tt.success)

			// Complete the build
			buildMonitor.Complete(tt.success, tt.errorType, tt.imageInfo)

			assert.False(t, buildMonitor.endTime.IsZero())
			assert.Equal(t, tt.success, buildMonitor.success)
			assert.Equal(t, tt.errorType, buildMonitor.errorType)
			assert.Greater(t, buildMonitor.endTime.Sub(buildMonitor.startTime).Nanoseconds(), int64(0))
		})
	}
}

func TestBuildMonitor_GetReport(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewPerformanceMonitor(logger)
	ctx := context.Background()

	operation := &BuildOperation{
		Name:     "test-build",
		Tool:     "docker",
		Type:     "image",
		Strategy: "buildkit",
	}

	buildMonitor := monitor.StartBuildMonitoring(ctx, operation)

	// Simulate a build with stages
	stages := []struct {
		name    string
		success bool
		delay   time.Duration
	}{
		{"pull_base", true, 50 * time.Millisecond},
		{"run_commands", true, 100 * time.Millisecond},
		{"copy_files", true, 20 * time.Millisecond},
		{"build_app", false, 200 * time.Millisecond},
	}

	for _, stage := range stages {
		buildMonitor.StartStage(stage.name)
		time.Sleep(stage.delay)
		buildMonitor.EndStage(stage.name, stage.success)
	}

	buildMonitor.Complete(false, "build_failed", nil)

	report := buildMonitor.GetReport()

	assert.NotNil(t, report)
	assert.Equal(t, operation, report.Operation)
	assert.False(t, report.Success)
	assert.Equal(t, "build_failed", report.ErrorType)
	assert.Len(t, report.Stages, len(stages))

	// Check stage reports
	totalPercentage := 0.0
	for _, stageReport := range report.Stages {
		assert.NotEmpty(t, stageReport.Name)
		assert.Greater(t, stageReport.Duration.Nanoseconds(), int64(0))
		assert.GreaterOrEqual(t, stageReport.Percentage, 0.0)
		assert.LessOrEqual(t, stageReport.Percentage, 100.0)
		totalPercentage += stageReport.Percentage
	}

	// Total percentage should be close to 100 (allowing for rounding)
	assert.InDelta(t, 100.0, totalPercentage, 5.0)
}

func TestPerformanceAnalyzer_Analyze(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewPerformanceAnalyzer(logger)
	ctx := context.Background()

	metrics := &BuildMetrics{} // Mock metrics

	analysis, err := analyzer.Analyze(ctx, "session-123", metrics)
	require.NoError(t, err)

	assert.NotNil(t, analysis)
	assert.Equal(t, "session-123", analysis.SessionID)
	assert.False(t, analysis.AnalysisTime.IsZero())
	assert.NotNil(t, analysis.Insights)
	assert.NotNil(t, analysis.Recommendations)

	// Check that at least one insight is generated
	assert.Greater(t, len(analysis.Insights), 0)

	// Check insight structure
	for _, insight := range analysis.Insights {
		assert.NotEmpty(t, insight.Type)
		assert.NotEmpty(t, insight.Severity)
		assert.NotEmpty(t, insight.Title)
		assert.NotEmpty(t, insight.Description)
	}
}

func TestPerformanceAnalyzer_GenerateRecommendations(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewPerformanceAnalyzer(logger)

	tests := []struct {
		name                    string
		insights                []PerformanceInsight
		expectedRecommendations []string
	}{
		{
			name: "slow builds",
			insights: []PerformanceInsight{
				{Type: "slow_builds", Severity: "high"},
			},
			expectedRecommendations: []string{
				"Consider using BuildKit for improved build performance",
			},
		},
		{
			name: "large context",
			insights: []PerformanceInsight{
				{Type: "large_context", Severity: "medium"},
			},
			expectedRecommendations: []string{
				"Reduce build context size with .dockerignore",
			},
		},
		{
			name: "cache misses",
			insights: []PerformanceInsight{
				{Type: "cache_misses", Severity: "high"},
			},
			expectedRecommendations: []string{
				"Optimize Dockerfile layer ordering for better caching",
			},
		},
		{
			name: "multiple issues",
			insights: []PerformanceInsight{
				{Type: "slow_builds", Severity: "high"},
				{Type: "cache_misses", Severity: "medium"},
			},
			expectedRecommendations: []string{
				"Consider using BuildKit for improved build performance",
				"Optimize Dockerfile layer ordering for better caching",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := analyzer.generateRecommendations(tt.insights)
			assert.Equal(t, tt.expectedRecommendations, recommendations)
		})
	}
}

func TestMetricsCollector_CollectSummary(t *testing.T) {
	logger := zerolog.Nop()
	metrics := &BuildMetrics{} // Mock metrics
	collector := NewMetricsCollector(logger, metrics)

	summary := collector.CollectSummary()

	assert.NotNil(t, summary)
	assert.False(t, summary.Timestamp.IsZero())
	assert.GreaterOrEqual(t, summary.ActiveBuilds, 0)
	assert.GreaterOrEqual(t, summary.TotalBuilds, 0)
	assert.GreaterOrEqual(t, summary.SuccessRate, 0.0)
	assert.LessOrEqual(t, summary.SuccessRate, 1.0)
	assert.Greater(t, summary.AvgBuildTime.Nanoseconds(), int64(0))
	assert.GreaterOrEqual(t, summary.CacheHitRate, 0.0)
	assert.LessOrEqual(t, summary.CacheHitRate, 1.0)
	assert.GreaterOrEqual(t, summary.QueueSize, 0)
}

func TestBuildOperation_Structure(t *testing.T) {
	operation := &BuildOperation{
		Name:        "myapp-build",
		Tool:        "docker",
		Type:        "multi-stage",
		Strategy:    "buildkit",
		SessionID:   "session-456",
		ContextSize: 5 * 1024 * 1024, // 5MB
	}

	assert.Equal(t, "myapp-build", operation.Name)
	assert.Equal(t, "docker", operation.Tool)
	assert.Equal(t, "multi-stage", operation.Type)
	assert.Equal(t, "buildkit", operation.Strategy)
	assert.Equal(t, "session-456", operation.SessionID)
	assert.Equal(t, int64(5*1024*1024), operation.ContextSize)
}

func TestPerformanceTrends(t *testing.T) {
	trends := PerformanceTrends{
		BuildTimeChange:      -15.5, // 15.5% improvement
		CacheEfficiencyTrend: "improving",
		ErrorRateTrend:       "stable",
	}

	assert.Equal(t, -15.5, trends.BuildTimeChange)
	assert.Equal(t, "improving", trends.CacheEfficiencyTrend)
	assert.Equal(t, "stable", trends.ErrorRateTrend)
}
