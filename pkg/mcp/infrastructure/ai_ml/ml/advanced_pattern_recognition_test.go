// Package ml provides tests for advanced error pattern recognition
package ml

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUnifiedSampler is a mock implementation of UnifiedSampler for testing
type MockUnifiedSampler struct {
	mock.Mock
}

func (m *MockUnifiedSampler) Sample(ctx context.Context, req domainsampling.Request) (domainsampling.Response, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(domainsampling.Response), args.Error(1)
}

func (m *MockUnifiedSampler) Stream(ctx context.Context, req domainsampling.Request) (<-chan domainsampling.StreamChunk, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(<-chan domainsampling.StreamChunk), args.Error(1)
}

func (m *MockUnifiedSampler) AnalyzeDockerfile(ctx context.Context, content string) (*domainsampling.DockerfileAnalysis, error) {
	args := m.Called(ctx, content)
	return args.Get(0).(*domainsampling.DockerfileAnalysis), args.Error(1)
}

func (m *MockUnifiedSampler) AnalyzeKubernetesManifest(ctx context.Context, content string) (*domainsampling.ManifestAnalysis, error) {
	args := m.Called(ctx, content)
	return args.Get(0).(*domainsampling.ManifestAnalysis), args.Error(1)
}

func (m *MockUnifiedSampler) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*domainsampling.SecurityAnalysis, error) {
	args := m.Called(ctx, scanResults)
	return args.Get(0).(*domainsampling.SecurityAnalysis), args.Error(1)
}

func (m *MockUnifiedSampler) FixDockerfile(ctx context.Context, content string, issues []string) (*domainsampling.DockerfileFix, error) {
	args := m.Called(ctx, content, issues)
	return args.Get(0).(*domainsampling.DockerfileFix), args.Error(1)
}

func (m *MockUnifiedSampler) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*domainsampling.ManifestFix, error) {
	args := m.Called(ctx, content, issues)
	return args.Get(0).(*domainsampling.ManifestFix), args.Error(1)
}

func TestAdvancedPatternRecognizer_AnalyzeErrorPatterns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response for AI analysis
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "docker_build_error",
			"confidence": 0.9,
			"category": "build",
			"severity": "high",
			"suggested_fix": "Check Docker daemon status and retry build",
			"auto_fixable": true,
			"retry_recommendation": "after_delay",
			"context": {"daemon_status": "unknown"},
			"patterns": ["build_failure", "docker_daemon"]
		}`,
	}

	mockSampler.On("Sample", mock.Anything, mock.MatchedBy(func(req domainsampling.Request) bool {
		return req.Temperature == 0.1 && req.MaxTokens == 1000
	})).Return(mockResponse, nil)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Test error and context
	testError := errors.New("docker build failed: daemon not responding")
	testContext := WorkflowContext{
		WorkflowID: "test-workflow-123",
		StepName:   "build",
		StepNumber: 3,
		TotalSteps: 10,
		RepoURL:    "https://github.com/test/repo",
		Language:   "go",
		Framework:  "gin",
	}

	// Analyze error patterns
	result, err := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "docker_build_error", result.ErrorType)
	assert.Equal(t, 0.9, result.Confidence)
	assert.Equal(t, CategoryBuild, result.Category)
	assert.Equal(t, SeverityHigh, result.Severity)
	assert.True(t, result.AutoFixable)
	assert.Equal(t, RetryAfterDelay, result.RetryRecommendation)
	assert.Contains(t, result.Patterns, "build_failure")
	assert.Contains(t, result.Patterns, "docker_daemon")
	assert.Greater(t, result.PatternScore, 0.0)

	// Verify mock was called
	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_CacheEfficiency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "network_error",
			"confidence": 0.8,
			"category": "network",
			"severity": "medium",
			"suggested_fix": "Check network connectivity",
			"auto_fixable": false,
			"retry_recommendation": "immediate",
			"context": {},
			"patterns": ["network_timeout"]
		}`,
	}

	// Mock should be called only once due to caching
	mockSampler.On("Sample", mock.Anything, mock.Anything).Return(mockResponse, nil).Once()

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Test error and context
	testError := errors.New("network timeout occurred")
	testContext := WorkflowContext{
		WorkflowID: "test-workflow-456",
		StepName:   "push",
		StepNumber: 6,
		TotalSteps: 10,
	}

	// First call - should hit the AI
	result1, err1 := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)
	assert.NoError(t, err1)
	assert.NotNil(t, result1)

	// Second call - should hit the cache
	result2, err2 := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)

	// Results should be identical
	assert.Equal(t, result1.ErrorType, result2.ErrorType)
	assert.Equal(t, result1.Confidence, result2.Confidence)
	assert.Equal(t, result1.Category, result2.Category)

	// Verify cache hit rate
	stats, err := recognizer.GetPatternStatistics(context.Background())
	assert.NoError(t, err)
	assert.Greater(t, stats.CacheHitRate, 0.0)

	// Verify mock was called only once
	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_LearningEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "dockerfile_error",
			"confidence": 0.7,
			"category": "dockerfile",
			"severity": "medium",
			"suggested_fix": "Fix FROM instruction",
			"auto_fixable": true,
			"retry_recommendation": "with_changes",
			"context": {},
			"patterns": ["dockerfile_syntax"]
		}`,
	}

	mockSampler.On("Sample", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Test multiple similar errors to train the learning engine
	testErrors := []error{
		errors.New("dockerfile syntax error in FROM instruction"),
		errors.New("dockerfile syntax error in RUN instruction"),
		errors.New("dockerfile syntax error in COPY instruction"),
	}

	testContext := WorkflowContext{
		WorkflowID: "test-workflow-789",
		StepName:   "dockerfile",
		StepNumber: 2,
		TotalSteps: 10,
	}

	// Process multiple errors
	for _, err := range testErrors {
		result, analyzeErr := recognizer.AnalyzeErrorPatterns(context.Background(), err, testContext)
		assert.NoError(t, analyzeErr)
		assert.NotNil(t, result)
	}

	// Get pattern statistics
	stats, err := recognizer.GetPatternStatistics(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Greater(t, stats.TotalErrors, 0)

	// Verify learning metrics
	assert.Greater(t, stats.LearningMetrics.TotalPatterns, 0)
	assert.GreaterOrEqual(t, stats.LearningMetrics.AccuracyRate, 0.0)
	assert.LessOrEqual(t, stats.LearningMetrics.AccuracyRate, 1.0)

	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_SimilarityEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "k8s_deployment_error",
			"confidence": 0.85,
			"category": "kubernetes",
			"severity": "high",
			"suggested_fix": "Check cluster connectivity",
			"auto_fixable": false,
			"retry_recommendation": "after_delay",
			"context": {},
			"patterns": ["k8s_cluster", "deployment_failed"]
		}`,
	}

	mockSampler.On("Sample", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// First, analyze an error to populate history
	firstError := errors.New("kubernetes deployment failed: cluster unreachable")
	firstContext := WorkflowContext{
		WorkflowID: "test-workflow-001",
		StepName:   "deploy",
		StepNumber: 9,
		TotalSteps: 10,
	}

	result1, err1 := recognizer.AnalyzeErrorPatterns(context.Background(), firstError, firstContext)
	assert.NoError(t, err1)
	assert.NotNil(t, result1)

	// Now analyze a similar error
	similarError := errors.New("kubernetes deployment failed: cluster connection timeout")
	similarContext := WorkflowContext{
		WorkflowID: "test-workflow-002",
		StepName:   "deploy",
		StepNumber: 9,
		TotalSteps: 10,
	}

	result2, err2 := recognizer.AnalyzeErrorPatterns(context.Background(), similarError, similarContext)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)

	// Verify similar errors are detected
	assert.Greater(t, len(result2.SimilarErrors), 0)

	// Verify similarity scores
	for _, similar := range result2.SimilarErrors {
		assert.Greater(t, similar.Similarity, 0.6) // Should be above threshold
		assert.LessOrEqual(t, similar.Similarity, 1.0)
	}

	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_TrendAnalysis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "build_timeout",
			"confidence": 0.9,
			"category": "build",
			"severity": "medium",
			"suggested_fix": "Increase build timeout",
			"auto_fixable": true,
			"retry_recommendation": "after_delay",
			"context": {},
			"patterns": ["timeout", "build_slow"]
		}`,
	}

	mockSampler.On("Sample", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Simulate multiple occurrences of the same error over time
	testError := errors.New("build timeout exceeded")
	testContext := WorkflowContext{
		WorkflowID: "test-workflow-trend",
		StepName:   "build",
		StepNumber: 3,
		TotalSteps: 10,
	}

	// Process error multiple times
	for i := 0; i < 5; i++ {
		result, analyzeErr := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)
		assert.NoError(t, analyzeErr)
		assert.NotNil(t, result)

		// Simulate time passing
		time.Sleep(10 * time.Millisecond)
	}

	// Get final analysis
	finalResult, err := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)
	assert.NoError(t, err)
	assert.NotNil(t, finalResult)

	// Verify trend analysis
	assert.Greater(t, finalResult.TrendAnalysis.Frequency, 0.0)
	assert.GreaterOrEqual(t, finalResult.TrendAnalysis.SuccessRate, 0.0)
	assert.LessOrEqual(t, finalResult.TrendAnalysis.SuccessRate, 1.0)

	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_Recommendations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Setup mock response
	mockResponse := domainsampling.Response{
		Content: `{
			"error_type": "registry_auth_error",
			"confidence": 0.95,
			"category": "registry",
			"severity": "high",
			"suggested_fix": "Check registry credentials",
			"auto_fixable": false,
			"retry_recommendation": "never",
			"context": {},
			"patterns": ["auth_failed", "registry_denied"]
		}`,
	}

	mockSampler.On("Sample", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Test error
	testError := errors.New("registry authentication failed")
	testContext := WorkflowContext{
		WorkflowID: "test-workflow-rec",
		StepName:   "push",
		StepNumber: 6,
		TotalSteps: 10,
	}

	// Analyze error
	result, err := recognizer.AnalyzeErrorPatterns(context.Background(), testError, testContext)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify recommendations are generated
	assert.Greater(t, len(result.RecommendedActions), 0)

	// Verify recommendation structure
	for _, action := range result.RecommendedActions {
		assert.NotEmpty(t, action.Action)
		assert.NotEmpty(t, action.Description)
		assert.Contains(t, []string{"high", "medium", "low"}, action.Priority)
		assert.Greater(t, action.Confidence, 0.0)
		assert.LessOrEqual(t, action.Confidence, 1.0)
	}

	// Verify learning insights
	assert.GreaterOrEqual(t, len(result.LearningInsights), 0)

	mockSampler.AssertExpectations(t)
}

func TestAdvancedPatternRecognizer_UpdatePatternDatabase(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Create training data
	trainingData := []PatternTrainingData{
		{
			ErrorPattern: "connection refused",
			Context: WorkflowContext{
				StepName: "push",
				Language: "go",
			},
			Resolution: "retry with exponential backoff",
			Success:    true,
			Metadata: map[string]interface{}{
				"category":    "network",
				"retry_count": 3,
			},
		},
		{
			ErrorPattern: "timeout exceeded",
			Context: WorkflowContext{
				StepName: "build",
				Language: "python",
			},
			Resolution: "increase timeout limit",
			Success:    true,
			Metadata: map[string]interface{}{
				"category":      "build",
				"timeout_value": 600,
			},
		},
	}

	// Update pattern database
	err := recognizer.UpdatePatternDatabase(context.Background(), trainingData)
	assert.NoError(t, err)

	// Verify learning metrics are updated
	stats, err := recognizer.GetPatternStatistics(context.Background())
	assert.NoError(t, err)
	assert.Greater(t, stats.LearningMetrics.TotalPatterns, 0)
	assert.Greater(t, stats.LearningMetrics.SuccessfulFixes, 0)
	assert.Greater(t, stats.LearningMetrics.PredictionsMade, 0)

	// Verify accuracy rate calculation
	expectedAccuracy := float64(stats.LearningMetrics.SuccessfulFixes) / float64(stats.LearningMetrics.PredictionsMade)
	assert.Equal(t, expectedAccuracy, stats.LearningMetrics.AccuracyRate)
}

func TestAdvancedPatternRecognizer_GetPatternStatistics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock sampler
	mockSampler := new(MockUnifiedSampler)

	// Create advanced pattern recognizer
	recognizer := NewAdvancedPatternRecognizer(mockSampler, logger)

	// Get initial statistics
	stats, err := recognizer.GetPatternStatistics(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// Verify statistics structure
	assert.GreaterOrEqual(t, stats.TotalErrors, 0)
	assert.GreaterOrEqual(t, stats.ResolvedErrors, 0)
	assert.GreaterOrEqual(t, stats.ResolutionRate, 0.0)
	assert.LessOrEqual(t, stats.ResolutionRate, 1.0)
	assert.GreaterOrEqual(t, stats.CacheHitRate, 0.0)
	assert.LessOrEqual(t, stats.CacheHitRate, 1.0)
	assert.NotNil(t, stats.CategoryCounts)
	assert.NotNil(t, stats.StepCounts)
	assert.NotNil(t, stats.LanguageCounts)
	assert.NotNil(t, stats.TopPatterns)
	assert.NotNil(t, stats.LearningMetrics)

	// Verify learning metrics structure
	assert.GreaterOrEqual(t, stats.LearningMetrics.TotalPatterns, 0)
	assert.GreaterOrEqual(t, stats.LearningMetrics.AccuracyRate, 0.0)
	assert.LessOrEqual(t, stats.LearningMetrics.AccuracyRate, 1.0)
	assert.GreaterOrEqual(t, stats.LearningMetrics.PatternsCovered, 0)
	assert.GreaterOrEqual(t, stats.LearningMetrics.PredictionsMade, 0)
	assert.GreaterOrEqual(t, stats.LearningMetrics.SuccessfulFixes, 0)
}
