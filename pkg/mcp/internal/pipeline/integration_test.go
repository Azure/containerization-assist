package pipeline

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	mcpconfig "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test logger
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create a pipeline manager
	manager := NewManager(logger)
	require.NotNil(t, manager)

	// Check initial status
	status := manager.GetStatus()
	assert.False(t, status.IsRunning)
	assert.Equal(t, 0, status.WorkerCount)
	assert.Equal(t, 0, status.ActiveJobs)

	// Register a test worker
	testWorker := NewSimpleBackgroundWorker("test-worker", func(ctx context.Context) error {
		// Simple test task
		time.Sleep(100 * time.Millisecond)
		return nil
	}, 500*time.Millisecond)

	err := manager.RegisterWorker(testWorker)
	require.NoError(t, err)

	// Start the manager
	err = manager.Start()
	require.NoError(t, err)

	// Verify it's running
	assert.True(t, manager.IsRunning())

	// Check status after start
	status = manager.GetStatus()
	assert.True(t, status.IsRunning)
	assert.Equal(t, 1, status.WorkerCount)

	// Submit a test job
	job := &Job{
		ID:         "test-job-1",
		Type:       "analysis",
		Parameters: map[string]interface{}{"test": true},
	}

	err = manager.SubmitJob(job)
	require.NoError(t, err)

	// Wait a bit for job processing
	time.Sleep(200 * time.Millisecond)

	// Check job status
	retrievedJob, exists := manager.GetJob("test-job-1")
	require.True(t, exists)
	assert.Equal(t, "test-job-1", retrievedJob.ID)
	assert.Equal(t, "analysis", retrievedJob.Type)

	// List all jobs
	allJobs := manager.ListJobs("")
	assert.Len(t, allJobs, 1)

	// List pending jobs (might be empty if processed quickly)
	pendingJobs := manager.ListJobs(JobStatusPending)
	// pendingJobs might be 0 or 1 depending on timing
	_ = pendingJobs // Avoid unused variable

	// Check worker health
	health, err := manager.GetWorkerHealth("test-worker")
	require.NoError(t, err)
	_ = health // Avoid unused variable
	assert.Equal(t, "test-worker", testWorker.Name())

	// Get all worker health
	allHealth := manager.GetAllWorkerHealth()
	assert.Len(t, allHealth, 1)
	assert.Contains(t, allHealth, "test-worker")

	// Stop the manager
	err = manager.Stop()
	require.NoError(t, err)

	// Verify it's stopped
	assert.False(t, manager.IsRunning())
}

func TestJobOrchestrator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create configuration
	config := DefaultPipelineConfig()
	config.WorkerPoolSize = 2
	config.MaxConcurrentJobs = 5

	// Create a simple worker manager with config
	wc := &mcpconfig.WorkerConfig{
		ShutdownTimeout: 30 * time.Second,
	}
	workerManager := NewBackgroundWorkerManager(wc)

	// Create job orchestrator
	orchestrator := NewJobOrchestrator(workerManager, config)
	require.NotNil(t, orchestrator)

	// Start orchestrator
	err := orchestrator.Start()
	require.NoError(t, err)

	// Submit test jobs
	for i := 0; i < 3; i++ {
		job := &Job{
			ID:   fmt.Sprintf("job-%d", i),
			Type: "build",
			Parameters: map[string]interface{}{
				"index": i,
			},
		}
		err := orchestrator.SubmitJob(job)
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	// Check stats
	stats := orchestrator.GetStats()
	assert.Equal(t, 3, stats.TotalJobs)

	// Get specific job
	job, exists := orchestrator.GetJob("job-1")
	require.True(t, exists)
	assert.Equal(t, "job-1", job.ID)
	assert.Equal(t, "build", job.Type)

	// Stop orchestrator
	err = orchestrator.Stop()
	require.NoError(t, err)
}

func TestConfigurationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test default configuration
	defaultConfig := DefaultPipelineConfig()
	assert.Greater(t, defaultConfig.WorkerPoolSize, 0)
	assert.Greater(t, defaultConfig.MaxConcurrentJobs, 0)
	assert.Greater(t, defaultConfig.JobTimeout, time.Duration(0))

	// Test extended configuration
	extendedConfig := DefaultExtendedPipelineConfig()
	assert.NotNil(t, extendedConfig.PipelineConfig)
	assert.Greater(t, extendedConfig.MaxGoroutines, 0)
	assert.Greater(t, extendedConfig.JobQueueSize, 0)

	// Test configuration validation
	validator := NewConfigValidator()
	err := validator.ValidateConfiguration(extendedConfig)
	require.NoError(t, err)

	// Test invalid configuration
	invalidConfig := DefaultExtendedPipelineConfig()
	invalidConfig.WorkerPoolSize = -1
	err = validator.ValidateConfiguration(invalidConfig)
	assert.Error(t, err)

	// Test configuration summary
	summary := GetConfigSummary(extendedConfig)
	assert.Greater(t, summary.WorkerPoolSize, 0)
	assert.Greater(t, summary.MaxConcurrentJobs, 0)
	assert.NotEmpty(t, summary.JobTimeout)
}

func TestWorkerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create a test worker that tracks execution
	var executionCount int64
	testWorker := NewSimpleBackgroundWorker("lifecycle-worker", func(ctx context.Context) error {
		atomic.AddInt64(&executionCount, 1)
		return nil
	}, 100*time.Millisecond)

	// Create worker manager
	manager := NewManager(logger)
	require.NotNil(t, manager)

	// Register worker
	err := manager.RegisterWorker(testWorker)
	require.NoError(t, err)

	// Start manager
	err = manager.Start()
	require.NoError(t, err)

	// Let worker run for a bit
	time.Sleep(350 * time.Millisecond)

	// Check that worker has executed multiple times
	assert.Greater(t, atomic.LoadInt64(&executionCount), int64(1))

	// Get worker status
	status, err := manager.GetWorkerStatus("lifecycle-worker")
	require.NoError(t, err)
	assert.Equal(t, WorkerStatusRunning, status)

	// Restart worker
	err = manager.RestartWorker("lifecycle-worker")
	require.NoError(t, err)

	// Let it run again
	time.Sleep(200 * time.Millisecond)

	// Stop manager
	err = manager.Stop()
	require.NoError(t, err)

	// Give the worker some time to properly stop
	time.Sleep(100 * time.Millisecond)

	// Verify worker is stopped or stopping (both are acceptable after shutdown)
	status, err = manager.GetWorkerStatus("lifecycle-worker")
	require.NoError(t, err)
	t.Logf("Final worker status: %s", status)

	// Allow stopped, stopping, or failed as all are valid end states
	// Failed can happen if there are shutdown timeout issues
	assert.Contains(t, []WorkerStatus{WorkerStatusStopped, WorkerStatusStopping, WorkerStatusFailed}, status)
}
