package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// Operations implements mcptypes.PipelineOperations directly without adapter pattern
type Operations struct {
	sessionManager *session.SessionManager
	clients        *mcptypes.MCPClients
	logger         zerolog.Logger
}

// NewOperations creates a new pipeline operations implementation
func NewOperations(
	sessionManager *session.SessionManager,
	clients *mcptypes.MCPClients,
	logger zerolog.Logger,
) *Operations {
	return &Operations{
		sessionManager: sessionManager,
		clients:        clients,
		logger:         logger.With().Str("component", "pipeline_operations").Logger(),
	}
}

// Session management operations

func (o *Operations) GetSessionWorkspace(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	session, err := o.sessionManager.GetSession(sessionID)
	if err != nil {
		o.logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get session")
		return ""
	}

	// Type assert to get the SessionState
	if sessionState, ok := session.(*core.SessionState); ok {
		return sessionState.WorkspaceDir
	}
	o.logger.Error().Str("session_id", sessionID).Msg("Session type assertion failed")
	return ""
}

func (o *Operations) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	return o.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		sess, ok := s.(*core.SessionState)
		if !ok {
			return
		}
		// Update session based on result type
		switch r := result.(type) {
		case *mcptypes.BuildResult:
			if r.Success {
				// Update image reference (ImageRef is a string field)
				sess.ImageRef = r.ImageRef
			}
		default:
			o.logger.Warn().Str("type", fmt.Sprintf("%T", result)).Msg("Unknown result type for session update")
		}

		sess.UpdatedAt = time.Now()
	})
}

// Docker operations

func (o *Operations) BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*mcptypes.BuildResult, error) {
	workspace := o.GetSessionWorkspace(sessionID)
	if workspace == "" {
		return nil, fmt.Errorf("invalid session workspace")
	}

	// Build the image using the Docker client
	ctx := context.Background()
	buildCtx := filepath.Dir(dockerfilePath)

	// Use the docker client's Build method with type assertion
	dockerClient := o.clients.Docker.(docker.DockerClient)
	_, err := dockerClient.Build(ctx, dockerfilePath, imageRef, buildCtx)
	if err != nil {
		return &mcptypes.BuildResult{
			Success: false,
			Error: &mcptypes.BuildError{
				Type:    "build_failed",
				Message: err.Error(),
			},
		}, nil
	}

	// Update session state
	o.UpdateSessionFromDockerResults(sessionID, &mcptypes.BuildResult{
		ImageID:  imageRef,
		ImageRef: imageRef,
		Success:  true,
	})

	return &mcptypes.BuildResult{
		ImageID:  imageRef,
		ImageRef: imageRef,
		Success:  true,
	}, nil
}

func (o *Operations) PullDockerImage(sessionID, imageRef string) error {
	// Docker client doesn't have a Pull method in the interface
	// This would need to be implemented or use docker CLI directly
	o.logger.Warn().Str("image_ref", imageRef).Msg("Pull operation not implemented in Docker client")
	return fmt.Errorf("pull operation not implemented")
}

func (o *Operations) PushDockerImage(sessionID, imageRef string) error {
	ctx := context.Background()
	dockerClient := o.clients.Docker.(docker.DockerClient)
	_, err := dockerClient.Push(ctx, imageRef)
	return err
}

func (o *Operations) TagDockerImage(sessionID, sourceRef, targetRef string) error {
	// Docker client doesn't have a Tag method in the interface
	// This would need to be implemented or use docker CLI directly
	o.logger.Warn().
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Msg("Tag operation not implemented in Docker client")
	return fmt.Errorf("tag operation not implemented")
}

func (o *Operations) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	// This would list Docker resources associated with the session
	// For now, return empty state
	return &mcptypes.DockerState{
		Images:     []string{},
		Containers: []string{},
		Networks:   []string{},
		Volumes:    []string{},
	}, nil
}

// Kubernetes operations

func (o *Operations) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	workspace := o.GetSessionWorkspace(sessionID)
	if workspace == "" {
		return nil, fmt.Errorf("invalid session workspace")
	}

	// This would generate K8s manifests
	// For now, return a basic result
	return &mcptypes.KubernetesManifestResult{
		Success: true,
		Manifests: []mcptypes.GeneratedManifest{
			{
				Kind: "Deployment",
				Name: appName,
				Path: filepath.Join(workspace, "deployment.yaml"),
			},
			{
				Kind: "Service",
				Name: appName,
				Path: filepath.Join(workspace, "service.yaml"),
			},
		},
	}, nil
}

func (o *Operations) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	ctx := context.Background()
	namespace := "default"

	for _, manifest := range manifests {
		kubeClient := o.clients.Kubernetes.(k8s.KubeRunner)
		if _, err := kubeClient.Apply(ctx, manifest); err != nil {
			return &mcptypes.KubernetesDeploymentResult{
				Success: false,
				Error: &mcp.RichError{
					Code:     "deploy_failed",
					Type:     "kubernetes_error",
					Severity: "high",
					Message:  err.Error(),
				},
			}, nil
		}
	}

	return &mcptypes.KubernetesDeploymentResult{
		Success:     true,
		Namespace:   namespace,
		Deployments: []string{},
		Services:    []string{},
	}, nil
}

func (o *Operations) CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	ctx := context.Background()

	// Get pods for the deployment
	labelSelector := fmt.Sprintf("app=%s", deploymentName)
	kubeClient := o.clients.Kubernetes.(k8s.KubeRunner)
	podsOutput, err := kubeClient.GetPods(ctx, namespace, labelSelector)
	if err != nil {
		return &mcptypes.HealthCheckResult{
			Healthy: false,
			Status:  "failed",
			Error: &mcptypes.HealthCheckError{
				Type:    "pods_not_found",
				Message: err.Error(),
			},
		}, nil
	}

	// Simple check - if we got pods output without error, consider it healthy
	// A more sophisticated implementation would parse the output
	healthy := podsOutput != "" && err == nil

	return &mcptypes.HealthCheckResult{
		Healthy: healthy,
		Status:  "running",
		PodStatuses: []mcptypes.PodStatus{
			{
				Name:   deploymentName,
				Ready:  healthy,
				Status: "Running",
			},
		},
	}, nil
}

// Resource management

func (o *Operations) AcquireResource(sessionID, resourceType string) error {
	// Resource management would be implemented here
	o.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Acquiring resource")
	return nil
}

func (o *Operations) ReleaseResource(sessionID, resourceType string) error {
	// Resource management would be implemented here
	o.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Releasing resource")
	return nil
}
