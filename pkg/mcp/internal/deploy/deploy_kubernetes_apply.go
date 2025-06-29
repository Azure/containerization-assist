package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp"

	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// performDeployment deploys manifests to Kubernetes cluster
func (t *AtomicDeployKubernetesTool) performDeployment(ctx context.Context, session *mcp.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
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
	deployResult, err := t.pipelineAdapter.DeployToKubernetes(
		session.SessionID,
		manifests,
	)
	result.DeploymentDuration = time.Since(deploymentStart)

	// Convert from mcptypes.KubernetesDeploymentResult to kubernetes.DeploymentResult
	if deployResult != nil {
		result.DeploymentResult = &kubernetes.DeploymentResult{
			Success:   deployResult.Success,
			Namespace: deployResult.Namespace,
		}
		if deployResult.Error != nil {
			result.DeploymentResult.Error = &kubernetes.DeploymentError{
				Type:    deployResult.Error.Type,
				Message: deployResult.Error.Message,
			}
		}
		// Convert deployments and services
		for _, d := range deployResult.Deployments {
			result.DeploymentResult.Resources = append(result.DeploymentResult.Resources, kubernetes.DeployedResource{
				Kind:      "Deployment",
				Name:      d,
				Namespace: deployResult.Namespace,
			})
		}
		for _, s := range deployResult.Services {
			result.DeploymentResult.Resources = append(result.DeploymentResult.Resources, kubernetes.DeployedResource{
				Kind:      "Service",
				Name:      s,
				Namespace: deployResult.Namespace,
			})
		}
	}

	if err != nil {
		_ = t.handleDeploymentError(ctx, err, result.DeploymentResult, result)
		return err
	}

	if deployResult != nil && !deployResult.Success {
		deploymentErr := mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("deployment failed: %s", deployResult.Error.Message), "deployment_error")
		_ = t.handleDeploymentError(ctx, deploymentErr, result.DeploymentResult, result)
		return deploymentErr
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
		FailureType:    richError.Type,
		FailureStage:   "deployment",
		RootCauses:     []string{richError.Message},
		ImpactSeverity: richError.Severity,
	}

	// Add immediate actions based on error type
	switch richError.Type {
	case "image_error":
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
	case "manifest_error":
		result.FailureAnalysis.ImmediateActions = []DeploymentRemediationAction{
			{
				Priority:    1,
				Action:      "Validate manifests",
				Command:     "kubectl apply --dry-run=client -f k8s/",
				Description: "Validate manifest syntax",
				Expected:    "Manifests should validate successfully",
				RiskLevel:   "low",
			},
		}
	case "resource_error":
		result.FailureAnalysis.ImmediateActions = []DeploymentRemediationAction{
			{
				Priority:    1,
				Action:      "Check cluster resources",
				Command:     "kubectl top nodes",
				Description: "Check available cluster resources",
				Expected:    "Sufficient resources should be available",
				RiskLevel:   "low",
			},
		}
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
		result.DeploymentContext.DeploymentErrors = append(result.DeploymentContext.DeploymentErrors, richError.Message)

		// Add troubleshooting tips
		result.DeploymentContext.TroubleshootingTips = []string{
			"Check if the namespace exists: kubectl get namespace " + result.Namespace,
			"Verify image pull policy and registry access",
			"Check resource quotas: kubectl describe resourcequota -n " + result.Namespace,
			"Review recent events: kubectl get events -n " + result.Namespace + " --sort-by=.lastTimestamp",
		}
	}

	return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("kubernetes deployment failed: %v", err), "deployment_error")
}

// buildSuccessResult creates a success result after fixing operations complete
func (t *AtomicDeployKubernetesTool) buildSuccessResult(_ context.Context, args AtomicDeployKubernetesArgs, _ *mcp.SessionState) (*AtomicDeployKubernetesResult, error) {
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
	session      *mcp.SessionState
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
	deployResult, err := op.tool.pipelineAdapter.DeployToKubernetes(
		op.session.SessionID,
		op.manifests,
	)

	if err != nil {
		op.logger.Warn().Err(err).Msg("Kubernetes deployment failed")
		return err
	}

	if deployResult == nil || !deployResult.Success {
		errorMsg := "unknown deployment error"
		if deployResult != nil && deployResult.Error != nil {
			errorMsg = deployResult.Error.Message
		}
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("kubernetes deployment failed: %s", errorMsg), "deployment_error")
	}

	op.logger.Info().
		Str("namespace", op.namespace).
		Msg("Kubernetes deployment completed successfully")

	return nil
}

// GetFailureAnalysis analyzes why the Kubernetes deployment failed
func (op *KubernetesDeployOperation) GetFailureAnalysis(_ context.Context, err error) (*mcp.RichError, error) {
	op.logger.Debug().Err(err).Msg("Analyzing Kubernetes deployment failure")

	// Convert error to RichError if it's not already one
	if richError, ok := err.(*mcp.RichError); ok {
		return &mcp.RichError{
			Code:     richError.Code,
			Type:     richError.Type,
			Severity: richError.Severity,
			Message:  richError.Message,
		}, nil
	}

	// Create a default RichError for non-rich errors
	return &mcp.RichError{
		Code:     "DEPLOYMENT_FAILED",
		Type:     "deployment_error",
		Severity: "High",
		Message:  err.Error(),
	}, nil
}

// PrepareForRetry applies fixes and prepares for the next deployment attempt
func (op *KubernetesDeployOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_strategy", fixAttempt.FixStrategy.Name).
		Msg("Preparing for retry after fix")

	// Apply fix based on the strategy type
	switch fixAttempt.FixStrategy.Type {
	case "manifest":
		return op.applyManifestFix(ctx, fixAttempt)
	case "dependency":
		return op.applyDependencyFix(ctx, fixAttempt)
	case "resource":
		return op.applyResourceFix(ctx, fixAttempt)
	default:
		op.logger.Warn().
			Str("fix_type", fixAttempt.FixStrategy.Type).
			Msg("Unknown fix type, applying generic fix")
		return op.applyGenericFix(ctx, fixAttempt)
	}
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
func (op *KubernetesDeployOperation) applyManifestFix(_ context.Context, fixAttempt *mcp.FixAttempt) error {
	if fixAttempt.FixedContent == "" {
		return mcp.NewRichError("INVALID_ARGUMENTS", "no fixed manifest content provided", "missing_content")
	}

	op.logger.Info().
		Int("content_length", len(fixAttempt.FixedContent)).
		Msg("Applying manifest fix")

	// Determine the manifest file path based on file changes or default
	manifestPath := filepath.Join(op.workspaceDir, "k8s", "deployment.yaml")

	// Check if there's a specific file path in FileChanges
	if len(fixAttempt.FixStrategy.FileChanges) > 0 {
		// Use the first file change path as the manifest path
		manifestPath = filepath.Join(op.workspaceDir, fixAttempt.FixStrategy.FileChanges[0].FilePath)
	}

	// Ensure the directory exists
	dir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create manifest directory: %v", err), "filesystem_error")
	}

	// Create backup of existing manifest if it exists
	if _, err := os.Stat(manifestPath); err == nil {
		backupPath := manifestPath + ".backup"
		data, err := os.ReadFile(manifestPath)
		if err == nil {
			if err := os.WriteFile(backupPath, data, 0600); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create manifest backup")
			}
		}
	}

	// Write the fixed manifest content
	if err := os.WriteFile(manifestPath, []byte(fixAttempt.FixedContent), 0600); err != nil {
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to write fixed manifest: %v", err), "file_error")
	}

	op.logger.Info().
		Str("manifest_path", manifestPath).
		Msg("Successfully applied manifest fix")

	return nil
}

// applyDependencyFix applies dependency-related fixes
func (op *KubernetesDeployOperation) applyDependencyFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "dependency").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying dependency fix")

	// Apply file changes for dependency fixes (e.g., updated image references)
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply dependency fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied dependency file change")
	}

	// Handle specific dependency fix patterns
	if fixAttempt.FixedContent != "" {
		// If we have fixed content for a manifest with updated dependencies
		return op.applyManifestFix(ctx, fixAttempt)
	}

	// Log any commands that might be needed (e.g., pulling new images)
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Dependency fix command identified (execution delegated to deployment tool)")
	}

	return nil
}

// applyResourceFix applies resource-related fixes
func (op *KubernetesDeployOperation) applyResourceFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "resource").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying resource fix")

	// Apply file changes for resource fixes (e.g., adjusted resource limits)
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply resource fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied resource file change")
	}

	// Handle manifest updates with adjusted resources
	if fixAttempt.FixedContent != "" {
		// Apply the manifest with updated resource specifications
		return op.applyManifestFix(ctx, fixAttempt)
	}

	// Log resource-related insights from the fix strategy
	if fixAttempt.FixStrategy.Type == "resource" {
		op.logger.Info().
			Str("fix_name", fixAttempt.FixStrategy.Name).
			Str("fix_description", fixAttempt.FixStrategy.Description).
			Msg("Applied resource adjustment fix")
	}

	return nil
}

// applyGenericFix applies generic fixes
func (op *KubernetesDeployOperation) applyGenericFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	// Generic fix application
	if fixAttempt.FixedContent != "" {
		return op.applyManifestFix(ctx, fixAttempt)
	}

	op.logger.Info().Msg("Applied generic fix (no specific action needed)")
	return nil
}

// applyFileChange applies a single file change operation
func (op *KubernetesDeployOperation) applyFileChange(change mcptypes.FileChange) error {
	filePath := filepath.Join(op.workspaceDir, change.FilePath)

	switch change.Operation {
	case "create":
		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create directory %s: %v", dir, err), "filesystem_error")
		}

		// Write the new file
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0600); err != nil {
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create file %s: %v", filePath, err), "file_error")
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
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0600); err != nil {
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to update file %s: %v", filePath, err), "file_error")
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
			return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to delete file %s: %v", filePath, err), "file_error")
		}

	default:
		return mcp.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("unknown file operation: %s", change.Operation), "invalid_operation")
	}

	op.logger.Info().
		Str("file", filePath).
		Str("operation", change.Operation).
		Msg("Applied file change")

	return nil
}
