package types

import "time"

// ErrorInputContext provides strongly typed input context for error situations
type ErrorInputContext struct {
	// Tool identification
	ToolName string `json:"tool_name"`

	// Input arguments that caused the error
	Arguments map[string]interface{} `json:"arguments"`

	// Configuration used during operation
	Configuration map[string]interface{} `json:"configuration,omitempty"`

	// User-provided input that triggered the failure
	UserInput string `json:"user_input,omitempty"`
}

// ErrorPartialOutput captures incomplete results from failed operations
type ErrorPartialOutput struct {
	// Steps that were completed successfully before failure
	CompletedSteps []string `json:"completed_steps"`

	// Intermediate results produced before failure
	IntermediateResults map[string]interface{} `json:"intermediate_results,omitempty"`

	// Output fragments captured before failure
	OutputFragments []string `json:"output_fragments,omitempty"`

	// Resources that were successfully created before failure
	ResourcesCreated []string `json:"resources_created,omitempty"`

	// Files generated before failure
	FilesGenerated []string `json:"files_generated,omitempty"`
}

// ErrorMetadata provides strongly typed metadata with fallback for unknown fields
type ErrorMetadata struct {
	// Core identifiers
	SessionID string `json:"session_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	Operation string `json:"operation,omitempty"`

	// Context-specific metadata
	BuildContext      *BuildMetadata      `json:"build_context,omitempty"`
	DeploymentContext *DeploymentMetadata `json:"deployment_context,omitempty"`
	RepositoryContext *RepositoryMetadata `json:"repository_context,omitempty"`
	SecurityContext   *SecurityMetadata   `json:"security_context,omitempty"`

	// Performance and timing information
	Timing *TimingMetadata `json:"timing,omitempty"`

	// Session state information
	SessionState *SessionStateMetadata `json:"session_state,omitempty"`

	// Custom metadata for extensibility and edge cases
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// BuildMetadata contains build-specific error context
type BuildMetadata struct {
	DockerfilePath     string            `json:"dockerfile_path,omitempty"`
	DockerfileContent  string            `json:"dockerfile_content,omitempty"`
	BuildContextPath   string            `json:"build_context_path,omitempty"`
	BuildContextSizeMB int64             `json:"build_context_size_mb,omitempty"`
	ImageRef           string            `json:"image_ref,omitempty"`
	Platform           string            `json:"platform,omitempty"`
	BaseImage          string            `json:"base_image,omitempty"`
	BuildArgs          map[string]string `json:"build_args,omitempty"`
}

// DeploymentMetadata contains deployment-specific error context
type DeploymentMetadata struct {
	Namespace        string   `json:"namespace,omitempty"`
	ManifestPaths    []string `json:"manifest_paths,omitempty"`
	ClusterName      string   `json:"cluster_name,omitempty"`
	K8sContext       string   `json:"k8s_context,omitempty"`
	PodsChecked      int      `json:"pods_checked,omitempty"`
	PodsReady        int      `json:"pods_ready,omitempty"`
	PodsFailed       int      `json:"pods_failed,omitempty"`
	ServicesCount    int      `json:"services_count,omitempty"`
	ResourcesApplied []string `json:"resources_applied,omitempty"`
}

// RepositoryMetadata contains repository analysis error context
type RepositoryMetadata struct {
	RepoURL            string   `json:"repo_url,omitempty"`
	Branch             string   `json:"branch,omitempty"`
	CommitHash         string   `json:"commit_hash,omitempty"`
	IsLocal            bool     `json:"is_local,omitempty"`
	CloneDir           string   `json:"clone_dir,omitempty"`
	CloneError         string   `json:"clone_error,omitempty"`
	AuthMethod         string   `json:"auth_method,omitempty"`
	LanguageHints      []string `json:"language_hints,omitempty"`
	DetectedFrameworks []string `json:"detected_frameworks,omitempty"`
}

// SecurityMetadata contains security scan error context
type SecurityMetadata struct {
	ScannerType      string    `json:"scanner_type,omitempty"`
	ScanTarget       string    `json:"scan_target,omitempty"`
	VulnCount        int       `json:"vuln_count,omitempty"`
	CriticalVulns    int       `json:"critical_vulns,omitempty"`
	HighVulns        int       `json:"high_vulns,omitempty"`
	ScanDuration     string    `json:"scan_duration,omitempty"`
	ScannerVersion   string    `json:"scanner_version,omitempty"`
	PolicyViolations []string  `json:"policy_violations,omitempty"`
	LastScanTime     time.Time `json:"last_scan_time,omitempty"`
}

// TimingMetadata contains performance and timing information
type TimingMetadata struct {
	StartTime      time.Time                `json:"start_time,omitempty"`
	EndTime        time.Time                `json:"end_time,omitempty"`
	Duration       time.Duration            `json:"duration,omitempty"`
	TimeoutReached bool                     `json:"timeout_reached,omitempty"`
	RetryCount     int                      `json:"retry_count,omitempty"`
	PhaseTimings   map[string]time.Duration `json:"phase_timings,omitempty"`
}

// SessionStateMetadata contains session state information
type SessionStateMetadata struct {
	SessionID          string    `json:"session_id,omitempty"`
	CurrentStage       string    `json:"current_stage,omitempty"`
	CompletedStages    []string  `json:"completed_stages,omitempty"`
	TotalStages        int       `json:"total_stages,omitempty"`
	Progress           float64   `json:"progress,omitempty"`
	WorkspaceDir       string    `json:"workspace_dir,omitempty"`
	WorkspaceSizeMB    int64     `json:"workspace_size_mb,omitempty"`
	ExpiresAt          time.Time `json:"expires_at,omitempty"`
	LastActivity       time.Time `json:"last_activity,omitempty"`
	ResourcesAllocated []string  `json:"resources_allocated,omitempty"`
	WorkspaceState     string    `json:"workspace_state,omitempty"`

	// Custom session metadata for extensibility
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// Helper functions for creating error contexts

// NewErrorInputContext creates a new ErrorInputContext
func NewErrorInputContext(toolName string, args map[string]interface{}) *ErrorInputContext {
	return &ErrorInputContext{
		ToolName:  toolName,
		Arguments: args,
	}
}

// NewErrorMetadata creates a new ErrorMetadata with basic information
func NewErrorMetadata(sessionID, toolName, operation string) *ErrorMetadata {
	return &ErrorMetadata{
		SessionID: sessionID,
		ToolName:  toolName,
		Operation: operation,
		Custom:    make(map[string]interface{}),
	}
}

// WithBuildContext adds build context to ErrorMetadata
func (em *ErrorMetadata) WithBuildContext(ctx *BuildMetadata) *ErrorMetadata {
	em.BuildContext = ctx
	return em
}

// WithDeploymentContext adds deployment context to ErrorMetadata
func (em *ErrorMetadata) WithDeploymentContext(ctx *DeploymentMetadata) *ErrorMetadata {
	em.DeploymentContext = ctx
	return em
}

// WithRepositoryContext adds repository context to ErrorMetadata
func (em *ErrorMetadata) WithRepositoryContext(ctx *RepositoryMetadata) *ErrorMetadata {
	em.RepositoryContext = ctx
	return em
}

// WithSecurityContext adds security context to ErrorMetadata
func (em *ErrorMetadata) WithSecurityContext(ctx *SecurityMetadata) *ErrorMetadata {
	em.SecurityContext = ctx
	return em
}

// WithTimingContext adds timing context to ErrorMetadata
func (em *ErrorMetadata) WithTimingContext(ctx *TimingMetadata) *ErrorMetadata {
	em.Timing = ctx
	return em
}

// WithSessionContext adds session context to ErrorMetadata
func (em *ErrorMetadata) WithSessionContext(ctx *SessionStateMetadata) *ErrorMetadata {
	em.SessionState = ctx
	return em
}

// AddCustom adds a custom field to the metadata
func (em *ErrorMetadata) AddCustom(key string, value interface{}) *ErrorMetadata {
	if em.Custom == nil {
		em.Custom = make(map[string]interface{})
	}
	em.Custom[key] = value
	return em
}

// NewSessionMetadata creates a new SessionMetadata with basic information
func NewSessionMetadata(id, currentStage string, completedStages []string) *SessionStateMetadata {
	return &SessionStateMetadata{
		SessionID:       id,
		CurrentStage:    currentStage,
		CompletedStages: completedStages,
		Custom:          make(map[string]interface{}),
	}
}

// AddCustomToSession adds a custom field to session metadata
func (sm *SessionStateMetadata) AddCustomToSession(key string, value interface{}) *SessionStateMetadata {
	if sm.Custom == nil {
		sm.Custom = make(map[string]interface{})
	}
	sm.Custom[key] = value
	return sm
}
