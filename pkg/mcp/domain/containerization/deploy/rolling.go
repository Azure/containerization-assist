package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

type RollingUpdateStrategy struct {
	*BaseStrategy
	logger *slog.Logger
}

func NewRollingUpdateStrategy(logger *slog.Logger) *RollingUpdateStrategy {
	return &RollingUpdateStrategy{
		BaseStrategy: NewBaseStrategy(logger),
		logger:       logger.With("strategy", "rolling"),
	}
}

func (r *RollingUpdateStrategy) GetName() string {
	return "rolling"
}

func (r *RollingUpdateStrategy) GetDescription() string {
	return "Rolling update deployment that gradually replaces old instances with new ones, ensuring zero downtime"
}

func (r *RollingUpdateStrategy) ValidatePrerequisites(ctx context.Context, config DeploymentConfig) error {
	r.logger.Debug("Validating rolling update prerequisites",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	if config.K8sDeployer == nil {
		return errors.NewError().Messagef("K8sDeployer is required for rolling update deployment").Build()
	}

	if config.AppName == "" {
		return errors.NewError().Messagef("app name is required for rolling update deployment").Build()
	}

	if config.ImageRef == "" {
		return errors.NewError().Messagef("image reference is required for rolling update deployment").Build()
	}

	if config.Namespace == "" {
		config.Namespace = "default"
	}

	if err := r.checkClusterConnection(ctx, config); err != nil {
		return errors.NewError().Message("cluster connection check failed").Cause(err).Build()
	}

	r.logger.Info("Rolling update prerequisites validated successfully",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	return nil
}

func (r *RollingUpdateStrategy) Deploy(ctx context.Context, config DeploymentConfig) (*DeploymentResult, error) {
	startTime := time.Now()
	r.logger.Info("Starting rolling update deployment",
		"app_name", config.AppName,
		"image_ref", config.ImageRef,
		"namespace", config.Namespace)

	result := &DeploymentResult{
		Strategy:  r.GetName(),
		StartTime: startTime,
		Resources: make([]DeployedResource, 0),
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.1, "Initializing rolling update")
		}
	}

	if err := r.ValidatePrerequisites(ctx, config); err != nil {
		return r.handleDeploymentError(result, "validation", err, startTime)
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.2, "Checking existing deployment")
		}
	}

	previousVersion, rollbackAvailable, err := r.getPreviousVersion(ctx, config)
	if err != nil {
		r.logger.Warn("Could not retrieve previous version information", "error", err)
	}
	result.PreviousVersion = previousVersion
	result.RollbackAvailable = rollbackAvailable

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.4, "Applying manifest updates")
		}
	}

	deploymentResult, err := r.performRollingUpdate(ctx, config)
	if err != nil {
		return r.handleDeploymentError(result, "deployment", err, startTime)
	}

	result.Resources = r.extractDeployedResources(deploymentResult)

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.6, "Waiting for rollout completion")
		}
	}

	if err := r.waitForRolloutCompletion(ctx, config); err != nil {
		return r.handleDeploymentError(result, "rollout", err, startTime)
	}

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(0.8, "Performing health checks")
		}
	}

	healthStatus, readyReplicas, totalReplicas, err := r.performHealthChecks(ctx, config)
	if err != nil {
		return r.handleDeploymentError(result, "health_check", err, startTime)
	}

	result.HealthStatus = healthStatus
	result.ReadyReplicas = readyReplicas
	result.TotalReplicas = totalReplicas

	if config.ProgressReporter != nil {
		if reporter, ok := config.ProgressReporter.(interface {
			ReportStage(float64, string)
		}); ok {
			reporter.ReportStage(1.0, "Rolling update completed successfully")
		}
	}

	result.Success = true
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	r.logger.Info("Rolling update deployment completed successfully",
		"app_name", config.AppName,
		"duration", result.Duration,
		"ready_replicas", result.ReadyReplicas,
		"total_replicas", result.TotalReplicas)

	return result, nil
}

func (r *RollingUpdateStrategy) Rollback(ctx context.Context, config DeploymentConfig) error {
	r.logger.Info("Starting rollback operation",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	previousVersion, rollbackAvailable, err := r.getPreviousVersion(ctx, config)
	if err != nil {
		return errors.NewError().Message("failed to check rollback availability").Cause(err).Build()
	}

	if !rollbackAvailable {
		return errors.NewError().Messagef("no previous version available for rollback").Build()
	}

	r.logger.Info("Rolling back to previous version",
		"previous_version", previousVersion)

	if err := r.performRollback(ctx, config); err != nil {
		return errors.NewError().Message("rollback failed").Cause(err).Build()
	}

	if err := r.waitForRolloutCompletion(ctx, config); err != nil {
		return errors.NewError().Message("rollback completion failed").Cause(err).Build()
	}

	healthStatus, readyReplicas, totalReplicas, err := r.performHealthChecks(ctx, config)
	if err != nil {
		return errors.NewError().Message("rollback health check failed").Cause(err).Build()
	}

	r.logger.Info("Rollback completed successfully",
		"health_status", healthStatus,
		"ready_replicas", readyReplicas,
		"total_replicas", totalReplicas)

	return nil
}

func (r *RollingUpdateStrategy) performRollingUpdate(ctx context.Context, config DeploymentConfig) (*kubernetes.DeploymentResult, error) {
	r.logger.Debug("Performing rolling update deployment",
		"manifest_path", config.ManifestPath,
		"namespace", config.Namespace)

	options := kubernetes.DeploymentOptions{
		Namespace:   config.Namespace,
		Wait:        true,
		WaitTimeout: config.WaitTimeout,
		DryRun:      config.DryRun,
		Force:       false,
		Validate:    true,
	}

	deploymentConfig := kubernetes.DeploymentConfig{
		ManifestPath: config.ManifestPath,
		Namespace:    config.Namespace,
		Options:      options,
	}

	return config.K8sDeployer.Deploy(deploymentConfig)
}

func (r *RollingUpdateStrategy) waitForRolloutCompletion(ctx context.Context, config DeploymentConfig) error {
	r.logger.Debug("Waiting for rollout completion",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	timeout := config.WaitTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rolloutConfig := kubernetes.RolloutConfig{
		ResourceType: "deployment",
		ResourceName: config.AppName,
		Namespace:    config.Namespace,
		Timeout:      timeout,
	}

	return config.K8sDeployer.WaitForRollout(timeoutCtx, rolloutConfig)
}

func (r *RollingUpdateStrategy) performHealthChecks(ctx context.Context, config DeploymentConfig) (string, int, int, error) {
	r.logger.Debug("Performing deployment health checks",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	healthOptions := kubernetes.HealthCheckOptions{
		Namespace:       config.Namespace,
		LabelSelector:   "app=" + config.AppName,
		IncludeEvents:   false,
		IncludeServices: false,
		Timeout:         config.WaitTimeout,
	}

	result, err := config.K8sDeployer.CheckApplicationHealth(ctx, healthOptions)
	if err != nil {
		return "unhealthy", 0, 0, errors.NewError().Message("health check failed").Cause(err).WithLocation().Build()
	}

	status := "healthy"
	if !result.Success {
		status = "unhealthy"
	}

	readyReplicas := result.Summary.ReadyPods
	totalReplicas := result.Summary.TotalPods

	r.logger.Info("Health check completed",
		"health_status", status,
		"ready_replicas", readyReplicas,
		"total_replicas", totalReplicas)

	return status, readyReplicas, totalReplicas, nil
}

func (r *RollingUpdateStrategy) getPreviousVersion(ctx context.Context, config DeploymentConfig) (string, bool, error) {
	r.logger.Debug("Checking previous version information",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	historyConfig := kubernetes.RolloutHistoryConfig{
		ResourceType: "deployment",
		ResourceName: config.AppName,
		Namespace:    config.Namespace,
	}

	history, err := config.K8sDeployer.GetRolloutHistory(ctx, historyConfig)
	if err != nil {
		return "", false, errors.NewError().Message("failed to get rollout history").Cause(err).WithLocation().Build()
	}

	if len(history.Revisions) < 2 {
		return "", false, nil
	}

	previousRevision := history.Revisions[len(history.Revisions)-2]
	return fmt.Sprintf("revision-%d", previousRevision.Number), true, nil
}

func (r *RollingUpdateStrategy) performRollback(ctx context.Context, config DeploymentConfig) error {
	r.logger.Debug("Executing rollback operation",
		"app_name", config.AppName,
		"namespace", config.Namespace)

	rollbackConfig := kubernetes.RollbackConfig{
		ResourceType: "deployment",
		ResourceName: config.AppName,
		Namespace:    config.Namespace,
	}

	return config.K8sDeployer.RollbackDeployment(ctx, rollbackConfig)
}

func (r *RollingUpdateStrategy) checkClusterConnection(ctx context.Context, config DeploymentConfig) error {
	testConfig := kubernetes.HealthCheckOptions{
		Namespace:     config.Namespace,
		LabelSelector: "app=test-connection",
		Timeout:       10 * time.Second,
	}

	_, err := config.K8sDeployer.CheckApplicationHealth(ctx, testConfig)
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no resources found") {
		return err
	}
	return nil
}

func (r *RollingUpdateStrategy) extractDeployedResources(deploymentResult *kubernetes.DeploymentResult) []DeployedResource {
	resources := make([]DeployedResource, 0)

	if deploymentResult == nil {
		return resources
	}

	for _, kubeResource := range deploymentResult.Resources {
		resource := DeployedResource{
			Kind:      kubeResource.Kind,
			Name:      kubeResource.Name,
			Namespace: kubeResource.Namespace,
			Status:    kubeResource.Status,
		}

		if kubeResource.Status != "" {
			resource.APIVersion = "apps/v1"
		}

		resources = append(resources, resource)
	}

	return resources
}

func (r *RollingUpdateStrategy) handleDeploymentError(result *DeploymentResult, stage string, err error, startTime time.Time) (*DeploymentResult, error) {
	result.Success = false
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	result.Error = err

	result.ConsolidatedFailureAnalysis = r.createFailureAnalysis(err, stage)

	r.logger.Error("Rolling update deployment failed",
		"error", err,
		"stage", stage,
		"duration", result.Duration)

	return result, nil
}

func (r *RollingUpdateStrategy) createFailureAnalysis(err error, stage string) *ConsolidatedFailureAnalysis {
	analysis := &ConsolidatedFailureAnalysis{
		Stage:    stage,
		Reason:   "deployment_failed",
		Message:  err.Error(),
		CanRetry: true,
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "unable to connect"):
		analysis.Reason = "cluster_connection_failed"
		analysis.Suggestions = []string{
			"Check if the Kubernetes cluster is running and accessible",
			"Verify kubectl configuration and current context",
			"Check network connectivity to the cluster",
			"Ensure cluster certificates are valid and not expired",
		}
		analysis.CanRollback = false

	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden"):
		analysis.Reason = "insufficient_permissions"
		analysis.Suggestions = []string{
			"Check RBAC permissions for the service account",
			"Verify authentication credentials",
			"Ensure proper ClusterRole/Role bindings are configured",
			"Check if the namespace exists and is accessible",
		}
		analysis.CanRollback = stage != "validation"

	case strings.Contains(errStr, "not found") && strings.Contains(errStr, "namespace"):
		analysis.Reason = "namespace_not_found"
		analysis.Suggestions = []string{
			"Create the target namespace before deployment",
			"Verify the namespace name is correct",
			"Check if you have permissions to access the namespace",
		}
		analysis.CanRollback = false

	case strings.Contains(errStr, "image") && (strings.Contains(errStr, "pull") || strings.Contains(errStr, "not found")):
		analysis.Reason = "image_pull_failed"
		analysis.Suggestions = []string{
			"Verify the image reference is correct and accessible",
			"Check image registry authentication",
			"Ensure the image exists in the specified registry",
			"Verify network connectivity to the image registry",
		}
		analysis.CanRollback = stage != "validation"

	case strings.Contains(errStr, "timeout"):
		analysis.Reason = "deployment_timeout"
		analysis.Suggestions = []string{
			"Increase the wait timeout duration",
			"Check if resources are sufficient for the deployment",
			"Verify pod startup time and resource requirements",
			"Check for any blocking conditions in the cluster",
		}
		analysis.CanRollback = stage != "validation"

	case strings.Contains(errStr, "quota") || strings.Contains(errStr, "limit"):
		analysis.Reason = "resource_quota_exceeded"
		analysis.Suggestions = []string{
			"Check resource quotas in the namespace",
			"Reduce resource requests/limits in the manifest",
			"Scale down other applications to free up resources",
			"Request quota increase from cluster administrator",
		}
		analysis.CanRollback = stage != "validation"

	default:
		analysis.Suggestions = []string{
			"Check the deployment manifest for syntax errors",
			"Verify all required fields are specified",
			"Review cluster events for additional context",
			"Check pod logs for application-specific errors",
		}
		analysis.CanRollback = stage != "validation"
	}

	return analysis
}
