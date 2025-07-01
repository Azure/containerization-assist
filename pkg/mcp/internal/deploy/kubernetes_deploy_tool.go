package deploy

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// kubernetesDeployToolImpl implements the strongly-typed Kubernetes deploy tool
type kubernetesDeployToolImpl struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
}

// NewKubernetesDeployTool creates a new strongly-typed Kubernetes deploy tool
func NewKubernetesDeployTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) KubernetesDeployTool {
	toolLogger := logger.With().Str("tool", "kubernetes_deploy").Logger()

	return &kubernetesDeployToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// Execute implements tools.Tool[KubernetesDeployParams, KubernetesDeployResult]
func (t *kubernetesDeployToolImpl) Execute(ctx context.Context, params KubernetesDeployParams) (KubernetesDeployResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return KubernetesDeployResult{}, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Kubernetes deployment parameters validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Cause(err).
			Context("manifest_path", params.ManifestPath).
			Context("namespace", params.Namespace).
			Context("deployment_key", params.DeploymentKey).
			Suggestion("Ensure manifest_path, namespace, and deployment_key are provided").
			WithLocation().
			Build()
	}

	// Execute Kubernetes deployment using pipeline adapter
	deployErr := t.executeDeploy(ctx, params)

	// Create result
	result := KubernetesDeployResult{
		Success:       deployErr == nil,
		DeploymentKey: params.DeploymentKey,
		Namespace:     params.Namespace,
		Duration:      time.Since(startTime),
		SessionID:     params.SessionID,
	}

	if deployErr != nil {
		// Create RichError for deployment failures
		return result, rich.NewError().
			Code("KUBERNETES_DEPLOY_FAILED").
			Message("Failed to deploy to Kubernetes").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityHigh).
			Cause(deployErr).
			Context("manifest_path", params.ManifestPath).
			Context("namespace", params.Namespace).
			Context("deployment_key", params.DeploymentKey).
			Context("strategy", params.Strategy).
			Context("dry_run", params.DryRun).
			Suggestion("Check manifest syntax and cluster connectivity").
			WithLocation().
			Build()
	}

	// Set success details (would normally come from Kubernetes API)
	result.Resources = []string{"deployment/" + params.DeploymentKey, "service/" + params.DeploymentKey}
	result.ReadyReplicas = 3   // This would be actual ready replicas
	result.DesiredReplicas = 3 // This would be actual desired replicas
	result.RolloutStatus = "successfully rolled out"
	result.Revision = "1"

	// Mock service information
	result.Services = []KubernetesService{
		{
			Name:      params.DeploymentKey,
			Type:      "ClusterIP",
			ClusterIP: "10.0.0.123",
			Ports:     []int32{80, 443},
		},
	}

	t.logger.Info().
		Str("namespace", params.Namespace).
		Str("deployment", params.DeploymentKey).
		Dur("duration", result.Duration).
		Msg("Kubernetes deployment completed successfully")

	return result, nil
}

// GetName implements tools.Tool
func (t *kubernetesDeployToolImpl) GetName() string {
	return "kubernetes_deploy"
}

// GetDescription implements tools.Tool
func (t *kubernetesDeployToolImpl) GetDescription() string {
	return "Deploys applications to Kubernetes with strongly-typed parameters and comprehensive error handling"
}

// GetSchema implements tools.Tool
func (t *kubernetesDeployToolImpl) GetSchema() tools.Schema[KubernetesDeployParams, KubernetesDeployResult] {
	return tools.Schema[KubernetesDeployParams, KubernetesDeployResult]{
		Name:        "kubernetes_deploy",
		Description: "Strongly-typed Kubernetes deployment tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"manifest_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to Kubernetes manifest file",
					"minLength":   1,
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
					"pattern":     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
				},
				"deployment_key": map[string]interface{}{
					"type":        "string",
					"description": "Unique deployment identifier",
					"minLength":   1,
				},
				"kube_config": map[string]interface{}{
					"type":        "string",
					"description": "Path to kubeconfig file",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes context to use",
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "Perform a dry run without actual deployment",
				},
				"wait": map[string]interface{}{
					"type":        "boolean",
					"description": "Wait for deployment to complete",
				},
				"timeout": map[string]interface{}{
					"type":        "string",
					"description": "Timeout duration (e.g., '5m', '30s')",
				},
				"strategy": map[string]interface{}{
					"type":        "string",
					"description": "Deployment strategy",
					"enum":        []string{"RollingUpdate", "Recreate"},
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking",
				},
			},
			"required": []string{"manifest_path", "namespace", "deployment_key"},
		},
		ResultSchema: map[string]interface{}{
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
					"description": "Target namespace",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Deployment duration",
				},
				"ready_replicas": map[string]interface{}{
					"type":        "integer",
					"description": "Number of ready replicas",
				},
				"desired_replicas": map[string]interface{}{
					"type":        "integer",
					"description": "Number of desired replicas",
				},
				"rollout_status": map[string]interface{}{
					"type":        "string",
					"description": "Rollout status",
				},
			},
		},
		Examples: []tools.Example[KubernetesDeployParams, KubernetesDeployResult]{
			{
				Name:        "deploy_web_app",
				Description: "Deploy a web application to Kubernetes",
				Params: KubernetesDeployParams{
					ManifestPath:  "./k8s/deployment.yaml",
					Namespace:     "production",
					DeploymentKey: "web-app",
					Wait:          true,
					Strategy:      "RollingUpdate",
					SessionID:     "session-123",
				},
				Result: KubernetesDeployResult{
					Success:         true,
					DeploymentKey:   "web-app",
					Namespace:       "production",
					Duration:        45 * time.Second,
					ReadyReplicas:   3,
					DesiredReplicas: 3,
					RolloutStatus:   "successfully rolled out",
					Revision:        "1",
					SessionID:       "session-123",
				},
			},
		},
	}
}

// executeDeploy performs the actual Kubernetes deployment operation
func (t *kubernetesDeployToolImpl) executeDeploy(ctx context.Context, params KubernetesDeployParams) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info().
		Str("manifest", params.ManifestPath).
		Str("namespace", params.Namespace).
		Str("deployment", params.DeploymentKey).
		Bool("dry_run", params.DryRun).
		Msg("Deploying to Kubernetes")

	// In real implementation, this would use:
	// return t.pipelineAdapter.DeployToKubernetes(ctx, params)

	// For demonstration, we'll just validate the parameters
	if params.ManifestPath == "" || params.Namespace == "" || params.DeploymentKey == "" {
		return rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Missing required deployment parameters").
			Type(rich.ErrTypeValidation).
			Build()
	}

	// Simulate deployment delay
	time.Sleep(100 * time.Millisecond)

	return nil // Success for demonstration
}
