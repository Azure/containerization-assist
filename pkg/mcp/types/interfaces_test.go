package types

import (
	"context"
	"testing"
	"time"
)

// Test interface conformance by implementing mock types

// MockAIAnalyzer implements AIAnalyzer interface
type MockAIAnalyzer struct{}

func (m *MockAIAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "mock analysis result", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "mock analysis with file tools", nil
}

func (m *MockAIAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "mock formatted analysis", nil
}

func (m *MockAIAnalyzer) GetTokenUsage() TokenUsage {
	return TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
}

func (m *MockAIAnalyzer) ResetTokenUsage() {}

// MockHealthChecker implements HealthChecker interface
type MockHealthChecker struct{}

func (m *MockHealthChecker) GetSystemResources() SystemResources {
	return SystemResources{
		CPUUsage:    50.0,
		MemoryUsage: 75.0,
		DiskUsage:   60.0,
		OpenFiles:   100,
		GoRoutines:  50,
	}
}

func (m *MockHealthChecker) GetSessionStats() SessionHealthStats {
	return SessionHealthStats{
		ActiveSessions:    5,
		TotalSessions:     100,
		FailedSessions:    2,
		AverageSessionAge: 250.0,
	}
}

func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]CircuitBreakerStatus {
	return map[string]CircuitBreakerStatus{
		"docker": {State: CircuitBreakerClosed},
		"k8s":    {State: CircuitBreakerOpen},
	}
}

func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []ServiceHealth {
	return []ServiceHealth{
		{
			Name:      "docker",
			Status:    "healthy",
			LastCheck: time.Now(),
		},
	}
}

func (m *MockHealthChecker) GetJobQueueStats() JobQueueStats {
	return JobQueueStats{
		QueuedJobs:    3,
		RunningJobs:   2,
		CompletedJobs: 95,
		FailedJobs:    5,
	}
}

func (m *MockHealthChecker) GetRecentErrors(limit int) []RecentError {
	return []RecentError{
		{
			Timestamp: time.Now(),
			Message:   "mock error",
			Component: "test",
			Context:   map[string]interface{}{"test": true},
		},
	}
}

// MockProgressReporter implements ProgressReporter interface
type MockProgressReporter struct{}

func (m *MockProgressReporter) ReportStage(stageProgress float64, message string) {}

func (m *MockProgressReporter) NextStage(message string) {}

func (m *MockProgressReporter) SetStage(stageIndex int, message string) {}

func (m *MockProgressReporter) ReportOverall(progress float64, message string) {}

func (m *MockProgressReporter) GetCurrentStage() (int, ProgressStage) {
	return 0, ProgressStage{
		Name:        "test",
		Weight:      0.5,
		Description: "test stage",
	}
}

// Note: SessionManager testing removed due to complex dependencies

// MockBaseAnalyzer implements BaseAnalyzer interface
type MockBaseAnalyzer struct{}

func (m *MockBaseAnalyzer) Analyze(ctx context.Context, input interface{}, options BaseAnalysisOptions) (*BaseAnalysisResult, error) {
	return &BaseAnalysisResult{
		Summary: BaseAnalysisSummary{
			TotalFindings: 0,
			OverallScore:  100,
		},
		Findings:        []BaseFinding{},
		Recommendations: []BaseRecommendation{},
		Metrics:         map[string]interface{}{},
		Context:         map[string]interface{}{},
	}, nil
}

func (m *MockBaseAnalyzer) GetName() string {
	return "mock-analyzer"
}

func (m *MockBaseAnalyzer) GetCapabilities() BaseAnalyzerCapabilities {
	return BaseAnalyzerCapabilities{
		SupportedTypes:   []string{"test"},
		SupportedAspects: []string{"security", "performance"},
		RequiresContext:  false,
		SupportsDeepScan: true,
	}
}

// MockBaseValidator removed - BaseValidator interface moved to base package

// Test interface conformance
func TestInterfaceConformance(t *testing.T) {
	// Test AIAnalyzer interface
	var aiAnalyzer AIAnalyzer = &MockAIAnalyzer{}
	if aiAnalyzer.GetTokenUsage().TotalTokens != 150 {
		t.Error("AIAnalyzer interface not properly implemented")
	}

	// Test HealthChecker interface
	var healthChecker HealthChecker = &MockHealthChecker{}
	stats := healthChecker.GetSessionStats()
	if stats.ActiveSessions != 5 {
		t.Error("HealthChecker interface not properly implemented")
	}

	// Test ProgressReporter interface
	var progressReporter ProgressReporter = &MockProgressReporter{}
	idx, stage := progressReporter.GetCurrentStage()
	if idx != 0 || stage.Name != "test" {
		t.Error("ProgressReporter interface not properly implemented")
	}

	// Note: SessionManager test removed due to complex dependencies

	// Test BaseAnalyzer interface
	var baseAnalyzer BaseAnalyzer = &MockBaseAnalyzer{}
	if baseAnalyzer.GetName() != "mock-analyzer" {
		t.Error("BaseAnalyzer interface not properly implemented")
	}

	// BaseValidator test removed - interface moved to base package
}

// Test interface type completeness
func TestTypeCompleteness(t *testing.T) {
	// Test that all required types are defined
	var tokenUsage TokenUsage
	tokenUsage.TotalTokens = 100

	var progressStage ProgressStage
	progressStage.Name = "test"

	var sessionState SessionState
	sessionState.SessionID = "test"

	// Test enum values
	if CircuitBreakerOpen == "" || CircuitBreakerClosed == "" {
		t.Error("CircuitBreaker constants not defined")
	}

	t.Log("All interface types are properly defined")
}
