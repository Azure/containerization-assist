package testutil

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// ExecutionCapture provides utilities to capture and verify tool executions
type ExecutionCapture struct {
	mu sync.RWMutex

	// Captured executions
	executions []CapturedExecution

	// Configuration
	captureEnabled bool
	logger         zerolog.Logger
}

// CapturedExecution represents a captured tool execution
type CapturedExecution struct {
	ToolName   string
	Args       interface{}
	Session    interface{}
	Result     interface{}
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Context    context.Context
	StackTrace string
}

// NewExecutionCapture creates a new execution capture
func NewExecutionCapture(logger zerolog.Logger) *ExecutionCapture {
	return &ExecutionCapture{
		executions:     make([]CapturedExecution, 0),
		captureEnabled: true,
		logger:         logger.With().Str("component", "execution_capture").Logger(),
	}
}

// CaptureExecution captures a tool execution
func (ec *ExecutionCapture) CaptureExecution(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
	executionFunc func() (interface{}, error),
) (interface{}, error) {
	if !ec.captureEnabled {
		return executionFunc()
	}

	startTime := time.Now()
	result, err := executionFunc()
	endTime := time.Now()

	ec.mu.Lock()
	defer ec.mu.Unlock()

	execution := CapturedExecution{
		ToolName:  toolName,
		Args:      args,
		Session:   session,
		Result:    result,
		Error:     err,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
		Context:   ctx,
	}

	ec.executions = append(ec.executions, execution)

	ec.logger.Debug().
		Str("tool", toolName).
		Dur("duration", execution.Duration).
		Bool("success", err == nil).
		Msg("Captured tool execution")

	return result, err
}

// GetExecutionCount returns the total number of captured executions
func (ec *ExecutionCapture) GetExecutionCount() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.executions)
}

// GetExecutions returns all captured executions
func (ec *ExecutionCapture) GetExecutions() []CapturedExecution {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	executions := make([]CapturedExecution, len(ec.executions))
	copy(executions, ec.executions)
	return executions
}

// GetExecutionsForTool returns executions for a specific tool
func (ec *ExecutionCapture) GetExecutionsForTool(toolName string) []CapturedExecution {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	var toolExecutions []CapturedExecution
	for _, execution := range ec.executions {
		if execution.ToolName == toolName {
			toolExecutions = append(toolExecutions, execution)
		}
	}
	return toolExecutions
}

// GetLastExecution returns the most recent execution
func (ec *ExecutionCapture) GetLastExecution() *CapturedExecution {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if len(ec.executions) == 0 {
		return nil
	}

	execution := ec.executions[len(ec.executions)-1]
	return &execution
}

// GetSuccessfulExecutions returns only successful executions
func (ec *ExecutionCapture) GetSuccessfulExecutions() []CapturedExecution {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	var successful []CapturedExecution
	for _, execution := range ec.executions {
		if execution.Error == nil {
			successful = append(successful, execution)
		}
	}
	return successful
}

// GetFailedExecutions returns only failed executions
func (ec *ExecutionCapture) GetFailedExecutions() []CapturedExecution {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	var failed []CapturedExecution
	for _, execution := range ec.executions {
		if execution.Error != nil {
			failed = append(failed, execution)
		}
	}
	return failed
}

// Clear resets the captured executions
func (ec *ExecutionCapture) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.executions = make([]CapturedExecution, 0)
}

// SetCaptureEnabled enables or disables execution capture
func (ec *ExecutionCapture) SetCaptureEnabled(enabled bool) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.captureEnabled = enabled
}

// ExecutionVerifier provides utilities for verifying captured executions
type ExecutionVerifier struct {
	capture *ExecutionCapture
	logger  zerolog.Logger
}

// NewExecutionVerifier creates a new execution verifier
func NewExecutionVerifier(capture *ExecutionCapture, logger zerolog.Logger) *ExecutionVerifier {
	return &ExecutionVerifier{
		capture: capture,
		logger:  logger.With().Str("component", "execution_verifier").Logger(),
	}
}

// VerifyExecutionCount verifies the total number of executions
func (ev *ExecutionVerifier) VerifyExecutionCount(expected int) error {
	actual := ev.capture.GetExecutionCount()
	if actual != expected {
		return types.NewRichError("EXECUTION_COUNT_MISMATCH", fmt.Sprintf("expected %d executions, got %d", expected, actual), "test_error")
	}
	return nil
}

// VerifyToolExecuted verifies that a specific tool was executed
func (ev *ExecutionVerifier) VerifyToolExecuted(toolName string) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}
	return nil
}

// VerifyToolExecutionCount verifies the number of executions for a specific tool
func (ev *ExecutionVerifier) VerifyToolExecutionCount(toolName string, expected int) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	actual := len(executions)
	if actual != expected {
		return types.NewRichError("TOOL_EXECUTION_COUNT_MISMATCH", fmt.Sprintf("expected %d executions for tool %s, got %d", expected, toolName, actual), "test_error")
	}
	return nil
}

// VerifyExecutionArgs verifies the arguments passed to a tool execution
func (ev *ExecutionVerifier) VerifyExecutionArgs(toolName string, expectedArgs interface{}) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}

	// Check the most recent execution
	lastExecution := executions[len(executions)-1]
	if !reflect.DeepEqual(lastExecution.Args, expectedArgs) {
		return types.NewRichError("EXECUTION_ARGS_MISMATCH", fmt.Sprintf("expected args %v for tool %s, got %v", expectedArgs, toolName, lastExecution.Args), "test_error")
	}

	return nil
}

// VerifyExecutionResult verifies the result of a tool execution
func (ev *ExecutionVerifier) VerifyExecutionResult(toolName string, expectedResult interface{}) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}

	// Check the most recent execution
	lastExecution := executions[len(executions)-1]
	if !reflect.DeepEqual(lastExecution.Result, expectedResult) {
		return types.NewRichError("EXECUTION_RESULT_MISMATCH", fmt.Sprintf("expected result %v for tool %s, got %v", expectedResult, toolName, lastExecution.Result), "test_error")
	}

	return nil
}

// VerifyExecutionSuccess verifies that a tool execution was successful
func (ev *ExecutionVerifier) VerifyExecutionSuccess(toolName string) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}

	// Check the most recent execution
	lastExecution := executions[len(executions)-1]
	if lastExecution.Error != nil {
		return types.NewRichError("EXECUTION_SHOULD_SUCCEED", fmt.Sprintf("expected successful execution for tool %s, got error: %v", toolName, lastExecution.Error), "test_error")
	}

	return nil
}

// VerifyExecutionFailure verifies that a tool execution failed
func (ev *ExecutionVerifier) VerifyExecutionFailure(toolName string) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}

	// Check the most recent execution
	lastExecution := executions[len(executions)-1]
	if lastExecution.Error == nil {
		return types.NewRichError("EXECUTION_SHOULD_FAIL", fmt.Sprintf("expected failed execution for tool %s, but it succeeded", toolName), "test_error")
	}

	return nil
}

// VerifyExecutionDuration verifies that a tool execution took the expected time
func (ev *ExecutionVerifier) VerifyExecutionDuration(toolName string, minDuration, maxDuration time.Duration) error {
	executions := ev.capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		return types.NewRichError("TOOL_NOT_EXECUTED", fmt.Sprintf("tool %s was not executed", toolName), "test_error")
	}

	// Check the most recent execution
	lastExecution := executions[len(executions)-1]
	duration := lastExecution.Duration

	if duration < minDuration {
		return types.NewRichError("EXECUTION_DURATION_TOO_SHORT", fmt.Sprintf("execution duration %v for tool %s is less than minimum %v", duration, toolName, minDuration), "test_error")
	}

	if duration > maxDuration {
		return types.NewRichError("EXECUTION_DURATION_TOO_LONG", fmt.Sprintf("execution duration %v for tool %s exceeds maximum %v", duration, toolName, maxDuration), "test_error")
	}

	return nil
}

// VerifyExecutionOrder verifies that tools were executed in the expected order
func (ev *ExecutionVerifier) VerifyExecutionOrder(expectedOrder []string) error {
	executions := ev.capture.GetExecutions()

	if len(executions) < len(expectedOrder) {
		return types.NewRichError("INSUFFICIENT_EXECUTIONS_FOR_ORDER", fmt.Sprintf("expected at least %d executions for order verification, got %d", len(expectedOrder), len(executions)), "test_error")
	}

	for i, expectedTool := range expectedOrder {
		if i >= len(executions) {
			return types.NewRichError("EXECUTION_ORDER_INCOMPLETE", fmt.Sprintf("expected tool %s at position %d, but only %d executions occurred", expectedTool, i, len(executions)), "test_error")
		}

		actualTool := executions[i].ToolName
		if actualTool != expectedTool {
			return types.NewRichError("EXECUTION_ORDER_MISMATCH", fmt.Sprintf("expected tool %s at position %d, got %s", expectedTool, i, actualTool), "test_error")
		}
	}

	return nil
}

// VerifyAllExecutionsSuccessful verifies that all captured executions were successful
func (ev *ExecutionVerifier) VerifyAllExecutionsSuccessful() error {
	failed := ev.capture.GetFailedExecutions()
	if len(failed) > 0 {
		return types.NewRichError("EXECUTIONS_FAILED", fmt.Sprintf("expected all executions to be successful, but %d failed", len(failed)), "test_error")
	}
	return nil
}

// VerifyNoExecutions verifies that no executions were captured
func (ev *ExecutionVerifier) VerifyNoExecutions() error {
	count := ev.capture.GetExecutionCount()
	if count > 0 {
		return types.NewRichError("UNEXPECTED_EXECUTIONS", fmt.Sprintf("expected no executions, but %d were captured", count), "test_error")
	}
	return nil
}

// ExecutionMatcher provides fluent interface for complex execution matching
type ExecutionMatcher struct {
	verifier    *ExecutionVerifier
	toolName    string
	filters     []ExecutionFilter
	expectation ExecutionExpectation
}

// ExecutionFilter represents a filter for executions
type ExecutionFilter func(execution CapturedExecution) bool

// ExecutionExpectation represents an expectation about executions
type ExecutionExpectation struct {
	Count       *int
	Success     *bool
	MinDuration *time.Duration
	MaxDuration *time.Duration
	Args        interface{}
	Result      interface{}
}

// NewExecutionMatcher creates a new execution matcher
func (ev *ExecutionVerifier) NewExecutionMatcher() *ExecutionMatcher {
	return &ExecutionMatcher{
		verifier: ev,
		filters:  make([]ExecutionFilter, 0),
	}
}

// ForTool sets the tool name filter
func (em *ExecutionMatcher) ForTool(toolName string) *ExecutionMatcher {
	em.toolName = toolName
	return em
}

// WithFilter adds a custom filter
func (em *ExecutionMatcher) WithFilter(filter ExecutionFilter) *ExecutionMatcher {
	em.filters = append(em.filters, filter)
	return em
}

// ExpectCount sets the expected count
func (em *ExecutionMatcher) ExpectCount(count int) *ExecutionMatcher {
	em.expectation.Count = &count
	return em
}

// ExpectSuccess sets the expected success state
func (em *ExecutionMatcher) ExpectSuccess(success bool) *ExecutionMatcher {
	em.expectation.Success = &success
	return em
}

// ExpectDurationBetween sets the expected duration range
func (em *ExecutionMatcher) ExpectDurationBetween(min, max time.Duration) *ExecutionMatcher {
	em.expectation.MinDuration = &min
	em.expectation.MaxDuration = &max
	return em
}

// ExpectArgs sets the expected arguments
func (em *ExecutionMatcher) ExpectArgs(args interface{}) *ExecutionMatcher {
	em.expectation.Args = args
	return em
}

// ExpectResult sets the expected result
func (em *ExecutionMatcher) ExpectResult(result interface{}) *ExecutionMatcher {
	em.expectation.Result = result
	return em
}

// Verify verifies the expectations
func (em *ExecutionMatcher) Verify() error {
	// Get executions
	var executions []CapturedExecution
	if em.toolName != "" {
		executions = em.verifier.capture.GetExecutionsForTool(em.toolName)
	} else {
		executions = em.verifier.capture.GetExecutions()
	}

	// Apply filters
	for _, filter := range em.filters {
		var filtered []CapturedExecution
		for _, execution := range executions {
			if filter(execution) {
				filtered = append(filtered, execution)
			}
		}
		executions = filtered
	}

	// Verify expectations
	if em.expectation.Count != nil {
		if len(executions) != *em.expectation.Count {
			return types.NewRichError("MATCHING_EXECUTION_COUNT_MISMATCH", fmt.Sprintf("expected %d matching executions, got %d", *em.expectation.Count, len(executions)), "test_error")
		}
	}

	// Verify other expectations on the most recent matching execution
	if len(executions) > 0 {
		lastExecution := executions[len(executions)-1]

		if em.expectation.Success != nil {
			actualSuccess := lastExecution.Error == nil
			if actualSuccess != *em.expectation.Success {
				return types.NewRichError("EXECUTION_SUCCESS_MISMATCH", fmt.Sprintf("expected success=%v, got success=%v", *em.expectation.Success, actualSuccess), "test_error")
			}
		}

		if em.expectation.MinDuration != nil && lastExecution.Duration < *em.expectation.MinDuration {
			return types.NewRichError("EXECUTION_DURATION_TOO_SHORT", fmt.Sprintf("execution duration %v is less than minimum %v", lastExecution.Duration, *em.expectation.MinDuration), "test_error")
		}

		if em.expectation.MaxDuration != nil && lastExecution.Duration > *em.expectation.MaxDuration {
			return types.NewRichError("EXECUTION_DURATION_TOO_LONG", fmt.Sprintf("execution duration %v exceeds maximum %v", lastExecution.Duration, *em.expectation.MaxDuration), "test_error")
		}

		if em.expectation.Args != nil && !reflect.DeepEqual(lastExecution.Args, em.expectation.Args) {
			return types.NewRichError("EXECUTION_ARGS_MISMATCH", fmt.Sprintf("expected args %v, got %v", em.expectation.Args, lastExecution.Args), "test_error")
		}

		if em.expectation.Result != nil && !reflect.DeepEqual(lastExecution.Result, em.expectation.Result) {
			return types.NewRichError("EXECUTION_RESULT_MISMATCH", fmt.Sprintf("expected result %v, got %v", em.expectation.Result, lastExecution.Result), "test_error")
		}
	}

	return nil
}
