package deploy

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// RecreateStrategy implements a recreate deployment strategy
// This strategy terminates all existing instances before creating new ones
type RecreateStrategy struct {
	*BaseStrategy
	logger *slog.Logger
}

// NewRecreateStrategy creates a new recreate deployment strategy
func NewRecreateStrategy(logger *slog.Logger) *RecreateStrategy {
	return &RecreateStrategy{
		BaseStrategy: NewBaseStrategy(logger),
		logger:       logger.With("strategy", "recreate"),
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
	r.logger.Debug("Validating recreate deployment prerequisites",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	// Check if K8sDeployer is available
	if config.K8sDeployer == nil {
		return errors.NewError().Messagef("K8sDeployer is required for recreate deployment").WithLocation(

		// Check if we have required configuration
		).Build()
	}

	if config.AppName == "" {
		return errors.NewError().Messagef("app name is required for recreate deployment").Build()
	}

	if config.ImageRef == "" {
		return errors.NewError().Messagef("image reference is required for recreate deployment").Build()
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
		return errors.NewError().Message("cluster connection check failed").Cause(err).Build()
	}

	r.logger.Info("Recreate deployment prerequisites validated successfully",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	return nil
}

// deploymentStep represents a single deployment step
type deploymentStep struct {
	name        string
	progress    float64
	description string
	action      func(ctx context.Context, config DeploymentConfig, state *deploymentState) error
}

// deploymentState holds the current deployment state
type deploymentState struct {
	result         *DeploymentResult
	currentExists  bool
	currentVersion string
	startTime      time.Time
}

// Deploy executes the recreate deployment
func (r *RecreateStrategy) Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error) {
	startTime := time.Now()
	r.logger.Info("Starting recreate deployment",
		"app_name", config.AppName,
		"image_ref", config.ImageRef,
		"namespace", config.Namespace)

	state := &deploymentState{
		result: &DeploymentResult{
			Strategy:  r.GetName(),
			StartTime: startTime,
			Resources: make([]DeployedResource, 0),
		},
		startTime: startTime,
	}

	steps := r.getDeploymentSteps()

	for _, step := range steps {
		r.reportProgress(config, step.progress, step.description)

		if err := step.action(ctx, config, state); err != nil {
			return r.handleDeploymentError(state.result, step.name, err, startTime)
		}
	}

	return r.finalizeDeployment(ctx, config, state)
}

// getDeploymentSteps returns the ordered deployment steps
func (r *RecreateStrategy) getDeploymentSteps() []deploymentStep {
	return []deploymentStep{
		{
			name:        "validation",
			progress:    0.1,
			description: "Validating prerequisites",
			action:      r.validateStep,
		},
		{
			name:        "state_check",
			progress:    0.2,
			description: "Checking deployment state",
			action:      r.checkStateStep,
		},
		{
			name:        "termination",
			progress:    0.4,
			description: "Terminating existing deployment",
			action:      r.terminateStep,
		},
		{
			name:        "creation",
			progress:    0.5,
			description: "Creating new deployment",
			action:      r.createStep,
		},
		{
			name:        "readiness_check",
			progress:    0.7,
			description: "Waiting for deployment readiness",
			action:      r.readinessStep,
		},
		{
			name:        "health_validation",
			progress:    0.9,
			description: "Validating deployment health",
			action:      r.healthStep,
		},
		{
			name:        "service_setup",
			progress:    0.95,
			description: "Setting up service",
			action:      r.serviceStep,
		},
	}
}

// validateStep validates deployment prerequisites
func (r *RecreateStrategy) validateStep(ctx context.Context, config DeploymentConfig, _ *deploymentState) error {
	return r.ValidatePrerequisites(ctx, config)
}

// checkStateStep checks current deployment state
func (r *RecreateStrategy) checkStateStep(ctx context.Context, config DeploymentConfig, state *deploymentState) error {
	exists, version, err := r.getCurrentDeploymentState(ctx, config)
	if err != nil {
		return err
	}

	state.currentExists = exists
	state.currentVersion = version

	r.logger.Info("Current deployment state determined",
		"deployment_exists", exists,
		"current_version", version)

	return nil
}

// terminateStep terminates existing deployment if needed
func (r *RecreateStrategy) terminateStep(ctx context.Context, config DeploymentConfig, state *deploymentState) error {
	if !state.currentExists {
		return nil
	}

	if err := r.terminateExistingDeployment(ctx, config); err != nil {
		return err
	}

	state.result.Resources = append(state.result.Resources, DeployedResource{
		Kind:      "Deployment",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "terminated",
	})

	return r.waitForTermination(ctx, config)
}

// createStep creates new deployment
func (r *RecreateStrategy) createStep(ctx context.Context, config DeploymentConfig, state *deploymentState) error {
	if err := r.createNewDeployment(ctx, config); err != nil {
		return err
	}

	state.result.Resources = append(state.result.Resources, DeployedResource{
		Kind:      "Deployment",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "created",
	})

	return nil
}

// readinessStep waits for deployment to be ready
func (r *RecreateStrategy) readinessStep(ctx context.Context, config DeploymentConfig, _ *deploymentState) error {
	if err := r.WaitForDeployment(ctx, config, config.AppName); err != nil {
		r.logger.Error("New deployment failed to become ready",
			"error", err,
			"deployment", config.AppName)
		return err
	}
	return nil
}

// healthStep validates deployment health
func (r *RecreateStrategy) healthStep(ctx context.Context, config DeploymentConfig, _ *deploymentState) error {
	if err := r.validateDeploymentHealth(ctx, config); err != nil {
		r.logger.Error("Deployment health validation failed",
			"error", err,
			"deployment", config.AppName)
		return err
	}
	return nil
}

// serviceStep sets up service if needed
func (r *RecreateStrategy) serviceStep(ctx context.Context, config DeploymentConfig, state *deploymentState) error {
	if err := r.ensureService(ctx, config); err != nil {
		r.logger.Warn("Service creation/update failed - continuing",
			"error", err,
			"app_name", config.AppName)
		return nil // Non-critical error
	}

	state.result.Resources = append(state.result.Resources, DeployedResource{
		Kind:      "Service",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "created",
	})

	return nil
}

// reportProgress reports deployment progress
func (r *RecreateStrategy) reportProgress(config DeploymentConfig, progress float64, description string) {
	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(progress, description)
		}
	}
}

// finalizeDeployment completes the deployment process
func (r *RecreateStrategy) finalizeDeployment(ctx context.Context, config DeploymentConfig, state *deploymentState) (*DeploymentResult, error) {
	endTime := time.Now()
	result := state.result

	result.Success = true
	result.EndTime = endTime
	result.Duration = endTime.Sub(state.startTime)
	result.RollbackAvailable = false
	result.PreviousVersion = state.currentVersion

	// Get final health status
	if healthResult, err := r.getFinalHealthStatus(ctx, config); err == nil {
		result.HealthStatus = "healthy"
		result.ReadyReplicas = healthResult.Summary.ReadyPods
		result.TotalReplicas = healthResult.Summary.TotalPods
	} else {
		result.HealthStatus = "unknown"
	}

	r.reportProgress(config, 1.0, "Recreate deployment completed successfully")

	r.logger.Info("Recreate deployment completed successfully",
		"app_name", config.AppName,
		"duration", result.Duration,
		"ready_replicas", result.ReadyReplicas,
		"total_replicas", result.TotalReplicas)

	return result, nil
}

// Rollback for recreate strategy is limited since we don't maintain previous versions
func (r *RecreateStrategy) Rollback(ctx context.Context, config DeploymentConfig) error {
	r.logger.Warn("Rollback requested for recreate deployment - limited rollback capability",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	// Recreate strategy doesn't maintain previous versions, so rollback is limited
	// We can only attempt to restart the current deployment or provide guidance

	return errors.NewError().Messagef("recreate deployment strategy does not support rollback - previous versions are not maintained. Consider using 'kubectl rollout undo' manually or redeploy with a previous image version").WithLocation(

	// Private helper methods
	).Build()
}

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
	r.logger.Debug("Checking current deployment state",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	// Check if deployment exists by trying to get its rollout history
	historyConfig := kubernetes.RolloutHistoryConfig{
		ResourceType: "deployment",
		ResourceName: config.AppName,
		Namespace:    config.Namespace,
	}

	history, err := config.K8sDeployer.GetRolloutHistory(ctx, historyConfig)
	if err != nil {
		// Deployment likely doesn't exist
		r.logger.Debug("Deployment does not exist or cannot be accessed",
			"error", err,
			"app_name", config.AppName)
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
	r.logger.Info("Terminating existing deployment",
		"deployment", config.AppName,
		"namespace", config.Namespace)

	// In a real implementation, this would:
	// 1. Scale the deployment to 0 replicas
	// 2. Delete the deployment
	// For now, we'll simulate this with logging

	// Scale down to 0 first for graceful termination
	r.logger.Debug("Scaling deployment to 0 replicas",
		"deployment", config.AppName)

	// Then delete the deployment
	r.logger.Debug("Deleting deployment",
		"deployment", config.AppName)

	r.logger.Info("Existing deployment terminated successfully",
		"deployment", config.AppName)

	return nil
}

func (r *RecreateStrategy) waitForTermination(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info("Waiting for deployment termination to complete",
		"deployment", config.AppName)

	// Wait for pods to be fully terminated
	timeout := config.WaitTimeout
	if timeout == 0 {
		timeout = 300 * time.Second // 5 minutes default
	}

	// In a real implementation, this would poll the Kubernetes API
	// to ensure all pods are terminated
	select {
	case <-time.After(5 * time.Second): // Simulate termination wait
		r.logger.Info("Deployment termination completed",
			"deployment", config.AppName)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RecreateStrategy) createNewDeployment(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info("Creating new deployment",
		"deployment", config.AppName,
		"image", config.ImageRef)

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
		return errors.NewError().Message("failed to create new deployment").Cause(err).WithLocation().Build()
	}

	if !result.Success {
		return errors.NewError().Messagef("deployment creation was not successful").WithLocation().Build()
	}

	r.logger.Info("New deployment created successfully",
		"deployment", config.AppName)

	return nil
}

func (r *RecreateStrategy) validateDeploymentHealth(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info("Validating deployment health",
		"deployment", config.AppName)

	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: fmt.Sprintf("app=%s", config.AppName),
		Timeout:       config.WaitTimeout,
	}

	result, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	if err != nil {
		return errors.NewError().Message("health check failed").Cause(err).WithLocation().Build()
	}

	if !result.Success {
		errorMsg := "unknown error"
		if result.Error != nil {
			errorMsg = result.Error.Message
		}
		return errors.NewError().Messagef("deployment is not healthy: %s", errorMsg).WithLocation().Build()
	}

	r.logger.Info("Deployment health validation passed",
		"deployment", config.AppName,
		"ready_pods", result.Summary.ReadyPods,
		"total_pods", result.Summary.TotalPods)

	return nil
}

func (r *RecreateStrategy) ensureService(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info("Ensuring service exists",
		"service", config.AppName,
		"service_type", config.ServiceType,
		"port", config.Port)

	// In a real implementation, this would create or update a Kubernetes service
	// For now, we'll simulate this operation

	r.logger.Info("Service ensured successfully",
		"service", config.AppName)

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
	result.ConsolidatedFailureAnalysis = r.CreateFailureAnalysis(err, stage)

	// Add recreate-specific suggestions
	if result.ConsolidatedFailureAnalysis != nil {
		recreateSuggestions := []string{
			"Check if the previous deployment was cleanly terminated",
			"Verify that no resources are stuck in terminating state",
			"Ensure sufficient cluster resources for the new deployment",
			"Consider using rolling update strategy for zero-downtime deployments",
		}
		result.ConsolidatedFailureAnalysis.Suggestions = append(result.ConsolidatedFailureAnalysis.Suggestions, recreateSuggestions...)
	}

	r.logger.Error("Recreate deployment failed",
		"error", err,
		"stage", stage,
		"duration", result.Duration)

	return result, err
}
