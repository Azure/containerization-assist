package build

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test NewAIContextEnhancer constructor
func TestNewAIContextEnhancer(t *testing.T) {
	logger := zerolog.Nop()
	contextSharer := &DefaultContextSharer{}
	enhancer := NewAIContextEnhancer(contextSharer, logger)
	if enhancer == nil {
		t.Error("NewAIContextEnhancer should not return nil")
	}
	if enhancer.contextSharer != contextSharer {
		t.Error("contextSharer should be set correctly")
	}
}

// Test AIToolContext type
func TestAIToolContext(t *testing.T) {
	startTime := time.Now()
	duration := time.Second * 5
	context := AIToolContext{
		ToolName:      "test_tool",
		OperationType: "build",
		StartTime:     startTime,
		Duration:      duration,
		Success:       true,
	}
	if context.ToolName != "test_tool" {
		t.Errorf("Expected ToolName to be 'test_tool', got '%s'", context.ToolName)
	}
	if context.OperationType != "build" {
		t.Errorf("Expected OperationType to be 'build', got '%s'", context.OperationType)
	}
	if context.StartTime != startTime {
		t.Error("StartTime should match")
	}
	if context.Duration != duration {
		t.Error("Duration should match")
	}
	if !context.Success {
		t.Error("Success should be true")
	}
}

// Test InputAnalysis type
func TestInputAnalysis(t *testing.T) {
	analysis := InputAnalysis{
		InputComplexity:   "medium",
		KeyParameters:     []string{"source_image", "target_image"},
		ValidationResults: []AIValidationResult{},
		PatternMatches:    []AIPatternMatch{},
		RiskFactors:       []string{"privilege_escalation"},
		InputMetadata:     map[string]interface{}{"size": 1024},
	}
	if analysis.InputComplexity != "medium" {
		t.Errorf("Expected InputComplexity to be 'medium', got '%s'", analysis.InputComplexity)
	}
	if len(analysis.KeyParameters) != 2 {
		t.Errorf("Expected 2 key parameters, got %d", len(analysis.KeyParameters))
	}
	if analysis.KeyParameters[0] != "source_image" {
		t.Errorf("Expected first parameter to be 'source_image', got '%s'", analysis.KeyParameters[0])
	}
	if analysis.ValidationResults == nil {
		t.Error("ValidationResults should not be nil")
	}
	if analysis.PatternMatches == nil {
		t.Error("PatternMatches should not be nil")
	}
	if len(analysis.RiskFactors) != 1 {
		t.Errorf("Expected 1 risk factor, got %d", len(analysis.RiskFactors))
	}
	if analysis.InputMetadata == nil {
		t.Error("InputMetadata should not be nil")
	}
}

// Test OutputAnalysis type
func TestOutputAnalysis(t *testing.T) {
	analysis := OutputAnalysis{
		OutputQuality:    "excellent",
		CompletionStatus: "complete",
		KeyArtifacts:     []string{"image.tar", "manifest.json"},
		QualityMetrics:   []AIQualityMetric{},
		ImprovementAreas: []string{"optimization"},
		OutputMetadata:   map[string]interface{}{"size": 2048},
	}
	if analysis.OutputQuality != "excellent" {
		t.Errorf("Expected OutputQuality to be 'excellent', got '%s'", analysis.OutputQuality)
	}
	if analysis.CompletionStatus != "complete" {
		t.Errorf("Expected CompletionStatus to be 'complete', got '%s'", analysis.CompletionStatus)
	}
	if len(analysis.KeyArtifacts) != 2 {
		t.Errorf("Expected 2 key artifacts, got %d", len(analysis.KeyArtifacts))
	}
	if analysis.KeyArtifacts[0] != "image.tar" {
		t.Errorf("Expected first artifact to be 'image.tar', got '%s'", analysis.KeyArtifacts[0])
	}
	if analysis.QualityMetrics == nil {
		t.Error("QualityMetrics should not be nil")
	}
	if len(analysis.ImprovementAreas) != 1 {
		t.Errorf("Expected 1 improvement area, got %d", len(analysis.ImprovementAreas))
	}
	if analysis.OutputMetadata == nil {
		t.Error("OutputMetadata should not be nil")
	}
}

// Test AIQualityMetric type
func TestAIQualityMetric(t *testing.T) {
	metric := AIQualityMetric{
		Name:  "build_time",
		Value: 45.5,
		Unit:  "seconds",
	}
	if metric.Name != "build_time" {
		t.Errorf("Expected Name to be 'build_time', got '%s'", metric.Name)
	}
	if metric.Value != 45.5 {
		t.Errorf("Expected Value to be 45.5, got %f", metric.Value)
	}
	if metric.Unit != "seconds" {
		t.Errorf("Expected Unit to be 'seconds', got '%s'", metric.Unit)
	}
}

// Test AISimilarError type
func TestAISimilarError(t *testing.T) {
	timestamp := time.Now()
	error := AISimilarError{
		ErrorMessage: "Build failed",
		Resolution:   "Retry with clean cache",
		Success:      true,
		Timestamp:    timestamp,
	}
	if error.ErrorMessage != "Build failed" {
		t.Errorf("Expected ErrorMessage to be 'Build failed', got '%s'", error.ErrorMessage)
	}
	if error.Resolution != "Retry with clean cache" {
		t.Errorf("Expected Resolution to be 'Retry with clean cache', got '%s'", error.Resolution)
	}
	if !error.Success {
		t.Error("Success should be true")
	}
	if error.Timestamp != timestamp {
		t.Error("Timestamp should match")
	}
}

// Test AIPerformanceIssue type
func TestAIPerformanceIssue(t *testing.T) {
	issue := AIPerformanceIssue{
		Type:        "memory",
		Severity:    "high",
		Description: "High memory usage detected",
		Suggestion:  "Optimize memory allocation",
	}
	if issue.Type != "memory" {
		t.Errorf("Expected Type to be 'memory', got '%s'", issue.Type)
	}
	if issue.Severity != "high" {
		t.Errorf("Expected Severity to be 'high', got '%s'", issue.Severity)
	}
	if issue.Description != "High memory usage detected" {
		t.Errorf("Expected Description to be 'High memory usage detected', got '%s'", issue.Description)
	}
	if issue.Suggestion != "Optimize memory allocation" {
		t.Errorf("Expected Suggestion to be 'Optimize memory allocation', got '%s'", issue.Suggestion)
	}
}

// Test AIFileChange type
func TestAIFileChange(t *testing.T) {
	change := AIFileChange{
		Path:      "/path/to/file.txt",
		Operation: "create",
		Size:      1024,
	}
	if change.Path != "/path/to/file.txt" {
		t.Errorf("Expected Path to be '/path/to/file.txt', got '%s'", change.Path)
	}
	if change.Operation != "create" {
		t.Errorf("Expected Operation to be 'create', got '%s'", change.Operation)
	}
	if change.Size != 1024 {
		t.Errorf("Expected Size to be 1024, got %d", change.Size)
	}
}

// Test AIEnvChange type
func TestAIEnvChange(t *testing.T) {
	change := AIEnvChange{
		Variable: "PATH",
		OldValue: "/usr/bin",
		NewValue: "/usr/bin:/usr/local/bin",
	}
	if change.Variable != "PATH" {
		t.Errorf("Expected Variable to be 'PATH', got '%s'", change.Variable)
	}
	if change.OldValue != "/usr/bin" {
		t.Errorf("Expected OldValue to be '/usr/bin', got '%s'", change.OldValue)
	}
	if change.NewValue != "/usr/bin:/usr/local/bin" {
		t.Errorf("Expected NewValue to be '/usr/bin:/usr/local/bin', got '%s'", change.NewValue)
	}
}

// Test AIActionRecommendation type
func TestAIActionRecommendation(t *testing.T) {
	recommendation := AIActionRecommendation{
		Action:     "optimize_build",
		Confidence: 0.85,
		Reasoning:  "Build time is above threshold",
		Priority:   1,
	}
	if recommendation.Action != "optimize_build" {
		t.Errorf("Expected Action to be 'optimize_build', got '%s'", recommendation.Action)
	}
	if recommendation.Confidence != 0.85 {
		t.Errorf("Expected Confidence to be 0.85, got %f", recommendation.Confidence)
	}
	if recommendation.Reasoning != "Build time is above threshold" {
		t.Errorf("Expected Reasoning to be 'Build time is above threshold', got '%s'", recommendation.Reasoning)
	}
	if recommendation.Priority != 1 {
		t.Errorf("Expected Priority to be 1, got %d", recommendation.Priority)
	}
}

// Test AIOptimizationTip type
func TestAIOptimizationTip(t *testing.T) {
	tip := AIOptimizationTip{
		Area:        "build_performance",
		Suggestion:  "Use multi-stage builds",
		ImpactLevel: "high",
	}
	if tip.Area != "build_performance" {
		t.Errorf("Expected Area to be 'build_performance', got '%s'", tip.Area)
	}
	if tip.Suggestion != "Use multi-stage builds" {
		t.Errorf("Expected Suggestion to be 'Use multi-stage builds', got '%s'", tip.Suggestion)
	}
	if tip.ImpactLevel != "high" {
		t.Errorf("Expected ImpactLevel to be 'high', got '%s'", tip.ImpactLevel)
	}
}

// Test gatherPerformanceMetrics function
func TestGatherPerformanceMetrics(t *testing.T) {
	logger := zerolog.Nop()
	contextSharer := &DefaultContextSharer{}
	enhancer := NewAIContextEnhancer(contextSharer, logger)
	metrics := enhancer.gatherPerformanceMetrics("test-session")
	if metrics == nil {
		t.Error("gatherPerformanceMetrics should not return nil")
	}
	if metrics.CacheHitRatio != 0.85 {
		t.Errorf("Expected CacheHitRatio to be 0.85, got %f", metrics.CacheHitRatio)
	}
	if metrics.ThroughputMBps != 10.5 {
		t.Errorf("Expected ThroughputMBps to be 10.5, got %f", metrics.ThroughputMBps)
	}
	if metrics.OptimizationScore != 0.75 {
		t.Errorf("Expected OptimizationScore to be 0.75, got %f", metrics.OptimizationScore)
	}
	if metrics.Bottlenecks == nil {
		t.Error("Bottlenecks should not be nil")
	}
}

// Test gatherResourceUsage function
func TestGatherResourceUsage(t *testing.T) {
	logger := zerolog.Nop()
	contextSharer := &DefaultContextSharer{}
	enhancer := NewAIContextEnhancer(contextSharer, logger)
	usage := enhancer.gatherResourceUsage()
	if usage == nil {
		t.Error("gatherResourceUsage should not return nil")
	}
	if usage.CPUUsage != 25.5 {
		t.Errorf("Expected CPUUsage to be 25.5, got %f", usage.CPUUsage)
	}
	expectedMemory := int64(1024 * 1024 * 512) // 512MB
	if usage.MemoryUsage != expectedMemory {
		t.Errorf("Expected MemoryUsage to be %d, got %d", expectedMemory, usage.MemoryUsage)
	}
	if usage.FileSystemChanges == nil {
		t.Error("FileSystemChanges should not be nil")
	}
	if usage.EnvironmentChanges == nil {
		t.Error("EnvironmentChanges should not be nil")
	}
}

// Test AIValidationResult type
func TestAIValidationResult(t *testing.T) {
	result := AIValidationResult{
		Field:   "source_image",
		Status:  "valid",
		Message: "Image reference is valid",
	}
	if result.Field != "source_image" {
		t.Errorf("Expected Field to be 'source_image', got '%s'", result.Field)
	}
	if result.Status != "valid" {
		t.Errorf("Expected Status to be 'valid', got '%s'", result.Status)
	}
	if result.Message != "Image reference is valid" {
		t.Errorf("Expected Message to be 'Image reference is valid', got '%s'", result.Message)
	}
}

// Test AIPatternMatch type
func TestAIPatternMatch(t *testing.T) {
	match := AIPatternMatch{
		Pattern:     "dockerfile_best_practice",
		Confidence:  0.95,
		Description: "Multi-stage build pattern detected",
	}
	if match.Pattern != "dockerfile_best_practice" {
		t.Errorf("Expected Pattern to be 'dockerfile_best_practice', got '%s'", match.Pattern)
	}
	if match.Confidence != 0.95 {
		t.Errorf("Expected Confidence to be 0.95, got %f", match.Confidence)
	}
	if match.Description != "Multi-stage build pattern detected" {
		t.Errorf("Expected Description to be 'Multi-stage build pattern detected', got '%s'", match.Description)
	}
}
