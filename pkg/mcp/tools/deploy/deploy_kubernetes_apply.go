package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// performDeployment deploys manifests to Kubernetes cluster
func (t *AtomicDeployKubernetesTool) performDeployment(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
	// Progress reporting removed

	deploymentStart := time.Now()

	// Deploy to Kubernetes using pipeline adapter
	// Get manifests from result
	manifests := []string{}
	if result.ManifestResult != nil {
		for _, manifest := range result.ManifestResult.Manifests {
			manifests = append(manifests, manifest.Path)
		}
	}
	// Deploy using typed interface
	// Convert to typed parameters for DeployKubernetesTyped
	deployParams := core.DeployParams{
		SessionID:     session.SessionID,
		ManifestPaths: manifests, // Use the manifests variable
		Namespace:     args.Namespace,
		DryRun:        args.DryRun,
		Wait:          args.WaitForReady,
		Timeout:       args.WaitTimeout,
	}
	deployResult, err := t.pipelineAdapter.DeployKubernetesTyped(ctx, session.SessionID, deployParams)
	result.DeploymentDuration = time.Since(deploymentStart)

	// Convert from typed result to kubernetes.DeploymentResult
	if deployResult != nil {
		// deployResult is already typed as *core.DeployResult
		result.DeploymentResult = &kubernetes.DeploymentResult{
			Success:   true, // Success since no error was returned
			Namespace: deployResult.Namespace,
		}
	}

	if err != nil {
		_ = t.handleDeploymentError(ctx, err, result.DeploymentResult, result)
		return err
	}

	// Deployment success is already handled above

	t.logger.Info("Kubernetes deployment completed successfully",
		"session_id", session.SessionID,
		"namespace", args.Namespace)

	// Progress reporting removed

	return nil
}

// handleDeploymentError creates an error for deployment failures and enriches the result with failure analysis
func (t *AtomicDeployKubernetesTool) handleDeploymentError(_ context.Context, err error, _ *kubernetes.DeploymentResult, result *AtomicDeployKubernetesResult) error {
	// Analyze the deployment error
	richError, analysisErr := analyzeDeploymentError(err)
	if analysisErr != nil {
		// Log analysis error but continue with basic error handling
		// TODO: Add proper logging when logger is available
	}

	// Create failure analysis
	result.ConsolidatedFailureAnalysis = &DeploymentFailureAnalysis{
		FailureType:    "deployment_error",
		FailureStage:   "deployment",
		RootCauses:     []string{err.Error()},
		ImpactSeverity: "high",
	}

	// Default actions for deployment errors
	result.ConsolidatedFailureAnalysis.ImmediateActions = []DeploymentRemediationAction{
		{
			Priority:    1,
			Action:      "Verify image exists",
			Command:     fmt.Sprintf("docker pull %s", result.ImageRef),
			Description: "Check if the image exists and can be pulled",
			Expected:    "Image should be successfully pulled",
			RiskLevel:   "low",
		},
		{
			Priority:    2,
			Action:      "Check registry credentials",
			Command:     "kubectl get secret -n " + result.Namespace,
			Description: "Verify registry credentials are configured",
			Expected:    "Registry secrets should be present",
			RiskLevel:   "low",
		},
	}

	// Add diagnostic commands
	result.ConsolidatedFailureAnalysis.DiagnosticCommands = []DiagnosticCommand{
		{
			Purpose:     "Check deployment status",
			Command:     fmt.Sprintf("kubectl describe deployment %s -n %s", result.AppName, result.Namespace),
			Explanation: "Shows detailed deployment status and events",
		},
		{
			Purpose:     "Check pod status",
			Command:     fmt.Sprintf("kubectl get pods -n %s -l app=%s", result.Namespace, result.AppName),
			Explanation: "Lists pods and their current state",
		},
		{
			Purpose:     "View pod logs",
			Command:     fmt.Sprintf("kubectl logs -n %s -l app=%s --tail=50", result.Namespace, result.AppName),
			Explanation: "Shows recent application logs",
		},
	}

	// Update deployment context with error details
	if result.DeploymentContext != nil {
		result.DeploymentContext.DeploymentStatus = "failed"
		result.DeploymentContext.DeploymentErrors = append(result.DeploymentContext.DeploymentErrors, richError.Error())

		// Add troubleshooting tips
		result.DeploymentContext.TroubleshootingTips = []string{
			"Check if the namespace exists: kubectl get namespace " + result.Namespace,
			"Verify image pull policy and registry access",
			"Check resource quotas: kubectl describe resourcequota -n " + result.Namespace,
			"Review recent events: kubectl get events -n " + result.Namespace + " --sort-by=.lastTimestamp",
		}
	}

	return errors.NewError().Messagef("error").WithLocation(

	// KubernetesDeployOperation implements ConsolidatedFixableOperation for Kubernetes deployments
	).Build()
}

type KubernetesDeployOperation struct {
	tool      *AtomicDeployKubernetesTool
	args      AtomicDeployKubernetesArgs
	session   *core.SessionState
	namespace string
	manifests []string
	logger    *slog.Logger
}

// ExecuteOnce performs a single Kubernetes deployment attempt
func (op *KubernetesDeployOperation) ExecuteOnce(_ context.Context) error {
	op.logger.Debug("Executing Kubernetes deployment",
		"image_ref", op.args.ImageRef,
		"namespace", op.namespace)

	// Deploy to Kubernetes via pipeline adapter
	// Convert to typed parameters for DeployKubernetesTyped
	deployParams := core.DeployParams{
		SessionID:     op.session.SessionID,
		ManifestPaths: op.manifests,
		Namespace:     op.namespace,
		DryRun:        false,
		Wait:          true,
		Timeout:       300, // 5 minutes default
	}
	deployResult, err := op.tool.pipelineAdapter.DeployKubernetesTyped(context.Background(), op.session.SessionID, deployParams)

	if err != nil {
		op.logger.Warn("Kubernetes deployment failed", "error", err)
		return err
	}

	if deployResult == nil {
		return errors.NewError().Messagef("deployment returned nil result").WithLocation(

		// deployResult is already typed as *core.DeployResult
		// Check success through the typed result structure
		).Build()
	}

	if deployResult != nil && !deployResult.Success {
		return errors.NewError().Messagef("deployment failed: %s", deployResult.Error).Build()
	}

	op.logger.Info("Kubernetes deployment completed successfully",
		"namespace", op.namespace)

	return nil
}

// GetFailureAnalysis analyzes why the Kubernetes deployment failed
func (op *KubernetesDeployOperation) GetFailureAnalysis(_ context.Context, err error) (error, error) {
	op.logger.Debug("Analyzing Kubernetes deployment failure", "error", err)

	return err, nil
}

// PrepareForRetry applies fixes and prepares for the next deployment attempt
func (op *KubernetesDeployOperation) PrepareForRetry(ctx context.Context, fixAttempt interface{}) error {
	op.logger.Info("Preparing for retry after fix",
		"fix_strategy", "retry")

	op.logger.Info("Fix preparation simplified")
	return nil
}

// CanRetry determines if the deployment operation can be retried
func (op *KubernetesDeployOperation) CanRetry() bool {
	// Kubernetes deployments can generally be retried unless there are fundamental issues
	return true
}

// Execute runs the operation (alias for ExecuteOnce for compatibility)
func (op *KubernetesDeployOperation) Execute(ctx context.Context) error {
	return op.ExecuteOnce(ctx)
}

// GetLastError returns the last error encountered (implementation for interface)
func (op *KubernetesDeployOperation) GetLastError() error {
	// This would typically store the last error in a field
	// For now, return nil as errors are handled in real-time
	return nil
}
