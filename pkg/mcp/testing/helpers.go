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

func NewTempDir(t testing.TB) *TempDirHelper {
	t.Helper()
	tempDir := t.TempDir()
	return &TempDirHelper{
		t:       t,
		tempDir: tempDir,
	}
}

func (td *TempDirHelper) Path(parts ...string) string {
	td.t.Helper()
	fullPath := append([]string{td.tempDir}, parts...)
	return filepath.Join(fullPath...)
}

func (td *TempDirHelper) WriteFile(filename string, data []byte, perm os.FileMode) {
	td.t.Helper()
	path := td.Path(filename)

	dir := filepath.Dir(path)
	if dir != td.tempDir {
		err := os.MkdirAll(dir, 0750)
		require.NoError(td.t, err, "Failed to create directory %s", dir)
	}

	err := os.WriteFile(path, data, perm)
	require.NoError(td.t, err, "Failed to write file %s", path)
}

func (td *TempDirHelper) ReadFile(filename string) []byte {
	td.t.Helper()
	path := td.Path(filename)
	data, err := os.ReadFile(path)
	require.NoError(td.t, err, "Failed to read file %s", path)
	return data
}

func (td *TempDirHelper) CreateDir(dirname string, perm os.FileMode) {
	td.t.Helper()
	path := td.Path(dirname)
	err := os.MkdirAll(path, perm)
	require.NoError(td.t, err, "Failed to create directory %s", path)
}

func (td *TempDirHelper) FileExists(filename string) bool {
	td.t.Helper()
	path := td.Path(filename)
	_, err := os.Stat(path)
	return err == nil
}

func (td *TempDirHelper) Root() string {
	return td.tempDir
}

// TimeHelper provides utilities for working with time in tests
type TimeHelper struct {
	fixedTime time.Time
	now       func() time.Time
}

func NewTimeHelper(fixedTime time.Time) *TimeHelper {
	return &TimeHelper{
		fixedTime: fixedTime,
		now:       func() time.Time { return fixedTime },
	}
}

func (th *TimeHelper) Now() time.Time {
	return th.now()
}

func (th *TimeHelper) After(d time.Duration) time.Time {
	return th.now().Add(d)
}

func (th *TimeHelper) Before(d time.Duration) time.Time {
	return th.now().Add(-d)
}

func (th *TimeHelper) AdvanceTime(d time.Duration) {
	th.fixedTime = th.fixedTime.Add(d)
	th.now = func() time.Time { return th.fixedTime }
}

// ContextHelper provides utilities for working with contexts in tests
type ContextHelper struct {
	timeout time.Duration
}

func NewContextHelper(timeout time.Duration) *ContextHelper {
	return &ContextHelper{timeout: timeout}
}

func (ch *ContextHelper) WithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ch.timeout)
}

func (ch *ContextHelper) WithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func (ch *ContextHelper) Background() context.Context {
	return context.Background()
}

// ErrorAssertions provides utilities for asserting errors in tests
type ErrorAssertions struct {
	t testing.TB
}

func NewErrorAssertions(t testing.TB) *ErrorAssertions {
	t.Helper()
	return &ErrorAssertions{t: t}
}

func (ea *ErrorAssertions) RequireError(err error, expectedMessage string) {
	ea.t.Helper()
	require.Error(ea.t, err, "Expected an error but got nil")
	require.Contains(ea.t, err.Error(), expectedMessage, "Error message does not contain expected text")
}

func (ea *ErrorAssertions) RequireNoError(err error, msgAndArgs ...interface{}) {
	ea.t.Helper()
	require.NoError(ea.t, err, msgAndArgs...)
}

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

func NewLoggerHelper() *LoggerHelper {
	return &LoggerHelper{
		output: io.Discard,
		level:  zerolog.DebugLevel,
	}
}

func (lh *LoggerHelper) WithOutput(w io.Writer) *LoggerHelper {
	lh.output = w
	return lh
}

func (lh *LoggerHelper) WithLevel(level zerolog.Level) *LoggerHelper {
	lh.level = level
	return lh
}

func (lh *LoggerHelper) Logger() zerolog.Logger {
	return zerolog.New(lh.output).Level(lh.level)
}

func (lh *LoggerHelper) SilentLogger() zerolog.Logger {
	return zerolog.New(io.Discard).Level(zerolog.Disabled)
}

// AssertHelper provides comprehensive assertion utilities
type AssertHelper struct {
	t testing.TB
}

func NewAssertHelper(t testing.TB) *AssertHelper {
	t.Helper()
	return &AssertHelper{t: t}
}

func (ah *AssertHelper) RequireNonEmpty(s string, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.NotEmpty(ah.t, s, msgAndArgs...)
}

func (ah *AssertHelper) RequireValidTime(tm time.Time, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.False(ah.t, tm.IsZero(), "Time should not be zero")

	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)
	oneHourFromNow := now.Add(time.Hour)

	require.True(ah.t, tm.After(oneHourAgo) && tm.Before(oneHourFromNow),
		"Time %v should be within reasonable range", tm)
}

func (ah *AssertHelper) RequireJSON(jsonStr string, msgAndArgs ...interface{}) {
	ah.t.Helper()
	require.JSONEq(ah.t, jsonStr, jsonStr, msgAndArgs...)
}

// DataGenerator provides utilities for generating test data
type DataGenerator struct {
	timeHelper *TimeHelper
}

func NewDataGenerator() *DataGenerator {
	return &DataGenerator{
		timeHelper: NewTimeHelper(time.Date(2025, 6, 24, 12, 0, 0, 0, time.UTC)),
	}
}

func (dg *DataGenerator) SessionID() string {
	return "test-session-" + dg.timeHelper.Now().Format("20060102-150405")
}

func (dg *DataGenerator) ImageName() string {
	return "test/image:latest"
}

func (dg *DataGenerator) ErrorMessage() string {
	return "test error: " + dg.timeHelper.Now().Format(time.RFC3339)
}

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

func NewPerformanceHelper(t testing.TB) *PerformanceHelper {
	t.Helper()
	return &PerformanceHelper{t: t}
}

func (ph *PerformanceHelper) MeasureTime(fn func()) time.Duration {
	ph.t.Helper()
	start := time.Now()
	fn()
	return time.Since(start)
}

func (ph *PerformanceHelper) RequireUnderTimeout(timeout time.Duration, fn func()) {
	ph.t.Helper()
	duration := ph.MeasureTime(fn)
	require.True(ph.t, duration < timeout,
		"Function took %v, expected under %v", duration, timeout)
}

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

func (m *MockHealthChecker) GetSystemResources() types.SystemResources {
	if m.SystemResourcesFunc != nil {
		return m.SystemResourcesFunc()
	}
	return types.SystemResources{}
}

func (m *MockHealthChecker) GetSessionStats() types.SessionHealthStats {
	if m.SessionStatsFunc != nil {
		return m.SessionStatsFunc()
	}
	return types.SessionHealthStats{}
}

func (m *MockHealthChecker) GetCircuitBreakerStats() map[string]types.CircuitBreakerStatus {
	if m.CircuitBreakerStatsFunc != nil {
		return m.CircuitBreakerStatsFunc()
	}
	return make(map[string]types.CircuitBreakerStatus)
}

func (m *MockHealthChecker) CheckServiceHealth(ctx context.Context) []types.ServiceHealth {
	if m.CheckServiceHealthFunc != nil {
		return m.CheckServiceHealthFunc(ctx)
	}
	return []types.ServiceHealth{}
}

func (m *MockHealthChecker) GetJobQueueStats() types.JobQueueStats {
	if m.JobQueueStatsFunc != nil {
		return m.JobQueueStatsFunc()
	}
	return types.JobQueueStats{}
}

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

func NewMockProgressReporter(stages []types.ProgressStage) *MockProgressReporter {
	return &MockProgressReporter{
		stages:   stages,
		messages: make([]string, 0),
	}
}

func (m *MockProgressReporter) ReportStage(stageProgress float64, message string) {
	m.stageProgress = stageProgress
	m.messages = append(m.messages, message)
	if m.ReportStageFunc != nil {
		m.ReportStageFunc(stageProgress, message)
	}
}

func (m *MockProgressReporter) NextStage(message string) {
	if m.currentStage < len(m.stages)-1 {
		m.currentStage++
	}
	m.messages = append(m.messages, message)
	if m.NextStageFunc != nil {
		m.NextStageFunc(message)
	}
}

func (m *MockProgressReporter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(m.stages) {
		m.currentStage = stageIndex
	}
	m.messages = append(m.messages, message)
	if m.SetStageFunc != nil {
		m.SetStageFunc(stageIndex, message)
	}
}

func (m *MockProgressReporter) ReportOverall(progress float64, message string) {
	m.overallProgress = progress
	m.messages = append(m.messages, message)
	if m.ReportOverallFunc != nil {
		m.ReportOverallFunc(progress, message)
	}
}

func (m *MockProgressReporter) GetCurrentStage() (int, types.ProgressStage) {
	if m.currentStage < len(m.stages) {
		return m.currentStage, m.stages[m.currentStage]
	}
	return m.currentStage, types.ProgressStage{}
}

func (m *MockProgressReporter) GetMessages() []string {
	return m.messages
}

func (m *MockProgressReporter) GetStageProgress() float64 {
	return m.stageProgress
}

func (m *MockProgressReporter) GetOverallProgress() float64 {
	return m.overallProgress
}
