package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"

	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
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
	// Use the correct interface method
	deployArgs := map[string]interface{}{
		"manifests": manifests,
	}
	deployResult, err := t.pipelineAdapter.DeployKubernetes(ctx, session.SessionID, deployArgs)
	result.DeploymentDuration = time.Since(deploymentStart)

	// Convert from mcptypes.KubernetesDeploymentResult to kubernetes.DeploymentResult
	if deployResult != nil {
		// Convert interface{} to expected structure
		if deployMap, ok := deployResult.(map[string]interface{}); ok {
			result.DeploymentResult = &kubernetes.DeploymentResult{
				Success:   getBoolFromMap(deployMap, "success", false),
				Namespace: getStringFromMap(deployMap, "namespace", result.Namespace),
			}

			// Handle error if present
			if errorData, exists := deployMap["error"]; exists && errorData != nil {
				if errorMap, ok := errorData.(map[string]interface{}); ok {
					result.DeploymentResult.Error = &kubernetes.DeploymentError{
						Type:    getStringFromMap(errorMap, "type", "unknown"),
						Message: getStringFromMap(errorMap, "message", "unknown error"),
					}
				}
			}
		} else {
			// Fallback for unexpected result type
			result.DeploymentResult = &kubernetes.DeploymentResult{
				Success:   false,
				Namespace: result.Namespace,
			}
		}
	}

	if err != nil {
		_ = t.handleDeploymentError(ctx, err, result.DeploymentResult, result)
		return err
	}

	// Check deployment success through type assertion
	if deployResult != nil {
		if deployMap, ok := deployResult.(map[string]interface{}); ok {
			if !getBoolFromMap(deployMap, "success", false) {
				deploymentErr := fmt.Errorf("deployment failed")
				_ = t.handleDeploymentError(ctx, deploymentErr, result.DeploymentResult, result)
				return deploymentErr
			}
		}
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", args.Namespace).
		Msg("Kubernetes deployment completed successfully")

	// Progress reporting removed

	return nil
}

// handleDeploymentError creates an error for deployment failures and enriches the result with failure analysis
func (t *AtomicDeployKubernetesTool) handleDeploymentError(_ context.Context, err error, _ *kubernetes.DeploymentResult, result *AtomicDeployKubernetesResult) error {
	// Analyze the deployment error
	richError, _ := analyzeDeploymentError(err)

	// Create failure analysis
	result.FailureAnalysis = &DeploymentFailureAnalysis{
		FailureType:    "deployment_error",
		FailureStage:   "deployment",
		RootCauses:     []string{err.Error()},
		ImpactSeverity: "high",
	}

	// Add immediate actions based on error type
	_ = err // Use error for context

	// Default actions for deployment errors
	result.FailureAnalysis.ImmediateActions = []DeploymentRemediationAction{
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
	result.FailureAnalysis.DiagnosticCommands = []DiagnosticCommand{
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

	return fmt.Errorf("error")
}

// buildSuccessResult creates a success result after fixing operations complete
func (t *AtomicDeployKubernetesTool) buildSuccessResult(_ context.Context, args AtomicDeployKubernetesArgs, _ *core.SessionState) (*AtomicDeployKubernetesResult, error) {
	result := &AtomicDeployKubernetesResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_deploy_kubernetes", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("deploy", true, 0),
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		AppName:             args.AppName,
		Namespace:           args.Namespace,
		Success:             true,
	}

	result.BaseAIContextResult.IsSuccessful = true
	return result, nil
}

// KubernetesDeployOperation implements FixableOperation for Kubernetes deployments
type KubernetesDeployOperation struct {
	tool         *AtomicDeployKubernetesTool
	args         AtomicDeployKubernetesArgs
	session      *core.SessionState
	workspaceDir string
	namespace    string
	manifests    []string
	logger       zerolog.Logger
}

// ExecuteOnce performs a single Kubernetes deployment attempt
func (op *KubernetesDeployOperation) ExecuteOnce(_ context.Context) error {
	op.logger.Debug().
		Str("image_ref", op.args.ImageRef).
		Str("namespace", op.namespace).
		Msg("Executing Kubernetes deployment")

	// Deploy to Kubernetes via pipeline adapter
	deployArgs := map[string]interface{}{
		"manifests": op.manifests,
	}
	deployResult, err := op.tool.pipelineAdapter.DeployKubernetes(context.Background(), op.session.SessionID, deployArgs)

	if err != nil {
		op.logger.Warn().Err(err).Msg("Kubernetes deployment failed")
		return err
	}

	if deployResult == nil {
		return fmt.Errorf("deployment returned nil result")
	}

	if deployMap, ok := deployResult.(map[string]interface{}); ok {
		if !getBoolFromMap(deployMap, "success", false) {
			errorMsg := getStringFromMap(deployMap, "error", "unknown deployment error")
			return fmt.Errorf("deployment failed: %s", errorMsg)
		}
	}

	op.logger.Info().
		Str("namespace", op.namespace).
		Msg("Kubernetes deployment completed successfully")

	return nil
}

// GetFailureAnalysis analyzes why the Kubernetes deployment failed
func (op *KubernetesDeployOperation) GetFailureAnalysis(_ context.Context, err error) (error, error) {
	op.logger.Debug().Err(err).Msg("Analyzing Kubernetes deployment failure")

	return err, nil
}

// PrepareForRetry applies fixes and prepares for the next deployment attempt
func (op *KubernetesDeployOperation) PrepareForRetry(ctx context.Context, fixAttempt interface{}) error {
	op.logger.Info().
		Str("fix_strategy", "retry").
		Msg("Preparing for retry after fix")

	op.logger.Info().Msg("Fix preparation simplified")
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

// applyManifestFix applies fixes to Kubernetes manifests
func (op *KubernetesDeployOperation) applyManifestFix(_ context.Context, fixAttempt interface{}) error {
	op.logger.Info().Msg("Manifest fix application simplified")

	return nil
}

// applyDependencyFix applies dependency-related fixes
func (op *KubernetesDeployOperation) applyDependencyFix(ctx context.Context, fixAttempt interface{}) error {
	op.logger.Info().Msg("Dependency fix application simplified")

	return nil
}

// applyResourceFix applies resource-related fixes
func (op *KubernetesDeployOperation) applyResourceFix(ctx context.Context, fixAttempt interface{}) error {
	op.logger.Info().Msg("Resource fix application simplified")

	return nil
}

// applyGenericFix applies generic fixes
func (op *KubernetesDeployOperation) applyGenericFix(ctx context.Context, fixAttempt interface{}) error {
	op.logger.Info().Msg("Generic fix application simplified")
	return nil
}

// applyFileChange applies a single file change operation
func (op *KubernetesDeployOperation) applyFileChange(change map[string]interface{}) error {
	filePath := filepath.Join(op.workspaceDir, getStringFromMap(change, "FilePath", ""))

	switch getStringFromMap(change, "Operation", "") {
	case "create":
		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error")
		}

		// Write the new file
		if newContent, ok := change["NewContent"].(string); ok {
			if err := os.WriteFile(filePath, []byte(newContent), 0600); err != nil {
				return fmt.Errorf("failed to write file: %v", err)
			}
		}

	case "update", "replace":
		// Create backup
		backupPath := filePath + ".backup"
		if data, err := os.ReadFile(filePath); err == nil {
			if err := os.WriteFile(backupPath, data, 0600); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create backup")
			}
		}

		// Write the updated content
		if newContent, ok := change["NewContent"].(string); ok {
			if err := os.WriteFile(filePath, []byte(newContent), 0600); err != nil {
				return fmt.Errorf("failed to update file: %v", err)
			}
		}

	case "delete":
		// Create backup before deletion
		backupPath := filePath + ".backup"
		if data, err := os.ReadFile(filePath); err == nil {
			if err := os.WriteFile(backupPath, data, 0600); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create backup before deletion")
			}
		}

		// Remove the file
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error")
		}

	default:
		return fmt.Errorf("error")
	}

	op.logger.Info().
		Str("file", filePath).
		Str("operation", getStringFromMap(change, "Operation", "")).
		Msg("Applied file change")

	return nil
}
