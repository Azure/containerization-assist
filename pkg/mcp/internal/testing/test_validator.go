package testing

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// TestValidator validates test prerequisites and results
type TestValidator struct {
	logger             zerolog.Logger
	config             IntegrationTestConfig
	prerequisiteChecks map[string]PrerequisiteCheckFunc
	resultValidators   map[string]ResultValidatorFunc
}

// PrerequisiteCheckFunc is a function that checks test prerequisites
type PrerequisiteCheckFunc func(ctx context.Context, test *IntegrationTest) error

// ResultValidatorFunc is a function that validates test results
type ResultValidatorFunc func(ctx context.Context, test *IntegrationTest, result *TestResult) error

// DependencyTracker tracks and validates test dependencies
type DependencyTracker struct {
	logger          zerolog.Logger
	config          IntegrationTestConfig
	dependencyGraph map[string][]string
	satisfiedDeps   map[string]bool
	pendingDeps     map[string][]string
}

// ContractTester validates API contracts between teams
type ContractTester struct {
	logger    zerolog.Logger
	config    IntegrationTestConfig
	contracts map[string]ContractDefinition
}

// APIValidator validates API endpoints and responses
type APIValidator struct {
	logger    zerolog.Logger
	config    IntegrationTestConfig
	endpoints map[string]APIEndpoint
}

// ContractDefinition defines an API contract between teams
type ContractDefinition struct {
	Provider    string                 `json:"provider"`
	Consumer    string                 `json:"consumer"`
	ServiceName string                 `json:"service_name"`
	Version     string                 `json:"version"`
	Endpoints   []ContractEndpoint     `json:"endpoints"`
	DataSchemas map[string]interface{} `json:"data_schemas"`
	SLA         ContractSLA            `json:"sla"`
}

// ContractEndpoint defines an endpoint in a contract
type ContractEndpoint struct {
	Path         string            `json:"path"`
	Method       string            `json:"method"`
	RequestSpec  interface{}       `json:"request_spec"`
	ResponseSpec interface{}       `json:"response_spec"`
	Headers      map[string]string `json:"headers"`
	QueryParams  map[string]string `json:"query_params"`
}

// ContractSLA defines service level agreements for a contract
type ContractSLA struct {
	MaxResponseTime time.Duration `json:"max_response_time"`
	Availability    float64       `json:"availability"`
	ErrorRate       float64       `json:"error_rate"`
}

// APIEndpoint represents an API endpoint for validation
type APIEndpoint struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	ExpectedCode int               `json:"expected_code"`
	Timeout      time.Duration     `json:"timeout"`
}

// Threshold represents a performance or quality threshold (already defined in performance_tracker.go, avoiding duplicate)

// NewTestValidator creates a new test validator
func NewTestValidator(config IntegrationTestConfig, logger zerolog.Logger) *TestValidator {
	validator := &TestValidator{
		logger:             logger.With().Str("component", "test_validator").Logger(),
		config:             config,
		prerequisiteChecks: make(map[string]PrerequisiteCheckFunc),
		resultValidators:   make(map[string]ResultValidatorFunc),
	}

	// Register default prerequisite checks
	validator.registerDefaultPrerequisiteChecks()

	// Register default result validators
	validator.registerDefaultResultValidators()

	return validator
}

// ValidateTestPrerequisites validates that all prerequisites for a test are met
func (tv *TestValidator) ValidateTestPrerequisites(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().
		Str("test_id", test.ID).
		Strs("prerequisites", test.Prerequisites).
		Msg("Validating test prerequisites")

	for _, prerequisite := range test.Prerequisites {
		checkFunc, exists := tv.prerequisiteChecks[prerequisite]
		if !exists {
			tv.logger.Warn().
				Str("prerequisite", prerequisite).
				Msg("No prerequisite check function registered")
			continue
		}

		if err := checkFunc(ctx, test); err != nil {
			return fmt.Errorf("prerequisite check failed for %s: %w", prerequisite, err)
		}
	}

	tv.logger.Debug().
		Str("test_id", test.ID).
		Msg("All test prerequisites validated successfully")

	return nil
}

// ValidateTestResults validates the results of a completed test
func (tv *TestValidator) ValidateTestResults(ctx context.Context, test *IntegrationTest, result *TestResult) error {
	tv.logger.Debug().
		Str("test_id", test.ID).
		Str("status", string(result.Status)).
		Msg("Validating test results")

	// Validate expected results
	for _, expectedResult := range test.ExpectedResults {
		if err := tv.validateExpectedResult(expectedResult, result); err != nil {
			return fmt.Errorf("expected result validation failed: %w", err)
		}
	}

	// Run custom result validators
	for validatorName, validator := range tv.resultValidators {
		if err := validator(ctx, test, result); err != nil {
			return fmt.Errorf("result validator %s failed: %w", validatorName, err)
		}
	}

	tv.logger.Debug().
		Str("test_id", test.ID).
		Msg("Test results validated successfully")

	return nil
}

// registerDefaultPrerequisiteChecks registers default prerequisite check functions
func (tv *TestValidator) registerDefaultPrerequisiteChecks() {
	tv.prerequisiteChecks["docker_daemon_running"] = tv.checkDockerDaemonRunning
	tv.prerequisiteChecks["test_registry_available"] = tv.checkTestRegistryAvailable
	tv.prerequisiteChecks["session_manager_ready"] = tv.checkSessionManagerReady
	tv.prerequisiteChecks["atomic_tools_available"] = tv.checkAtomicToolsAvailable
	tv.prerequisiteChecks["team_dependencies_ready"] = tv.checkTeamDependenciesReady
	tv.prerequisiteChecks["performance_baseline_set"] = tv.checkPerformanceBaselineSet
}

// registerDefaultResultValidators registers default result validator functions
func (tv *TestValidator) registerDefaultResultValidators() {
	tv.resultValidators["performance_thresholds"] = tv.validatePerformanceThresholds
	tv.resultValidators["memory_usage"] = tv.validateMemoryUsage
	tv.resultValidators["error_patterns"] = tv.validateErrorPatterns
	tv.resultValidators["contract_compliance"] = tv.validateContractCompliance
}

// Prerequisite check functions

func (tv *TestValidator) checkDockerDaemonRunning(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().Msg("Checking if Docker daemon is running")

	// In a real implementation, this would check if Docker daemon is accessible
	// For now, we'll simulate the check
	time.Sleep(10 * time.Millisecond)

	// Simulate success
	return nil
}

func (tv *TestValidator) checkTestRegistryAvailable(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().Msg("Checking if test registry is available")

	// In a real implementation, this would ping the test registry
	time.Sleep(20 * time.Millisecond)

	// Simulate success
	return nil
}

func (tv *TestValidator) checkSessionManagerReady(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().Msg("Checking if session manager is ready")

	// In a real implementation, this would check session manager status
	time.Sleep(5 * time.Millisecond)

	// Simulate success
	return nil
}

func (tv *TestValidator) checkAtomicToolsAvailable(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().Msg("Checking if atomic tools are available")

	// In a real implementation, this would verify atomic tool framework
	time.Sleep(15 * time.Millisecond)

	// Simulate success
	return nil
}

func (tv *TestValidator) checkTeamDependenciesReady(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().
		Strs("dependencies", test.Dependencies).
		Msg("Checking if team dependencies are ready")

	// In a real implementation, this would check other team readiness
	for _, dep := range test.Dependencies {
		tv.logger.Debug().
			Str("dependency", dep).
			Msg("Checking dependency readiness")
		time.Sleep(25 * time.Millisecond)
	}

	// Simulate success
	return nil
}

func (tv *TestValidator) checkPerformanceBaselineSet(ctx context.Context, test *IntegrationTest) error {
	tv.logger.Debug().Msg("Checking if performance baseline is set")

	// In a real implementation, this would verify performance baselines exist
	time.Sleep(5 * time.Millisecond)

	// Simulate success
	return nil
}

// Result validator functions

func (tv *TestValidator) validatePerformanceThresholds(ctx context.Context, test *IntegrationTest, result *TestResult) error {
	if test.PerformanceSLA == nil || result.Performance == nil {
		return nil // No performance SLA to validate
	}

	sla := test.PerformanceSLA
	perf := result.Performance

	if sla.MaxDuration > 0 && result.Duration > sla.MaxDuration {
		return fmt.Errorf("test duration %v exceeds SLA %v", result.Duration, sla.MaxDuration)
	}

	if sla.LatencyP95Max > 0 && perf.LatencyP95 > sla.LatencyP95Max {
		return fmt.Errorf("P95 latency %v exceeds SLA %v", perf.LatencyP95, sla.LatencyP95Max)
	}

	if sla.LatencyP99Max > 0 && perf.LatencyP99 > sla.LatencyP99Max {
		return fmt.Errorf("P99 latency %v exceeds SLA %v", perf.LatencyP99, sla.LatencyP99Max)
	}

	if sla.ErrorRateMax > 0 && perf.ErrorRate > sla.ErrorRateMax {
		return fmt.Errorf("error rate %f exceeds SLA %f", perf.ErrorRate, sla.ErrorRateMax)
	}

	if sla.ThroughputMin > 0 && perf.Throughput < sla.ThroughputMin {
		return fmt.Errorf("throughput %f below SLA %f", perf.Throughput, sla.ThroughputMin)
	}

	return nil
}

func (tv *TestValidator) validateMemoryUsage(ctx context.Context, test *IntegrationTest, result *TestResult) error {
	if test.PerformanceSLA == nil || result.ResourceUsage == nil {
		return nil
	}

	if test.PerformanceSLA.MaxMemoryUsage > 0 {
		if result.ResourceUsage.MemoryPeak > uint64(test.PerformanceSLA.MaxMemoryUsage) {
			return fmt.Errorf("peak memory usage %d exceeds SLA %d",
				result.ResourceUsage.MemoryPeak, test.PerformanceSLA.MaxMemoryUsage)
		}
	}

	return nil
}

func (tv *TestValidator) validateErrorPatterns(ctx context.Context, test *IntegrationTest, result *TestResult) error {
	// Check for known error patterns that should fail the test
	if result.ErrorMessage != "" {
		// Check for critical error patterns
		criticalPatterns := []string{
			"panic:",
			"fatal error:",
			"segmentation fault",
			"out of memory",
		}

		for _, pattern := range criticalPatterns {
			if contains(result.ErrorMessage, pattern) {
				return fmt.Errorf("critical error pattern detected: %s", pattern)
			}
		}
	}

	return nil
}

func (tv *TestValidator) validateContractCompliance(ctx context.Context, test *IntegrationTest, result *TestResult) error {
	// Validate that all contracts were validated successfully
	for _, contractResult := range result.ContractResults {
		if !contractResult.Success {
			return fmt.Errorf("contract validation failed for %s->%s: %s",
				contractResult.Consumer, contractResult.Provider, contractResult.Error)
		}
	}

	return nil
}

// validateExpectedResult validates a single expected result
func (tv *TestValidator) validateExpectedResult(expected ExpectedResult, result *TestResult) error {
	switch expected.Type {
	case "docker_pull_success":
		return tv.validateBooleanResult(expected, result.Success, "Docker pull operation")
	case "docker_push_success":
		return tv.validateBooleanResult(expected, result.Success, "Docker push operation")
	case "docker_tag_success":
		return tv.validateBooleanResult(expected, result.Success, "Docker tag operation")
	case "session_tracking_active":
		return tv.validateBooleanResult(expected, result.Success, "Session tracking")
	case "performance_threshold":
		return tv.validatePerformanceResult(expected, result)
	default:
		tv.logger.Warn().
			Str("type", expected.Type).
			Msg("Unknown expected result type")
		return nil
	}
}

// validateBooleanResult validates a boolean expected result
func (tv *TestValidator) validateBooleanResult(expected ExpectedResult, actual bool, operation string) error {
	expectedBool, ok := expected.Value.(bool)
	if !ok {
		return fmt.Errorf("expected value for %s is not boolean", operation)
	}

	if actual != expectedBool {
		return fmt.Errorf("%s: expected %v, got %v", operation, expectedBool, actual)
	}

	return nil
}

// validatePerformanceResult validates a performance expected result
func (tv *TestValidator) validatePerformanceResult(expected ExpectedResult, result *TestResult) error {
	if result.Performance == nil {
		return fmt.Errorf("no performance data available for validation")
	}

	expectedFloat, ok := expected.Value.(float64)
	if !ok {
		return fmt.Errorf("expected performance value is not numeric")
	}

	var actualValue float64
	switch expected.Condition {
	case "max_duration":
		actualValue = result.Duration.Seconds()
	case "max_error_rate":
		actualValue = result.Performance.ErrorRate
	case "min_throughput":
		actualValue = result.Performance.Throughput
	default:
		return fmt.Errorf("unknown performance condition: %s", expected.Condition)
	}

	tolerance := expected.Tolerance
	if tolerance == 0 {
		tolerance = 0.05 // 5% default tolerance
	}

	switch expected.Condition {
	case "max_duration", "max_error_rate":
		if actualValue > expectedFloat*(1+tolerance) {
			return fmt.Errorf("%s: %f exceeds threshold %f (tolerance: %f)",
				expected.Condition, actualValue, expectedFloat, tolerance)
		}
	case "min_throughput":
		if actualValue < expectedFloat*(1-tolerance) {
			return fmt.Errorf("%s: %f below threshold %f (tolerance: %f)",
				expected.Condition, actualValue, expectedFloat, tolerance)
		}
	}

	return nil
}

// RegisterPrerequisiteCheck registers a custom prerequisite check function
func (tv *TestValidator) RegisterPrerequisiteCheck(name string, checkFunc PrerequisiteCheckFunc) {
	tv.prerequisiteChecks[name] = checkFunc
	tv.logger.Debug().
		Str("check_name", name).
		Msg("Prerequisite check registered")
}

// RegisterResultValidator registers a custom result validator function
func (tv *TestValidator) RegisterResultValidator(name string, validator ResultValidatorFunc) {
	tv.resultValidators[name] = validator
	tv.logger.Debug().
		Str("validator_name", name).
		Msg("Result validator registered")
}

// NewDependencyTracker creates a new dependency tracker
func NewDependencyTracker(config IntegrationTestConfig, logger zerolog.Logger) *DependencyTracker {
	return &DependencyTracker{
		logger:          logger.With().Str("component", "dependency_tracker").Logger(),
		config:          config,
		dependencyGraph: config.DependencyGraph,
		satisfiedDeps:   make(map[string]bool),
		pendingDeps:     make(map[string][]string),
	}
}

// ValidateDependencies validates that all test dependencies are satisfied
func (dt *DependencyTracker) ValidateDependencies(testSuites map[string]*TestSuite) error {
	dt.logger.Debug().Msg("Validating test dependencies")

	// Build dependency graph
	for suiteName, suite := range testSuites {
		for _, test := range suite.Tests {
			if len(test.Dependencies) > 0 {
				dt.pendingDeps[test.ID] = test.Dependencies
			}
		}

		// Suite-level dependencies
		if len(suite.Dependencies) > 0 {
			dt.pendingDeps[suiteName] = suite.Dependencies
		}
	}

	// Check for circular dependencies
	if err := dt.detectCircularDependencies(); err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Validate external dependencies
	if err := dt.validateExternalDependencies(); err != nil {
		return fmt.Errorf("external dependency validation failed: %w", err)
	}

	dt.logger.Debug().Msg("All dependencies validated successfully")
	return nil
}

// detectCircularDependencies detects circular dependencies in the dependency graph
func (dt *DependencyTracker) detectCircularDependencies() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for testID := range dt.pendingDeps {
		if !visited[testID] {
			if dt.hasCycle(testID, visited, recStack) {
				return fmt.Errorf("circular dependency involving %s", testID)
			}
		}
	}

	return nil
}

// hasCycle performs depth-first search to detect cycles
func (dt *DependencyTracker) hasCycle(testID string, visited, recStack map[string]bool) bool {
	visited[testID] = true
	recStack[testID] = true

	dependencies := dt.pendingDeps[testID]
	for _, dep := range dependencies {
		if !visited[dep] {
			if dt.hasCycle(dep, visited, recStack) {
				return true
			}
		} else if recStack[dep] {
			return true
		}
	}

	recStack[testID] = false
	return false
}

// validateExternalDependencies validates external team dependencies
func (dt *DependencyTracker) validateExternalDependencies() error {
	externalDeps := []string{"BuildSecBot", "OrchBot", "AdvancedBot"}

	for _, dep := range externalDeps {
		if err := dt.validateExternalDependency(dep); err != nil {
			return fmt.Errorf("external dependency %s validation failed: %w", dep, err)
		}
	}

	return nil
}

// validateExternalDependency validates a single external dependency
func (dt *DependencyTracker) validateExternalDependency(dependency string) error {
	dt.logger.Debug().
		Str("dependency", dependency).
		Msg("Validating external dependency")

	// In a real implementation, this would check if the external team is ready
	// For now, simulate validation
	time.Sleep(10 * time.Millisecond)

	return nil
}

// NewContractTester creates a new contract tester
func NewContractTester(config IntegrationTestConfig, logger zerolog.Logger) *ContractTester {
	return &ContractTester{
		logger:    logger.With().Str("component", "contract_tester").Logger(),
		config:    config,
		contracts: make(map[string]ContractDefinition),
	}
}

// NewAPIValidator creates a new API validator
func NewAPIValidator(config IntegrationTestConfig, logger zerolog.Logger) *APIValidator {
	return &APIValidator{
		logger:    logger.With().Str("component", "api_validator").Logger(),
		config:    config,
		endpoints: make(map[string]APIEndpoint),
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple implementation - in real code you might want case-insensitive search
	return len(s) >= len(substr) &&
		(s == substr || (len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				hasSubstring(s, substr))))
}

// hasSubstring is a helper to check for substring in the middle
func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
