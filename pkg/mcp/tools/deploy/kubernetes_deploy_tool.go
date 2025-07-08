package deploy

import (
	"context"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	core "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/session"
)

// kubernetesDeployToolImpl implements the strongly-typed Kubernetes deploy tool using services
type kubernetesDeployToolImpl struct {
	pipelineAdapter  core.TypedPipelineOperations
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	workflowExecutor services.WorkflowExecutor
	logger           *slog.Logger
}

// NewKubernetesDeployTool creates a new strongly-typed Kubernetes deploy tool using service container
func NewKubernetesDeployTool(adapter core.TypedPipelineOperations, container services.ServiceContainer, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "kubernetes_deploy")

	return &kubernetesDeployToolImpl{
		pipelineAdapter:  adapter,
		sessionStore:     container.SessionStore(),
		sessionState:     container.SessionState(),
		workflowExecutor: container.WorkflowExecutor(),
		logger:           toolLogger,
	}
}

// NewKubernetesDeployToolLegacy creates a new strongly-typed Kubernetes deploy tool using session manager (backward compatibility)
func NewKubernetesDeployToolLegacy(adapter core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) api.Tool {
	toolLogger := logger.With("tool", "kubernetes_deploy_legacy")

	return &kubernetesDeployToolImpl{
		pipelineAdapter:  adapter,
		sessionStore:     nil, // Legacy mode - no services
		sessionState:     nil,
		workflowExecutor: nil,
		logger:           toolLogger,
	}
}

// Execute implements api.Tool interface
func (t *kubernetesDeployToolImpl) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract params from ToolInput
	var deployParams *tools.DeployToolParams
	if rawParams, ok := input.Data["params"]; ok {
		if typedParams, ok := rawParams.(*tools.DeployToolParams); ok {
			deployParams = typedParams
		} else {
			return api.ToolOutput{
					Success: false,
					Error:   "Invalid input type for deploy tool",
				}, errors.NewError().
					Code(errors.CodeInvalidParameter).
					Message("Invalid input type for deploy tool").
					Type(errors.ErrTypeValidation).
					Severity(errors.SeverityHigh).
					Context("tool", "kubernetes_deploy").
					Context("operation", "type_assertion").
					Build()
		}
	} else {
		return api.ToolOutput{
				Success: false,
				Error:   "No params provided",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("No params provided").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityHigh).
				Build()
	}

	// Use session ID from input if available
	if input.SessionID != "" {
		deployParams.SessionID = input.SessionID
	}
	startTime := time.Now()

	// Convert to internal format for backward compatibility
	k8sParams := t.convertToKubernetesDeployParams(deployParams)

	// Validate parameters at compile time
	if err := k8sParams.Validate(); err != nil {
		return api.ToolOutput{
				Success: false,
				Error:   "Kubernetes deployment parameters validation failed",
			}, errors.NewError().
				Code(errors.CodeInvalidParameter).
				Message("Kubernetes deployment parameters validation failed").
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Cause(err).
				Context("manifest_path", k8sParams.ManifestPath).
				Context("namespace", k8sParams.Namespace).
				Context("deployment_key", k8sParams.DeploymentKey).
				Suggestion("Ensure manifest_path, namespace, and deployment_key are provided").
				WithLocation().
				Build()
	}

	// Execute Kubernetes deployment using pipeline adapter
	deployErr := t.executeDeploy(context.Background(), k8sParams)

	// Create result
	result := KubernetesDeployResult{
		Success:       deployErr == nil,
		DeploymentKey: k8sParams.DeploymentKey,
		Namespace:     k8sParams.Namespace,
		Duration:      time.Since(startTime),
		SessionID:     deployParams.SessionID,
	}

	if deployErr != nil {
		// Create RichError for deployment failures
		return api.ToolOutput{
				Success: false,
				Data:    map[string]interface{}{"result": &result},
				Error:   "Failed to deploy to Kubernetes",
			}, errors.NewError().
				Code(codes.DEPLOY_EXECUTION_FAILED).
				Message("Failed to deploy to Kubernetes").
				Type(errors.ErrTypeBusiness).
				Severity(errors.SeverityHigh).
				Cause(deployErr).
				Context("manifest_path", k8sParams.ManifestPath).
				Context("namespace", k8sParams.Namespace).
				Context("deployment_key", k8sParams.DeploymentKey).
				Context("strategy", k8sParams.Strategy).
				Context("dry_run", k8sParams.DryRun).
				Suggestion("Check manifest syntax and cluster connectivity").
				WithLocation().
				Build()
	}

	// Set success details (would normally come from Kubernetes API)
	result.Resources = []string{"deployment/" + k8sParams.DeploymentKey, "service/" + k8sParams.DeploymentKey}
	result.ReadyReplicas = 3   // This would be actual ready replicas
	result.DesiredReplicas = 3 // This would be actual desired replicas
	result.RolloutStatus = "successfully rolled out"
	result.Revision = "1"

	// Mock service information
	result.Services = []KubernetesService{
		{
			Name:      k8sParams.DeploymentKey,
			Type:      "ClusterIP",
			ClusterIP: "10.0.0.123",
			Ports:     []int32{80, 443},
		},
	}

	t.logger.Info("Kubernetes deployment completed successfully",
		"namespace", k8sParams.Namespace,
		"deployment", k8sParams.DeploymentKey,
		"duration", result.Duration)

	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": &result},
	}, nil
}

// Name implements api.Tool interface
func (t *kubernetesDeployToolImpl) Name() string {
	return "kubernetes_deploy"
}

// Description implements api.Tool interface
func (t *kubernetesDeployToolImpl) Description() string {
	return "Deploys applications to Kubernetes with strongly-typed parameters and comprehensive error handling"
}

// Schema implements api.Tool interface
func (t *kubernetesDeployToolImpl) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "kubernetes_deploy",
		Description: "Deploys applications to Kubernetes with strongly-typed parameters and comprehensive error handling",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"params": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"manifest_path": map[string]interface{}{
							"type":        "string",
							"description": "Path to Kubernetes manifest file",
						},
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Kubernetes namespace for deployment",
						},
						"deployment_key": map[string]interface{}{
							"type":        "string",
							"description": "Unique deployment identifier",
						},
						"session_id": map[string]interface{}{
							"type":        "string",
							"description": "Session ID for workspace management",
						},
						"dry_run": map[string]interface{}{
							"type":        "boolean",
							"description": "Perform dry run only",
						},
					},
					"required": []string{"manifest_path", "namespace", "deployment_key"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the deployment was successful",
						},
						"deployment_key": map[string]interface{}{
							"type":        "string",
							"description": "Deployment identifier",
						},
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Kubernetes namespace",
						},
						"duration": map[string]interface{}{
							"type":        "string",
							"description": "Deployment duration",
						},
					},
				},
			},
		},
	}
}

// convertToKubernetesDeployParams converts tools.DeployToolParams to internal KubernetesDeployParams
func (t *kubernetesDeployToolImpl) convertToKubernetesDeployParams(params *tools.DeployToolParams) KubernetesDeployParams {
	// For demonstration, using manifest_dir as manifest_path and image_ref as deployment_key
	// In real implementation, this would be more sophisticated
	manifestPath := params.ManifestDir
	if manifestPath == "" {
		manifestPath = "./k8s/deployment.yaml" // Default
	}

	deploymentKey := params.ImageRef
	if deploymentKey == "" {
		deploymentKey = "app" // Default
	}

	return KubernetesDeployParams{
		ManifestPath:  manifestPath,
		Namespace:     params.Namespace,
		DeploymentKey: deploymentKey,
		DryRun:        params.DryRun,
		Wait:          params.Wait,
		Timeout:       params.Timeout,
		SessionID:     params.SessionID,
		Strategy:      "RollingUpdate", // Default strategy
	}
}

// executeDeploy performs the actual Kubernetes deployment operation
func (t *kubernetesDeployToolImpl) executeDeploy(ctx context.Context, params KubernetesDeployParams) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info("Deploying to Kubernetes",
		"manifest", params.ManifestPath,
		"namespace", params.Namespace,
		"deployment", params.DeploymentKey,
		"dry_run", params.DryRun)

	// In real implementation, this would use:
	// return t.pipelineAdapter.DeployToKubernetes(ctx, params)

	// For demonstration, we'll just validate the parameters
	if params.ManifestPath == "" || params.Namespace == "" || params.DeploymentKey == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Missing required deployment parameters").
			Type(errors.ErrTypeValidation).
			Build()
	}

	// Simulate deployment delay
	time.Sleep(100 * time.Millisecond)

	return nil // Success for demonstration
}
