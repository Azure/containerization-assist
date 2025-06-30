package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// TestRunner orchestrates the execution of test suites and manages test scheduling
type TestRunner struct {
	logger    zerolog.Logger
	config    IntegrationTestConfig
	executor  *TestExecutor
	scheduler *TestScheduler

	// State management
	runningTests   map[string]*RunningTest
	completedTests map[string]*TestResult
	mutex          sync.RWMutex

	// Coordination
	coordinator *TeamCoordinator
	validator   *TestValidator
}

// RunningTest tracks the state of a currently executing test
type RunningTest struct {
	TestID     string
	SuiteName  string
	StartTime  time.Time
	Status     TestStatus
	Context    context.Context
	CancelFunc context.CancelFunc
}

// TestScheduler manages test execution scheduling and dependencies
type TestScheduler struct {
	logger          zerolog.Logger
	dependencyGraph map[string][]string
	readyTests      chan *ScheduledTest
	waitingTests    map[string]*ScheduledTest
	completedTests  map[string]bool
	mutex           sync.RWMutex
}

// ScheduledTest represents a test ready for execution
type ScheduledTest struct {
	Test              *IntegrationTest
	SuiteName         string
	Dependencies      []string
	Priority          TestPriority
	EstimatedDuration time.Duration
}

// NewTestRunner creates a new test runner
func NewTestRunner(config IntegrationTestConfig, logger zerolog.Logger) *TestRunner {
	return &TestRunner{
		logger:         logger.With().Str("component", "test_runner").Logger(),
		config:         config,
		executor:       NewTestExecutor(config, logger),
		scheduler:      NewTestScheduler(config, logger),
		runningTests:   make(map[string]*RunningTest),
		completedTests: make(map[string]*TestResult),
		coordinator:    NewTeamCoordinator(config, logger),
		validator:      NewTestValidator(config, logger),
	}
}

// ExecuteTestSuite executes all tests in a test suite
func (tr *TestRunner) ExecuteTestSuite(ctx context.Context, suite *TestSuite) (*SuiteResult, error) {
	tr.logger.Info().
		Str("suite_name", suite.Name).
		Int("test_count", len(suite.Tests)).
		Msg("Starting test suite execution")

	result := &SuiteResult{
		SuiteName:   suite.Name,
		StartTime:   time.Now(),
		Status:      TestStatusRunning,
		TestResults: make([]*TestResult, 0, len(suite.Tests)),
	}

	// Run suite setup if provided
	if suite.Setup != nil {
		if err := suite.Setup(ctx); err != nil {
			result.Status = TestStatusFailed
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, fmt.Errorf("suite setup failed: %w", err)
		}
	}

	// Schedule tests based on dependencies and configuration
	if err := tr.scheduler.ScheduleTests(suite.Tests, suite.Name); err != nil {
		result.Status = TestStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("test scheduling failed: %w", err)
	}

	// Execute tests
	if tr.config.ParallelExecution {
		if err := tr.executeTestsParallel(ctx, suite, result); err != nil {
			return result, err
		}
	} else {
		if err := tr.executeTestsSequential(ctx, suite, result); err != nil {
			return result, err
		}
	}

	// Run suite teardown if provided
	if suite.Teardown != nil {
		if err := suite.Teardown(ctx); err != nil {
			tr.logger.Warn().
				Err(err).
				Str("suite_name", suite.Name).
				Msg("Suite teardown failed")
		}
	}

	// Finalize results
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Determine overall status
	allPassed := true
	for _, testResult := range result.TestResults {
		if !testResult.Success {
			allPassed = false
			break
		}
	}

	if allPassed {
		result.Status = TestStatusPassed
	} else {
		result.Status = TestStatusFailed
	}

	// Generate performance summary
	result.Performance = tr.calculateSuitePerformance(result.TestResults)
	result.Summary = tr.generateSuiteSummary(result)

	tr.logger.Info().
		Str("suite_name", suite.Name).
		Str("status", string(result.Status)).
		Dur("duration", result.Duration).
		Int("total_tests", len(result.TestResults)).
		Msg("Test suite execution completed")

	return result, nil
}

// executeTestsParallel executes tests in parallel with dependency management
func (tr *TestRunner) executeTestsParallel(ctx context.Context, suite *TestSuite, result *SuiteResult) error {
	var wg sync.WaitGroup
	resultsChan := make(chan *TestResult, len(suite.Tests))
	errorsChan := make(chan error, len(suite.Tests))

	// Start test execution workers
	for i := 0; i < tr.getMaxParallelTests(); i++ {
		wg.Add(1)
		go tr.testExecutionWorker(ctx, &wg, resultsChan, errorsChan)
	}

	// Wait for all tests to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	var errors []error
	for {
		select {
		case testResult, ok := <-resultsChan:
			if !ok {
				resultsChan = nil
			} else {
				result.TestResults = append(result.TestResults, testResult)
				tr.mutex.Lock()
				tr.completedTests[testResult.TestID] = testResult
				tr.mutex.Unlock()
			}
		case err, ok := <-errorsChan:
			if !ok {
				errorsChan = nil
			} else {
				errors = append(errors, err)
			}
		}

		if resultsChan == nil && errorsChan == nil {
			break
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("test execution errors: %v", errors)
	}

	return nil
}

// executeTestsSequential executes tests sequentially
func (tr *TestRunner) executeTestsSequential(ctx context.Context, suite *TestSuite, result *SuiteResult) error {
	for tr.scheduler.HasPendingTests() {
		scheduledTest := tr.scheduler.GetNextTest()
		if scheduledTest == nil {
			// Wait for dependencies
			time.Sleep(100 * time.Millisecond)
			continue
		}

		testResult, err := tr.executeScheduledTest(ctx, scheduledTest)
		if err != nil {
			tr.logger.Error().
				Err(err).
				Str("test_id", scheduledTest.Test.ID).
				Msg("Test execution failed")
			return err
		}

		result.TestResults = append(result.TestResults, testResult)
		tr.scheduler.MarkTestCompleted(scheduledTest.Test.ID, testResult.Success)

		tr.mutex.Lock()
		tr.completedTests[testResult.TestID] = testResult
		tr.mutex.Unlock()
	}

	return nil
}

// testExecutionWorker is a worker goroutine that executes tests from the scheduler
func (tr *TestRunner) testExecutionWorker(ctx context.Context, wg *sync.WaitGroup, resultsChan chan<- *TestResult, errorsChan chan<- error) {
	defer wg.Done()

	for {
		scheduledTest := tr.scheduler.GetNextTest()
		if scheduledTest == nil {
			// Check if there are more tests to wait for
			if !tr.scheduler.HasPendingTests() {
				return
			}
			// Wait a bit for dependencies
			time.Sleep(50 * time.Millisecond)
			continue
		}

		testResult, err := tr.executeScheduledTest(ctx, scheduledTest)
		if err != nil {
			errorsChan <- err
			continue
		}

		resultsChan <- testResult
		tr.scheduler.MarkTestCompleted(scheduledTest.Test.ID, testResult.Success)
	}
}

// executeScheduledTest executes a scheduled test
func (tr *TestRunner) executeScheduledTest(ctx context.Context, scheduledTest *ScheduledTest) (*TestResult, error) {
	test := scheduledTest.Test
	suiteName := scheduledTest.SuiteName

	// Create test context
	testCtx, cancel := context.WithTimeout(ctx, test.Timeout)
	defer cancel()

	// Track running test
	runningTest := &RunningTest{
		TestID:     test.ID,
		SuiteName:  suiteName,
		StartTime:  time.Now(),
		Status:     TestStatusRunning,
		Context:    testCtx,
		CancelFunc: cancel,
	}

	tr.mutex.Lock()
	tr.runningTests[test.ID] = runningTest
	tr.mutex.Unlock()

	defer func() {
		tr.mutex.Lock()
		delete(tr.runningTests, test.ID)
		tr.mutex.Unlock()
	}()

	// Pre-execution validation
	if err := tr.validator.ValidateTestPrerequisites(testCtx, test); err != nil {
		return nil, fmt.Errorf("test prerequisites validation failed: %w", err)
	}

	// Coordinate with other teams if needed
	if len(test.Dependencies) > 0 {
		if err := tr.coordinator.CoordinateTestExecution(testCtx, test); err != nil {
			return nil, fmt.Errorf("team coordination failed: %w", err)
		}
	}

	// Execute the test
	testResult, err := tr.executor.ExecuteTest(testCtx, test, suiteName)
	if err != nil {
		return nil, fmt.Errorf("test execution failed: %w", err)
	}

	// Post-execution validation
	if err := tr.validator.ValidateTestResults(testCtx, test, testResult); err != nil {
		testResult.Success = false
		testResult.Status = TestStatusFailed
		testResult.FailureReason = "Post-execution validation failed"
		testResult.ErrorMessage = err.Error()
	}

	return testResult, nil
}

// getMaxParallelTests returns the maximum number of tests to run in parallel
func (tr *TestRunner) getMaxParallelTests() int {
	// Default to number of CPU cores, but can be configured
	return 4 // Placeholder
}

// calculateSuitePerformance calculates performance metrics for the entire suite
func (tr *TestRunner) calculateSuitePerformance(testResults []*TestResult) *PerformanceMetrics {
	if len(testResults) == 0 {
		return &PerformanceMetrics{}
	}

	var totalDuration time.Duration
	var totalErrors int
	var latencies []time.Duration

	for _, result := range testResults {
		totalDuration += result.Duration
		if !result.Success {
			totalErrors++
		}
		if result.Performance != nil {
			latencies = append(latencies, result.Performance.LatencyP95)
		}
	}

	avgDuration := totalDuration / time.Duration(len(testResults))
	errorRate := float64(totalErrors) / float64(len(testResults))

	// Calculate throughput as tests per second
	throughput := float64(len(testResults)) / totalDuration.Seconds()

	return &PerformanceMetrics{
		Duration:   totalDuration,
		Throughput: throughput,
		ErrorRate:  errorRate,
		LatencyP95: avgDuration, // Simplified calculation
	}
}

// generateSuiteSummary generates a human-readable summary of the test suite results
func (tr *TestRunner) generateSuiteSummary(result *SuiteResult) string {
	passed := 0
	failed := 0
	skipped := 0

	for _, testResult := range result.TestResults {
		switch testResult.Status {
		case TestStatusPassed:
			passed++
		case TestStatusFailed:
			failed++
		case TestStatusSkipped:
			skipped++
		}
	}

	return fmt.Sprintf("Test suite completed: %d passed, %d failed, %d skipped in %v",
		passed, failed, skipped, result.Duration)
}

// GetRunningTests returns information about currently running tests
func (tr *TestRunner) GetRunningTests() map[string]*RunningTest {
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()

	// Return a copy to avoid data races
	running := make(map[string]*RunningTest)
	for id, test := range tr.runningTests {
		testCopy := *test
		running[id] = &testCopy
	}

	return running
}

// CancelTest cancels a running test
func (tr *TestRunner) CancelTest(testID string) error {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	runningTest, exists := tr.runningTests[testID]
	if !exists {
		return fmt.Errorf("test %s is not currently running", testID)
	}

	runningTest.CancelFunc()
	runningTest.Status = TestStatusTimedOut

	tr.logger.Info().
		Str("test_id", testID).
		Msg("Test cancelled")

	return nil
}

// Cleanup cleans up runner resources
func (tr *TestRunner) Cleanup(ctx context.Context) error {
	tr.logger.Info().Msg("Cleaning up test runner")

	// Cancel all running tests
	tr.mutex.Lock()
	for testID, runningTest := range tr.runningTests {
		runningTest.CancelFunc()
		tr.logger.Info().
			Str("test_id", testID).
			Msg("Cancelled running test during cleanup")
	}
	tr.runningTests = make(map[string]*RunningTest)
	tr.mutex.Unlock()

	// Cleanup components
	if err := tr.executor.Cleanup(ctx); err != nil {
		return fmt.Errorf("executor cleanup failed: %w", err)
	}

	if err := tr.coordinator.Cleanup(ctx); err != nil {
		return fmt.Errorf("coordinator cleanup failed: %w", err)
	}

	return nil
}

// NewTestScheduler creates a new test scheduler
func NewTestScheduler(config IntegrationTestConfig, logger zerolog.Logger) *TestScheduler {
	return &TestScheduler{
		logger:          logger.With().Str("component", "test_scheduler").Logger(),
		dependencyGraph: config.DependencyGraph,
		readyTests:      make(chan *ScheduledTest, 100),
		waitingTests:    make(map[string]*ScheduledTest),
		completedTests:  make(map[string]bool),
	}
}

// ScheduleTests schedules tests for execution based on dependencies
func (ts *TestScheduler) ScheduleTests(tests []*IntegrationTest, suiteName string) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// Clear previous state
	ts.waitingTests = make(map[string]*ScheduledTest)
	ts.completedTests = make(map[string]bool)

	// Create scheduled tests
	for _, test := range tests {
		scheduledTest := &ScheduledTest{
			Test:              test,
			SuiteName:         suiteName,
			Dependencies:      test.Dependencies,
			Priority:          TestPriority(test.Tags[0]), // Simplified priority assignment
			EstimatedDuration: test.Timeout,
		}

		if len(test.Dependencies) == 0 {
			// No dependencies, ready to run
			select {
			case ts.readyTests <- scheduledTest:
			default:
				return fmt.Errorf("ready tests channel full")
			}
		} else {
			// Has dependencies, wait for them
			ts.waitingTests[test.ID] = scheduledTest
		}
	}

	ts.logger.Debug().
		Int("ready_tests", len(ts.readyTests)).
		Int("waiting_tests", len(ts.waitingTests)).
		Msg("Tests scheduled")

	return nil
}

// GetNextTest returns the next test ready for execution
func (ts *TestScheduler) GetNextTest() *ScheduledTest {
	select {
	case test := <-ts.readyTests:
		return test
	default:
		return nil
	}
}

// MarkTestCompleted marks a test as completed and checks if waiting tests can now run
func (ts *TestScheduler) MarkTestCompleted(testID string, success bool) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.completedTests[testID] = success

	// Check if any waiting tests can now run
	for waitingTestID, scheduledTest := range ts.waitingTests {
		if ts.areDependenciesSatisfied(scheduledTest.Dependencies) {
			// Move to ready queue
			select {
			case ts.readyTests <- scheduledTest:
				delete(ts.waitingTests, waitingTestID)
			default:
				ts.logger.Warn().
					Str("test_id", waitingTestID).
					Msg("Could not queue ready test - channel full")
			}
		}
	}
}

// HasPendingTests returns true if there are tests still pending execution
func (ts *TestScheduler) HasPendingTests() bool {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	return len(ts.waitingTests) > 0 || len(ts.readyTests) > 0
}

// areDependenciesSatisfied checks if all dependencies for a test are satisfied
func (ts *TestScheduler) areDependenciesSatisfied(dependencies []string) bool {
	for _, dep := range dependencies {
		if completed, exists := ts.completedTests[dep]; !exists || !completed {
			return false
		}
	}
	return true
}

// MockConfig represents configuration for mock services
type MockConfig struct {
	Type     string                 `json:"type"`
	Endpoint string                 `json:"endpoint"`
	Config   map[string]interface{} `json:"config"`
}
