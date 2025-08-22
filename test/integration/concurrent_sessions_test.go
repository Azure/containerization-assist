package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/service"
	"github.com/Azure/containerization-assist/pkg/service/session"
	"github.com/Azure/containerization-assist/pkg/service/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentWorkflowSessions tests multiple workflow sessions running concurrently
func TestConcurrentWorkflowSessions(t *testing.T) {
	// Setup test environment
	tmpDir, err := os.MkdirTemp("", "concurrent_sessions_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create concurrent session manager
	sessionManager, err := session.NewConcurrentBoltAdapter(dbPath, logger, 24*time.Hour, 100)
	require.NoError(t, err)
	defer sessionManager.Stop(context.Background())

	ctx := context.Background()
	sessionManager.StartCleanupRoutine(ctx, 5*time.Minute)

	// Test concurrent workflow operations
	numSessions := 10
	numOperationsPerSession := 20
	var wg sync.WaitGroup

	// Track results
	results := make(map[string]*tools.SimpleWorkflowState)
	var resultsMu sync.Mutex

	// Create and run concurrent workflow sessions
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()

			sessionID := fmt.Sprintf("test-session-%d", sessionNum)

			// Simulate workflow steps
			for step := 0; step < numOperationsPerSession; step++ {
				// Load state
				state, err := tools.LoadWorkflowState(ctx, sessionManager, sessionID)
				assert.NoError(t, err)

				// Simulate some processing
				stepName := fmt.Sprintf("step-%d", step)
				state.MarkStepCompleted(stepName)
				state.CurrentStep = stepName
				state.Status = "running"

				// Update artifacts
				if state.Artifacts == nil {
					state.Artifacts = make(map[string]interface{})
				}
				state.Artifacts[stepName] = map[string]interface{}{
					"session":   sessionNum,
					"step":      step,
					"timestamp": time.Now().Unix(),
				}

				// Save state
				err = tools.SaveWorkflowState(ctx, sessionManager, state)
				assert.NoError(t, err)

				// Small delay to simulate processing
				time.Sleep(time.Millisecond * 5)
			}

			// Load final state
			finalState, err := tools.LoadWorkflowState(ctx, sessionManager, sessionID)
			assert.NoError(t, err)

			// Store result
			resultsMu.Lock()
			results[sessionID] = finalState
			resultsMu.Unlock()
		}(i)
	}

	// Wait for all sessions to complete
	wg.Wait()

	// Verify results
	assert.Equal(t, numSessions, len(results))

	for sessionID, state := range results {
		// Each session should have completed all steps
		assert.Equal(t, numOperationsPerSession, len(state.CompletedSteps))
		assert.Equal(t, numOperationsPerSession, len(state.Artifacts))

		// Verify no steps were lost
		for step := 0; step < numOperationsPerSession; step++ {
			stepName := fmt.Sprintf("step-%d", step)
			assert.True(t, state.IsStepCompleted(stepName),
				"Session %s missing step %s", sessionID, stepName)
			assert.Contains(t, state.Artifacts, stepName)
		}
	}
}

// TestSessionIsolation verifies that sessions don't interfere with each other
func TestSessionIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_isolation_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sessionManager, err := session.NewConcurrentBoltAdapter(dbPath, logger, 24*time.Hour, 100)
	require.NoError(t, err)
	defer sessionManager.Stop(context.Background())

	ctx := context.Background()

	// Create two sessions with different data
	session1ID := "isolation-session-1"
	session2ID := "isolation-session-2"

	// Setup session 1
	state1, err := tools.LoadWorkflowState(ctx, sessionManager, session1ID)
	require.NoError(t, err)
	state1.RepoPath = "/repo/path/1"
	state1.MarkStepCompleted("step-a")
	state1.MarkStepCompleted("step-b")
	state1.Artifacts = map[string]interface{}{"key": "value1"}
	err = tools.SaveWorkflowState(ctx, sessionManager, state1)
	require.NoError(t, err)

	// Setup session 2
	state2, err := tools.LoadWorkflowState(ctx, sessionManager, session2ID)
	require.NoError(t, err)
	state2.RepoPath = "/repo/path/2"
	state2.MarkStepCompleted("step-x")
	state2.MarkStepCompleted("step-y")
	state2.Artifacts = map[string]interface{}{"key": "value2"}
	err = tools.SaveWorkflowState(ctx, sessionManager, state2)
	require.NoError(t, err)

	// Verify session 1 data is unchanged
	verifyState1, err := tools.LoadWorkflowState(ctx, sessionManager, session1ID)
	require.NoError(t, err)
	assert.Equal(t, "/repo/path/1", verifyState1.RepoPath)
	assert.True(t, verifyState1.IsStepCompleted("step-a"))
	assert.True(t, verifyState1.IsStepCompleted("step-b"))
	assert.False(t, verifyState1.IsStepCompleted("step-x"))
	assert.Equal(t, "value1", verifyState1.Artifacts["key"])

	// Verify session 2 data is unchanged
	verifyState2, err := tools.LoadWorkflowState(ctx, sessionManager, session2ID)
	require.NoError(t, err)
	assert.Equal(t, "/repo/path/2", verifyState2.RepoPath)
	assert.True(t, verifyState2.IsStepCompleted("step-x"))
	assert.True(t, verifyState2.IsStepCompleted("step-y"))
	assert.False(t, verifyState2.IsStepCompleted("step-a"))
	assert.Equal(t, "value2", verifyState2.Artifacts["key"])
}

// TestHighContentionScenario tests behavior under very high contention
func TestHighContentionScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high contention test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "high_contention_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sessionManager, err := session.NewConcurrentBoltAdapter(dbPath, logger, 24*time.Hour, 1000)
	require.NoError(t, err)
	defer sessionManager.Stop(context.Background())

	ctx := context.Background()
	sessionManager.StartCleanupRoutine(ctx, 1*time.Minute)

	// Single session with many concurrent updates
	sessionID := "high-contention-session"
	numWorkers := 100
	updatesPerWorker := 50

	// Initialize session with atomic update
	err = tools.AtomicUpdateWorkflowState(ctx, sessionManager, sessionID, func(state *tools.SimpleWorkflowState) error {
		state.Metadata["counter"] = 0
		state.Metadata["operations"] = []string{}
		return nil
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < updatesPerWorker; j++ {
				// Use atomic helpers for concurrent-safe updates
				_, err := tools.AtomicIncrementCounter(ctx, sessionManager, sessionID, "counter")
				if err != nil {
					t.Logf("Worker %d counter increment %d failed: %v", workerID, j, err)
				}

				// Append operation record atomically
				operation := fmt.Sprintf("worker-%d-update-%d", workerID, j)
				err = tools.AtomicAppendToList(ctx, sessionManager, sessionID, "operations", operation)
				if err != nil {
					t.Logf("Worker %d operation append %d failed: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Verify final state
	finalState, err := tools.LoadWorkflowState(ctx, sessionManager, sessionID)
	require.NoError(t, err)

	// Check counter
	finalCounter := 0
	if val, ok := finalState.Metadata["counter"].(float64); ok {
		finalCounter = int(val)
	} else if val, ok := finalState.Metadata["counter"].(int); ok {
		finalCounter = val
	}
	expectedCount := numWorkers * updatesPerWorker
	assert.Equal(t, expectedCount, finalCounter,
		"Counter mismatch: expected %d, got %d", expectedCount, finalCounter)

	// Check operations count
	var operations []string
	if ops, ok := finalState.Metadata["operations"].([]interface{}); ok {
		for _, op := range ops {
			if str, ok := op.(string); ok {
				operations = append(operations, str)
			}
		}
	}
	assert.Equal(t, expectedCount, len(operations),
		"Operations count mismatch: expected %d, got %d", expectedCount, len(operations))

	t.Logf("High contention test completed in %v for %d workers with %d updates each",
		duration, numWorkers, updatesPerWorker)
	t.Logf("Total successful updates: %d", finalCounter)
}

// TestNPMGeneratedSessions tests sessions created by the NPM package
func TestNPMGeneratedSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "npm_sessions_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sessionManager, err := session.NewConcurrentBoltAdapter(dbPath, logger, 24*time.Hour, 100)
	require.NoError(t, err)
	defer sessionManager.Stop(context.Background())

	ctx := context.Background()

	// Simulate NPM-style session IDs
	npmSessionIDs := []string{
		"session-2024-01-15T10-30-45-abc123def",
		"session-2024-01-15T10-30-46-xyz789ghi",
		"session-2024-01-15T10-30-47-qrs456tuv",
	}

	// Create sessions concurrently (simulating multiple NPM tool invocations)
	var wg sync.WaitGroup
	for _, sessionID := range npmSessionIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// First tool invocation (e.g., analyze_repository)
			state, err := tools.LoadWorkflowState(ctx, sessionManager, id)
			assert.NoError(t, err)
			state.RepoPath = "/test/repo"
			state.MarkStepCompleted("analyze_repository")
			err = tools.SaveWorkflowState(ctx, sessionManager, state)
			assert.NoError(t, err)

			// Second tool invocation (e.g., generate_dockerfile)
			state2, err := tools.LoadWorkflowState(ctx, sessionManager, id)
			assert.NoError(t, err)
			assert.True(t, state2.IsStepCompleted("analyze_repository"))
			state2.MarkStepCompleted("generate_dockerfile")
			err = tools.SaveWorkflowState(ctx, sessionManager, state2)
			assert.NoError(t, err)
		}(sessionID)
	}

	wg.Wait()

	// Verify all sessions were created and updated correctly
	for _, sessionID := range npmSessionIDs {
		state, err := tools.LoadWorkflowState(ctx, sessionManager, sessionID)
		require.NoError(t, err)
		assert.Equal(t, "/test/repo", state.RepoPath)
		assert.True(t, state.IsStepCompleted("analyze_repository"))
		assert.True(t, state.IsStepCompleted("generate_dockerfile"))
	}
}

// TestServerIntegration tests the full server with concurrent sessions
func TestServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping server integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "server_integration_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create server config
	config := workflow.ServerConfig{
		WorkspaceDir: tmpDir,
		StorePath:    filepath.Join(tmpDir, "sessions.db"),
		SessionTTL:   24 * time.Hour,
		MaxSessions:  100,
		LogLevel:     "info",
	}

	// Initialize server
	server, err := service.InitializeServer(logger, config)
	require.NoError(t, err)

	// Start server
	ctx := context.Background()
	go func() {
		err := server.Start(ctx)
		if err != nil {
			t.Logf("Server start error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	err = server.Stop(ctx)
	assert.NoError(t, err)
}
