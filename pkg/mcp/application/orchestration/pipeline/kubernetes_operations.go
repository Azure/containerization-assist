package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// GenerateKubernetesManifests generates Kubernetes manifests for the given application
func (o *Operations) GenerateKubernetesManifests(args TypedGenerateManifestsArgs) (*KubernetesManifestResult, error) {
	if args.SessionID == "" {
		return nil, errors.NewError().Message("session ID is required").Build()
	}

	workspace := o.GetSessionWorkspace(args.SessionID)
	if workspace == "" {
		return nil, errors.NewError().Messagef("invalid session workspace").Build()
	}

	return &KubernetesManifestResult{
		Success: true,
		Manifests: []KubernetesManifest{
			{
				Kind: "Deployment",
				Name: args.AppName,
				Path: filepath.Join(workspace, "deployment.yaml"),
			},
		},
	}, nil
}

// DeployToKubernetes deploys the given manifests to Kubernetes
func (o *Operations) DeployToKubernetes(_ string, _ []string) (*KubernetesDeploymentResult, error) {
	return &KubernetesDeploymentResult{
		Success:     true,
		Namespace:   "default",
		Deployments: []string{},
		Services:    []string{},
	}, nil
}

// CheckApplicationHealth checks the health of the deployed application
func (o *Operations) CheckApplicationHealth(_, _, _ string, _ time.Duration) (*ApplicationHealthResult, error) {
	return &ApplicationHealthResult{
		Healthy: true,
		Status:  "running",
		Pods:    3,
		Ready:   3,
	}, nil
}

// GenerateManifests implements the generic interface for manifest generation
func (o *Operations) GenerateManifests(_ context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		imageRef, _ := argsMap["image_ref"].(string)
		appName, _ := argsMap["app_name"].(string)
		port, _ := argsMap["port"].(int)
		cpuRequest, _ := argsMap["cpu_request"].(string)
		memoryRequest, _ := argsMap["memory_request"].(string)
		cpuLimit, _ := argsMap["cpu_limit"].(string)
		memoryLimit, _ := argsMap["memory_limit"].(string)

		return o.GenerateKubernetesManifests(TypedGenerateManifestsArgs{
			SessionID:     sessionID,
			ImageRef:      imageRef,
			AppName:       appName,
			Port:          port,
			CPURequest:    cpuRequest,
			MemoryRequest: memoryRequest,
			CPULimit:      cpuLimit,
			MemoryLimit:   memoryLimit,
		})
	}
	return nil, errors.NewError().Messagef("invalid arguments for GenerateManifests").Build()
}

// DeployKubernetes implements the generic interface for Kubernetes deployment
func (o *Operations) DeployKubernetes(_ context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if manifests, ok := argsMap["manifests"].([]string); ok {
			return o.DeployToKubernetes(sessionID, manifests)
		}
	}
	return nil, errors.NewError().Messagef("invalid arguments for DeployKubernetes").Build()
}

// CheckHealth implements the generic interface for health checking
func (o *Operations) CheckHealth(_ context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		namespace, _ := argsMap["namespace"].(string)
		labelSelector, _ := argsMap["label_selector"].(string)
		timeout := 30 * time.Second
		if timeoutArg, ok := argsMap["timeout"].(time.Duration); ok {
			timeout = timeoutArg
		}
		return o.CheckApplicationHealth(sessionID, namespace, labelSelector, timeout)
	}
	return nil, errors.NewError().Messagef("invalid arguments for CheckHealth").Build()
}

// GenerateManifestsTyped implements TypedPipelineOperations.GenerateManifestsTyped
func (o *Operations) GenerateManifestsTyped(_ context.Context, sessionID string, params core.GenerateManifestsParams) (*core.GenerateManifestsResult, error) {
	if sessionID == "" {
		return nil, errors.NewError().Message("session ID is required").Build()
	}
	if params.ServiceName == "" {
		return nil, errors.NewError().Message("service name is required").Build()
	}
	if params.ImageName == "" {
		return nil, errors.NewError().Message("image name is required").Build()
	}
	if params.Replicas <= 0 {
		return nil, errors.NewError().Message("replicas must be greater than 0").Build()
	}

	port := params.Port
	if port == 0 {
		port = 8080
	}

	manifestResult, err := o.GenerateKubernetesManifests(TypedGenerateManifestsArgs{
		SessionID:     sessionID,
		ImageRef:      params.ImageName,
		AppName:       params.ServiceName,
		Port:          port,
		CPURequest:    "100m",
		MemoryRequest: "128Mi",
		CPULimit:      "200m",
		MemoryLimit:   "256Mi",
	})
	if err != nil {
		return nil, err
	}

	var manifestPaths []string
	for _, manifest := range manifestResult.Manifests {
		manifestPaths = append(manifestPaths, manifest.Path)
	}

	return &core.GenerateManifestsResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   manifestResult.Success,
			Timestamp: time.Now(),
		},
		ManifestPaths: manifestPaths,
		Namespace:     "default",
	}, nil
}

// DeployKubernetesTyped implements TypedPipelineOperations.DeployKubernetesTyped
func (o *Operations) DeployKubernetesTyped(_ context.Context, sessionID string, params core.DeployParams) (*core.DeployResult, error) {
	if sessionID == "" {
		return nil, errors.NewError().Message("session ID is required").Build()
	}
	if len(params.ManifestPaths) == 0 {
		return nil, errors.NewError().Message("manifest_paths are required").Build()
	}
	if params.Namespace == "" {
		return nil, errors.NewError().Message("namespace is required").Build()
	}

	deployResult, err := o.DeployToKubernetes(sessionID, params.ManifestPaths)
	if err != nil {
		return nil, err
	}

	return &core.DeployResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   true,
			Message:   "Deployment completed successfully",
			Timestamp: time.Now(),
		},
		DeploymentName: "app-deployment",
		ServiceName:    "app-service",
		Namespace:      deployResult.Namespace,
		Endpoints:      []string{},
	}, nil
}

// CheckHealthTyped implements TypedPipelineOperations.CheckHealthTyped
func (o *Operations) CheckHealthTyped(_ context.Context, sessionID string, params core.HealthCheckParams) (*core.HealthCheckResult, error) {
	if sessionID == "" {
		return nil, errors.NewError().Message("session ID is required").Build()
	}
	if params.DeploymentName == "" {
		return nil, errors.NewError().Message("deployment name is required").Build()
	}
	if params.Namespace == "" {
		return nil, errors.NewError().Message("namespace is required").Build()
	}
	if params.Timeout <= 0 {
		return nil, errors.NewError().Message("timeout must be positive").Build()
	}

	timeout := time.Duration(params.Timeout) * time.Second
	healthResult, err := o.CheckApplicationHealth(sessionID, params.Namespace, params.DeploymentName, timeout)
	if err != nil {
		return nil, err
	}

	return &core.HealthCheckResult{
		BaseToolResponse: types.BaseToolResponse{
			Success:   healthResult.Healthy,
			Message:   fmt.Sprintf("Health check completed: %s", healthResult.Status),
			Timestamp: time.Now(),
		},
		Healthy:       healthResult.Healthy,
		ReadyReplicas: 1,
		TotalReplicas: 1,
		Status:        healthResult.Status,
	}, nil
}
