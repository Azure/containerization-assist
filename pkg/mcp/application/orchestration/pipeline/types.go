package pipeline

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain"
	sessionsvc "github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// TypedBuildImageArgs is an alias for mcptypes.TypedBuildImageArgs
type TypedBuildImageArgs = mcptypes.TypedBuildImageArgs
type TypedPushImageArgs = mcptypes.TypedPushImageArgs
type TypedPullImageArgs = mcptypes.TypedPullImageArgs
type TypedTagImageArgs = mcptypes.TypedTagImageArgs
type TypedGenerateManifestsArgs = mcptypes.TypedGenerateManifestsArgs
type TypedDeployKubernetesArgs = mcptypes.TypedDeployKubernetesArgs
type TypedCheckHealthArgs = mcptypes.TypedCheckHealthArgs
type TypedAnalyzeRepositoryArgs = mcptypes.TypedAnalyzeRepositoryArgs
type TypedValidateDockerfileArgs = mcptypes.TypedValidateDockerfileArgs
type TypedScanSecurityArgs = mcptypes.TypedScanSecurityArgs

// TypedScanSecretsArgs represents scan secrets arguments
type TypedScanSecretsArgs struct {
	SessionID   string `json:"session_id"`
	RepoPath    string `json:"repo_path"`
	FilePattern string `json:"file_pattern,omitempty"`
}

// TypedOperationResult represents a generic operation result with timing and metadata
type TypedOperationResult struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// DockerBuildResult represents the result of a Docker build operation
type DockerBuildResult struct {
	Success  bool   `json:"success"`
	ImageRef string `json:"image_ref"`
	BuildID  string `json:"build_id,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DockerStateResult represents Docker state information
type DockerStateResult struct {
	Images     []string `json:"images"`
	Containers []string `json:"containers"`
	Networks   []string `json:"networks"`
	Volumes    []string `json:"volumes"`
}

// KubernetesManifestResult represents Kubernetes manifest generation result
type KubernetesManifestResult struct {
	Success   bool                 `json:"success"`
	Manifests []KubernetesManifest `json:"manifests"`
}

// KubernetesManifest represents a single Kubernetes manifest
type KubernetesManifest struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// KubernetesDeploymentResult represents deployment result
type KubernetesDeploymentResult struct {
	Success     bool     `json:"success"`
	Namespace   string   `json:"namespace"`
	Deployments []string `json:"deployments"`
	Services    []string `json:"services"`
}

// ApplicationHealthResult represents application health check result
type ApplicationHealthResult struct {
	Healthy bool   `json:"healthy"`
	Status  string `json:"status"`
	Pods    int    `json:"pods,omitempty"`
	Ready   int    `json:"ready,omitempty"`
}

// SessionOperationData represents typed session operation tracking data
type SessionOperationData struct {
	Operation string `json:"operation"`
	ImageRef  string `json:"image_ref,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
	TargetRef string `json:"target_ref,omitempty"`
	Status    string `json:"status"`
	JobID     string `json:"job_id,omitempty"`
	Error     string `json:"error,omitempty"`
	Output    string `json:"output,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// SessionErrorData represents typed session error tracking data
type SessionErrorData struct {
	Operation string `json:"operation"`
	ImageRef  string `json:"image_ref,omitempty"`
	Output    string `json:"output,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// JobCompletionData represents typed job completion data
type JobCompletionData struct {
	Operation string `json:"operation"`
	ImageRef  string `json:"image_ref,omitempty"`
	Output    string `json:"output,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// Operations implements TypedPipelineOperations directly without adapter pattern
type Operations struct {
	sessionManager *sessionsvc.SessionManager
	clients        *mcptypes.MCPClients
	dockerClient   docker.DockerClient
	logger         *slog.Logger
}

// Ensure Operations implements the required interfaces
var _ mcptypes.TypedPipelineOperations = (*Operations)(nil)
