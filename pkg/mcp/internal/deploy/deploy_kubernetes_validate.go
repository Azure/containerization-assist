package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
)

// performHealthCheck verifies deployment health
func (t *AtomicDeployKubernetesTool) performHealthCheck(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
	// Progress reporting removed

	healthStart := time.Now()
	timeout := 300 * time.Second // Default 5 minutes
	if args.WaitTimeout > 0 {
		timeout = time.Duration(args.WaitTimeout) * time.Second
	}

	// Check deployment health using pipeline adapter
	healthArgs := map[string]interface{}{
		"namespace":     args.Namespace,
		"labelSelector": "app=" + args.AppName,
		"timeout":       timeout,
	}
	healthResult, err := t.pipelineAdapter.CheckHealth(ctx, session.SessionID, healthArgs)
	result.HealthCheckDuration = time.Since(healthStart)

	// Convert from interface{} to kubernetes.HealthCheckResult
	if healthResult != nil {
		// Convert interface{} to expected structure
		if healthMap, ok := healthResult.(map[string]interface{}); ok {
			result.HealthResult = &kubernetes.HealthCheckResult{
				Success:   getBoolFromMap(healthMap, "healthy", false),
				Namespace: args.Namespace,
				Duration:  result.HealthCheckDuration,
			}

			// Handle error if present
			if errorData, exists := healthMap["error"]; exists && errorData != nil {
				if errorMap, ok := errorData.(map[string]interface{}); ok {
					result.HealthResult.Error = &kubernetes.HealthCheckError{
						Type:    getStringFromMap(errorMap, "type", "unknown"),
						Message: getStringFromMap(errorMap, "message", "unknown error"),
					}
				}
			}
		} else {
			// Fallback for unexpected result type
			result.HealthResult = &kubernetes.HealthCheckResult{
				Success:   false,
				Namespace: args.Namespace,
				Duration:  result.HealthCheckDuration,
			}
		}
	}

	if err != nil {
		_ = t.handleHealthCheckError(ctx, err, result.HealthResult, result)
		return err
	}

	// Check health result success through type assertion
	if healthResult != nil {
		if healthMap, ok := healthResult.(map[string]interface{}); ok && !getBoolFromMap(healthMap, "healthy", false) {
			var readyPods, totalPods int
			if result.HealthResult != nil {
				readyPods = result.HealthResult.Summary.ReadyPods
				totalPods = result.HealthResult.Summary.TotalPods
			}
			healthErr := fmt.Errorf("application health check failed: %d/%d pods ready", readyPods, totalPods)
			_ = t.handleHealthCheckError(ctx, healthErr, result.HealthResult, result)
			return healthErr
		}
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", args.Namespace).
		Str("app_name", args.AppName).
		Msg("Deployment health check passed")

	// Progress reporting removed

	return nil
}

// handleHealthCheckError creates an error for health check failures
func (t *AtomicDeployKubernetesTool) handleHealthCheckError(_ context.Context, err error, _ *kubernetes.HealthCheckResult, _ *AtomicDeployKubernetesResult) error {
	return fmt.Errorf("error")
}

// updateSessionState updates session with deployment results
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

// Validate validates the tool arguments (unified interface)
func (t *AtomicDeployKubernetesTool) Validate(_ context.Context, args interface{}) error {
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		return utils.NewWithData("invalid_arguments", "Invalid argument type for atomic_deploy_kubernetes", map[string]interface{}{
			"expected": "AtomicDeployKubernetesArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	if deployArgs.ImageRef == "" {
		return utils.NewWithData("missing_required_field", "ImageRef is required", map[string]interface{}{
			"field": "image_ref",
		})
	}

	if deployArgs.SessionID == "" {
		return utils.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}
