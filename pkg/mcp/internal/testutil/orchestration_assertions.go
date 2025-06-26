package testutil

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// AssertionHelper provides type-safe assertion utilities for orchestration testing
type AssertionHelper struct {
	t *testing.T
}

// NewAssertionHelper creates a new assertion helper
func NewAssertionHelper(t *testing.T) *AssertionHelper {
	return &AssertionHelper{t: t}
}

// Orchestrator Assertions

// AssertExecutionCount verifies the total number of executions
func (a *AssertionHelper) AssertExecutionCount(orchestrator *MockToolOrchestrator, expected int) {
	a.t.Helper()
	actual := orchestrator.GetExecutionCount()
	if actual != expected {
		a.t.Errorf("Expected %d total executions, got %d", expected, actual)
	}
}

// AssertToolExecuted verifies that a specific tool was executed
func (a *AssertionHelper) AssertToolExecuted(orchestrator *MockToolOrchestrator, toolName string) {
	a.t.Helper()
	count := orchestrator.GetExecutionCountForTool(toolName)
	if count == 0 {
		a.t.Errorf("Expected tool %s to be executed, but it was not", toolName)
	}
}

// AssertToolNotExecuted verifies that a specific tool was not executed
func (a *AssertionHelper) AssertToolNotExecuted(orchestrator *MockToolOrchestrator, toolName string) {
	a.t.Helper()
	count := orchestrator.GetExecutionCountForTool(toolName)
	if count > 0 {
		a.t.Errorf("Expected tool %s not to be executed, but it was executed %d times", toolName, count)
	}
}

// AssertToolExecutionCount verifies the number of executions for a specific tool
func (a *AssertionHelper) AssertToolExecutionCount(orchestrator *MockToolOrchestrator, toolName string, expected int) {
	a.t.Helper()
	actual := orchestrator.GetExecutionCountForTool(toolName)
	if actual != expected {
		a.t.Errorf("Expected tool %s to be executed %d times, got %d", toolName, expected, actual)
	}
}

// AssertLastExecutionArgs verifies the arguments of the last execution
func (a *AssertionHelper) AssertLastExecutionArgs(orchestrator *MockToolOrchestrator, expected interface{}) {
	a.t.Helper()
	lastExecution := orchestrator.GetLastExecution()
	if lastExecution == nil {
		a.t.Errorf("Expected at least one execution, but none found")
		return
	}

	if !reflect.DeepEqual(lastExecution.Args, expected) {
		a.t.Errorf("Expected last execution args %v, got %v", expected, lastExecution.Args)
	}
}

// AssertLastExecutionSuccess verifies that the last execution was successful
func (a *AssertionHelper) AssertLastExecutionSuccess(orchestrator *MockToolOrchestrator) {
	a.t.Helper()
	lastExecution := orchestrator.GetLastExecution()
	if lastExecution == nil {
		a.t.Errorf("Expected at least one execution, but none found")
		return
	}

	if lastExecution.Error != nil {
		a.t.Errorf("Expected last execution to succeed, but it failed with error: %v", lastExecution.Error)
	}
}

// AssertLastExecutionFailure verifies that the last execution failed
func (a *AssertionHelper) AssertLastExecutionFailure(orchestrator *MockToolOrchestrator) {
	a.t.Helper()
	lastExecution := orchestrator.GetLastExecution()
	if lastExecution == nil {
		a.t.Errorf("Expected at least one execution, but none found")
		return
	}

	if lastExecution.Error == nil {
		a.t.Errorf("Expected last execution to fail, but it succeeded")
	}
}

// AssertExecutionDuration verifies that an execution took the expected time
func (a *AssertionHelper) AssertExecutionDuration(execution *ExecutionRecord, minDuration, maxDuration time.Duration) {
	a.t.Helper()
	if execution.Duration < minDuration {
		a.t.Errorf("Expected execution duration to be at least %v, got %v", minDuration, execution.Duration)
	}
	if execution.Duration > maxDuration {
		a.t.Errorf("Expected execution duration to be at most %v, got %v", maxDuration, execution.Duration)
	}
}

// Registry Assertions

// AssertToolRegistered verifies that a tool is registered
func (a *AssertionHelper) AssertToolRegistered(registry *MockToolRegistry, toolName string) {
	a.t.Helper()
	if !registry.IsToolRegistered(toolName) {
		a.t.Errorf("Expected tool %s to be registered, but it was not", toolName)
	}
}

// AssertToolNotRegistered verifies that a tool is not registered
func (a *AssertionHelper) AssertToolNotRegistered(registry *MockToolRegistry, toolName string) {
	a.t.Helper()
	if registry.IsToolRegistered(toolName) {
		a.t.Errorf("Expected tool %s not to be registered, but it was", toolName)
	}
}

// AssertRegistrationCount verifies the total number of registrations
func (a *AssertionHelper) AssertRegistrationCount(registry *MockToolRegistry, expected int) {
	a.t.Helper()
	actual := registry.GetRegistrationCount()
	if actual != expected {
		a.t.Errorf("Expected %d tool registrations, got %d", expected, actual)
	}
}

// AssertRegisteredTools verifies the set of registered tools
func (a *AssertionHelper) AssertRegisteredTools(registry *MockToolRegistry, expectedTools []string) {
	a.t.Helper()
	actualTools := registry.GetRegisteredToolNames()

	// Check count
	if len(actualTools) != len(expectedTools) {
		a.t.Errorf("Expected %d registered tools, got %d", len(expectedTools), len(actualTools))
		return
	}

	// Check each expected tool is present
	toolMap := make(map[string]bool)
	for _, tool := range actualTools {
		toolMap[tool] = true
	}

	for _, expectedTool := range expectedTools {
		if !toolMap[expectedTool] {
			a.t.Errorf("Expected tool %s to be registered, but it was not", expectedTool)
		}
	}
}

// Factory Assertions

// AssertToolCreated verifies that a tool was created
func (a *AssertionHelper) AssertToolCreated(factory *MockToolFactory, toolName string) {
	a.t.Helper()
	count := factory.GetCreationCountForTool(toolName)
	if count == 0 {
		a.t.Errorf("Expected tool %s to be created, but it was not", toolName)
	}
}

// AssertToolCreationCount verifies the number of creations for a specific tool
func (a *AssertionHelper) AssertToolCreationCount(factory *MockToolFactory, toolName string, expected int) {
	a.t.Helper()
	actual := factory.GetCreationCountForTool(toolName)
	if actual != expected {
		a.t.Errorf("Expected tool %s to be created %d times, got %d", toolName, expected, actual)
	}
}

// AssertTotalCreationCount verifies the total number of tool creations
func (a *AssertionHelper) AssertTotalCreationCount(factory *MockToolFactory, expected int) {
	a.t.Helper()
	actual := factory.GetCreationCount()
	if actual != expected {
		a.t.Errorf("Expected %d total tool creations, got %d", expected, actual)
	}
}

// Execution Capture Assertions

// AssertCapturedExecutionCount verifies the number of captured executions
func (a *AssertionHelper) AssertCapturedExecutionCount(capture *ExecutionCapture, expected int) {
	a.t.Helper()
	actual := capture.GetExecutionCount()
	if actual != expected {
		a.t.Errorf("Expected %d captured executions, got %d", expected, actual)
	}
}

// AssertCapturedToolExecution verifies that a tool execution was captured
func (a *AssertionHelper) AssertCapturedToolExecution(capture *ExecutionCapture, toolName string) {
	a.t.Helper()
	executions := capture.GetExecutionsForTool(toolName)
	if len(executions) == 0 {
		a.t.Errorf("Expected captured execution for tool %s, but none found", toolName)
	}
}

// AssertAllExecutionsSuccessful verifies that all captured executions were successful
func (a *AssertionHelper) AssertAllExecutionsSuccessful(capture *ExecutionCapture) {
	a.t.Helper()
	failed := capture.GetFailedExecutions()
	if len(failed) > 0 {
		a.t.Errorf("Expected all executions to be successful, but %d failed", len(failed))
		for _, execution := range failed {
			a.t.Logf("Failed execution: tool=%s, error=%v", execution.ToolName, execution.Error)
		}
	}
}

// AssertExecutionOrder verifies that tools were executed in the expected order
func (a *AssertionHelper) AssertExecutionOrder(capture *ExecutionCapture, expectedOrder []string) {
	a.t.Helper()
	executions := capture.GetExecutions()

	if len(executions) < len(expectedOrder) {
		a.t.Errorf("Expected at least %d executions for order verification, got %d", len(expectedOrder), len(executions))
		return
	}

	for i, expectedTool := range expectedOrder {
		if i >= len(executions) {
			a.t.Errorf("Expected tool %s at position %d, but only %d executions occurred", expectedTool, i, len(executions))
			return
		}

		actualTool := executions[i].ToolName
		if actualTool != expectedTool {
			a.t.Errorf("Expected tool %s at position %d, got %s", expectedTool, i, actualTool)
		}
	}
}

// Generic Assertions

// AssertNoError verifies that no error occurred
func (a *AssertionHelper) AssertNoError(err error) {
	a.t.Helper()
	if err != nil {
		a.t.Errorf("Expected no error, got: %v", err)
	}
}

// AssertError verifies that an error occurred
func (a *AssertionHelper) AssertError(err error) {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("Expected an error, but none occurred")
	}
}

// AssertErrorContains verifies that an error contains specific text
func (a *AssertionHelper) AssertErrorContains(err error, expectedText string) {
	a.t.Helper()
	if err == nil {
		a.t.Errorf("Expected an error containing '%s', but no error occurred", expectedText)
		return
	}

	if !contains(err.Error(), expectedText) {
		a.t.Errorf("Expected error to contain '%s', but got: %v", expectedText, err)
	}
}

// AssertEqual verifies that two values are equal
func (a *AssertionHelper) AssertEqual(actual, expected interface{}) {
	a.t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		a.t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual verifies that two values are not equal
func (a *AssertionHelper) AssertNotEqual(actual, unexpected interface{}) {
	a.t.Helper()
	if reflect.DeepEqual(actual, unexpected) {
		a.t.Errorf("Expected values to be different, but both were %v", actual)
	}
}

// AssertNil verifies that a value is nil
func (a *AssertionHelper) AssertNil(value interface{}) {
	a.t.Helper()
	if value != nil {
		a.t.Errorf("Expected nil, got %v", value)
	}
}

// AssertNotNil verifies that a value is not nil
func (a *AssertionHelper) AssertNotNil(value interface{}) {
	a.t.Helper()
	if value == nil {
		a.t.Errorf("Expected non-nil value, got nil")
	}
}

// AssertTrue verifies that a boolean value is true
func (a *AssertionHelper) AssertTrue(value bool) {
	a.t.Helper()
	if !value {
		a.t.Errorf("Expected true, got false")
	}
}

// AssertFalse verifies that a boolean value is false
func (a *AssertionHelper) AssertFalse(value bool) {
	a.t.Helper()
	if value {
		a.t.Errorf("Expected false, got true")
	}
}

// Advanced Assertions

// AssertEventuallyTrue waits for a condition to become true within a timeout
func (a *AssertionHelper) AssertEventuallyTrue(condition func() bool, timeout time.Duration, checkInterval time.Duration) {
	a.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		select {
		case <-ctx.Done():
			a.t.Errorf("Condition did not become true within %v", timeout)
			return
		case <-ticker.C:
			// Continue checking
		}
	}
}

// AssertNeverTrue verifies that a condition never becomes true within a timeout
func (a *AssertionHelper) AssertNeverTrue(condition func() bool, duration time.Duration, checkInterval time.Duration) {
	a.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if condition() {
			a.t.Errorf("Expected condition to remain false, but it became true")
			return
		}

		select {
		case <-ctx.Done():
			// Success - condition never became true
			return
		case <-ticker.C:
			// Continue checking
		}
	}
}

// Convenience Methods

// RequireNoError is like AssertNoError but fails the test immediately
func (a *AssertionHelper) RequireNoError(err error) {
	a.t.Helper()
	if err != nil {
		a.t.Fatalf("Expected no error, got: %v", err)
	}
}

// RequireNotNil is like AssertNotNil but fails the test immediately
func (a *AssertionHelper) RequireNotNil(value interface{}) {
	a.t.Helper()
	if value == nil {
		a.t.Fatalf("Expected non-nil value, got nil")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
