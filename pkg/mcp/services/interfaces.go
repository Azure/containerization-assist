package services

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

type (
	// Session represents a user session with metadata
	Session struct {
		ID        string                 `json:"id"`
		Metadata  map[string]interface{} `json:"metadata"`
		CreatedAt time.Time              `json:"created_at"`
		UpdatedAt time.Time              `json:"updated_at"`
	}

	// BuildConfig represents build configuration
	BuildConfig struct {
		ContextPath string            `json:"context_path"`
		Dockerfile  string            `json:"dockerfile"`
		Tags        []string          `json:"tags"`
		Args        map[string]string `json:"args"`
		Target      string            `json:"target,omitempty"`
		Platform    string            `json:"platform,omitempty"`
	}

	// BuildResult represents build output
	BuildResult struct {
		ImageID  string   `json:"image_id"`
		Tags     []string `json:"tags"`
		Size     int64    `json:"size"`
		Duration int64    `json:"duration_ms"`
	}

	// PushConfig represents push configuration
	PushConfig struct {
		ImageTag string `json:"image_tag"`
		Registry string `json:"registry"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// PullConfig represents pull configuration
	PullConfig struct {
		ImageTag string `json:"image_tag"`
		Registry string `json:"registry"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// ScanConfig represents security scan configuration
	ScanConfig struct {
		ImageTag string `json:"image_tag"`
		Scanner  string `json:"scanner"`
		Severity string `json:"severity"`
	}

	// ScanResult represents security scan result
	ScanResult struct {
		Vulnerabilities []Vulnerability `json:"vulnerabilities"`
		Summary         ScanSummary     `json:"summary"`
	}

	// Vulnerability represents a single vulnerability
	Vulnerability struct {
		ID          string `json:"id"`
		Severity    string `json:"severity"`
		Package     string `json:"package"`
		Version     string `json:"version"`
		FixedIn     string `json:"fixed_in,omitempty"`
		Description string `json:"description"`
	}

	// ScanSummary represents scan summary
	ScanSummary struct {
		Total    int `json:"total"`
		Critical int `json:"critical"`
		High     int `json:"high"`
		Medium   int `json:"medium"`
		Low      int `json:"low"`
	}

	// RepoScanConfig represents repository scan configuration
	RepoScanConfig struct {
		Path    string `json:"path"`
		Scanner string `json:"scanner"`
	}

	// DeployConfig represents deployment configuration
	DeployConfig struct {
		Image       string            `json:"image"`
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Replicas    int32             `json:"replicas"`
		Ports       []int32           `json:"ports"`
		Environment map[string]string `json:"environment"`
		Strategy    string            `json:"strategy"`
	}

	// DeployResult represents deployment result
	DeployResult struct {
		DeploymentID string    `json:"deployment_id"`
		Status       string    `json:"status"`
		CreatedAt    time.Time `json:"created_at"`
	}

	// DeployStatus represents deployment status
	DeployStatus struct {
		ID            string    `json:"id"`
		Status        string    `json:"status"`
		ReadyReplicas int32     `json:"ready_replicas"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	// Workflow represents a workflow definition
	Workflow struct {
		ID    string                 `json:"id"`
		Name  string                 `json:"name"`
		Steps []WorkflowStep         `json:"steps"`
		Vars  map[string]interface{} `json:"vars"`
	}

	// WorkflowStep represents a single workflow step
	WorkflowStep struct {
		Name    string                 `json:"name"`
		Tool    string                 `json:"tool"`
		Params  map[string]interface{} `json:"params"`
		Depends []string               `json:"depends_on"`
	}

	// WorkflowResult represents workflow execution result
	WorkflowResult struct {
		ID        string                 `json:"id"`
		Status    string                 `json:"status"`
		Results   map[string]interface{} `json:"results"`
		StartedAt time.Time              `json:"started_at"`
		EndedAt   time.Time              `json:"ended_at"`
	}

	// WorkflowStatus represents workflow status
	WorkflowStatus struct {
		ID             string    `json:"id"`
		Status         string    `json:"status"`
		CurrentStep    string    `json:"current_step"`
		CompletedSteps []string  `json:"completed_steps"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	// SessionConfig represents session configuration
	SessionConfig struct {
		DatabasePath string `json:"database_path"`
		Timeout      int    `json:"timeout_seconds"`
	}

	// ServiceMetrics represents service metrics
	ServiceMetrics struct {
		RequestCount  int64     `json:"request_count"`
		ErrorCount    int64     `json:"error_count"`
		AvgDuration   int64     `json:"avg_duration_ms"`
		LastRequested time.Time `json:"last_requested"`
	}

	// HealthStatus represents service health
	HealthStatus struct {
		Status    string            `json:"status"`
		Version   string            `json:"version"`
		Uptime    time.Duration     `json:"uptime"`
		Checks    map[string]string `json:"checks"`
		Timestamp time.Time         `json:"timestamp"`
	}
)

// SessionStore provides session persistence operations
type SessionStore interface {
	Create(ctx context.Context, metadata map[string]interface{}) (string, error)
	Get(ctx context.Context, sessionID string) (*api.Session, error)
	Update(ctx context.Context, sessionID string, data map[string]interface{}) error
	Delete(ctx context.Context, sessionID string) error
}

// SessionState manages session state and checkpoints
type SessionState interface {
	SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error
	LoadState(ctx context.Context, sessionID string) (map[string]interface{}, error)
	SaveCheckpoint(ctx context.Context, sessionID string, data interface{}) error
	LoadCheckpoint(ctx context.Context, sessionID string) (interface{}, error)
}

// BuildExecutor handles container build operations
type BuildExecutor interface {
	BuildImage(ctx context.Context, args *api.BuildArgs) (*api.BuildResult, error)
	GetBuildStatus(ctx context.Context, buildID string) (*api.BuildStatus, error)
	CancelBuild(ctx context.Context, buildID string) error
	ClearBuildCache(ctx context.Context) error
	GetCacheStats(ctx context.Context) (*api.CacheStats, error)
}

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	RegisterTool(tool api.Tool, opts ...api.RegistryOption) error
	UnregisterTool(name string) error
	GetTool(name string) (api.Tool, error)
	ListTools() []string
	GetMetadata(name string) (api.ToolMetadata, error)
}

// WorkflowExecutor orchestrates multi-step operations
type WorkflowExecutor interface {
	Execute(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error)
	GetStatus(workflowID string) (*api.WorkflowStatus, error)
	Cancel(workflowID string) error
	Validate(workflow *api.Workflow) error
}

// Scanner provides security scanning capabilities
type Scanner interface {
	ScanImage(ctx context.Context, config *ScanConfig) (*ScanResult, error)
	ScanRepository(ctx context.Context, config *RepoScanConfig) (*ScanResult, error)
	GetSecurityReport(ctx context.Context, target string) (*ScanResult, error)
}

// ConfigValidator validates configurations using unified validation
type ConfigValidator interface {
	ValidateSession(metadata map[string]interface{}) error
	ValidateBuild(args *api.BuildArgs) error
	ValidateWorkflow(workflow *api.Workflow) error
	ValidateDeploy(config *DeployConfig) error
}

// ErrorReporter provides unified error reporting
type ErrorReporter interface {
	Report(ctx context.Context, err error) error
	Wrap(err error, message string) error
	New(message string) error
}

// ServiceContainer provides access to all services for dependency injection
type ServiceContainer interface {
	// Session services
	SessionStore() SessionStore
	SessionState() SessionState

	// Build services
	BuildExecutor() BuildExecutor

	// Registry services
	ToolRegistry() ToolRegistry

	// Workflow services
	WorkflowExecutor() WorkflowExecutor

	// Security services
	Scanner() Scanner

	// Cross-cutting services
	ConfigValidator() ConfigValidator
	ErrorReporter() ErrorReporter

	// Lifecycle
	Close() error
}

// Service is the base interface that all services implement
type Service interface {
	Name() string
	Version() string
	Health() HealthStatus
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Metrics() ServiceMetrics
}
