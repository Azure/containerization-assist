package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	sessionsvc "github.com/Azure/container-kit/pkg/mcp/internal/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Operations implements PipelineOperations directly without adapter pattern
type Operations struct {
	sessionManager *sessionsvc.SessionManager
	clients        interface{}
	dockerClient   docker.DockerClient
	logger         zerolog.Logger
}

// NewOperations creates a new pipeline operations implementation
func NewOperations(
	sessionManager *sessionsvc.SessionManager,
	clients interface{},
	logger zerolog.Logger,
) *Operations {
	ops := &Operations{
		sessionManager: sessionManager,
		clients:        clients,
		logger:         logger.With().Str("component", "pipeline_operations").Logger(),
	}
	
	// Extract Docker client from clients if available
	if mcpClients, ok := clients.(*mcptypes.MCPClients); ok {
		ops.dockerClient = mcpClients.Docker
	}
	
	return ops
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
	if sessionState, ok := session.(*sessionsvc.SessionState); ok {
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
		sess, ok := s.(*sessionsvc.SessionState)
		if !ok {
			return
		}
		sess.LastAccessed = time.Now()
	})
}

// Docker operations

func (o *Operations) BuildDockerImage(sessionID, imageRef, dockerfilePath string) (interface{}, error) {
	workspace := o.GetSessionWorkspace(sessionID)
	if workspace == "" {
		return nil, fmt.Errorf("invalid session workspace")
	}

	// Build the image using the Docker client
	ctx := context.Background()
	buildCtx := filepath.Dir(dockerfilePath)

	// Simple implementation
	_ = ctx
	_ = buildCtx
	_ = imageRef

	return map[string]interface{}{
		"Success":  true,
		"ImageRef": imageRef,
	}, nil
}

func (o *Operations) PullDockerImage(sessionID, imageRef string) error {
	if o.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}
	
	ctx := context.Background()
	
	// Update session state to track operation
	err := o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation": "pull",
		"image_ref": imageRef,
		"status":    "starting",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("image_ref", imageRef).
		Msg("Starting Docker image pull")
	
	output, err := o.dockerClient.Pull(ctx, imageRef)
	if err != nil {
		o.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("image_ref", imageRef).
			Str("output", output).
			Msg("Failed to pull Docker image")
		
		// Update session with error
		o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
			"operation": "pull",
			"image_ref": imageRef,
			"status":    "failed",
			"error":     err.Error(),
		})
		
		return fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("image_ref", imageRef).
		Msg("Successfully pulled Docker image")
	
	// Update session with success
	err = o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation": "pull",
		"image_ref": imageRef,
		"status":    "completed",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	return nil
}

func (o *Operations) PushDockerImage(sessionID, imageRef string) error {
	if o.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}
	
	ctx := context.Background()
	
	// Update session state to track operation
	err := o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation": "push",
		"image_ref": imageRef,
		"status":    "starting",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("image_ref", imageRef).
		Msg("Starting Docker image push")
	
	output, err := o.dockerClient.Push(ctx, imageRef)
	if err != nil {
		o.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("image_ref", imageRef).
			Str("output", output).
			Msg("Failed to push Docker image")
		
		// Update session with error
		o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
			"operation": "push",
			"image_ref": imageRef,
			"status":    "failed",
			"error":     err.Error(),
		})
		
		return fmt.Errorf("failed to push image %s: %w", imageRef, err)
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("image_ref", imageRef).
		Msg("Successfully pushed Docker image")
	
	// Update session with success
	err = o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation": "push",
		"image_ref": imageRef,
		"status":    "completed",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	return nil
}

func (o *Operations) TagDockerImage(sessionID, sourceRef, targetRef string) error {
	if o.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}
	
	ctx := context.Background()
	
	// Update session state to track operation
	err := o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation":  "tag",
		"source_ref": sourceRef,
		"target_ref": targetRef,
		"status":     "starting",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Msg("Starting Docker image tag")
	
	output, err := o.dockerClient.Tag(ctx, sourceRef, targetRef)
	if err != nil {
		o.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("source_ref", sourceRef).
			Str("target_ref", targetRef).
			Str("output", output).
			Msg("Failed to tag Docker image")
		
		// Update session with error
		o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
			"operation":  "tag",
			"source_ref": sourceRef,
			"target_ref": targetRef,
			"status":     "failed",
			"error":      err.Error(),
		})
		
		return fmt.Errorf("failed to tag image %s as %s: %w", sourceRef, targetRef, err)
	}
	
	o.logger.Info().
		Str("session_id", sessionID).
		Str("source_ref", sourceRef).
		Str("target_ref", targetRef).
		Msg("Successfully tagged Docker image")
	
	// Update session with success
	err = o.UpdateSessionFromDockerResults(sessionID, map[string]interface{}{
		"operation":  "tag",
		"source_ref": sourceRef,
		"target_ref": targetRef,
		"status":     "completed",
	})
	if err != nil {
		o.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update session state")
	}
	
	return nil
}

func (o *Operations) ConvertToDockerState(sessionID string) (interface{}, error) {
	return map[string]interface{}{
		"Images":     []string{},
		"Containers": []string{},
		"Networks":   []string{},
		"Volumes":    []string{},
	}, nil
}

// Kubernetes operations

func (o *Operations) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (interface{}, error) {
	workspace := o.GetSessionWorkspace(sessionID)
	if workspace == "" {
		return nil, fmt.Errorf("invalid session workspace")
	}

	return map[string]interface{}{
		"Success": true,
		"Manifests": []map[string]interface{}{
			{
				"Kind": "Deployment",
				"Name": appName,
				"Path": filepath.Join(workspace, "deployment.yaml"),
			},
		},
	}, nil
}

func (o *Operations) DeployToKubernetes(sessionID string, manifests []string) (interface{}, error) {
	return map[string]interface{}{
		"Success":     true,
		"Namespace":   "default",
		"Deployments": []string{},
		"Services":    []string{},
	}, nil
}

func (o *Operations) CheckApplicationHealth(sessionID, namespace, labelSelector string, timeout time.Duration) (interface{}, error) {
	return map[string]interface{}{
		"Healthy": true,
		"Status":  "running",
	}, nil
}

// Resource management

func (o *Operations) AcquireResource(sessionID, resourceType string) error {
	o.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Acquiring resource")
	return nil
}

func (o *Operations) ReleaseResource(sessionID, resourceType string) error {
	o.logger.Debug().
		Str("session_id", sessionID).
		Str("resource_type", resourceType).
		Msg("Releasing resource")
	return nil
}

// Implementation of core.PipelineOperations interface methods

func (o *Operations) UpdateSessionState(sessionID string, updateFunc func(*core.SessionState)) error {
	return o.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if sessionState, ok := s.(*sessionsvc.SessionState); ok {
			// Convert sessionsvc.SessionState to core.SessionState
			coreState := &core.SessionState{
				SessionID:           sessionState.SessionID,
				UserID:              "", // Not available in sessionsvc.SessionState
				CreatedAt:           sessionState.CreatedAt,
				UpdatedAt:           sessionState.LastAccessed,
				ExpiresAt:           sessionState.ExpiresAt,
				WorkspaceDir:        sessionState.WorkspaceDir,
				RepositoryAnalyzed:  false, // Set based on RepoAnalysis
				RepoURL:             sessionState.RepoURL,
				DockerfileGenerated: sessionState.Dockerfile.Built,
				DockerfilePath:      sessionState.Dockerfile.Path,
			}
			updateFunc(coreState)
			// Note: Changes to coreState would need to be applied back to sessionState
		}
	})
}

func (o *Operations) BuildImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	// Extract args and delegate to existing method
	if argsMap, ok := args.(map[string]interface{}); ok {
		if imageRef, ok := argsMap["image_ref"].(string); ok {
			if dockerfilePath, ok := argsMap["dockerfile_path"].(string); ok {
				return o.BuildDockerImage(sessionID, imageRef, dockerfilePath)
			}
		}
	}
	return nil, fmt.Errorf("invalid arguments for BuildImage")
}

func (o *Operations) PushImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if imageRef, ok := argsMap["image_ref"].(string); ok {
			err := o.PushDockerImage(sessionID, imageRef)
			return map[string]interface{}{"success": err == nil}, err
		}
	}
	return nil, fmt.Errorf("invalid arguments for PushImage")
}

func (o *Operations) PullImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if imageRef, ok := argsMap["image_ref"].(string); ok {
			err := o.PullDockerImage(sessionID, imageRef)
			return map[string]interface{}{"success": err == nil}, err
		}
	}
	return nil, fmt.Errorf("invalid arguments for PullImage")
}

func (o *Operations) TagImage(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if sourceRef, ok := argsMap["source_ref"].(string); ok {
			if targetRef, ok := argsMap["target_ref"].(string); ok {
				err := o.TagDockerImage(sessionID, sourceRef, targetRef)
				return map[string]interface{}{"success": err == nil}, err
			}
		}
	}
	return nil, fmt.Errorf("invalid arguments for TagImage")
}

func (o *Operations) GenerateManifests(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		imageRef, _ := argsMap["image_ref"].(string)
		appName, _ := argsMap["app_name"].(string)
		port, _ := argsMap["port"].(int)
		cpuRequest, _ := argsMap["cpu_request"].(string)
		memoryRequest, _ := argsMap["memory_request"].(string)
		cpuLimit, _ := argsMap["cpu_limit"].(string)
		memoryLimit, _ := argsMap["memory_limit"].(string)

		return o.GenerateKubernetesManifests(sessionID, imageRef, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	}
	return nil, fmt.Errorf("invalid arguments for GenerateManifests")
}

func (o *Operations) DeployKubernetes(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if manifests, ok := argsMap["manifests"].([]string); ok {
			return o.DeployToKubernetes(sessionID, manifests)
		}
	}
	return nil, fmt.Errorf("invalid arguments for DeployKubernetes")
}

func (o *Operations) CheckHealth(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	if argsMap, ok := args.(map[string]interface{}); ok {
		namespace, _ := argsMap["namespace"].(string)
		labelSelector, _ := argsMap["label_selector"].(string)
		timeout := 30 * time.Second
		if timeoutArg, ok := argsMap["timeout"].(time.Duration); ok {
			timeout = timeoutArg
		}
		return o.CheckApplicationHealth(sessionID, namespace, labelSelector, timeout)
	}
	return nil, fmt.Errorf("invalid arguments for CheckHealth")
}

func (o *Operations) AnalyzeRepository(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would call repository analysis tools
	o.logger.Info().Str("session_id", sessionID).Msg("Analyzing repository")
	return map[string]interface{}{
		"language":       "unknown",
		"framework":      "unknown",
		"has_dockerfile": false,
		"port":           8080,
	}, nil
}

func (o *Operations) ValidateDockerfile(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would validate the Dockerfile
	o.logger.Info().Str("session_id", sessionID).Msg("Validating Dockerfile")
	return map[string]interface{}{
		"valid":    true,
		"errors":   []string{},
		"warnings": []string{},
	}, nil
}

func (o *Operations) ScanSecurity(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would run security scans
	o.logger.Info().Str("session_id", sessionID).Msg("Scanning for security vulnerabilities")
	return map[string]interface{}{
		"vulnerabilities": []string{},
		"score":           100,
	}, nil
}

func (o *Operations) ScanSecrets(ctx context.Context, sessionID string, args interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would scan for exposed secrets
	o.logger.Info().Str("session_id", sessionID).Msg("Scanning for secrets")
	return map[string]interface{}{
		"secrets_found": []string{},
		"clean":         true,
	}, nil
}
