package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// ExamplePipelineUsage demonstrates how to use the pipeline system
func ExamplePipelineUsage() {
	logger := zerolog.New(zerolog.NewConsoleWriter())

	manager := NewManager(logger)

	registerExampleWorkers(manager, logger)

	if err := manager.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to start pipeline")
		return
	}

	submitExampleJobs(manager, logger)

	monitorPipeline(manager, logger, 10*time.Second)

	if err := manager.Stop(); err != nil {
		logger.Error().Err(err).Msg("Failed to stop pipeline")
	}

	logger.Info().Msg("Pipeline example completed")
}

// registerExampleWorkers registers example workers with the manager
func registerExampleWorkers(manager *Manager, logger zerolog.Logger) {
	analysisWorker := NewSimpleBackgroundWorker(
		"repo-analyzer",
		func(ctx context.Context) error {
			logger.Debug().Msg("Repository analysis worker running")
			time.Sleep(100 * time.Millisecond)
			return nil
		},
		2*time.Second,
	)

	buildWorker := NewSimpleBackgroundWorker(
		"docker-builder",
		func(ctx context.Context) error {
			logger.Debug().Msg("Docker build worker running")
			time.Sleep(200 * time.Millisecond)
			return nil
		},
		3*time.Second,
	)

	deployWorker := NewSimpleBackgroundWorker(
		"k8s-deployer",
		func(ctx context.Context) error {
			logger.Debug().Msg("Kubernetes deployment worker running")
			time.Sleep(150 * time.Millisecond)
			return nil
		},
		4*time.Second,
	)

	if err := manager.RegisterWorker(analysisWorker); err != nil {
		logger.Error().Err(err).Msg("Failed to register analysis worker")
	}

	if err := manager.RegisterWorker(buildWorker); err != nil {
		logger.Error().Err(err).Msg("Failed to register build worker")
	}

	if err := manager.RegisterWorker(deployWorker); err != nil {
		logger.Error().Err(err).Msg("Failed to register deploy worker")
	}

	logger.Info().Int("worker_count", 3).Msg("Registered example workers")
}

// submitExampleJobs submits example jobs to the pipeline
func submitExampleJobs(manager *Manager, logger zerolog.Logger) {
	jobs := []*Job{
		{
			ID:   "analyze-repo-1",
			Type: "analysis",
			Parameters: map[string]interface{}{
				"repo_url": "https://github.com/example/repo1",
				"branch":   "main",
			},
		},
		{
			ID:   "build-image-1",
			Type: "build",
			Parameters: map[string]interface{}{
				"dockerfile_path": "./Dockerfile",
				"image_tag":       "example/app:latest",
			},
		},
		{
			ID:   "deploy-app-1",
			Type: "deploy",
			Parameters: map[string]interface{}{
				"namespace": "production",
				"image_ref": "example/app:latest",
				"replicas":  3,
			},
		},
		{
			ID:   "analyze-repo-2",
			Type: "analysis",
			Parameters: map[string]interface{}{
				"repo_url": "https://github.com/example/repo2",
				"branch":   "develop",
			},
		},
		{
			ID:   "build-image-2",
			Type: "build",
			Parameters: map[string]interface{}{
				"dockerfile_path": "./docker/Dockerfile",
				"image_tag":       "example/service:v1.0.0",
			},
		},
	}

	for _, job := range jobs {
		if err := manager.SubmitJob(job); err != nil {
			logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to submit job")
		} else {
			logger.Info().Str("job_id", job.ID).Str("job_type", job.Type).Msg("Submitted job")
		}
	}

	logger.Info().Int("job_count", len(jobs)).Msg("Submitted example jobs")
}

// monitorPipeline monitors the pipeline status for a specified duration
func monitorPipeline(manager *Manager, logger zerolog.Logger, duration time.Duration) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration)

	logger.Info().Dur("duration", duration).Msg("Starting pipeline monitoring")

	for {
		select {
		case <-timeout:
			logger.Info().Msg("Pipeline monitoring completed")
			return

		case <-ticker.C:
			status := manager.GetStatus()

			jobStats := manager.GetOrchestratorStats()

			workerHealth := manager.GetAllWorkerHealth()
			healthyWorkers := 0
			for _, health := range workerHealth {
				if health.Status == "healthy" {
					healthyWorkers++
				}
			}

			logger.Info().
				Bool("is_running", status.IsRunning).
				Bool("is_healthy", status.IsHealthy).
				Int("worker_count", status.WorkerCount).
				Int("healthy_workers", healthyWorkers).
				Int("total_jobs", jobStats.TotalJobs).
				Int("pending_jobs", jobStats.PendingJobs).
				Int("running_jobs", jobStats.RunningJobs).
				Int("completed_jobs", jobStats.CompletedJobs).
				Int("failed_jobs", jobStats.FailedJobs).
				Msg("Pipeline status")

			allJobs := manager.ListJobs("")
			for _, job := range allJobs {
				logger.Debug().
					Str("job_id", job.ID).
					Str("job_type", job.Type).
					Str("status", string(job.Status)).
					Time("created_at", job.CreatedAt).
					Msg("Job status")
			}
		}
	}
}

// CreateTestPipeline creates a simple test pipeline for demonstration
func CreateTestPipeline() *Manager {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	manager := NewManager(logger)

	testWorker := NewSimpleBackgroundWorker(
		"test-worker",
		func(ctx context.Context) error {
			return nil
		},
		5*time.Second,
	)

	if err := manager.RegisterWorker(testWorker); err != nil {
		logger.Error().Err(err).Msg("Failed to register test worker")
	}

	return manager
}

// ValidatePipelineConfiguration validates a pipeline configuration and returns any issues
func ValidatePipelineConfiguration(config *ExtendPipelineConfig) []string {
	var issues []string

	validator := NewConfigValidator()
	if err := validator.ValidateConfiguration(config); err != nil {
		issues = append(issues, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	if config.WorkerPoolSize > config.MaxGoroutines {
		issues = append(issues, "Worker pool size exceeds max goroutines limit")
	}

	if config.JobTimeout < time.Minute {
		issues = append(issues, "Job timeout is very short (< 1 minute)")
	}

	if config.MaxConcurrentJobs < config.WorkerPoolSize {
		issues = append(issues, "Max concurrent jobs is less than worker pool size")
	}

	if config.HealthCheckInterval > time.Minute {
		issues = append(issues, "Health check interval is very long (> 1 minute)")
	}

	return issues
}

// GetPipelineRecommendations returns configuration recommendations based on use case
func GetPipelineRecommendations(useCase string) *ExtendPipelineConfig {
	base := DefaultExtendedPipelineConfig()

	switch useCase {
	case "development":
		base.WorkerPoolSize = 3
		base.MaxConcurrentJobs = 5
		base.JobTimeout = 5 * time.Minute
		base.MaxMemoryPerWorker = 256 * 1024 * 1024
		base.MaxCPUPerWorker = 0.5

	case "testing":
		base.WorkerPoolSize = 5
		base.MaxConcurrentJobs = 10
		base.JobTimeout = 10 * time.Minute
		base.MaxMemoryPerWorker = 512 * 1024 * 1024
		base.MaxCPUPerWorker = 1.0

	case "production":
		base.WorkerPoolSize = 10
		base.MaxConcurrentJobs = 20
		base.JobTimeout = 30 * time.Minute
		base.MaxMemoryPerWorker = 1024 * 1024 * 1024
		base.MaxCPUPerWorker = 2.0
		base.EnableMetrics = true
		base.EnableTracing = true

	case "high-volume":
		base.WorkerPoolSize = 20
		base.MaxConcurrentJobs = 50
		base.JobTimeout = 15 * time.Minute
		base.JobQueueSize = 1000
		base.MaxMemoryPerWorker = 2048 * 1024 * 1024
		base.MaxCPUPerWorker = 4.0
		base.EnableMetrics = true
		base.EnableTracing = true

	default:
		return base
	}

	return base
}
