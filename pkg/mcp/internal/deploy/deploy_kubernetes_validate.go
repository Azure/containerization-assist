package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
)

// performHealthCheck verifies deployment health
func (t *AtomicDeployKubernetesTool) performHealthCheck(ctx context.Context, session *sessiontypes.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
	// Progress reporting removed

	healthStart := time.Now()
	timeout := 300 * time.Second // Default 5 minutes
	if args.WaitTimeout > 0 {
		timeout = time.Duration(args.WaitTimeout) * time.Second
	}

	// Check deployment health using pipeline adapter
	healthResult, err := t.pipelineAdapter.CheckApplicationHealth(
		session.SessionID,
		args.Namespace,
		"app="+args.AppName, // label selector
		timeout,
	)
	result.HealthCheckDuration = time.Since(healthStart)

	// Convert from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
	if healthResult != nil {
		result.HealthResult = &kubernetes.HealthCheckResult{
			Success:   healthResult.Healthy,
			Namespace: args.Namespace,
			Duration:  result.HealthCheckDuration,
		}
		if healthResult.Error != nil {
			result.HealthResult.Error = &kubernetes.HealthCheckError{
				Type:    healthResult.Error.Type,
				Message: healthResult.Error.Message,
			}
		}
		// Convert pod statuses
		for _, ps := range healthResult.PodStatuses {
			podStatus := kubernetes.DetailedPodStatus{
				Name:      ps.Name,
				Namespace: args.Namespace,
				Status:    ps.Status,
				Ready:     ps.Ready,
			}
			result.HealthResult.Pods = append(result.HealthResult.Pods, podStatus)
		}
		// Update summary
		result.HealthResult.Summary = kubernetes.HealthSummary{
			TotalPods:   len(result.HealthResult.Pods),
			ReadyPods:   0,
			FailedPods:  0,
			PendingPods: 0,
		}
		for _, pod := range result.HealthResult.Pods {
			if pod.Ready {
				result.HealthResult.Summary.ReadyPods++
			} else if pod.Status == "Failed" || pod.Phase == "Failed" {
				result.HealthResult.Summary.FailedPods++
			} else if pod.Status == "Pending" || pod.Phase == "Pending" {
				result.HealthResult.Summary.PendingPods++
			}
		}
		if result.HealthResult.Summary.TotalPods > 0 {
			result.HealthResult.Summary.HealthyRatio = float64(result.HealthResult.Summary.ReadyPods) / float64(result.HealthResult.Summary.TotalPods)
		}
	}

	if err != nil {
		_ = t.handleHealthCheckError(ctx, err, result.HealthResult, result)
		return err
	}

	if healthResult != nil && !healthResult.Healthy {
		var readyPods, totalPods int
		if result.HealthResult != nil {
			readyPods = result.HealthResult.Summary.ReadyPods
			totalPods = result.HealthResult.Summary.TotalPods
		}
		healthErr := types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("deployment health check failed: %d/%d pods ready", readyPods, totalPods), "health_check_error")
		_ = t.handleHealthCheckError(ctx, healthErr, result.HealthResult, result)
		return healthErr
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
	return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("health check failed: %v", err), "health_check_error")
}

// updateSessionState updates session with deployment results
func (t *AtomicDeployKubernetesTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicDeployKubernetesResult) error {
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
	session.Metadata["deployment_namespace"] = result.Namespace
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

	session.UpdateLastAccessed()

	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
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
