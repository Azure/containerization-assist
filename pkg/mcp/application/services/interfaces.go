// Package services provides focused service interfaces as part of the EPSILON workstream.
// These interfaces replace the monolithic Manager interfaces with better testability
// and separation of concerns.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// ServiceContainer provides access to all services through dependency injection.
// This is the central interface for wiring services together.
type ServiceContainer interface {
	// Core services
	SessionStore() SessionStore
	SessionState() SessionState
	BuildExecutor() BuildExecutor
	ToolRegistry() ToolRegistry
	WorkflowExecutor() WorkflowExecutor
	Scanner() Scanner
	ConfigValidator() ConfigValidator
	ErrorReporter() ErrorReporter

	// Additional services
	StateManager() StateManager
	KnowledgeBase() KnowledgeBase
	K8sClient() K8sClient
	Analyzer() Analyzer
	Persistence() Persistence
	Logger() *slog.Logger
}

// SessionStore handles session persistence operations
type SessionStore interface {
	// Create creates a new session
	Create(ctx context.Context, session *api.Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*api.Session, error)

	// Update updates an existing session
	Update(ctx context.Context, session *api.Session) error

	// Delete removes a session
	Delete(ctx context.Context, sessionID string) error

	// List returns all sessions
	List(ctx context.Context) ([]*api.Session, error)
}

// SessionState manages session state and checkpoints
type SessionState interface {
	// SaveState saves the current state for a session
	SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error

	// GetState retrieves the state for a session
	GetState(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// CreateCheckpoint creates a checkpoint for rollback
	CreateCheckpoint(ctx context.Context, sessionID string, name string) error

	// RestoreCheckpoint restores to a previous checkpoint
	RestoreCheckpoint(ctx context.Context, sessionID string, name string) error

	// ListCheckpoints lists available checkpoints
	ListCheckpoints(ctx context.Context, sessionID string) ([]string, error)

	// GetWorkspaceDir gets the workspace directory for a session
	GetWorkspaceDir(ctx context.Context, sessionID string) (string, error)

	// SetWorkspaceDir sets the workspace directory for a session
	SetWorkspaceDir(ctx context.Context, sessionID string, dir string) error
}

// BuildExecutor handles container build operations
type BuildExecutor interface {
	// BuildImage builds a container image
	BuildImage(ctx context.Context, options BuildOptions) (*BuildResult, error)

	// PushImage pushes an image to a registry
	PushImage(ctx context.Context, options PushOptions) error

	// PullImage pulls an image from a registry
	PullImage(ctx context.Context, options PullOptions) error

	// TagImage tags an image
	TagImage(ctx context.Context, source, target string) error
}

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	// Register registers a new tool
	Register(name string, tool api.Tool) error

	// GetTool retrieves a tool by name
	GetTool(name string) (api.Tool, error)

	// ListTools lists all registered tools
	ListTools() []string

	// GetMetrics returns registry metrics
	GetMetrics() api.RegistryMetrics
}

// WorkflowExecutor handles multi-step workflow execution
type WorkflowExecutor interface {
	// ExecuteWorkflow runs a complete workflow
	ExecuteWorkflow(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error)

	// ExecuteStep executes a single workflow step
	ExecuteStep(ctx context.Context, step *api.WorkflowStep) (*api.StepResult, error)

	// ValidateWorkflow validates a workflow definition
	ValidateWorkflow(workflow *api.Workflow) error
}

// Scanner provides security scanning capabilities
type Scanner interface {
	// ScanImage scans a container image for vulnerabilities
	ScanImage(ctx context.Context, image string, options ScanOptions) (*ScanResult, error)

	// ScanDirectory scans a directory for secrets
	ScanDirectory(ctx context.Context, path string, options ScanOptions) (*ScanResult, error)

	// GetScanners returns available scanner types
	GetScanners() []string
}

// ConfigValidator validates configuration using BETA's validation framework
type ConfigValidator interface {
	// ValidateDockerfile validates a Dockerfile
	ValidateDockerfile(content string) (*ValidationResult, error)

	// ValidateManifest validates a Kubernetes manifest
	ValidateManifest(content string) (*ValidationResult, error)

	// ValidateConfig validates general configuration
	ValidateConfig(config map[string]interface{}) (*ValidationResult, error)
}

// ErrorReporter provides unified error reporting and recovery
type ErrorReporter interface {
	// ReportError reports an error with context
	ReportError(ctx context.Context, err error, context map[string]interface{})

	// GetErrorStats returns error statistics
	GetErrorStats() ErrorStats

	// SuggestFix suggests fixes for common errors
	SuggestFix(err error) []string
}

// StateManager manages application state beyond sessions
type StateManager interface {
	// SaveState saves state with a key
	SaveState(ctx context.Context, key string, state interface{}) error

	// GetState retrieves state by key
	GetState(ctx context.Context, key string, state interface{}) error

	// UpdateState updates existing state
	UpdateState(ctx context.Context, key string, tool string, state interface{}) error

	// DeleteState removes state
	DeleteState(ctx context.Context, key string) error
}

// KnowledgeBase stores and retrieves learned patterns
type KnowledgeBase interface {
	// Store stores a pattern or solution
	Store(ctx context.Context, key string, value interface{}) error

	// Retrieve gets a stored pattern
	Retrieve(ctx context.Context, key string) (interface{}, error)

	// Search searches for patterns
	Search(ctx context.Context, query string) ([]interface{}, error)
}

// K8sClient provides Kubernetes operations
type K8sClient interface {
	// Apply applies manifests to a cluster
	Apply(ctx context.Context, manifests string, namespace string) error

	// Delete removes resources from a cluster
	Delete(ctx context.Context, manifests string, namespace string) error

	// GetStatus gets resource status
	GetStatus(ctx context.Context, resource, name, namespace string) (interface{}, error)
}

// Analyzer provides code and repository analysis
type Analyzer interface {
	// AnalyzeRepository analyzes a repository
	AnalyzeRepository(ctx context.Context, path string) (*AnalysisResult, error)

	// DetectFramework detects the framework used
	DetectFramework(ctx context.Context, path string) (string, error)

	// GenerateDockerfile generates a Dockerfile
	GenerateDockerfile(ctx context.Context, analysis *AnalysisResult) (string, error)
}

// Supporting types

// BuildOptions configures image builds
type BuildOptions struct {
	ContextPath    string
	DockerfilePath string
	Tags           []string
	BuildArgs      map[string]string
	NoCache        bool
	Platform       string
}

// BuildResult contains build results
type BuildResult struct {
	ImageID   string
	ImageSize int64
	Duration  time.Duration
	Logs      []string
}

// PushOptions configures image push operations
type PushOptions struct {
	Image    string
	Registry string
	Auth     AuthConfig
}

// PullOptions configures image pull operations
type PullOptions struct {
	Image    string
	Registry string
	Auth     AuthConfig
}

// AuthConfig contains registry authentication
type AuthConfig struct {
	Username string
	Password string
	Token    string
}

// ScanOptions configures scanning
type ScanOptions struct {
	Severity  string
	Scanners  []string
	Timeout   time.Duration
	MaxIssues int
}

// ScanResult contains scan results
type ScanResult struct {
	Vulnerabilities []Vulnerability
	Secrets         []Secret
	Score           int
	Summary         string
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string
	Severity    string
	Package     string
	Version     string
	FixVersion  string
	Description string
}

// Secret represents a detected secret
type Secret struct {
	Type     string
	File     string
	Line     int
	Value    string
	Severity string
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
	Score    int
}

// ValidationError represents a validation error
type ValidationError struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int
	Column  int
	Message string
	Rule    string
}

// ErrorStats contains error statistics
type ErrorStats struct {
	TotalErrors  int64
	ErrorsByType map[string]int64
	RecentErrors []ErrorEntry
	RecoveryRate float64
}

// ErrorEntry represents a single error occurrence
type ErrorEntry struct {
	Timestamp time.Time
	Error     string
	Context   map[string]interface{}
	Fixed     bool
}

// AnalysisResult contains repository analysis results
type AnalysisResult struct {
	Language     string
	Framework    string
	Dependencies []string
	EntryPoint   string
	Port         int
	BuildCommand string
	RunCommand   string
}

// Persistence handles persistent storage operations
type Persistence interface {
	// Put stores a key-value pair
	Put(ctx context.Context, bucket string, key string, value interface{}) error
	
	// Get retrieves a value by key
	Get(ctx context.Context, bucket string, key string, result interface{}) error
	
	// Delete removes a key-value pair
	Delete(ctx context.Context, bucket string, key string) error
	
	// List returns all key-value pairs in a bucket
	List(ctx context.Context, bucket string) (map[string]interface{}, error)
	
	// Close closes the persistence layer
	Close() error
}
