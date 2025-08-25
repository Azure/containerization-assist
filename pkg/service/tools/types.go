// Package tools provides typed structures for tool operations
package tools

import (
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
)

// ToolMetadata represents metadata for tool operations
type ToolMetadata struct {
	SessionID   string            `json:"session_id,omitempty"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	Step        string            `json:"step,omitempty"`
	Timestamp   time.Time         `json:"timestamp,omitempty"`
	Version     string            `json:"version,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}

// ToolParameters represents parameters passed to tools
type ToolParameters struct {
	RepoPath        string            `json:"repo_path,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	Registry        string            `json:"registry,omitempty"`
	Tag             string            `json:"tag,omitempty"`
	FixingMode      bool              `json:"fixing_mode,omitempty"`
	PreviousError   string            `json:"previous_error,omitempty"`
	FailedTool      string            `json:"failed_tool,omitempty"`
	RedirectAttempt int               `json:"redirect_attempt,omitempty"`
	MaxRetries      int               `json:"max_retries,omitempty"`
	Custom          map[string]string `json:"custom,omitempty"`
}

// WorkflowArtifacts represents artifacts produced during workflow execution
type WorkflowArtifacts struct {
	AnalyzeResult    *AnalyzeArtifact    `json:"analyze_result,omitempty"`
	DockerfileResult *DockerfileArtifact `json:"dockerfile_result,omitempty"`
	BuildResult      *BuildArtifact      `json:"build_result,omitempty"`
	K8sResult        *K8sArtifact        `json:"k8s_result,omitempty"`
	ScanResult       *ScanArtifact       `json:"scan_result,omitempty"`
}

// AnalyzeArtifact represents repository analysis results
type AnalyzeArtifact struct {
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	Port            int                    `json:"port"`
	BuildCommand    string                 `json:"build_command"`
	StartCommand    string                 `json:"start_command"`
	Dependencies    []workflow.Dependency  `json:"dependencies,omitempty"`
	DevDependencies []string               `json:"dev_dependencies,omitempty"`
	RepoPath        string                 `json:"repo_path"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// DockerfileArtifact represents generated Dockerfile information
type DockerfileArtifact struct {
	Content   string                 `json:"content"`
	Path      string                 `json:"path"`
	BaseImage string                 `json:"base_image,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BuildArtifact represents Docker build results
type BuildArtifact struct {
	ImageID   string                 `json:"image_id"`
	ImageRef  string                 `json:"image_ref"`
	ImageSize int64                  `json:"image_size"`
	BuildTime string                 `json:"build_time"`
	Layers    []string               `json:"layers,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// K8sArtifact represents Kubernetes deployment artifacts
type K8sArtifact struct {
	Manifests []string               `json:"manifests"`
	Namespace string                 `json:"namespace"`
	Endpoint  string                 `json:"endpoint"`
	Services  []string               `json:"services,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ScanArtifact represents security scan results
type ScanArtifact struct {
	VulnerabilityCount int                    `json:"vulnerability_count"`
	Critical           int                    `json:"critical"`
	High               int                    `json:"high"`
	Medium             int                    `json:"medium"`
	Low                int                    `json:"low"`
	ScanTime           time.Time              `json:"scan_time"`
	Scanner            string                 `json:"scanner"`
	Details            []VulnerabilityDetail  `json:"details,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// VulnerabilityDetail represents a single vulnerability
type VulnerabilityDetail struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	Description string `json:"description"`
	FixVersion  string `json:"fix_version,omitempty"`
}

// StepResult represents the result of a workflow step
type StepResult struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
	Error    *StepError        `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// StepError represents an error from a workflow step
type StepError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    string            `json:"details,omitempty"`
	Retryable  bool              `json:"retryable"`
	Suggestion string            `json:"suggestion,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
}

// ProgressUpdate represents a progress update event
type ProgressUpdate struct {
	Step       int               `json:"step"`
	Total      int               `json:"total"`
	Message    string            `json:"message"`
	Percentage int               `json:"percentage"`
	Stage      string            `json:"stage"`
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ManifestContent represents Kubernetes manifest files
type ManifestContent struct {
	Deployment string `json:"deployment.yaml"`
	Service    string `json:"service.yaml"`
	Ingress    string `json:"ingress.yaml,omitempty"`
	ConfigMap  string `json:"configmap.yaml,omitempty"`
	Secret     string `json:"secret.yaml,omitempty"`
}
