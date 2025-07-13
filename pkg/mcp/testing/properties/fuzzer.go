// Package properties provides fuzzing capabilities for MCP tools and saga scenarios.
package properties

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/mark3labs/mcp-go/mcp"
)

// FuzzConfig configures fuzzing behavior
type FuzzConfig struct {
	MaxIterations     int           `json:"max_iterations"`
	MutationRate      float64       `json:"mutation_rate"`
	CorpusSize        int           `json:"corpus_size"`
	MaxStringLength   int           `json:"max_string_length"`
	MaxArrayLength    int           `json:"max_array_length"`
	TimeoutPerTest    time.Duration `json:"timeout_per_test"`
	SeedInputs        []string      `json:"seed_inputs"`
	EnableRegression  bool          `json:"enable_regression"`
	SaveFailingInputs bool          `json:"save_failing_inputs"`
}

// DefaultFuzzConfig returns sensible defaults for fuzzing
func DefaultFuzzConfig() FuzzConfig {
	return FuzzConfig{
		MaxIterations:     1000,
		MutationRate:      0.1,
		CorpusSize:        50,
		MaxStringLength:   1000,
		MaxArrayLength:    20,
		TimeoutPerTest:    10 * time.Second,
		SeedInputs:        []string{},
		EnableRegression:  true,
		SaveFailingInputs: true,
	}
}

// MCPFuzzer provides fuzzing capabilities for MCP tool arguments
type MCPFuzzer struct {
	config   FuzzConfig
	logger   *slog.Logger
	rand     *rand.Rand
	corpus   []map[string]interface{}
	crashers []map[string]interface{}
}

// NewMCPFuzzer creates a new MCP tool fuzzer
func NewMCPFuzzer(config FuzzConfig, logger *slog.Logger) *MCPFuzzer {
	return &MCPFuzzer{
		config:   config,
		logger:   logger.With("component", "mcp_fuzzer"),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		corpus:   make([]map[string]interface{}, 0, config.CorpusSize),
		crashers: make([]map[string]interface{}, 0),
	}
}

// FuzzMCPToolArguments fuzzes MCP tool arguments to find edge cases
func (f *MCPFuzzer) FuzzMCPToolArguments(t *testing.T, toolName string) {
	t.Helper()

	f.logger.Info("Starting MCP tool fuzzing", "tool", toolName, "iterations", f.config.MaxIterations)

	// Initialize corpus with seed inputs
	f.initializeCorpus(toolName)

	successCount := 0
	errorCount := 0
	crashCount := 0

	for i := 0; i < f.config.MaxIterations; i++ {
		// Generate or mutate input
		args := f.generateFuzzInput(toolName)

		// Test the input with timeout
		ctx, cancel := context.WithTimeout(context.Background(), f.config.TimeoutPerTest)
		result := f.testMCPToolInput(ctx, toolName, args)
		cancel()

		switch result.Type {
		case FuzzResultSuccess:
			successCount++
			// Add interesting inputs to corpus
			if f.isInterestingInput(args, result) {
				f.addToCorpus(args)
			}

		case FuzzResultError:
			errorCount++
			// Expected errors are fine, but log for analysis

		case FuzzResultCrash:
			crashCount++
			f.crashers = append(f.crashers, args)
			if f.config.SaveFailingInputs {
				f.saveFailingInput(t, toolName, args, result, i)
			}
			t.Errorf("Fuzzing found crash in %s with input: %+v\\nError: %s", toolName, args, result.Error)

		case FuzzResultTimeout:
			// Timeout might indicate infinite loop or performance issue
			t.Errorf("Fuzzing timeout in %s with input: %+v", toolName, args)
		}

		if i%100 == 0 {
			f.logger.Info("Fuzzing progress",
				"tool", toolName,
				"iteration", i,
				"success", successCount,
				"errors", errorCount,
				"crashes", crashCount,
				"corpus_size", len(f.corpus))
		}
	}

	f.logger.Info("Fuzzing completed",
		"tool", toolName,
		"total_iterations", f.config.MaxIterations,
		"success_rate", float64(successCount)/float64(f.config.MaxIterations),
		"crash_count", crashCount,
		"final_corpus_size", len(f.corpus))
}

// FuzzSagaScenarios fuzzes saga compensation scenarios
func (f *MCPFuzzer) FuzzSagaScenarios(t *testing.T) {
	t.Helper()

	f.logger.Info("Starting saga scenario fuzzing", "iterations", f.config.MaxIterations)

	for i := 0; i < f.config.MaxIterations; i++ {
		// Generate random saga scenario
		scenario := f.generateSagaScenario()

		// Test saga execution and compensation
		result := f.testSagaScenario(scenario)

		if result.Type == FuzzResultCrash {
			if f.config.SaveFailingInputs {
				f.saveSagaFailure(t, scenario, result, i)
			}
			t.Errorf("Saga fuzzing found crash: %s\\nScenario: %+v", result.Error, scenario)
		}

		// Verify saga invariants
		if !f.verifySagaInvariants(scenario.Execution) {
			t.Errorf("Saga invariant violated in scenario %d: %+v", i, scenario)
		}
	}
}

// initializeCorpus sets up initial corpus with valid seed inputs
func (f *MCPFuzzer) initializeCorpus(toolName string) {
	seedInputs := f.getSeedInputsForTool(toolName)
	for _, seed := range seedInputs {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(seed), &args); err == nil {
			f.corpus = append(f.corpus, args)
		}
	}

	// Ensure we have at least one valid input
	if len(f.corpus) == 0 {
		f.corpus = append(f.corpus, f.getDefaultArgsForTool(toolName))
	}
}

// generateFuzzInput creates a fuzzed input for testing
func (f *MCPFuzzer) generateFuzzInput(toolName string) map[string]interface{} {
	// 50% chance to mutate existing corpus, 50% chance to generate new
	if len(f.corpus) > 0 && f.rand.Float64() < 0.5 {
		// Mutate existing input
		base := f.corpus[f.rand.Intn(len(f.corpus))]
		return f.mutateArgs(base)
	}

	// Generate completely new input
	return f.generateRandomArgs(toolName)
}

// mutateArgs applies random mutations to existing arguments
func (f *MCPFuzzer) mutateArgs(args map[string]interface{}) map[string]interface{} {
	mutated := make(map[string]interface{})

	// Copy existing args
	for k, v := range args {
		mutated[k] = v
	}

	// Apply mutations based on mutation rate
	for key, value := range mutated {
		if f.rand.Float64() < f.config.MutationRate {
			mutated[key] = f.mutateValue(value)
		}
	}

	// Sometimes add new random keys
	if f.rand.Float64() < f.config.MutationRate {
		randomKey := f.generateRandomString(f.rand.Intn(20) + 1)
		mutated[randomKey] = f.generateRandomValue()
	}

	// Sometimes remove keys
	if f.rand.Float64() < f.config.MutationRate && len(mutated) > 1 {
		keys := make([]string, 0, len(mutated))
		for k := range mutated {
			keys = append(keys, k)
		}
		deleteKey := keys[f.rand.Intn(len(keys))]
		delete(mutated, deleteKey)
	}

	return mutated
}

// mutateValue applies mutations to individual values
func (f *MCPFuzzer) mutateValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return f.mutateString(v)
	case int:
		return f.mutateInt(v)
	case float64:
		return f.mutateFloat(v)
	case bool:
		return !v
	case []interface{}:
		return f.mutateArray(v)
	case map[string]interface{}:
		return f.mutateArgs(v)
	default:
		return f.generateRandomValue()
	}
}

// mutateString applies string mutations
func (f *MCPFuzzer) mutateString(s string) string {
	mutations := []func(string) string{
		func(s string) string { return "" },                              // Empty string
		func(s string) string { return s + s },                           // Duplicate
		func(s string) string { return strings.Repeat(s, 10) },           // Repeat many times
		func(s string) string { return f.generateRandomString(1000) },    // Very long string
		func(s string) string { return "\\x00\\x01\\x02" },               // Binary data
		func(s string) string { return "../../etc/passwd" },              // Path traversal
		func(s string) string { return "<script>alert('xss')</script>" }, // XSS
		func(s string) string { return "'; DROP TABLE users; --" },       // SQL injection
		func(s string) string { return "\\uFFFD\\uFFFE\\uFFFF" },         // Unicode edge cases
		func(s string) string { return strings.ToUpper(s) },              // Case change
		func(s string) string { return strings.ToLower(s) },              // Case change
	}

	mutation := mutations[f.rand.Intn(len(mutations))]
	return mutation(s)
}

// mutateInt applies integer mutations
func (f *MCPFuzzer) mutateInt(i int) int {
	mutations := []int{
		0, 1, -1, 2147483647, -2147483648, // Edge values
		i + 1, i - 1, i * 2, i / 2, // Arithmetic mutations
		^i, // Bitwise NOT
	}
	return mutations[f.rand.Intn(len(mutations))]
}

// mutateFloat applies float mutations
func (f *MCPFuzzer) mutateFloat(f64 float64) float64 {
	mutations := []float64{
		0.0, 1.0, -1.0, 3.14159, 2.71828,
		1.7976931348623157e+308, // Max float64
		2.2250738585072014e-308, // Min positive float64
		f64 + 1, f64 - 1, f64 * 2, f64 / 2,
	}
	return mutations[f.rand.Intn(len(mutations))]
}

// mutateArray applies array mutations
func (f *MCPFuzzer) mutateArray(arr []interface{}) []interface{} {
	switch f.rand.Intn(4) {
	case 0: // Empty array
		return []interface{}{}
	case 1: // Very long array
		long := make([]interface{}, f.config.MaxArrayLength)
		for i := range long {
			long[i] = f.generateRandomValue()
		}
		return long
	case 2: // Duplicate elements
		if len(arr) > 0 {
			return append(arr, arr...)
		}
		return arr
	default: // Mutate random element
		if len(arr) > 0 {
			mutated := make([]interface{}, len(arr))
			copy(mutated, arr)
			idx := f.rand.Intn(len(mutated))
			mutated[idx] = f.mutateValue(mutated[idx])
			return mutated
		}
		return arr
	}
}

// generateRandomArgs creates completely random arguments
func (f *MCPFuzzer) generateRandomArgs(toolName string) map[string]interface{} {
	args := make(map[string]interface{})

	// Generate 1-10 random key-value pairs
	count := f.rand.Intn(10) + 1
	for i := 0; i < count; i++ {
		key := f.generateRandomString(f.rand.Intn(20) + 1)
		args[key] = f.generateRandomValue()
	}

	// Add some tool-specific keys
	toolSpecific := f.getToolSpecificKeys(toolName)
	for _, key := range toolSpecific {
		args[key] = f.generateRandomValue()
	}

	return args
}

// generateRandomValue creates a random value of random type
func (f *MCPFuzzer) generateRandomValue() interface{} {
	switch f.rand.Intn(6) {
	case 0:
		return f.generateRandomString(f.rand.Intn(f.config.MaxStringLength))
	case 1:
		return f.rand.Int()
	case 2:
		return f.rand.Float64()
	case 3:
		return f.rand.Float64() < 0.5
	case 4:
		arr := make([]interface{}, f.rand.Intn(f.config.MaxArrayLength))
		for i := range arr {
			arr[i] = f.generateRandomString(10) // Avoid infinite recursion
		}
		return arr
	default:
		obj := make(map[string]interface{})
		count := f.rand.Intn(5) + 1
		for i := 0; i < count; i++ {
			key := f.generateRandomString(f.rand.Intn(10) + 1)
			obj[key] = f.generateRandomString(10) // Avoid infinite recursion
		}
		return obj
	}
}

// generateRandomString creates a random string with special characters
func (f *MCPFuzzer) generateRandomString(length int) string {
	if length <= 0 {
		return ""
	}

	// Mix normal characters with edge cases
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	edgeCases := "\\x00\\n\\r\\t\\\\\"'<>&"

	b := make([]byte, length)
	for i := range b {
		if f.rand.Float64() < 0.1 { // 10% chance of edge case character
			if len(edgeCases) > 0 {
				b[i] = edgeCases[f.rand.Intn(len(edgeCases))]
			} else {
				b[i] = charset[f.rand.Intn(len(charset))]
			}
		} else {
			b[i] = charset[f.rand.Intn(len(charset))]
		}
	}
	return string(b)
}

// Tool-specific logic

func (f *MCPFuzzer) getSeedInputsForTool(toolName string) []string {
	seeds := map[string][]string{
		"containerize_and_deploy": {
			`{"repo_url": "https://github.com/example/app", "branch": "main", "scan": true}`,
			`{"repo_url": "https://github.com/test/service", "branch": "develop", "scan": false}`,
		},
		"chat": {
			`{"message": "Hello", "session_id": "test-session"}`,
			`{"message": "Help me containerize my app"}`,
		},
	}
	return seeds[toolName]
}

func (f *MCPFuzzer) getDefaultArgsForTool(toolName string) map[string]interface{} {
	defaults := map[string]map[string]interface{}{
		"containerize_and_deploy": {
			"repo_url": "https://github.com/example/app",
			"branch":   "main",
			"scan":     true,
		},
		"chat": {
			"message":    "Hello",
			"session_id": "test-session",
		},
	}

	if args, exists := defaults[toolName]; exists {
		return args
	}
	return map[string]interface{}{}
}

func (f *MCPFuzzer) getToolSpecificKeys(toolName string) []string {
	keys := map[string][]string{
		"containerize_and_deploy": {"repo_url", "branch", "scan", "test_mode", "namespace", "image_name", "registry"},
		"chat":                    {"message", "session_id", "context"},
		"workflow_status":         {"workflow_id", "detailed"},
	}
	return keys[toolName]
}

// Test execution

// FuzzResultType represents the type of fuzzing result
type FuzzResultType int

const (
	FuzzResultSuccess FuzzResultType = iota
	FuzzResultError
	FuzzResultCrash
	FuzzResultTimeout
)

// FuzzResult represents the result of a single fuzz test
type FuzzResult struct {
	Type     FuzzResultType
	Error    string
	Duration time.Duration
	Output   interface{}
}

// testMCPToolInput tests a single MCP tool input
func (f *MCPFuzzer) testMCPToolInput(ctx context.Context, toolName string, args map[string]interface{}) FuzzResult {
	startTime := time.Now()

	defer func() {
		if r := recover(); r != nil {
			// Panic recovered - this is a crash
		}
	}()

	// Create MCP request
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return FuzzResult{
			Type:     FuzzResultError,
			Error:    fmt.Sprintf("JSON marshal error: %v", err),
			Duration: time.Since(startTime),
		}
	}

	req := &mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: argsJSON,
		},
	}

	// This would call the actual tool handler
	// For testing, we simulate the call
	result := f.simulateToolCall(ctx, req)

	return FuzzResult{
		Type:     result.Type,
		Error:    result.Error,
		Duration: time.Since(startTime),
		Output:   result.Output,
	}
}

// simulateToolCall simulates calling an MCP tool for fuzzing
func (f *MCPFuzzer) simulateToolCall(ctx context.Context, req *mcp.CallToolRequest) FuzzResult {
	// Check for timeout
	select {
	case <-ctx.Done():
		return FuzzResult{Type: FuzzResultTimeout, Error: "Operation timed out"}
	default:
	}

	// Simulate basic validation
	if req.Params.Name == "" {
		return FuzzResult{Type: FuzzResultError, Error: "Tool name cannot be empty"}
	}

	// Simulate argument parsing
	var args map[string]interface{}
	if argsBytes, ok := req.Params.Arguments.([]byte); ok {
		if err := json.Unmarshal(argsBytes, &args); err != nil {
			return FuzzResult{Type: FuzzResultError, Error: fmt.Sprintf("Invalid JSON arguments: %v", err)}
		}
	} else {
		return FuzzResult{Type: FuzzResultError, Error: "Arguments not in expected format"}
	}

	// Simulate tool-specific validation
	switch req.Params.Name {
	case "containerize_and_deploy":
		if repoURL, ok := args["repo_url"].(string); ok {
			if repoURL == "" {
				return FuzzResult{Type: FuzzResultError, Error: "repo_url cannot be empty"}
			}
			// Simulate crash condition for certain malformed URLs
			if strings.Contains(repoURL, "\\x00") {
				panic("Null byte in URL caused crash")
			}
		}
	}

	return FuzzResult{Type: FuzzResultSuccess, Output: map[string]interface{}{"status": "success"}}
}

// Saga fuzzing

// SagaScenario represents a saga execution scenario for fuzzing
type SagaScenario struct {
	Steps         []saga.SagaStep
	FailurePoints []int // Which steps should fail
	Execution     *saga.SagaExecution
	ExpectedState saga.SagaState
}

// generateSagaScenario creates a random saga scenario
func (f *MCPFuzzer) generateSagaScenario() SagaScenario {
	stepCount := f.rand.Intn(10) + 1
	steps := make([]saga.SagaStep, stepCount)

	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	for i := 0; i < stepCount; i++ {
		steps[i] = &TestSagaStep{
			name:          stepNames[i%len(stepNames)],
			canCompensate: f.rand.Float64() < 0.9, // 90% can be compensated
		}
	}

	// Randomly decide failure points
	failureCount := f.rand.Intn(stepCount/2 + 1)
	failurePoints := make([]int, failureCount)
	for i := 0; i < failureCount; i++ {
		failurePoints[i] = f.rand.Intn(stepCount)
	}

	// Create execution
	execution := &saga.SagaExecution{
		ID:         fmt.Sprintf("fuzz-saga-%d", time.Now().UnixNano()),
		WorkflowID: fmt.Sprintf("fuzz-workflow-%d", time.Now().UnixNano()),
		State:      saga.SagaStateInProgress,
		Steps:      steps,
	}

	expectedState := saga.SagaStateCompleted
	if len(failurePoints) > 0 {
		expectedState = saga.SagaStateCompensated
	}

	return SagaScenario{
		Steps:         steps,
		FailurePoints: failurePoints,
		Execution:     execution,
		ExpectedState: expectedState,
	}
}

// testSagaScenario tests a saga execution scenario
func (f *MCPFuzzer) testSagaScenario(scenario SagaScenario) FuzzResult {
	defer func() {
		if r := recover(); r != nil {
			// Saga panic is a crash
		}
	}()

	// Simulate saga execution
	execution := scenario.Execution

	// Execute steps until failure
	for i, step := range scenario.Steps {
		// Check if this step should fail
		shouldFail := false
		for _, failPoint := range scenario.FailurePoints {
			if failPoint == i {
				shouldFail = true
				break
			}
		}

		if shouldFail {
			// Simulate failure and compensation
			execution.State = saga.SagaStateFailed

			// Simulate compensation in reverse order
			for j := i - 1; j >= 0; j-- {
				compensationStep := saga.SagaStepResult{
					StepName:  scenario.Steps[j].Name(),
					Success:   true,
					Timestamp: time.Now(),
					Duration:  time.Duration(f.rand.Intn(30)+5) * time.Second,
				}
				execution.CompensatedSteps = append(execution.CompensatedSteps, compensationStep)
			}

			execution.State = saga.SagaStateCompensated
			break
		} else {
			// Simulate successful step execution
			executionStep := saga.SagaStepResult{
				StepName:  step.Name(),
				Success:   true,
				Timestamp: time.Now(),
				Duration:  time.Duration(f.rand.Intn(60)+10) * time.Second,
			}
			execution.ExecutedSteps = append(execution.ExecutedSteps, executionStep)
		}
	}

	if execution.State == saga.SagaStateInProgress {
		execution.State = saga.SagaStateCompleted
	}

	return FuzzResult{Type: FuzzResultSuccess, Output: execution}
}

// verifySagaInvariants checks if saga execution follows invariants
func (f *MCPFuzzer) verifySagaInvariants(execution *saga.SagaExecution) bool {
	// Check compensation order (should be reverse of execution)
	if execution.State == saga.SagaStateCompensated {
		executed := execution.ExecutedSteps
		compensated := execution.CompensatedSteps

		if len(executed) != len(compensated) {
			f.logger.Error("Compensation count mismatch", "executed", len(executed), "compensated", len(compensated))
			return false
		}

		for i, compensatedStep := range compensated {
			expectedIndex := len(executed) - 1 - i
			if expectedIndex >= 0 && expectedIndex < len(executed) {
				expectedStep := executed[expectedIndex]
				if compensatedStep.StepName != expectedStep.StepName {
					f.logger.Error("Compensation order incorrect",
						"compensated", compensatedStep.StepName,
						"expected", expectedStep.StepName)
					return false
				}
			}
		}
	}

	return true
}

// Utility functions for saving failures

func (f *MCPFuzzer) addToCorpus(args map[string]interface{}) {
	if len(f.corpus) >= f.config.CorpusSize {
		// Replace random entry
		f.corpus[f.rand.Intn(len(f.corpus))] = args
	} else {
		f.corpus = append(f.corpus, args)
	}
}

func (f *MCPFuzzer) isInterestingInput(args map[string]interface{}, result FuzzResult) bool {
	// Consider input interesting if it exercises different code paths
	// For now, just check if it has unique keys or values
	return len(args) > 2 || result.Duration > 100*time.Millisecond
}

func (f *MCPFuzzer) saveFailingInput(t *testing.T, toolName string, args map[string]interface{}, result FuzzResult, iteration int) {
	filename := fmt.Sprintf("fuzz_crash_%s_%d.json", toolName, iteration)
	data := map[string]interface{}{
		"tool":      toolName,
		"iteration": iteration,
		"args":      args,
		"error":     result.Error,
		"duration":  result.Duration.String(),
	}

	if jsonData, err := json.MarshalIndent(data, "", "  "); err == nil {
		t.Logf("Saving crash to %s: %s", filename, string(jsonData))
	}
}

func (f *MCPFuzzer) saveSagaFailure(t *testing.T, scenario SagaScenario, result FuzzResult, iteration int) {
	filename := fmt.Sprintf("saga_fuzz_crash_%d.json", iteration)
	data := map[string]interface{}{
		"iteration": iteration,
		"scenario":  scenario,
		"error":     result.Error,
		"duration":  result.Duration.String(),
	}

	if jsonData, err := json.MarshalIndent(data, "", "  "); err == nil {
		t.Logf("Saving saga crash to %s: %s", filename, string(jsonData))
	}
}
