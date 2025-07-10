// Package services provides focused service interfaces as part of the EPSILON workstream.
// These interfaces replace the monolithic Manager interfaces with better testability
// and separation of concerns.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// PipelineService defines the interface for pipeline orchestration without importing the concrete type
type PipelineService interface {
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool

	// Job management
	CancelJob(ctx context.Context, jobID string) error
}

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

	// Conversation services
	ConversationService() ConversationService
	PromptService() PromptService

	// Infrastructure services
	DockerService() docker.Service
	ManifestService() kubernetes.ManifestService
	DeploymentService() kubernetes.Service
	PipelineService() PipelineService
}

// ConversationService handles chat-based interactions per ADR-006
type ConversationService interface {
	// ProcessMessage handles a user message in a conversation
	ProcessMessage(ctx context.Context, sessionID, message string) (*ConversationResponse, error)

	// GetConversationState returns the current conversation state
	GetConversationState(ctx context.Context, sessionID string) (*ConversationState, error)

	// UpdateConversationStage updates the conversation stage
	UpdateConversationStage(ctx context.Context, sessionID string, stage domaintypes.ConversationStage) error

	// GetConversationHistory returns conversation history
	GetConversationHistory(ctx context.Context, sessionID string, limit int) ([]ConversationTurn, error)

	// ClearConversationContext clears conversation context
	ClearConversationContext(ctx context.Context, sessionID string) error
}

// PromptService manages AI prompt interactions per ADR-006
type PromptService interface {
	// BuildPrompt creates a prompt for the given stage and context
	BuildPrompt(ctx context.Context, stage domaintypes.ConversationStage, promptContext map[string]interface{}) (string, error)

	// ProcessPromptResponse processes AI response and updates state
	ProcessPromptResponse(ctx context.Context, response string, state *ConversationState) error

	// DetectWorkflowIntent detects if message indicates workflow intent
	DetectWorkflowIntent(ctx context.Context, message string) (*WorkflowIntent, error)

	// ShouldAutoAdvance determines if conversation should auto-advance
	ShouldAutoAdvance(ctx context.Context, state *ConversationState) (bool, *AutoAdvanceConfig)
}

// ConversationResponse represents a response from the conversation service
type ConversationResponse struct {
	SessionID     string                         `json:"session_id"`
	Message       string                         `json:"message"`
	Stage         domaintypes.ConversationStage  `json:"stage"`
	Status        string                         `json:"status"`
	Options       []Option                       `json:"options,omitempty"`
	Artifacts     []ArtifactSummary              `json:"artifacts,omitempty"`
	NextSteps     []string                       `json:"next_steps,omitempty"`
	Progress      *StageProgress                 `json:"progress,omitempty"`
	ToolCalls     []ToolCall                     `json:"tool_calls,omitempty"`
	RequiresInput bool                           `json:"requires_input"`
	NextStage     *domaintypes.ConversationStage `json:"next_stage,omitempty"`
	AutoAdvance   *AutoAdvanceConfig             `json:"auto_advance,omitempty"`
}

// ConversationState represents the state of a conversation
type ConversationState struct {
	SessionID        string                        `json:"session_id"`
	CurrentStage     domaintypes.ConversationStage `json:"current_stage"`
	History          []ConversationTurn            `json:"conversation_history"`
	Preferences      domaintypes.UserPreferences   `json:"user_preferences"`
	PendingDecision  *DecisionPoint                `json:"pending_decision,omitempty"`
	WorkflowSession  *WorkflowSession              `json:"workflow_session,omitempty"`
	LastActivity     time.Time                     `json:"last_activity"`
	RetryState       *RetryState                   `json:"retry_state,omitempty"`
	AutoAdvanceState *AutoAdvanceState             `json:"auto_advance_state,omitempty"`
}

// ConversationTurn represents a single turn in a conversation
type ConversationTurn struct {
	ID        string                        `json:"id"`
	Timestamp time.Time                     `json:"timestamp"`
	Role      string                        `json:"role"`
	Content   string                        `json:"content"`
	Stage     domaintypes.ConversationStage `json:"stage"`
	Metadata  map[string]interface{}        `json:"metadata,omitempty"`
}

// Supporting types for the conversation service
type Option struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ArtifactSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type StageProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type ToolCall struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Args   interface{} `json:"args"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type DecisionPoint struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []Option `json:"options"`
}

type WorkflowSession struct {
	SessionID string                 `json:"session_id"`
	Context   map[string]interface{} `json:"context"`
}

type RetryState struct {
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	LastError   string    `json:"last_error,omitempty"`
}

type AutoAdvanceConfig struct {
	Enabled bool          `json:"enabled"`
	Delay   time.Duration `json:"delay"`
}

type AutoAdvanceState struct {
	Scheduled bool      `json:"scheduled"`
	NextTime  time.Time `json:"next_time"`
}

type WorkflowIntent struct {
	Detected   bool                   `json:"detected"`
	Workflow   string                 `json:"workflow"`
	Parameters map[string]interface{} `json:"parameters"`
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

	// GetSessionMetadata gets session metadata
	GetSessionMetadata(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// UpdateSessionData updates session data
	UpdateSessionData(ctx context.Context, sessionID string, data map[string]interface{}) error
}

// BuildExecutor handles container build operations
// This interface wraps the core docker.Service to provide focused build operations
type BuildExecutor interface {
	// QuickBuild performs a quick build without template generation
	QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options docker.BuildOptions) (*docker.BuildResult, error)

	// QuickPush performs a quick push of an already built image
	QuickPush(ctx context.Context, imageRef string, options docker.PushOptions) (*docker.RegistryPushResult, error)

	// QuickPull performs a quick pull of an image
	QuickPull(ctx context.Context, imageRef string) (*docker.PullResult, error)
}

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	// Register registers a new tool
	Register(ctx context.Context, name string, tool api.Tool) error

	// GetTool retrieves a tool by name
	GetTool(ctx context.Context, name string) (api.Tool, error)

	// ListTools lists all registered tools
	ListTools(ctx context.Context) []string

	// GetMetrics returns registry metrics
	GetMetrics(ctx context.Context) api.RegistryMetrics
}

// WorkflowExecutor handles multi-step workflow execution
type WorkflowExecutor interface {
	// ExecuteWorkflow runs a complete workflow
	ExecuteWorkflow(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error)

	// ExecuteStep executes a single workflow step
	ExecuteStep(ctx context.Context, step *api.WorkflowStep) (*api.StepResult, error)

	// ValidateWorkflow validates a workflow definition
	ValidateWorkflow(ctx context.Context, workflow *api.Workflow) error
}

// Scanner provides security scanning capabilities
// This interface wraps the core security.Service
type Scanner interface {
	// ScanImage scans a container image for vulnerabilities
	ScanImage(ctx context.Context, image string, options security.ScanOptionsService) (*security.ScanResult, error)

	// ScanDirectory scans a directory for secrets
	ScanDirectory(ctx context.Context, path string, options security.ScanOptionsService) (*security.ScanResult, error)

	// GetAvailableScanners returns available scanner types
	GetAvailableScanners(ctx context.Context) []string
}

// ConfigValidator validates configuration using BETA's validation framework
type ConfigValidator interface {
	// ValidateDockerfile validates a Dockerfile
	ValidateDockerfile(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateManifest validates a Kubernetes manifest
	ValidateManifest(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateConfig validates general configuration
	ValidateConfig(ctx context.Context, config map[string]interface{}) (*ValidationResult, error)
}

// ErrorReporter provides unified error reporting and recovery
type ErrorReporter interface {
	// ReportError reports an error with context
	ReportError(ctx context.Context, err error, context map[string]interface{})

	// GetErrorStats returns error statistics
	GetErrorStats(ctx context.Context) ErrorStats

	// SuggestFix suggests fixes for common errors
	SuggestFix(ctx context.Context, err error) []string
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
// This interface wraps the core analysis.RepositoryAnalyzer
type Analyzer interface {
	// AnalyzeRepository analyzes a repository (matches core analysis.RepositoryAnalyzer)
	AnalyzeRepository(ctx context.Context, repoPath string) (*analysis.AnalysisResult, error)
}

// AnalysisService provides analysis operations for backward compatibility
type AnalysisService interface {
	// AnalyzeRepository analyzes a repository with progress callback
	AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error)

	// AnalyzeWithAI performs AI-powered analysis
	AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)

	// GetAnalysisProgress gets the progress of an ongoing analysis
	GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error)
}

// Supporting types

// Use core docker types directly
// type BuildOptions = docker.BuildOptions
// type BuildResult = docker.BuildResult
// type PushOptions = docker.PushOptions
// type PullOptions = docker.PullOptions

// Use core security types directly
// type ScanOptions = security.ScanOptionsService
// type ScanResult = security.ScanResult

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

// Use the core analysis.AnalysisResult directly
// type AnalysisResult = analysis.AnalysisResult

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

// Analysis-related types for backward compatibility

// ProgressCallback is called during long-running operations to report progress
type ProgressCallback func(progress AnalysisProgress)

// RepositoryAnalysis represents the result of analyzing a repository
type RepositoryAnalysis struct {
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	Dependencies    []string               `json:"dependencies"`
	EntryPoint      string                 `json:"entry_point"`
	Port            int                    `json:"port"`
	BuildCommand    string                 `json:"build_command"`
	RunCommand      string                 `json:"run_command"`
	Issues          []AnalysisIssue        `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// AnalysisIssue represents an issue found during analysis
type AnalysisIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Suggestion string `json:"suggestion"`
}

// AIAnalysis represents the result of AI-powered analysis
type AIAnalysis struct {
	Summary         string                 `json:"summary"`
	Insights        []string               `json:"insights"`
	Recommendations []string               `json:"recommendations"`
	Confidence      float64                `json:"confidence"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// AnalysisProgress represents the progress of an ongoing analysis
type AnalysisProgress struct {
	AnalysisID    string         `json:"analysis_id"`
	Status        string         `json:"status"`
	CurrentStep   string         `json:"current_step"`
	StepNumber    int            `json:"step_number"`
	TotalSteps    int            `json:"total_steps"`
	Percentage    float64        `json:"percentage"`
	ElapsedTime   time.Duration  `json:"elapsed_time"`
	EstimatedTime *time.Duration `json:"estimated_time,omitempty"`
	LastUpdate    time.Time      `json:"last_update"`
}
