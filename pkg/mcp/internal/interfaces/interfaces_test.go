package interfaces

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemResources tests the SystemResources struct
func TestSystemResources(t *testing.T) {
	now := time.Now()
	resources := SystemResources{
		CPUUsage:    75.5,
		MemoryUsage: 80.2,
		DiskUsage:   45.1,
		OpenFiles:   1024,
		GoRoutines:  100,
		HeapSize:    1024 * 1024 * 1024, // 1GB
		LastUpdated: now,
	}

	assert.Equal(t, 75.5, resources.CPUUsage)
	assert.Equal(t, 80.2, resources.MemoryUsage)
	assert.Equal(t, 45.1, resources.DiskUsage)
	assert.Equal(t, 1024, resources.OpenFiles)
	assert.Equal(t, 100, resources.GoRoutines)
	assert.Equal(t, int64(1024*1024*1024), resources.HeapSize)
	assert.Equal(t, now, resources.LastUpdated)
}

// TestSessionHealthStats tests the SessionHealthStats struct
func TestSessionHealthStats(t *testing.T) {
	stats := SessionHealthStats{
		ActiveSessions:    25,
		TotalSessions:     100,
		FailedSessions:    5,
		AverageSessionAge: 45.5,
		SessionErrors:     3,
	}

	assert.Equal(t, 25, stats.ActiveSessions)
	assert.Equal(t, 100, stats.TotalSessions)
	assert.Equal(t, 5, stats.FailedSessions)
	assert.Equal(t, 45.5, stats.AverageSessionAge)
	assert.Equal(t, 3, stats.SessionErrors)
}

// TestCircuitBreakerStatus tests the CircuitBreakerStatus struct
func TestCircuitBreakerStatus(t *testing.T) {
	lastFailure := time.Now().Add(-5 * time.Minute)
	nextRetry := time.Now().Add(1 * time.Minute)

	status := CircuitBreakerStatus{
		State:         "half-open",
		FailureCount:  3,
		LastFailure:   lastFailure,
		NextRetry:     nextRetry,
		TotalRequests: 1000,
		SuccessCount:  995,
	}

	assert.Equal(t, "half-open", status.State)
	assert.Equal(t, 3, status.FailureCount)
	assert.Equal(t, lastFailure, status.LastFailure)
	assert.Equal(t, nextRetry, status.NextRetry)
	assert.Equal(t, int64(1000), status.TotalRequests)
	assert.Equal(t, int64(995), status.SuccessCount)
}

// TestServiceHealth tests the ServiceHealth struct
func TestServiceHealth(t *testing.T) {
	lastCheck := time.Now().Add(-30 * time.Second)
	responseTime := 150 * time.Millisecond
	metadata := map[string]interface{}{
		"endpoint": "https://api.example.com",
		"version":  "v1.2.3",
	}

	health := ServiceHealth{
		Name:         "external-api",
		Status:       "healthy",
		LastCheck:    lastCheck,
		ResponseTime: responseTime,
		ErrorMessage: "",
		Metadata:     metadata,
	}

	assert.Equal(t, "external-api", health.Name)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, lastCheck, health.LastCheck)
	assert.Equal(t, responseTime, health.ResponseTime)
	assert.Empty(t, health.ErrorMessage)
	assert.Equal(t, "https://api.example.com", health.Metadata["endpoint"])
	assert.Equal(t, "v1.2.3", health.Metadata["version"])
}

// TestServiceHealth_WithError tests ServiceHealth with error conditions
func TestServiceHealth_WithError(t *testing.T) {
	health := ServiceHealth{
		Name:         "failing-service",
		Status:       "unhealthy",
		LastCheck:    time.Now(),
		ResponseTime: 5 * time.Second,
		ErrorMessage: "Connection timeout",
		Metadata: map[string]interface{}{
			"error_code":  500,
			"retry_after": 60,
		},
	}

	assert.Equal(t, "failing-service", health.Name)
	assert.Equal(t, "unhealthy", health.Status)
	assert.Equal(t, "Connection timeout", health.ErrorMessage)
	assert.Equal(t, 500, health.Metadata["error_code"])
}

// TestJobQueueStats tests the JobQueueStats struct
func TestJobQueueStats(t *testing.T) {
	stats := JobQueueStats{
		QueuedJobs:      15,
		RunningJobs:     5,
		CompletedJobs:   1500,
		FailedJobs:      25,
		AverageWaitTime: 2.5,
	}

	assert.Equal(t, 15, stats.QueuedJobs)
	assert.Equal(t, 5, stats.RunningJobs)
	assert.Equal(t, int64(1500), stats.CompletedJobs)
	assert.Equal(t, int64(25), stats.FailedJobs)
	assert.Equal(t, 2.5, stats.AverageWaitTime)
}

// TestRecentError tests the RecentError struct
func TestRecentError(t *testing.T) {
	timestamp := time.Now()
	context := map[string]interface{}{
		"session_id": "sess-123",
		"user_id":    "user-456",
		"operation":  "build_image",
	}

	err := RecentError{
		Timestamp: timestamp,
		Message:   "Failed to build Docker image",
		Component: "build-service",
		Severity:  "error",
		Context:   context,
	}

	assert.Equal(t, timestamp, err.Timestamp)
	assert.Equal(t, "Failed to build Docker image", err.Message)
	assert.Equal(t, "build-service", err.Component)
	assert.Equal(t, "error", err.Severity)
	assert.Equal(t, "sess-123", err.Context["session_id"])
	assert.Equal(t, "build_image", err.Context["operation"])
}

// TestProgressStage tests the ProgressStage struct
func TestProgressStage(t *testing.T) {
	stage := ProgressStage{
		Name:        "Build",
		Weight:      0.5,
		Description: "Building Docker image from source",
	}

	assert.Equal(t, "Build", stage.Name)
	assert.Equal(t, 0.5, stage.Weight)
	assert.Equal(t, "Building Docker image from source", stage.Description)
}

// TestSessionData tests the SessionData struct
func TestSessionData(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-1 * time.Hour)
	updatedAt := now.Add(-5 * time.Minute)
	expiresAt := now.Add(23 * time.Hour)
	lastAccess := now.Add(-2 * time.Minute)

	metadata := map[string]interface{}{
		"user_id":      "user-123",
		"project_name": "my-project",
		"language":     "Go",
	}

	sessionData := SessionData{
		ID:           "session-abc-123",
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		ExpiresAt:    expiresAt,
		CurrentStage: "build_complete",
		Metadata:     metadata,
		IsActive:     true,
		LastAccess:   lastAccess,
	}

	assert.Equal(t, "session-abc-123", sessionData.ID)
	assert.Equal(t, createdAt, sessionData.CreatedAt)
	assert.Equal(t, updatedAt, sessionData.UpdatedAt)
	assert.Equal(t, expiresAt, sessionData.ExpiresAt)
	assert.Equal(t, "build_complete", sessionData.CurrentStage)
	assert.True(t, sessionData.IsActive)
	assert.Equal(t, lastAccess, sessionData.LastAccess)
	assert.Equal(t, "user-123", sessionData.Metadata["user_id"])
	assert.Equal(t, "Go", sessionData.Metadata["language"])
}

// TestSessionManagerStats tests the SessionManagerStats struct
func TestSessionManagerStats(t *testing.T) {
	stats := SessionManagerStats{
		TotalSessions:   150,
		ActiveSessions:  45,
		ExpiredSessions: 20,
		AverageAge:      12.5,
		OldestSession:   "session-old-123",
		NewestSession:   "session-new-456",
	}

	assert.Equal(t, 150, stats.TotalSessions)
	assert.Equal(t, 45, stats.ActiveSessions)
	assert.Equal(t, 20, stats.ExpiredSessions)
	assert.Equal(t, 12.5, stats.AverageAge)
	assert.Equal(t, "session-old-123", stats.OldestSession)
	assert.Equal(t, "session-new-456", stats.NewestSession)
}

// TestStandardBuildStages tests the StandardBuildStages function
func TestStandardBuildStages(t *testing.T) {
	stages := StandardBuildStages()

	require.Len(t, stages, 5)

	// Test first stage
	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, 0.10, stages[0].Weight)
	assert.Contains(t, stages[0].Description, "Loading session")

	// Test build stage (should have highest weight)
	assert.Equal(t, "Build", stages[2].Name)
	assert.Equal(t, 0.50, stages[2].Weight)
	assert.Contains(t, stages[2].Description, "Building Docker image")

	// Test last stage
	assert.Equal(t, "Finalize", stages[4].Name)
	assert.Equal(t, 0.05, stages[4].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardDeployStages tests the StandardDeployStages function
func TestStandardDeployStages(t *testing.T) {
	stages := StandardDeployStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Generate", stages[1].Name)
	assert.Equal(t, "Deploy", stages[2].Name)
	assert.Equal(t, "Verify", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Deploy should have highest weight
	assert.Equal(t, 0.40, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardScanStages tests the StandardScanStages function
func TestStandardScanStages(t *testing.T) {
	stages := StandardScanStages()

	require.Len(t, stages, 4)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Scan", stages[1].Name)
	assert.Equal(t, "Analyze", stages[2].Name)
	assert.Equal(t, "Report", stages[3].Name)

	// Scan should have highest weight
	assert.Equal(t, 0.60, stages[1].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardAnalysisStages tests the StandardAnalysisStages function
func TestStandardAnalysisStages(t *testing.T) {
	stages := StandardAnalysisStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Discover", stages[1].Name)
	assert.Equal(t, "Analyze", stages[2].Name)
	assert.Equal(t, "Generate", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Analyze should have highest weight
	assert.Equal(t, 0.40, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardPushStages tests the StandardPushStages function
func TestStandardPushStages(t *testing.T) {
	stages := StandardPushStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Authenticate", stages[1].Name)
	assert.Equal(t, "Push", stages[2].Name)
	assert.Equal(t, "Verify", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Push should have highest weight
	assert.Equal(t, 0.60, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardPullStages tests the StandardPullStages function
func TestStandardPullStages(t *testing.T) {
	stages := StandardPullStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Authenticate", stages[1].Name)
	assert.Equal(t, "Pull", stages[2].Name)
	assert.Equal(t, "Verify", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Pull should have highest weight
	assert.Equal(t, 0.60, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardTagStages tests the StandardTagStages function
func TestStandardTagStages(t *testing.T) {
	stages := StandardTagStages()

	require.Len(t, stages, 4)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Tag", stages[1].Name)
	assert.Equal(t, "Verify", stages[2].Name)
	assert.Equal(t, "Finalize", stages[3].Name)

	// Tag should have highest weight
	assert.Equal(t, 0.60, stages[1].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardValidationStages tests the StandardValidationStages function
func TestStandardValidationStages(t *testing.T) {
	stages := StandardValidationStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Parse", stages[1].Name)
	assert.Equal(t, "Validate", stages[2].Name)
	assert.Equal(t, "Report", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Validate should have highest weight
	assert.Equal(t, 0.50, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardHealthStages tests the StandardHealthStages function
func TestStandardHealthStages(t *testing.T) {
	stages := StandardHealthStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Connect", stages[1].Name)
	assert.Equal(t, "Check", stages[2].Name)
	assert.Equal(t, "Analyze", stages[3].Name)
	assert.Equal(t, "Report", stages[4].Name)

	// Check should have highest weight
	assert.Equal(t, 0.50, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestStandardGenerateStages tests the StandardGenerateStages function
func TestStandardGenerateStages(t *testing.T) {
	stages := StandardGenerateStages()

	require.Len(t, stages, 5)

	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Template", stages[1].Name)
	assert.Equal(t, "Generate", stages[2].Name)
	assert.Equal(t, "Validate", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)

	// Generate should have highest weight
	assert.Equal(t, 0.40, stages[2].Weight)

	// Verify weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

// TestAllStandardStages tests that all standard stage functions return valid stages
func TestAllStandardStages(t *testing.T) {
	allStages := []struct {
		name   string
		stages []ProgressStage
	}{
		{"Build", StandardBuildStages()},
		{"Deploy", StandardDeployStages()},
		{"Scan", StandardScanStages()},
		{"Analysis", StandardAnalysisStages()},
		{"Push", StandardPushStages()},
		{"Pull", StandardPullStages()},
		{"Tag", StandardTagStages()},
		{"Validation", StandardValidationStages()},
		{"Health", StandardHealthStages()},
		{"Generate", StandardGenerateStages()},
	}

	for _, stageSet := range allStages {
		t.Run(stageSet.name, func(t *testing.T) {
			stages := stageSet.stages

			// All stage sets should have at least one stage
			assert.NotEmpty(t, stages, "Stage set %s should not be empty", stageSet.name)

			totalWeight := 0.0
			for i, stage := range stages {
				// Each stage should have a name
				assert.NotEmpty(t, stage.Name, "Stage %d in %s should have a name", i, stageSet.name)

				// Weight should be between 0 and 1
				assert.GreaterOrEqual(t, stage.Weight, 0.0, "Stage %s weight should be >= 0", stage.Name)
				assert.LessOrEqual(t, stage.Weight, 1.0, "Stage %s weight should be <= 1", stage.Name)

				// Description should not be empty
				assert.NotEmpty(t, stage.Description, "Stage %s should have a description", stage.Name)

				totalWeight += stage.Weight
			}

			// Total weight should sum to 1.0 (with some tolerance for floating point)
			assert.InDelta(t, 1.0, totalWeight, 0.001, "Total weight for %s stages should be 1.0", stageSet.name)
		})
	}
}

// MockHealthChecker implements HealthChecker for testing
type MockHealthChecker struct {
	systemResources        SystemResources
	sessionStats           SessionHealthStats
	circuitBreakerStats    map[string]CircuitBreakerStatus
	serviceHealth          []ServiceHealth
	jobQueueStats          JobQueueStats
	recentErrors           []RecentError
	checkServiceHealthFunc func(ctx context.Context) []ServiceHealth
}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		systemResources: SystemResources{
			CPUUsage:    50.0,
			MemoryUsage: 60.0,
			DiskUsage:   30.0,
			OpenFiles:   512,
			GoRoutines:  50,
			HeapSize:    512 * 1024 * 1024,
			LastUpdated: time.Now(),
		},
		sessionStats: SessionHealthStats{
			ActiveSessions:    10,
			TotalSessions:     50,
			FailedSessions:    2,
			AverageSessionAge: 30.0,
			SessionErrors:     1,
		},
		circuitBreakerStats: map[string]CircuitBreakerStatus{
			"database": {
				State:         "closed",
				FailureCount:  0,
				LastFailure:   time.Time{},
				NextRetry:     time.Time{},
				TotalRequests: 1000,
				SuccessCount:  1000,
			},
		},
		serviceHealth: []ServiceHealth{
			{
				Name:         "database",
				Status:       "healthy",
				LastCheck:    time.Now(),
				ResponseTime: 10 * time.Millisecond,
			},
		},
		jobQueueStats: JobQueueStats{
			QueuedJobs:      5,
			RunningJobs:     2,
			CompletedJobs:   100,
			FailedJobs:      3,
			AverageWaitTime: 1.5,
		},
		recentErrors: []RecentError{
			{
				Timestamp: time.Now(),
				Message:   "Test error",
				Component: "test",
				Severity:  "warning",
			},
		},
	}
}

func (m *MockHealthChecker) GetSystemResources() SystemResources {
	return m.systemResources
}

func (m *MockHealthChecker) GetSessionStats() SessionHealthStats {
	return m.sessionStats
}

func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]CircuitBreakerStatus {
	return m.circuitBreakerStats
}

func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []ServiceHealth {
	if m.checkServiceHealthFunc != nil {
		return m.checkServiceHealthFunc(ctx)
	}
	return m.serviceHealth
}

func (m *MockHealthChecker) GetJobQueueStats() JobQueueStats {
	return m.jobQueueStats
}

func (m *MockHealthChecker) GetRecentErrors(limit int) []RecentError {
	if limit <= 0 || limit >= len(m.recentErrors) {
		return m.recentErrors
	}
	return m.recentErrors[:limit]
}

// TestHealthCheckerInterface tests the HealthChecker interface implementation
func TestHealthCheckerInterface(t *testing.T) {
	checker := NewMockHealthChecker()

	// Test GetSystemResources
	resources := checker.GetSystemResources()
	assert.Equal(t, 50.0, resources.CPUUsage)
	assert.Equal(t, 60.0, resources.MemoryUsage)
	assert.Equal(t, 512, resources.OpenFiles)

	// Test GetSessionStats
	stats := checker.GetSessionStats()
	assert.Equal(t, 10, stats.ActiveSessions)
	assert.Equal(t, 50, stats.TotalSessions)
	assert.Equal(t, 30.0, stats.AverageSessionAge)

	// Test GetCircuitBreakerStats
	breakerStats := checker.GetCircuitBreakerStats()
	assert.Contains(t, breakerStats, "database")
	assert.Equal(t, "closed", breakerStats["database"].State)
	assert.Equal(t, int64(1000), breakerStats["database"].TotalRequests)

	// Test CheckServiceHealth
	ctx := context.Background()
	serviceHealth := checker.CheckServiceHealth(ctx)
	require.Len(t, serviceHealth, 1)
	assert.Equal(t, "database", serviceHealth[0].Name)
	assert.Equal(t, "healthy", serviceHealth[0].Status)

	// Test GetJobQueueStats
	queueStats := checker.GetJobQueueStats()
	assert.Equal(t, 5, queueStats.QueuedJobs)
	assert.Equal(t, 2, queueStats.RunningJobs)
	assert.Equal(t, int64(100), queueStats.CompletedJobs)

	// Test GetRecentErrors
	errors := checker.GetRecentErrors(10)
	require.Len(t, errors, 1)
	assert.Equal(t, "Test error", errors[0].Message)
	assert.Equal(t, "test", errors[0].Component)
	assert.Equal(t, "warning", errors[0].Severity)

	// Test GetRecentErrors with limit
	errors = checker.GetRecentErrors(0)
	require.Len(t, errors, 1) // Should return all when limit is 0
}

// MockProgressReporter implements ProgressReporter for testing
type MockProgressReporter struct {
	stages          []ProgressStage
	currentStage    int
	stageProgress   float64
	overallProgress float64
	messages        []string
}

func NewMockProgressReporter(stages []ProgressStage) *MockProgressReporter {
	return &MockProgressReporter{
		stages:   stages,
		messages: make([]string, 0),
	}
}

func (m *MockProgressReporter) ReportStage(stageProgress float64, message string) {
	m.stageProgress = stageProgress
	m.messages = append(m.messages, message)
}

func (m *MockProgressReporter) NextStage(message string) {
	if m.currentStage < len(m.stages)-1 {
		m.currentStage++
	}
	m.messages = append(m.messages, message)
}

func (m *MockProgressReporter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(m.stages) {
		m.currentStage = stageIndex
	}
	m.messages = append(m.messages, message)
}

func (m *MockProgressReporter) ReportOverall(progress float64, message string) {
	m.overallProgress = progress
	m.messages = append(m.messages, message)
}

func (m *MockProgressReporter) GetCurrentStage() (int, ProgressStage) {
	if m.currentStage < len(m.stages) {
		return m.currentStage, m.stages[m.currentStage]
	}
	return m.currentStage, ProgressStage{}
}

// TestProgressReporterInterface tests the ProgressReporter interface implementation
func TestProgressReporterInterface(t *testing.T) {
	stages := StandardBuildStages()
	reporter := NewMockProgressReporter(stages)

	// Test initial state
	stageIndex, stage := reporter.GetCurrentStage()
	assert.Equal(t, 0, stageIndex)
	assert.Equal(t, "Initialize", stage.Name)

	// Test ReportStage
	reporter.ReportStage(0.5, "Halfway through initialization")
	assert.Equal(t, 0.5, reporter.stageProgress)
	assert.Contains(t, reporter.messages, "Halfway through initialization")

	// Test NextStage
	reporter.NextStage("Moving to analysis")
	stageIndex, stage = reporter.GetCurrentStage()
	assert.Equal(t, 1, stageIndex)
	assert.Equal(t, "Analyze", stage.Name)
	assert.Contains(t, reporter.messages, "Moving to analysis")

	// Test SetStage
	reporter.SetStage(3, "Jumping to verify stage")
	stageIndex, stage = reporter.GetCurrentStage()
	assert.Equal(t, 3, stageIndex)
	assert.Equal(t, "Verify", stage.Name)
	assert.Contains(t, reporter.messages, "Jumping to verify stage")

	// Test ReportOverall
	reporter.ReportOverall(0.75, "75% complete overall")
	assert.Equal(t, 0.75, reporter.overallProgress)
	assert.Contains(t, reporter.messages, "75% complete overall")

	// Test SetStage with invalid index
	reporter.SetStage(10, "Invalid stage")
	stageIndex, _ = reporter.GetCurrentStage()
	assert.Equal(t, 3, stageIndex) // Should remain unchanged
}

// MockProgressTracker implements ProgressTracker for testing
type MockProgressTracker struct {
	operations []string
	stages     [][]ProgressStage
	errors     []error
}

func NewMockProgressTracker() *MockProgressTracker {
	return &MockProgressTracker{
		operations: make([]string, 0),
		stages:     make([][]ProgressStage, 0),
		errors:     make([]error, 0),
	}
}

func (m *MockProgressTracker) RunWithProgress(
	ctx context.Context,
	operation string,
	stages []ProgressStage,
	fn func(ctx context.Context, reporter ProgressReporter) error,
) error {
	m.operations = append(m.operations, operation)
	m.stages = append(m.stages, stages)

	reporter := NewMockProgressReporter(stages)
	err := fn(ctx, reporter)
	m.errors = append(m.errors, err)

	return err
}

// TestProgressTrackerInterface tests the ProgressTracker interface implementation
func TestProgressTrackerInterface(t *testing.T) {
	tracker := NewMockProgressTracker()
	ctx := context.Background()

	// Test successful operation
	stages := StandardBuildStages()
	err := tracker.RunWithProgress(ctx, "test-build", stages, func(ctx context.Context, reporter ProgressReporter) error {
		reporter.ReportStage(0.5, "Building...")
		reporter.NextStage("Verifying...")
		return nil
	})

	assert.NoError(t, err)
	assert.Len(t, tracker.operations, 1)
	assert.Equal(t, "test-build", tracker.operations[0])
	assert.Equal(t, stages, tracker.stages[0])
	assert.NoError(t, tracker.errors[0])

	// Test operation with error
	err = tracker.RunWithProgress(ctx, "test-deploy", StandardDeployStages(), func(ctx context.Context, reporter ProgressReporter) error {
		return assert.AnError
	})

	assert.Error(t, err)
	assert.Len(t, tracker.operations, 2)
	assert.Equal(t, "test-deploy", tracker.operations[1])
	assert.Error(t, tracker.errors[1])
}

// Benchmark tests
func BenchmarkStandardBuildStages(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = StandardBuildStages()
	}
}

func BenchmarkStandardDeployStages(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = StandardDeployStages()
	}
}

func BenchmarkProgressStageWeightCalculation(b *testing.B) {
	stages := StandardBuildStages()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		totalWeight := 0.0
		for _, stage := range stages {
			totalWeight += stage.Weight
		}
		_ = totalWeight
	}
}
