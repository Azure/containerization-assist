//go:build integration

package workflow

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProgressSink for capturing progress updates in tests
type TestProgressSink struct {
	updates []progress.Update
}

func (t *TestProgressSink) Publish(ctx context.Context, update progress.Update) error {
	t.updates = append(t.updates, update)
	return nil
}

func (t *TestProgressSink) Close() error {
	return nil
}

// TestProgressFactory creates a test progress tracker
type TestProgressFactory struct {
	sink progress.Sink
}

func (f *TestProgressFactory) CreateTracker(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) *progress.Tracker {
	tracker := progress.NewTracker(ctx, totalSteps, f.sink)
	return tracker
}

// Add helper function for pointer
func ptrBool(b bool) *bool { return &b }

// MockStep for testing
type MockStep struct {
	name    string
	retries int
}

func (m *MockStep) Name() string                                            { return m.name }
func (m *MockStep) MaxRetries() int                                         { return m.retries }
func (m *MockStep) Execute(ctx context.Context, state *WorkflowState) error { return nil }

// MockStepProvider for testing
type MockStepProvider struct{}

func (m *MockStepProvider) GetAnalyzeStep() Step    { return &MockStep{name: "analyze", retries: 2} }
func (m *MockStepProvider) GetDockerfileStep() Step { return &MockStep{name: "dockerfile", retries: 1} }
func (m *MockStepProvider) GetBuildStep() Step      { return &MockStep{name: "build", retries: 2} }
func (m *MockStepProvider) GetScanStep() Step       { return &MockStep{name: "scan", retries: 1} }
func (m *MockStepProvider) GetTagStep() Step        { return &MockStep{name: "tag", retries: 1} }
func (m *MockStepProvider) GetPushStep() Step       { return &MockStep{name: "push", retries: 2} }
func (m *MockStepProvider) GetManifestStep() Step   { return &MockStep{name: "manifest", retries: 1} }
func (m *MockStepProvider) GetClusterStep() Step    { return &MockStep{name: "cluster", retries: 2} }
func (m *MockStepProvider) GetDeployStep() Step     { return &MockStep{name: "deploy", retries: 2} }
func (m *MockStepProvider) GetVerifyStep() Step     { return &MockStep{name: "verify", retries: 1} }

func TestWorkflowOrchestrator_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a test repository
	tempDir := t.TempDir()

	// Create a simple Node.js project
	err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{
  "name": "integration-test-app",
  "version": "1.0.0",
  "description": "Test application for integration testing",
  "main": "server.js",
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "express": "^4.18.0"
  },
  "engines": {
    "node": ">=16.0.0"
  }
}`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "server.js"), []byte(`
const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ message: 'Hello World!', version: '1.0.0' });
});

app.get('/health', (req, res) => {
  res.status(200).json({ status: 'healthy' });
});

const server = app.listen(PORT, () => {
  console.log('Server running on port', PORT);
});

module.exports = app;
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte(`
# Integration Test App

A Node.js Express application for integration testing.
`), 0644)
	require.NoError(t, err)

	// Test sink to capture progress
	testSink := &TestProgressSink{}

	// Create progress factory
	progressFactory := &TestProgressFactory{sink: testSink}

	// Create step factory with mock provider
	mockProvider := &MockStepProvider{}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)

	// Create base orchestrator with test progress factory
	// Using nil for tracer as it's not needed in integration tests
	baseOrchestrator := NewBaseOrchestrator(stepFactory, progressFactory, logger, nil)

	// Create workflow arguments
	args := &ContainerizeAndDeployArgs{
		RepoURL:  "file://" + tempDir, // Use local directory
		Branch:   "main",
		Scan:     false,          // Skip scanning in tests
		Deploy:   ptrBool(false), // Skip deployment in tests
		TestMode: true,           // Enable test mode
	}

	// Create a dummy MCP request for the test
	req := &mcp.CallToolRequest{}

	// Execute workflow
	result, err := baseOrchestrator.Execute(ctx, req, args)

	// The workflow might fail due to missing Git repo or other dependencies
	// but we should still capture progress and verify the orchestrator behavior
	if err != nil {
		t.Logf("Workflow failed as expected: %v", err)
		// Check that we still got some progress updates
		assert.NotEmpty(t, testSink.updates, "Should have progress updates even on failure")

		// Verify error handling produces progress updates
		var hasFailure bool
		for _, update := range testSink.updates {
			if update.Status == "failed" || update.Status == "retrying" {
				hasFailure = true
				break
			}
		}
		assert.True(t, hasFailure, "Should have failure/retry progress updates")
		return
	}

	// If successful, verify the results
	require.NotNil(t, result, "Result should not be nil")

	// Verify progress updates were captured
	assert.NotEmpty(t, testSink.updates, "Should have received progress updates")

	// Check for workflow progression
	var hasStarted, hasProgress, hasCompleted bool
	for _, update := range testSink.updates {
		if update.Status == "started" || update.Status == "running" {
			hasStarted = true
		}
		if update.Step > 0 {
			hasProgress = true
		}
		if update.Status == "completed" {
			hasCompleted = true
		}
	}

	assert.True(t, hasStarted, "Should have started/running status")
	assert.True(t, hasProgress, "Should have step progress")
	assert.True(t, hasCompleted, "Should have completed status")

	t.Logf("Integration test completed with %d progress updates", len(testSink.updates))

	// Log all progress updates for debugging
	for i, update := range testSink.updates {
		t.Logf("Progress %d: Step %d/%d (%d%%) - %s: %s",
			i, update.Step, update.Total, update.Percentage, update.Status, update.Message)
	}
}

func TestWorkflowOrchestrator_InvalidRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise for error test
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testSink := &TestProgressSink{}
	progressFactory := &TestProgressFactory{sink: testSink}
	mockProvider := &MockStepProvider{}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	baseOrchestrator := NewBaseOrchestrator(stepFactory, progressFactory, logger, nil)

	// Test with invalid repository URL
	args := &ContainerizeAndDeployArgs{
		RepoURL:  "https://github.com/non-existent/repository.git",
		Branch:   "main",
		Scan:     false,
		Deploy:   ptrBool(false),
		TestMode: true,
	}

	req := &mcp.CallToolRequest{}
	result, err := baseOrchestrator.Execute(ctx, req, args)

	// Should handle error gracefully
	assert.Error(t, err, "Should error with invalid repository")

	// Should still have progress updates for error case
	assert.NotEmpty(t, testSink.updates, "Should have progress updates even for errors")

	// Should indicate failure in progress
	var hasError bool
	for _, update := range testSink.updates {
		if update.Status == "failed" {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "Should have failed status in progress updates")

	// Result might be nil or indicate failure
	if result != nil {
		assert.False(t, result.Success, "Result should indicate failure")
	}
}

func TestWorkflowOrchestrator_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Create a very short context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	testSink := &TestProgressSink{}
	progressFactory := &TestProgressFactory{sink: testSink}
	mockProvider := &MockStepProvider{}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	baseOrchestrator := NewBaseOrchestrator(stepFactory, progressFactory, logger, nil)

	args := &ContainerizeAndDeployArgs{
		RepoURL:  "https://github.com/example/repo.git",
		Branch:   "main",
		Scan:     false,
		Deploy:   ptrBool(false),
		TestMode: true,
	}

	req := &mcp.CallToolRequest{}
	_, err := orchestrator.Execute(ctx, req, args)

	// Should handle context cancellation
	assert.Error(t, err, "Should error when context is cancelled")

	// Error should be context-related
	assert.Contains(t, []string{
		context.DeadlineExceeded.Error(),
		context.Canceled.Error(),
		"context",
		"timeout",
	}, err.Error(), "Should return context-related error")
}

func TestWorkflowOrchestrator_ProgressTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a simple test repository
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "app.py"), []byte(`print("Hello World")`), 0644)
	require.NoError(t, err)

	testSink := &TestProgressSink{}
	progressFactory := &TestProgressFactory{sink: testSink}
	mockProvider := &MockStepProvider{}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	baseOrchestrator := NewBaseOrchestrator(stepFactory, progressFactory, logger, nil)

	args := &ContainerizeAndDeployArgs{
		RepoURL:  "file://" + tempDir,
		Branch:   "main",
		Scan:     false,
		Deploy:   ptrBool(false),
		TestMode: true,
	}

	// Execute workflow - expect it to fail but capture progress
	req := &mcp.CallToolRequest{}
	_, err = orchestrator.Execute(ctx, req, args)

	// Focus on progress tracking regardless of workflow success
	assert.NotEmpty(t, testSink.updates, "Should capture progress updates")

	// Verify progress updates have required fields
	for i, update := range testSink.updates {
		assert.GreaterOrEqual(t, update.Step, 0, "Update %d should have valid step", i)
		assert.Greater(t, update.Total, 0, "Update %d should have valid total", i)
		assert.GreaterOrEqual(t, update.Percentage, 0, "Update %d should have valid percentage", i)
		assert.LessOrEqual(t, update.Percentage, 100, "Update %d should have valid percentage", i)
		assert.NotEmpty(t, update.Status, "Update %d should have status", i)
		assert.NotEmpty(t, update.Message, "Update %d should have message", i)
	}

	// Verify progress sequence makes sense
	if len(testSink.updates) > 1 {
		// Steps should generally increase or stay the same
		for i := 1; i < len(testSink.updates); i++ {
			assert.GreaterOrEqual(t, testSink.updates[i].Step, testSink.updates[i-1].Step,
				"Step should not decrease between updates %d and %d", i-1, i)
		}
	}

	t.Logf("Captured %d progress updates with proper tracking", len(testSink.updates))
}

func TestWorkflowOrchestrator_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise for concurrent test
	}))

	// Test multiple concurrent workflow executions
	numConcurrent := 3
	done := make(chan bool, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(workflowID int) {
			defer func() { done <- true }()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			testSink := &TestProgressSink{}
			progressFactory := &TestProgressFactory{sink: testSink}
			mockProvider := &MockStepProvider{}
			stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
			orchestrator := NewBaseOrchestrator(stepFactory, progressFactory, logger)

			args := &ContainerizeAndDeployArgs{
				RepoURL:  "https://github.com/non-existent/repo.git", // Will fail fast
				Branch:   "main",
				Scan:     false,
				Deploy:   ptrBool(false),
				TestMode: true,
			}

			req := &mcp.CallToolRequest{}
			_, err := orchestrator.Execute(ctx, req, args)

			// Expected to fail, but should handle concurrency gracefully
			assert.Error(t, err, "Workflow %d should fail with invalid repo", workflowID)
			assert.NotEmpty(t, testSink.updates, "Workflow %d should have progress updates", workflowID)
		}(i)
	}

	// Wait for all workflows to complete
	for i := 0; i < numConcurrent; i++ {
		<-done
	}

	t.Logf("All %d concurrent workflows completed", numConcurrent)
}
