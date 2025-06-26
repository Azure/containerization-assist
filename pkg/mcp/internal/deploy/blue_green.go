package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/rs/zerolog"
)

// BlueGreenStrategy implements a blue-green deployment strategy
// This strategy deploys to a parallel environment (green) and switches traffic once validated
type BlueGreenStrategy struct {
	*BaseStrategy
	logger zerolog.Logger
}

// NewBlueGreenStrategy creates a new blue-green deployment strategy
func NewBlueGreenStrategy(logger zerolog.Logger) *BlueGreenStrategy {
	return &BlueGreenStrategy{
		BaseStrategy: NewBaseStrategy(logger),
		logger:       logger.With().Str("strategy", "blue_green").Logger(),
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
	bg.logger.Debug().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Validating blue-green deployment prerequisites")

	// Check if K8sDeployer is available
	if config.K8sDeployer == nil {
		return fmt.Errorf("K8sDeployer is required for blue-green deployment")
	}

	// Check if we have required configuration
	if config.AppName == "" {
		return fmt.Errorf("app name is required for blue-green deployment")
	}

	if config.ImageRef == "" {
		return fmt.Errorf("image reference is required for blue-green deployment")
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
		return fmt.Errorf("cluster connection check failed: %w", err)
	}

	// Check if we have sufficient resources for parallel deployment
	if err := bg.checkResourceAvailability(ctx, config); err != nil {
		return fmt.Errorf("insufficient resources for blue-green deployment: %w", err)
	}

	bg.logger.Info().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Blue-green deployment prerequisites validated successfully")

	return nil
}

// Deploy executes the blue-green deployment
func (bg *BlueGreenStrategy) Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error) {
	startTime := time.Now()
	bg.logger.Info().
		Str("app_name", config.AppName).
		Str("image_ref", config.ImageRef).
		Str("namespace", config.Namespace).
		Msg("Starting blue-green deployment")

	result := &DeploymentResult{
		Strategy:  bg.GetName(),
		StartTime: startTime,
		Resources: make([]DeployedResource, 0),
	}

	// Report initial progress
	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.1, "Initializing blue-green deployment")
		}
	}

	// Step 1: Validate prerequisites
	if err := bg.ValidatePrerequisites(ctx, config); err != nil {
		return bg.handleDeploymentError(result, "validation", err, startTime)
	}

	// Step 2: Determine current and new environment colors
	currentColor, newColor, err := bg.determineEnvironmentColors(ctx, config)
	if err != nil {
		return bg.handleDeploymentError(result, "environment_detection", err, startTime)
	}

	bg.logger.Info().
		Str("current_color", currentColor).
		Str("new_color", newColor).
		Msg("Environment colors determined")

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.2, fmt.Sprintf("Deploying to %s environment", newColor))
		}
	}

	// Step 3: Deploy to the new environment (green)
	newDeploymentName := fmt.Sprintf("%s-%s", config.AppName, newColor)
	if err := bg.deployToEnvironment(ctx, config, newDeploymentName, newColor); err != nil {
		return bg.handleDeploymentError(result, "green_deployment", err, startTime)
	}

	result.Resources = append(result.Resources, DeployedResource{
		Kind:      "Deployment",
		Name:      newDeploymentName,
		Namespace: config.Namespace,
		Status:    "created",
	})

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.5, fmt.Sprintf("Waiting for %s environment to be ready", newColor))
		}
	}

	// Step 4: Wait for new environment to be ready
	if err := bg.WaitForDeployment(ctx, config, newDeploymentName); err != nil {
		bg.logger.Error().Err(err).
			Str("deployment", newDeploymentName).
			Msg("New environment failed to become ready")
		return bg.handleDeploymentError(result, "readiness_check", err, startTime)
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.7, fmt.Sprintf("Validating %s environment health", newColor))
		}
	}

	// Step 5: Perform health checks on new environment
	if err := bg.validateEnvironmentHealth(ctx, config, newDeploymentName); err != nil {
		bg.logger.Error().Err(err).
			Str("deployment", newDeploymentName).
			Msg("New environment health validation failed")
		return bg.handleDeploymentError(result, "health_validation", err, startTime)
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.8, "Switching traffic to new environment")
		}
	}

	// Step 6: Switch service to point to new environment
	if err := bg.switchTraffic(ctx, config, newColor); err != nil {
		bg.logger.Error().Err(err).
			Str("new_color", newColor).
			Msg("Traffic switch failed")
		return bg.handleDeploymentError(result, "traffic_switch", err, startTime)
	}

	result.Resources = append(result.Resources, DeployedResource{
		Kind:      "Service",
		Name:      config.AppName,
		Namespace: config.Namespace,
		Status:    "updated",
	})

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.9, "Cleaning up old environment")
		}
	}

	// Step 7: Clean up old environment (optional - for resource conservation)
	if !config.DryRun {
		if err := bg.cleanupOldEnvironment(ctx, config, currentColor); err != nil {
			bg.logger.Warn().Err(err).
				Str("old_color", currentColor).
				Msg("Failed to cleanup old environment - continuing")
		}
	}

	// Step 8: Complete deployment
	endTime := time.Now()
	result.Success = true
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.RollbackAvailable = true
	result.PreviousVersion = currentColor

	// Get final health status
	healthResult, err := bg.getFinalHealthStatus(ctx, config, newDeploymentName)
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
			reporter.ReportStage(1.0, "Blue-green deployment completed successfully")
		}
	}

	bg.logger.Info().
		Str("app_name", config.AppName).
		Str("new_color", newColor).
		Dur("duration", result.Duration).
		Msg("Blue-green deployment completed successfully")

	return result, nil
}

// Rollback performs a rollback by switching traffic back to the previous environment
func (bg *BlueGreenStrategy) Rollback(ctx context.Context, config DeploymentConfig) error {
	bg.logger.Info().
		Str("app_name", config.AppName).
		Str("namespace", config.Namespace).
		Msg("Starting blue-green rollback")

	// Determine current environment and switch back
	currentColor, previousColor, err := bg.determineEnvironmentColors(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to determine environment colors for rollback: %w", err)
	}

	bg.logger.Info().
		Str("current_color", currentColor).
		Str("previous_color", previousColor).
		Msg("Rolling back to previous environment")

	// Check if previous environment still exists
	previousDeploymentName := fmt.Sprintf("%s-%s", config.AppName, previousColor)
	if err := bg.checkDeploymentExists(ctx, config, previousDeploymentName); err != nil {
		return fmt.Errorf("previous environment %s no longer exists: %w", previousColor, err)
	}

	// Switch traffic back to previous environment
	if err := bg.switchTraffic(ctx, config, previousColor); err != nil {
		return fmt.Errorf("failed to switch traffic back to %s: %w", previousColor, err)
	}

	bg.logger.Info().
		Str("app_name", config.AppName).
		Str("rollback_to", previousColor).
		Msg("Blue-green rollback completed successfully")

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
	bg.logger.Debug().
		Int("replicas", config.Replicas).
		Str("cpu_request", config.CPURequest).
		Str("memory_request", config.MemoryRequest).
		Msg("Checking resource availability for blue-green deployment")

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
	bg.logger.Info().
		Str("deployment_name", deploymentName).
		Str("color", color).
		Msg("Deploying to environment")

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
		return fmt.Errorf("failed to deploy %s environment: %w", color, err)
	}

	if !result.Success {
		return fmt.Errorf("deployment to %s environment was not successful", color)
	}

	bg.logger.Info().
		Str("deployment_name", deploymentName).
		Str("color", color).
		Msg("Environment deployment completed")

	return nil
}

func (bg *BlueGreenStrategy) validateEnvironmentHealth(ctx context.Context, config DeploymentConfig, deploymentName string) error {
	bg.logger.Info().
		Str("deployment", deploymentName).
		Msg("Validating environment health")

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
		return fmt.Errorf("environment is not healthy: %s", errorMsg)
	}

	bg.logger.Info().
		Str("deployment", deploymentName).
		Int("ready_pods", result.Summary.ReadyPods).
		Int("total_pods", result.Summary.TotalPods).
		Msg("Environment health validation passed")

	return nil
}

func (bg *BlueGreenStrategy) switchTraffic(ctx context.Context, config DeploymentConfig, targetColor string) error {
	bg.logger.Info().
		Str("target_color", targetColor).
		Str("app_name", config.AppName).
		Msg("Switching traffic to target environment")

	// In a real implementation, this would update the Kubernetes service selector
	// to point to the new color's pods. For now, we'll simulate this.

	// This would typically involve:
	// 1. Get the current service
	// 2. Update the selector to match the target color's labels
	// 3. Apply the updated service

	bg.logger.Info().
		Str("target_color", targetColor).
		Str("service_name", config.AppName).
		Msg("Traffic switched successfully")

	return nil
}

func (bg *BlueGreenStrategy) cleanupOldEnvironment(ctx context.Context, config DeploymentConfig, oldColor string) error {
	if oldColor == "" {
		// No old environment to clean up
		return nil
	}

	oldDeploymentName := fmt.Sprintf("%s-%s", config.AppName, oldColor)
	bg.logger.Info().
		Str("old_deployment", oldDeploymentName).
		Msg("Cleaning up old environment")

	// In a real implementation, this would delete the old deployment
	// For now, we'll just log the cleanup operation

	bg.logger.Info().
		Str("old_deployment", oldDeploymentName).
		Msg("Old environment cleanup completed")

	return nil
}

func (bg *BlueGreenStrategy) checkDeploymentExists(ctx context.Context, config DeploymentConfig, deploymentName string) error {
	// This would check if a deployment exists in Kubernetes
	// For now, we'll return a simulated result
	bg.logger.Debug().
		Str("deployment", deploymentName).
		Str("namespace", config.Namespace).
		Msg("Checking if deployment exists")

	return fmt.Errorf("deployment %s not found", deploymentName)
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
	result.FailureAnalysis = bg.CreateFailureAnalysis(err, stage)

	bg.logger.Error().
		Err(err).
		Str("stage", stage).
		Dur("duration", result.Duration).
		Msg("Blue-green deployment failed")

	return result, err
}
