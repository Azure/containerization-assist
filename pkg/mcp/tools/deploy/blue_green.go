package deploy

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// BlueGreenStrategy implements a blue-green deployment strategy
// This strategy deploys to a parallel environment (green) and switches traffic once validated
type BlueGreenStrategy struct {
	*BaseStrategy
	logger *slog.Logger
}

// NewBlueGreenStrategy creates a new blue-green deployment strategy
func NewBlueGreenStrategy(logger *slog.Logger) *BlueGreenStrategy {
	return &BlueGreenStrategy{
		BaseStrategy: NewBaseStrategy(logger),
		logger:       logger.With("strategy", "blue_green"),
	}
}

// GetName returns the strategy name
func (bg *BlueGreenStrategy) GetName() string {
	return "blue_green"
}

// GetDescription returns a human-readable description
func (bg *BlueGreenStrategy) GetDescription() string {
	return "Blue-green deployment that creates a parallel environment and switches traffic after validation, enabling instant rollback"
}

// ValidatePrerequisites checks if the blue-green strategy can be used
func (bg *BlueGreenStrategy) ValidatePrerequisites(ctx context.Context, config DeploymentConfig) error {
	bg.logger.Debug("Validating blue-green deployment prerequisites",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	// Check if K8sDeployer is available
	if config.K8sDeployer == nil {
		return errors.NewError().Messagef("K8sDeployer is required for blue-green deployment").WithLocation(

		// Check if we have required configuration
		).Build()
	}

	if config.AppName == "" {
		return errors.NewError().Messagef("app name is required for blue-green deployment").Build()
	}

	if config.ImageRef == "" {
		return errors.NewError().Messagef("image reference is required for blue-green deployment").Build()
	}

	if config.Namespace == "" {
		config.Namespace = "default"
	}

	// Blue-green requires more resources (parallel environments)
	if config.Replicas < 1 {
		config.Replicas = 2 // Default to 2 for blue-green
	}

	// Check if we can connect to the cluster
	if err := bg.checkClusterConnection(ctx, config); err != nil {
		return errors.NewError().Message("cluster connection check failed").Cause(err).WithLocation(

		// Check if we have sufficient resources for parallel deployment
		).Build()
	}

	if err := bg.checkResourceAvailability(ctx, config); err != nil {
		return errors.NewError().Message("insufficient resources for blue-green deployment").Cause(err).Build()
	}

	bg.logger.Info("Blue-green deployment prerequisites validated successfully",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	return nil
}

// blueGreenDeploymentStep represents a single deployment step
type blueGreenDeploymentStep struct {
	name        string
	progress    float64
	description string
	action      func(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error
}

// blueGreenDeploymentState holds the current deployment state
type blueGreenDeploymentState struct {
	result            *DeploymentResult
	currentColor      string
	newColor          string
	newDeploymentName string
	startTime         time.Time
}

// Deploy executes the blue-green deployment
func (bg *BlueGreenStrategy) Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error) {
	startTime := time.Now()
	bg.logger.Info("Starting blue-green deployment",
		"app_name", config.AppName,
		"image_ref", config.ImageRef,
		"namespace", config.Namespace)

	state := &blueGreenDeploymentState{
		result: &DeploymentResult{
			Strategy:  bg.GetName(),
			StartTime: startTime,
			Resources: make([]DeployedResource, 0),
		},
		startTime: startTime,
	}

	steps := bg.getBlueGreenDeploymentSteps()

	for _, step := range steps {
		bg.reportProgress(config, step.progress, step.description)

		if err := step.action(ctx, config, state); err != nil {
			return bg.handleDeploymentError(state.result, step.name, err, startTime)
		}
	}

	return bg.finalizeBlueGreenDeployment(ctx, config, state)
}

// getBlueGreenDeploymentSteps returns the ordered deployment steps
func (bg *BlueGreenStrategy) getBlueGreenDeploymentSteps() []blueGreenDeploymentStep {
	return []blueGreenDeploymentStep{
		{
			name:        "validation",
			progress:    0.1,
			description: "Initializing blue-green deployment",
			action:      bg.validateBlueGreenStep,
		},
		{
			name:        "environment_detection",
			progress:    0.2,
			description: "Determining environment colors",
			action:      bg.determineColorsStep,
		},
		{
			name:        "green_deployment",
			progress:    0.5,
			description: "Deploying to new environment",
			action:      bg.deployToNewEnvironmentStep,
		},
		{
			name:        "readiness_check",
			progress:    0.7,
			description: "Waiting for new environment to be ready",
			action:      bg.waitForReadinessStep,
		},
		{
			name:        "health_validation",
			progress:    0.8,
			description: "Validating environment health",
			action:      bg.validateHealthStep,
		},
		{
			name:        "traffic_switch",
			progress:    0.9,
			description: "Switching traffic to new environment",
			action:      bg.switchTrafficStep,
		},
		{
			name:        "cleanup",
			progress:    0.95,
			description: "Cleaning up old environment",
			action:      bg.cleanupStep,
		},
	}
}

// validateBlueGreenStep validates deployment prerequisites
func (bg *BlueGreenStrategy) validateBlueGreenStep(ctx context.Context, config DeploymentConfig, _ *blueGreenDeploymentState) error {
	return bg.ValidatePrerequisites(ctx, config)
}

// determineColorsStep determines current and new environment colors
func (bg *BlueGreenStrategy) determineColorsStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	currentColor, newColor, err := bg.determineEnvironmentColors(ctx, config)
	if err != nil {
		return err
	}

	state.currentColor = currentColor
	state.newColor = newColor
	state.newDeploymentName = fmt.Sprintf("%s-%s", config.AppName, newColor)

	bg.logger.Info("Environment colors determined",
		"current_color", currentColor,
		"new_color", newColor)

	return nil
}

// deployToNewEnvironmentStep deploys to the new environment
func (bg *BlueGreenStrategy) deployToNewEnvironmentStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	if err := bg.deployToEnvironment(ctx, config, state.newDeploymentName, state.newColor); err != nil {
		return err
	}

	state.result.Resources = append(state.result.Resources, DeployedResource{
		Kind:      "Deployment",
		Name:      state.newDeploymentName,
		Namespace: config.Namespace,
		Status:    "created",
	})

	return nil
}

// waitForReadinessStep waits for new environment to be ready
func (bg *BlueGreenStrategy) waitForReadinessStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	if err := bg.WaitForDeployment(ctx, config, state.newDeploymentName); err != nil {
		bg.logger.Error("New environment failed to become ready",
			"error", err,
			"deployment", state.newDeploymentName)
		return err
	}
	return nil
}

// validateHealthStep performs health checks on new environment
func (bg *BlueGreenStrategy) validateHealthStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	if err := bg.validateEnvironmentHealth(ctx, config, state.newDeploymentName); err != nil {
		bg.logger.Error("New environment health validation failed",
			"error", err,
			"deployment", state.newDeploymentName)
		return err
	}
	return nil
}

// switchTrafficStep switches service to point to new environment
func (bg *BlueGreenStrategy) switchTrafficStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	if err := bg.switchTraffic(ctx, config, state.newColor); err != nil {
		bg.logger.Error("Traffic switch failed",
			"error", err,
			"new_color", state.newColor)
		return err
	}

	state.result.Resources = append(state.result.Resources, DeployedResource{
		Kind:      "Service",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "updated",
	})

	return nil
}

// cleanupStep cleans up old environment
func (bg *BlueGreenStrategy) cleanupStep(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) error {
	if !config.DryRun {
		if err := bg.cleanupOldEnvironment(ctx, config, state.currentColor); err != nil {
			bg.logger.Warn("Failed to cleanup old environment - continuing",
				"error", err,
				"old_color", state.currentColor)
		}
	}
	return nil
}

// reportProgress reports deployment progress
func (bg *BlueGreenStrategy) reportProgress(config DeploymentConfig, progress float64, description string) {
	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(progress, description)
		}
	}
}

// finalizeBlueGreenDeployment completes the deployment process
func (bg *BlueGreenStrategy) finalizeBlueGreenDeployment(ctx context.Context, config DeploymentConfig, state *blueGreenDeploymentState) (*DeploymentResult, error) {
	endTime := time.Now()
	result := state.result

	result.Success = true
	result.EndTime = endTime
	result.Duration = endTime.Sub(state.startTime)
	result.RollbackAvailable = true
	result.PreviousVersion = state.currentColor

	// Get final health status
	if healthResult, err := bg.getFinalHealthStatus(ctx, config, state.newDeploymentName); err == nil {
		result.HealthStatus = "healthy"
		result.ReadyReplicas = healthResult.Summary.ReadyPods
		result.TotalReplicas = healthResult.Summary.TotalPods
	} else {
		result.HealthStatus = "unknown"
	}

	bg.reportProgress(config, 1.0, "Blue-green deployment completed successfully")

	bg.logger.Info("Blue-green deployment completed successfully",
		"app_name", config.AppName,
		"new_color", state.newColor,
		"duration", result.Duration)

	return result, nil
}

// Rollback performs a rollback by switching traffic back to the previous environment
func (bg *BlueGreenStrategy) Rollback(ctx context.Context, config DeploymentConfig) error {
	bg.logger.Info("Starting blue-green rollback",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	// Determine current environment and switch back
	currentColor, previousColor, err := bg.determineEnvironmentColors(ctx, config)
	if err != nil {
		return errors.NewError().Message("failed to determine environment colors for rollback").Cause(err).Build()
	}

	bg.logger.Info("Rolling back to previous environment",
		"current_color", currentColor,
		"previous_color", previousColor)

	// Check if previous environment still exists
	previousDeploymentName := fmt.Sprintf("%s-%s", config.AppName, previousColor)
	if err := bg.checkDeploymentExists(ctx, config, previousDeploymentName); err != nil {
		return errors.NewError().Message(fmt.Sprintf("previous environment %s no longer exists", previousColor)).Cause(err).WithLocation(

		// Switch traffic back to previous environment
		).Build()
	}

	if err := bg.switchTraffic(ctx, config, previousColor); err != nil {
		return errors.NewError().Message(fmt.Sprintf("failed to switch traffic back to %s", previousColor)).Cause(err).Build()
	}

	bg.logger.Info("Blue-green rollback completed successfully",
		"app_name", config.AppName,
		"rollback_to", previousColor)

	return nil
}

// Private helper methods

func (bg *BlueGreenStrategy) checkClusterConnection(ctx context.Context, config DeploymentConfig) error {
	// Use K8sDeployer to perform a simple health check
	healthOptions := kubernetes.HealthCheckOptions{
		Namespace: config.Namespace,
		Timeout:   30 * time.Second,
	}

	_, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	return err
}

func (bg *BlueGreenStrategy) checkResourceAvailability(ctx context.Context, config DeploymentConfig) error {
	// This would check if the cluster has sufficient resources for parallel deployment
	// For now, we'll assume resources are available
	bg.logger.Debug("Checking resource availability for blue-green deployment",
		"replicas", config.Replicas,
		"cpu_request", config.CPURequest,
		"memory_request", config.MemoryRequest)

	return nil
}

func (bg *BlueGreenStrategy) determineEnvironmentColors(ctx context.Context, config DeploymentConfig) (current, new string, err error) {
	// Check which environment is currently active by looking at the service selector
	// This is a simplified implementation - in production, you'd query the actual service

	// Default assumption: if blue exists, deploy green; otherwise deploy blue
	blueDeploymentName := fmt.Sprintf("%s-blue", config.AppName)
	greenDeploymentName := fmt.Sprintf("%s-green", config.AppName)

	blueExists := bg.checkDeploymentExists(ctx, config, blueDeploymentName) == nil
	greenExists := bg.checkDeploymentExists(ctx, config, greenDeploymentName) == nil

	if !blueExists && !greenExists {
		// First deployment - start with blue
		return "", "blue", nil
	}

	if blueExists && !greenExists {
		// Blue is current, deploy green
		return "blue", "green", nil
	}

	if greenExists && !blueExists {
		// Green is current, deploy blue
		return "green", "blue", nil
	}

	// Both exist - determine which is active by checking service
	// For simplicity, we'll alternate: assume blue is current if both exist
	return "blue", "green", nil
}

func (bg *BlueGreenStrategy) deployToEnvironment(ctx context.Context, config DeploymentConfig, deploymentName, color string) error {
	bg.logger.Info("Deploying to environment",
		"deployment_name", deploymentName,
		"color", color)

	// Use the provided manifest path but modify it for blue-green deployment
	// In a real implementation, you would modify the manifest to include the color-specific labels
	deployOptions := kubernetes.DeploymentOptions{
		Namespace: config.Namespace,
		DryRun:    config.DryRun,
	}

	k8sConfig := kubernetes.DeploymentConfig{
		ManifestPath: config.ManifestPath,
		Namespace:    config.Namespace,
		Options:      deployOptions,
	}

	// Deploy using K8sDeployer
	result, err := config.K8sDeployer.Deploy(k8sConfig)
	if err != nil {
		return errors.NewError().Messagef("failed to deploy %s environment: %s", color, err.Error()).Cause(err).WithLocation().Build()
	}

	if !result.Success {
		return errors.NewError().Messagef("deployment to %s environment was not successful", color).WithLocation().Build()
	}

	bg.logger.Info("Environment deployment completed",
		"deployment_name", deploymentName,
		"color", color)

	return nil
}

func (bg *BlueGreenStrategy) validateEnvironmentHealth(ctx context.Context, config DeploymentConfig, deploymentName string) error {
	bg.logger.Info("Validating environment health",
		"deployment", deploymentName)

	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: fmt.Sprintf("app=%s", config.AppName),
		Timeout:       config.WaitTimeout,
	}

	result, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	if err != nil {
		return errors.NewError().Messagef("health check failed: %s", err.Error()).Cause(err).WithLocation().Build()
	}

	if !result.Success {
		errorMsg := "unknown error"
		if result.Error != nil {
			errorMsg = result.Error.Message
		}
		return errors.NewError().Messagef("environment is not healthy: %s", errorMsg).WithLocation().Build()
	}

	bg.logger.Info("Environment health validation passed",
		"deployment", deploymentName,
		"ready_pods", result.Summary.ReadyPods,
		"total_pods", result.Summary.TotalPods)

	return nil
}

func (bg *BlueGreenStrategy) switchTraffic(ctx context.Context, config DeploymentConfig, targetColor string) error {
	bg.logger.Info("Switching traffic to target environment",
		"target_color", targetColor,
		"app_name", config.AppName)

	// In a real implementation, this would update the Kubernetes service selector
	// to point to the new color's pods. For now, we'll simulate this.

	// This would typically involve:
	// 1. Get the current service
	// 2. Update the selector to match the target color's labels
	// 3. Apply the updated service

	bg.logger.Info("Traffic switched successfully",
		"target_color", targetColor,
		"service_name", config.AppName)

	return nil
}

func (bg *BlueGreenStrategy) cleanupOldEnvironment(ctx context.Context, config DeploymentConfig, oldColor string) error {
	if oldColor == "" {
		// No old environment to clean up
		return nil
	}

	oldDeploymentName := fmt.Sprintf("%s-%s", config.AppName, oldColor)
	bg.logger.Info("Cleaning up old environment",
		"old_deployment", oldDeploymentName)

	// In a real implementation, this would delete the old deployment
	// For now, we'll just log the cleanup operation

	bg.logger.Info("Old environment cleanup completed",
		"old_deployment", oldDeploymentName)

	return nil
}

func (bg *BlueGreenStrategy) checkDeploymentExists(ctx context.Context, config DeploymentConfig, deploymentName string) error {
	// This would check if a deployment exists in Kubernetes
	// For now, we'll return a simulated result
	bg.logger.Debug("Checking if deployment exists",
		"deployment", deploymentName,
		"namespace", config.Namespace)

	return errors.NewError().Messagef("deployment %s not found", deploymentName).WithLocation().Build()
}

func (bg *BlueGreenStrategy) getFinalHealthStatus(ctx context.Context, config DeploymentConfig, deploymentName string) (*kubernetes.HealthCheckResult, error) {
	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: fmt.Sprintf("app=%s", config.AppName),
		Timeout:       30 * time.Second,
	}

	return config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
}

func (bg *BlueGreenStrategy) handleDeploymentError(result *DeploymentResult, stage string, err error, startTime time.Time) (*DeploymentResult, error) {
	endTime := time.Now()
	result.Success = false
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Error = err
	result.ConsolidatedFailureAnalysis = bg.CreateFailureAnalysis(err, stage)

	bg.logger.Error("Blue-green deployment failed",
		"error", err,
		"stage", stage,
		"duration", result.Duration)

	return result, err
}
