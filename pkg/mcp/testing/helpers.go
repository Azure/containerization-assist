// Package testing provides shared test utilities for all MCP workstreams
package testing

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TempDirHelper provides utilities for working with temporary directories in tests
type TempDirHelper struct {
	t       testing.TB
	tempDir string
}

// NewTempDir creates a new temporary directory helper for tests
func NewTempDir(t testing.TB) *TempDirHelper {
	t.Helper()
	tempDir := t.TempDir()
	return &TempDirHelper{
		t:       t,
		tempDir: tempDir,
	}
}

// Path returns the full path to a file or directory within the temp directory
func (td *TempDirHelper) Path(parts ...string) string {
	td.t.Helper()
	fullPath := append([]string{td.tempDir}, parts...)
	return filepath.Join(fullPath...)
}

// WriteFile writes data to a file within the temp directory
func (td *TempDirHelper) WriteFile(filename string, data []byte, perm os.FileMode) {
	td.t.Helper()
	path := td.Path(filename)

	// Create directory if needed
	dir := filepath.Dir(path)
	if dir != td.tempDir {
		err := os.MkdirAll(dir, 0750)
		require.NoError(td.t, err, "Failed to create directory %s", dir)
	}

	err := os.WriteFile(path, data, perm)
	require.NoError(td.t, err, "Failed to write file %s", path)
}

// ReadFile reads data from a file within the temp directory
func (td *TempDirHelper) ReadFile(filename string) []byte {
	td.t.Helper()
	path := td.Path(filename)
	data, err := os.ReadFile(path)
	require.NoError(td.t, err, "Failed to read file %s", path)
	return data
}

// CreateDir creates a directory within the temp directory
func (td *TempDirHelper) CreateDir(dirname string, perm os.FileMode) {
	td.t.Helper()
	path := td.Path(dirname)
	err := os.MkdirAll(path, perm)
	require.NoError(td.t, err, "Failed to create directory %s", path)
}

// FileExists checks if a file exists within the temp directory
func (td *TempDirHelper) FileExists(filename string) bool {
	td.t.Helper()
	path := td.Path(filename)
	_, err := os.Stat(path)
	return err == nil
}

// Root returns the root path of the temporary directory
func (td *TempDirHelper) Root() string {
	return td.tempDir
}

// TimeHelper provides utilities for working with time in tests
type TimeHelper struct {
	fixedTime time.Time
	now       func() time.Time
}

// NewTimeHelper creates a new time helper with a fixed time
func NewTimeHelper(fixedTime time.Time) *TimeHelper {
	return &TimeHelper{
		fixedTime: fixedTime,
		now:       func() time.Time { return fixedTime },
	}
}

// Now returns the current time (fixed time in tests)
func (th *TimeHelper) Now() time.Time {
	return th.now()
}

// After returns a time after the current time
func (th *TimeHelper) After(d time.Duration) time.Time {
	return th.now().Add(d)
}

// Before returns a time before the current time
func (th *TimeHelper) Before(d time.Duration) time.Time {
	return th.now().Add(-d)
}

// AdvanceTime advances the fixed time by the given duration
func (th *TimeHelper) AdvanceTime(d time.Duration) {
	th.fixedTime = th.fixedTime.Add(d)
	th.now = func() time.Time { return th.fixedTime }
}

// ContextHelper provides utilities for working with contexts in tests
type ContextHelper struct {
	timeout time.Duration
}

// NewContextHelper creates a new context helper with default timeout
func NewContextHelper(timeout time.Duration) *ContextHelper {
	return &ContextHelper{timeout: timeout}
}

// WithTimeout creates a context with the configured timeout
func (ch *ContextHelper) WithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ch.timeout)
}

// WithCancel creates a cancellable context
func (ch *ContextHelper) WithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// Background returns a background context
func (ch *ContextHelper) Background() context.Context {
	return context.Background()
}

// ErrorAssertions provides utilities for asserting errors in tests
type ErrorAssertions struct {
	t testing.TB
}

// NewErrorAssertions creates a new error assertions helper
func NewErrorAssertions(t testing.TB) *ErrorAssertions {
	t.Helper()
	return &ErrorAssertions{t: t}
}

// RequireError requires that an error occurred and has the expected message
func (ea *ErrorAssertions) RequireError(err error, expectedMessage string) {
	ea.t.Helper()
	require.Error(ea.t, err, "Expected an error but got nil")
	require.Contains(ea.t, err.Error(), expectedMessage, "Error message does not contain expected text")
}

// RequireNoError requires that no error occurred
func (ea *ErrorAssertions) RequireNoError(err error, msgAndArgs ...interface{}) {
	ea.t.Helper()
	require.NoError(ea.t, err, msgAndArgs...)
}

// RequireErrorType requires that an error is of a specific type
func (ea *ErrorAssertions) RequireErrorType(err error, expectedType interface{}) {
	ea.t.Helper()
	require.Error(ea.t, err, "Expected an error but got nil")
	require.IsType(ea.t, expectedType, err, "Error is not of expected type")
}

// LoggerHelper provides utilities for working with loggers in tests
type LoggerHelper struct {
	output io.Writer
	level  zerolog.Level
}

// NewLoggerHelper creates a new logger helper
func NewLoggerHelper() *LoggerHelper {
	return &LoggerHelper{
		output: io.Discard, // Silent by default
		level:  zerolog.DebugLevel,
	}
}

// WithOutput sets the output writer for the logger
func (lh *LoggerHelper) WithOutput(w io.Writer) *LoggerHelper {
	lh.output = w
	return lh
}

// WithLevel sets the log level
func (lh *LoggerHelper) WithLevel(level zerolog.Level) *LoggerHelper {
	lh.level = level
	return lh
}

// Logger creates a zerolog logger with the configured settings
func (lh *LoggerHelper) Logger() zerolog.Logger {
	return zerolog.New(lh.output).Level(lh.level)
}

// SilentLogger creates a logger that discards all output
func (lh *LoggerHelper) SilentLogger() zerolog.Logger {
	return zerolog.New(io.Discard).Level(zerolog.Disabled)
}

// AssertHelper provides comprehensive assertion utilities
type AssertHelper struct {
	t testing.TB
}

// NewAssertHelper creates a new assertion helper
func NewAssertHelper(t testing.TB) *AssertHelper {
	t.Helper()
	return &AssertHelper{t: t}
}

// RequireNonEmpty requires that a string is not empty
func (ah *AssertHelper) RequireNonEmpty(s string, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.NotEmpty(ah.t, s, msgAndArgs...)
}

// RequireValidTime requires that a time is not zero and is reasonable
func (ah *AssertHelper) RequireValidTime(tm time.Time, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.False(ah.t, tm.IsZero(), "Time should not be zero")

	// Time should be within last hour and next hour (reasonable for tests)
	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)
	oneHourFromNow := now.Add(time.Hour)

	require.True(ah.t, tm.After(oneHourAgo) && tm.Before(oneHourFromNow),
		"Time %v should be within reasonable range", tm)
}

// RequireJSON requires that a string is valid JSON
func (ah *AssertHelper) RequireJSON(jsonStr string, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.JSONEq(ah.t, jsonStr, jsonStr, msgAndArgs...) // JSONEq validates JSON syntax
}

// DataGenerator provides utilities for generating test data
type DataGenerator struct {
	timeHelper *TimeHelper
}

// NewDataGenerator creates a new data generator
func NewDataGenerator() *DataGenerator {
	return &DataGenerator{
		timeHelper: NewTimeHelper(time.Date(2025, 6, 24, 12, 0, 0, 0, time.UTC)),
	}
}

// SessionID generates a test session ID
func (dg *DataGenerator) SessionID() string {
	return "test-session-" + dg.timeHelper.Now().Format("20060102-150405")
}

// ImageName generates a test image name
func (dg *DataGenerator) ImageName() string {
	return "test/image:latest"
}

// ErrorMessage generates a test error message
func (dg *DataGenerator) ErrorMessage() string {
	return "test error: " + dg.timeHelper.Now().Format(time.RFC3339)
}

// TestMetadata generates test metadata map
func (dg *DataGenerator) TestMetadata() map[string]interface{} {
	return map[string]interface{}{
		"test_id":    dg.SessionID(),
		"created_at": dg.timeHelper.Now(),
		"language":   "go",
		"framework":  "test",
	}
}

// TestConfig provides a complete test configuration
type TestConfig struct {
	TempDir *TempDirHelper
	Time    *TimeHelper
	Context *ContextHelper
	Errors  *ErrorAssertions
	Logger  *LoggerHelper
	Assert  *AssertHelper
	Data    *DataGenerator
}

// NewTestConfig creates a complete test configuration with all helpers
func NewTestConfig(t testing.TB) *TestConfig {
	t.Helper()

	return &TestConfig{
		TempDir: NewTempDir(t),
		Time:    NewTimeHelper(time.Date(2025, 6, 24, 12, 0, 0, 0, time.UTC)),
		Context: NewContextHelper(30 * time.Second),
		Errors:  NewErrorAssertions(t),
		Logger:  NewLoggerHelper(),
		Assert:  NewAssertHelper(t),
		Data:    NewDataGenerator(),
	}
}

// PerformanceHelper provides utilities for performance testing
type PerformanceHelper struct {
	t testing.TB
}

// NewPerformanceHelper creates a new performance helper
func NewPerformanceHelper(t testing.TB) *PerformanceHelper {
	t.Helper()
	return &PerformanceHelper{t: t}
}

// MeasureTime measures the execution time of a function
func (ph *PerformanceHelper) MeasureTime(fn func()) time.Duration {
	ph.t.Helper()
	start := time.Now()
	fn()
	return time.Since(start)
}

// RequireUnderTimeout requires that a function executes within a timeout
func (ph *PerformanceHelper) RequireUnderTimeout(timeout time.Duration, fn func()) {
	ph.t.Helper()
	duration := ph.MeasureTime(fn)
	require.True(ph.t, duration < timeout,
		"Function took %v, expected under %v", duration, timeout)
}

// BenchmarkHelper provides utilities for benchmark tests
func (ph *PerformanceHelper) BenchmarkHelper(b *testing.B, fn func()) {
	b.Helper()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn()
	}
}

// MockHealthChecker implements types.HealthChecker for testing
type MockHealthChecker struct {
	SystemResourcesFunc     func() types.SystemResources
	SessionStatsFunc        func() types.SessionHealthStats
	CircuitBreakerStatsFunc func() map[string]types.CircuitBreakerStatus
	CheckServiceHealthFunc  func(ctx context.Context) []types.ServiceHealth
	JobQueueStatsFunc       func() types.JobQueueStats
	RecentErrorsFunc        func(limit int) []types.RecentError
}

// NewMockHealthChecker creates a new mock health checker with default implementations
func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		SystemResourcesFunc: func() types.SystemResources {
			return types.SystemResources{
				CPUUsage:    50.0,
				MemoryUsage: 60.0,
				DiskUsage:   30.0,
				OpenFiles:   100,
				GoRoutines:  50,
				HeapSize:    1024 * 1024,
				LastUpdated: time.Now(),
			}
		},
		SessionStatsFunc: func() types.SessionHealthStats {
			return types.SessionHealthStats{
				ActiveSessions:    5,
				TotalSessions:     20,
				FailedSessions:    1,
				AverageSessionAge: 30.0,
				SessionErrors:     0,
			}
		},
		CircuitBreakerStatsFunc: func() map[string]types.CircuitBreakerStatus {
			return make(map[string]types.CircuitBreakerStatus)
		},
		CheckServiceHealthFunc: func(ctx context.Context) []types.ServiceHealth {
			return []types.ServiceHealth{
				{
					Name:         "test-service",
					Status:       "healthy",
					LastCheck:    time.Now(),
					ResponseTime: 10 * time.Millisecond,
				},
			}
		},
		JobQueueStatsFunc: func() types.JobQueueStats {
			return types.JobQueueStats{
				QueuedJobs:      0,
				RunningJobs:     1,
				CompletedJobs:   10,
				FailedJobs:      0,
				AverageWaitTime: 1.0,
			}
		},
		RecentErrorsFunc: func(limit int) []types.RecentError {
			return []types.RecentError{}
		},
	}
}

// GetSystemResources implements types.HealthChecker
func (m *MockHealthChecker) GetSystemResources() types.SystemResources {
	if m.SystemResourcesFunc != nil {
		return m.SystemResourcesFunc()
	}
	return types.SystemResources{}
}

// GetSessionStats implements types.HealthChecker
func (m *MockHealthChecker) GetSessionStats() types.SessionHealthStats {
	if m.SessionStatsFunc != nil {
		return m.SessionStatsFunc()
	}
	return types.SessionHealthStats{}
}

// GetCircuitBreakerStats implements types.HealthChecker
func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]types.CircuitBreakerStatus {
	if m.CircuitBreakerStatsFunc != nil {
		return m.CircuitBreakerStatsFunc()
	}
	return make(map[string]types.CircuitBreakerStatus)
}

// CheckServiceHealth implements types.HealthChecker
func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []types.ServiceHealth {
	if m.CheckServiceHealthFunc != nil {
		return m.CheckServiceHealthFunc(ctx)
	}
	return []types.ServiceHealth{}
}

// GetJobQueueStats implements types.HealthChecker
func (m *MockHealthChecker) GetJobQueueStats() types.JobQueueStats {
	if m.JobQueueStatsFunc != nil {
		return m.JobQueueStatsFunc()
	}
	return types.JobQueueStats{}
}

// GetRecentErrors implements types.HealthChecker
func (m *MockHealthChecker) GetRecentErrors(limit int) []types.RecentError {
	if m.RecentErrorsFunc != nil {
		return m.RecentErrorsFunc(limit)
	}
	return []types.RecentError{}
}

// MockProgressReporter implements types.ProgressReporter for testing
type MockProgressReporter struct {
	stages            []types.ProgressStage
	currentStage      int
	stageProgress     float64
	overallProgress   float64
	messages          []string
	ReportStageFunc   func(stageProgress float64, message string)
	NextStageFunc     func(message string)
	SetStageFunc      func(stageIndex int, message string)
	ReportOverallFunc func(progress float64, message string)
}

// NewMockProgressReporter creates a new mock progress reporter
func NewMockProgressReporter(stages []types.ProgressStage) *MockProgressReporter {
	return &MockProgressReporter{
		stages:   stages,
		messages: make([]string, 0),
	}
}

// ReportStage implements types.ProgressReporter
func (m *MockProgressReporter) ReportStage(stageProgress float64, message string) {
	m.stageProgress = stageProgress
	m.messages = append(m.messages, message)
	if m.ReportStageFunc != nil {
		m.ReportStageFunc(stageProgress, message)
	}
}

// NextStage implements types.ProgressReporter
func (m *MockProgressReporter) NextStage(message string) {
	if m.currentStage < len(m.stages)-1 {
		m.currentStage++
	}
	m.messages = append(m.messages, message)
	if m.NextStageFunc != nil {
		m.NextStageFunc(message)
	}
}

// SetStage implements types.ProgressReporter
func (m *MockProgressReporter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(m.stages) {
		m.currentStage = stageIndex
	}
	m.messages = append(m.messages, message)
	if m.SetStageFunc != nil {
		m.SetStageFunc(stageIndex, message)
	}
}

// ReportOverall implements types.ProgressReporter
func (m *MockProgressReporter) ReportOverall(progress float64, message string) {
	m.overallProgress = progress
	m.messages = append(m.messages, message)
	if m.ReportOverallFunc != nil {
		m.ReportOverallFunc(progress, message)
	}
}

// GetCurrentStage implements types.ProgressReporter
func (m *MockProgressReporter) GetCurrentStage() (int, types.ProgressStage) {
	if m.currentStage < len(m.stages) {
		return m.currentStage, m.stages[m.currentStage]
	}
	return m.currentStage, types.ProgressStage{}
}

// GetMessages returns all messages received by the mock
func (m *MockProgressReporter) GetMessages() []string {
	return m.messages
}

// GetStageProgress returns the last reported stage progress
func (m *MockProgressReporter) GetStageProgress() float64 {
	return m.stageProgress
}

// GetOverallProgress returns the last reported overall progress
func (m *MockProgressReporter) GetOverallProgress() float64 {
	return m.overallProgress
}
