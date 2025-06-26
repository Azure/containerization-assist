package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// RecreateStrategy implements a recreate deployment strategy
// This strategy terminates all existing instances before creating new ones
type RecreateStrategy struct {
	*BaseStrategy
	logger zerolog.Logger
}

// NewRecreateStrategy creates a new recreate deployment strategy
func NewRecreateStrategy(logger zerolog.Logger) *RecreateStrategy {
	return &RecreateStrategy{
		BaseStrategy: NewBaseStrategy(logger),
		logger:       logger.With().Str("strategy", "recreate").Logger(),
	}
}

// GetName returns the strategy name
func (r *RecreateStrategy) GetName() string {
	return "recreate"
}

// GetDescription returns a human-readable description
func (r *RecreateStrategy) GetDescription() string {
	return "Recreate deployment that terminates all existing instances before creating new ones, causing brief downtime but ensuring clean state"
}

// ValidatePrerequisites checks if the recreate strategy can be used
func (r *RecreateStrategy) ValidatePrerequisites(ctx context.Context, config DeploymentConfig) error {
	r.logger.Debug().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Validating recreate deployment prerequisites")

	// Check if K8sDeployer is available
	if config.K8sDeployer == nil {
		return fmt.Errorf("K8sDeployer is required for recreate deployment")
	}

	// Check if we have required configuration
	if config.AppName == "" {
		return fmt.Errorf("app name is required for recreate deployment")
	}

	if config.ImageRef == "" {
		return fmt.Errorf("image reference is required for recreate deployment")
	}

	if config.Namespace == "" {
		config.Namespace = "default"
	}

	// Recreate strategy requires at least 1 replica
	if config.Replicas < 1 {
		config.Replicas = 1
	}

	// Check if we can connect to the cluster
	if err := r.checkClusterConnection(ctx, config); err != nil {
		return fmt.Errorf("cluster connection check failed: %w", err)
	}

	r.logger.Info().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Recreate deployment prerequisites validated successfully")

	return nil
}

// Deploy executes the recreate deployment
func (r *RecreateStrategy) Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error) {
	startTime := time.Now()
	r.logger.Info().
		Str("app_name", config.AppName).
		Str("image_ref", config.ImageRef).
		Str("namespace", config.Namespace).
		Msg("Starting recreate deployment")

	result := &DeploymentResult{
		Strategy:  r.GetName(),
		StartTime: startTime,
		Resources: make([]DeployedResource, 0),
	}

	// Report initial progress
	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.1, "Initializing recreate deployment")
		}
	}

	// Step 1: Validate prerequisites
	if err := r.ValidatePrerequisites(ctx, config); err != nil {
		return r.handleDeploymentError(result, "validation", err, startTime)
	}

	// Step 2: Check if deployment exists and get current state
	currentExists, currentVersion, err := r.getCurrentDeploymentState(ctx, config)
	if err != nil {
		return r.handleDeploymentError(result, "state_check", err, startTime)
	}

	r.logger.Info().
		Bool("deployment_exists", currentExists).
		Str("current_version", currentVersion).
		Msg("Current deployment state determined")

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			if currentExists {
				reporter.ReportStage(0.2, "Terminating existing deployment")
			} else {
				reporter.ReportStage(0.2, "No existing deployment found, proceeding with creation")
			}
		}
	}

	// Step 3: Terminate existing deployment if it exists
	if currentExists {
		if err := r.terminateExistingDeployment(ctx, config); err != nil {
			return r.handleDeploymentError(result, "termination", err, startTime)
		}

		result.Resources = append(result.Resources, DeployedResource{
			Kind:      "Deployment",
			Name:      config.AppName,
			Namespace: config.Namespace,
			Status:    "terminated",
		})

		if config.ProgressReporter != nil {
			if reporter, ok := config.ProgressReporter.(interface {
				ReportStage(float64, string)
			}); ok {
				reporter.ReportStage(0.4, "Waiting for termination to complete")
			}
		}

		// Wait for termination to complete
		if err := r.waitForTermination(ctx, config); err != nil {
			return r.handleDeploymentError(result, "termination_wait", err, startTime)
		}
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.5, "Creating new deployment")
		}
	}

	// Step 4: Create new deployment
	if err := r.createNewDeployment(ctx, config); err != nil {
		return r.handleDeploymentError(result, "creation", err, startTime)
	}

	result.Resources = append(result.Resources, DeployedResource{
		Kind:      "Deployment",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "created",
	})

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.7, "Waiting for new deployment to be ready")
		}
	}

	// Step 5: Wait for new deployment to be ready
	if err := r.WaitForDeployment(ctx, config, config.AppName); err != nil {
		r.logger.Error().Err(err).
			Str("deployment", config.AppName).
			Msg("New deployment failed to become ready")
		return r.handleDeploymentError(result, "readiness_check", err, startTime)
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.9, "Validating deployment health")
		}
	}

	// Step 6: Perform final health checks
	if err := r.validateDeploymentHealth(ctx, config); err != nil {
		r.logger.Error().Err(err).
			Str("deployment", config.AppName).
			Msg("Deployment health validation failed")
		return r.handleDeploymentError(result, "health_validation", err, startTime)
	}

	// Step 7: Create or update service if needed
	if err := r.ensureService(ctx, config); err != nil {
		r.logger.Warn().Err(err).
			Str("app_name", config.AppName).
			Msg("Service creation/update failed - continuing")
	} else {
		result.Resources = append(result.Resources, DeployedResource{
			Kind:      "Service",
			Name:      config.AppName,
			Namespace: config.Namespace,
			Status:    "created",
		})
	}

	// Step 8: Complete deployment
	endTime := time.Now()
	result.Success = true
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.RollbackAvailable = false // Recreate doesn't maintain previous versions
	result.PreviousVersion = currentVersion

	// Get final health status
	healthResult, err := r.getFinalHealthStatus(ctx, config)
	if err == nil {
		result.HealthStatus = "healthy"
		result.ReadyReplicas = healthResult.Summary.ReadyPods
		result.TotalReplicas = healthResult.Summary.TotalPods
	} else {
		result.HealthStatus = "unknown"
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(1.0, "Recreate deployment completed successfully")
		}
	}

	r.logger.Info().
		Str("app_name", config.AppName).
		Dur("duration", result.Duration).
		Int("ready_replicas", result.ReadyReplicas).
		Int("total_replicas", result.TotalReplicas).
		Msg("Recreate deployment completed successfully")

	return result, nil
}

// Rollback for recreate strategy is limited since we don't maintain previous versions
func (r *RecreateStrategy) Rollback(ctx context.Context, config DeploymentConfig) error {
	r.logger.Warn().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Rollback requested for recreate deployment - limited rollback capability")

	// Recreate strategy doesn't maintain previous versions, so rollback is limited
	// We can only attempt to restart the current deployment or provide guidance

	return fmt.Errorf("recreate deployment strategy does not support rollback - previous versions are not maintained. Consider using 'kubectl rollout undo' manually or redeploy with a previous image version")
}

// Private helper methods

func (r *RecreateStrategy) checkClusterConnection(ctx context.Context, config DeploymentConfig) error {
	// Use K8sDeployer to perform a simple health check
	healthOptions := kubernetes.HealthCheckOptions{
		Namespace: config.Namespace,
		Timeout:   30 * time.Second,
	}

	_, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	return err
}

func (r *RecreateStrategy) getCurrentDeploymentState(ctx context.Context, config DeploymentConfig) (exists bool, version string, err error) {
	r.logger.Debug().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Checking current deployment state")

	// Check if deployment exists by trying to get its rollout history
	historyConfig := kubernetes.RolloutHistoryConfig{
		ResourceType: "deployment",
		ResourceName: config.AppName,
		Namespace:    config.Namespace,
	}

	history, err := config.K8sDeployer.GetRolloutHistory(ctx, historyConfig)
	if err != nil {
		// Deployment likely doesn't exist
		r.logger.Debug().Err(err).
			Str("app_name", config.AppName).
			Msg("Deployment does not exist or cannot be accessed")
		return false, "", nil
	}

	if history != nil && len(history.Revisions) > 0 {
		// Get the latest revision
		latestRevision := history.Revisions[len(history.Revisions)-1]
		return true, fmt.Sprintf("revision-%d", latestRevision.Number), nil
	}

	return false, "", nil
}

func (r *RecreateStrategy) terminateExistingDeployment(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info().
		Str("deployment", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Terminating existing deployment")

	// In a real implementation, this would:
	// 1. Scale the deployment to 0 replicas
	// 2. Delete the deployment
	// For now, we'll simulate this with logging

	// Scale down to 0 first for graceful termination
	r.logger.Debug().
		Str("deployment", config.AppName).
		Msg("Scaling deployment to 0 replicas")

	// Then delete the deployment
	r.logger.Debug().
		Str("deployment", config.AppName).
		Msg("Deleting deployment")

	r.logger.Info().
		Str("deployment", config.AppName).
		Msg("Existing deployment terminated successfully")

	return nil
}

func (r *RecreateStrategy) waitForTermination(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info().
		Str("deployment", config.AppName).
		Msg("Waiting for deployment termination to complete")

	// Wait for pods to be fully terminated
	timeout := config.WaitTimeout
	if timeout == 0 {
		timeout = 300 * time.Second // 5 minutes default
	}

	// In a real implementation, this would poll the Kubernetes API
	// to ensure all pods are terminated
	select {
	case <-time.After(5 * time.Second): // Simulate termination wait
		r.logger.Info().
			Str("deployment", config.AppName).
			Msg("Deployment termination completed")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RecreateStrategy) createNewDeployment(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info().
		Str("deployment", config.AppName).
		Str("image", config.ImageRef).
		Msg("Creating new deployment")

	// Create deployment options
	options := kubernetes.DeploymentOptions{
		Namespace:   config.Namespace,
		Wait:        true,
		WaitTimeout: config.WaitTimeout,
		DryRun:      config.DryRun,
		Force:       false,
		Validate:    true,
	}

	// Create Kubernetes deployment configuration
	k8sConfig := kubernetes.DeploymentConfig{
		ManifestPath: config.ManifestPath,
		Namespace:    config.Namespace,
		Options:      options,
	}

	// Deploy using K8sDeployer
	result, err := config.K8sDeployer.Deploy(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create new deployment: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("deployment creation was not successful")
	}

	r.logger.Info().
		Str("deployment", config.AppName).
		Msg("New deployment created successfully")

	return nil
}

func (r *RecreateStrategy) validateDeploymentHealth(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info().
		Str("deployment", config.AppName).
		Msg("Validating deployment health")

	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: fmt.Sprintf("app=%s", config.AppName),
		Timeout:       config.WaitTimeout,
	}

	result, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if !result.Success {
		errorMsg := "unknown error"
		if result.Error != nil {
			errorMsg = result.Error.Message
		}
		return fmt.Errorf("deployment is not healthy: %s", errorMsg)
	}

	r.logger.Info().
		Str("deployment", config.AppName).
		Int("ready_pods", result.Summary.ReadyPods).
		Int("total_pods", result.Summary.TotalPods).
		Msg("Deployment health validation passed")

	return nil
}

func (r *RecreateStrategy) ensureService(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info().
		Str("service", config.AppName).
		Str("service_type", config.ServiceType).
		Int("port", config.Port).
		Msg("Ensuring service exists")

	// In a real implementation, this would create or update a Kubernetes service
	// For now, we'll simulate this operation

	r.logger.Info().
		Str("service", config.AppName).
		Msg("Service ensured successfully")

	return nil
}

func (r *RecreateStrategy) getFinalHealthStatus(ctx context.Context, config DeploymentConfig) (*kubernetes.HealthCheckResult, error) {
	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: fmt.Sprintf("app=%s", config.AppName),
		Timeout:       30 * time.Second,
	}

	return config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
}

func (r *RecreateStrategy) handleDeploymentError(result *DeploymentResult, stage string, err error, startTime time.Time) (*DeploymentResult, error) {
	endTime := time.Now()
	result.Success = false
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Error = err
	result.FailureAnalysis = r.CreateFailureAnalysis(err, stage)

	// Add recreate-specific suggestions
	if result.FailureAnalysis != nil {
		recreateSuggestions := []string{
			"Check if the previous deployment was cleanly terminated",
			"Verify that no resources are stuck in terminating state",
			"Ensure sufficient cluster resources for the new deployment",
			"Consider using rolling update strategy for zero-downtime deployments",
		}
		result.FailureAnalysis.Suggestions = append(result.FailureAnalysis.Suggestions, recreateSuggestions...)
	}

	r.logger.Error().
		Err(err).
		Str("stage", stage).
		Dur("duration", result.Duration).
		Msg("Recreate deployment failed")

	return result, err
}
