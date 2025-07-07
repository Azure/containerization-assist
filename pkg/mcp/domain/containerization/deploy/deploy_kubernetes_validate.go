package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// performHealthCheck verifies deployment health
func (t *AtomicDeployKubernetesTool) performHealthCheck(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
	// Progress reporting removed

	healthStart := time.Now()

	// Check deployment health using pipeline adapter
	// Convert to typed parameters for CheckHealthTyped
	healthParams := core.HealthCheckParams{
		Namespace:   args.Namespace,
		AppName:     args.AppName,
		Resources:   []string{}, // Default empty
		WaitTimeout: 300,        // 5 minutes default
	}
	healthResult, err := t.pipelineAdapter.CheckHealthTyped(ctx, session.SessionID, healthParams)
	result.HealthCheckDuration = time.Since(healthStart)

	// Convert from typed result to kubernetes.HealthCheckResult
	if healthResult != nil {
		// healthResult is already typed as *core.HealthCheckResult
		isHealthy := healthResult.OverallHealth == "healthy"
		result.HealthResult = &kubernetes.HealthCheckResult{
			Success:   isHealthy,
			Namespace: args.Namespace,
			Duration:  result.HealthCheckDuration,
		}

		// Handle error if present - check for unhealthy resources
		if !isHealthy && len(healthResult.UnhealthyResources) > 0 {
			result.HealthResult.Error = &kubernetes.HealthCheckError{
				Type:    "health_check_error",
				Message: fmt.Sprintf("Unhealthy resources: %v", healthResult.UnhealthyResources),
			}
		}
	}

	if err != nil {
		_ = t.handleHealthCheckError(ctx, err, result.HealthResult, result)
		return err
	}

	// Check health result success through typed result
	if healthResult != nil && healthResult.OverallHealth != "healthy" {
		var readyPods, totalPods int
		if result.HealthResult != nil {
			readyPods = result.HealthResult.Summary.ReadyPods
			totalPods = result.HealthResult.Summary.TotalPods
		}
		healthErr := errors.NewError().Messagef("application health check failed: %d/%d pods ready", readyPods, totalPods).WithLocation().Build()
		_ = t.handleHealthCheckError(ctx, healthErr, result.HealthResult, result)
		return healthErr
	}

	t.logger.Info("Deployment health check passed",
		"session_id", session.SessionID,
		"namespace", args.Namespace,
		"app_name", args.AppName)

	// Progress reporting removed

	return nil
}

// handleHealthCheckError creates an error for health check failures
func (t *AtomicDeployKubernetesTool) handleHealthCheckError(_ context.Context, err error, _ *kubernetes.HealthCheckResult, _ *AtomicDeployKubernetesResult) error {
	return errors.NewError().Messagef("error").WithLocation(

	// updateSessionState updates session with deployment results
	).Build()
}

func (t *AtomicDeployKubernetesTool) updateSessionState(session *core.SessionState, result *AtomicDeployKubernetesResult) error {
	// Update session with deployment results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	// Update session state fields (using Metadata since SessionState doesn't have these fields)
	if result.Success {
		session.Metadata["deployed"] = true
		session.Metadata["deployment_namespace"] = result.Namespace
		session.Metadata["deployment_name"] = result.AppName
	}

	// Update metadata for backward compatibility and additional details
	session.Metadata["last_deployed_image"] = result.ImageRef
	session.Metadata["last_deployment_namespace"] = result.Namespace
	session.Metadata["last_deployment_app"] = result.AppName
	session.Metadata["last_deployment_success"] = result.Success
	session.Metadata["deployed_image_ref"] = result.ImageRef
	// Note: deployment_namespace already set above in success case
	session.Metadata["deployment_app"] = result.AppName
	session.Metadata["deployment_success"] = result.Success

	if result.Success {
		session.Metadata["deployment_duration_seconds"] = result.TotalDuration.Seconds()
		session.Metadata["generation_duration_seconds"] = result.GenerationDuration.Seconds()
		if result.DeploymentDuration > 0 {
			session.Metadata["deploy_duration_seconds"] = result.DeploymentDuration.Seconds()
		}
		if result.HealthCheckDuration > 0 {
			session.Metadata["health_check_duration_seconds"] = result.HealthCheckDuration.Seconds()
		}
	}

	session.UpdatedAt = time.Now()

	// Use pipeline adapter to update session state
	return t.pipelineAdapter.UpdateSessionState(session.SessionID, func(s *core.SessionState) {
		*s = *session
	})
}
