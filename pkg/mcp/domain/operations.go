package domain

import "time"

// Pipeline operation argument types

// TypedGenerateManifestsArgs represents generate manifests arguments
type TypedGenerateManifestsArgs struct {
	SessionID     string `json:"session_id"`
	ImageRef      string `json:"image_ref"`
	AppName       string `json:"app_name"`
	Port          int    `json:"port,omitempty"`
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// TypedBuildImageArgs represents build image arguments
type TypedBuildImageArgs struct {
	SessionID      string            `json:"session_id"`
	ImageRef       string            `json:"image_ref"`
	DockerfilePath string            `json:"dockerfile_path"`
	Context        string            `json:"context,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
}

// TypedPushImageArgs represents push image arguments
type TypedPushImageArgs struct {
	SessionID string `json:"session_id"`
	ImageRef  string `json:"image_ref"`
	Registry  string `json:"registry,omitempty"`
}

// TypedPullImageArgs represents pull image arguments
type TypedPullImageArgs struct {
	SessionID string `json:"session_id"`
	ImageRef  string `json:"image_ref"`
	Registry  string `json:"registry,omitempty"`
}

// TypedTagImageArgs represents tag image arguments
type TypedTagImageArgs struct {
	SessionID string `json:"session_id"`
	SourceRef string `json:"source_ref"`
	TargetRef string `json:"target_ref"`
}

// TypedDeployKubernetesArgs represents deploy kubernetes arguments
type TypedDeployKubernetesArgs struct {
	SessionID string   `json:"session_id"`
	Manifests []string `json:"manifests"`
	Namespace string   `json:"namespace,omitempty"`
}

// TypedCheckHealthArgs represents check health arguments
type TypedCheckHealthArgs struct {
	SessionID     string        `json:"session_id"`
	Namespace     string        `json:"namespace,omitempty"`
	LabelSelector string        `json:"label_selector,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
}

// TypedAnalyzeRepositoryArgs represents analyze repository arguments
type TypedAnalyzeRepositoryArgs struct {
	SessionID string `json:"session_id"`
	RepoPath  string `json:"repo_path"`
	Branch    string `json:"branch,omitempty"`
}

// TypedValidateDockerfileArgs represents validate dockerfile arguments
type TypedValidateDockerfileArgs struct {
	SessionID      string `json:"session_id"`
	DockerfilePath string `json:"dockerfile_path"`
	Content        string `json:"content,omitempty"`
}

// TypedScanSecurityArgs represents scan security arguments
type TypedScanSecurityArgs struct {
	SessionID string `json:"session_id"`
	ImageRef  string `json:"image_ref"`
	ScanType  string `json:"scan_type,omitempty"`
}

// Build and analyzer operation argument types

// FixRequest represents a request to fix a build error
type FixRequest struct {
	SessionID     string `json:"session_id"`
	ToolName      string `json:"tool_name"`
	OperationType string `json:"operation_type"`
	Error         error  `json:"error"`
	MaxAttempts   int    `json:"max_attempts"`
	BaseDir       string `json:"base_dir"`
}

// BuildOperationConfig represents configuration for creating build operations
type BuildOperationConfig struct {
	Tool           interface{} `json:"tool"`    // *AtomicBuildImageTool
	Args           interface{} `json:"args"`    // AtomicBuildImageArgs
	Session        interface{} `json:"session"` // *core.SessionState
	WorkspaceDir   string      `json:"workspace_dir"`
	BuildContext   string      `json:"build_context"`
	DockerfilePath string      `json:"dockerfile_path"`
	Logger         interface{} `json:"logger"` // logging.Standards
}

// AIContextEnhanceConfig represents configuration for AI context enhancement
type AIContextEnhanceConfig struct {
	SessionID     string      `json:"session_id"`
	ToolName      string      `json:"tool_name"`
	OperationType string      `json:"operation_type"`
	ToolResult    interface{} `json:"tool_result"`
	ToolError     error       `json:"tool_error"`
}

// BuildContextGenerateConfig represents configuration for build context generation
type BuildContextGenerateConfig struct {
	SessionID      string            `json:"session_id"`
	WorkspaceDir   string            `json:"workspace_dir"`
	ImageName      string            `json:"image_name"`
	ImageTag       string            `json:"image_tag"`
	DockerfilePath string            `json:"dockerfile_path"`
	BuildContext   string            `json:"build_context"`
	Platform       string            `json:"platform"`
	BuildArgs      map[string]string `json:"build_args"`
}

// StateSyncConfig represents configuration for state synchronization
type StateSyncConfig struct {
	Manager    interface{}   `json:"manager"`     // *UnifiedStateManager
	SourceType string        `json:"source_type"` // StateType
	TargetType string        `json:"target_type"` // StateType
	Mapping    interface{}   `json:"mapping"`     // StateMapping
	Interval   time.Duration `json:"interval"`
}

// SecurityEventConfig represents configuration for security event recording
type SecurityEventConfig struct {
	SessionID   string                 `json:"session_id"`
	Operation   string                 `json:"operation"`
	EventType   string                 `json:"event_type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Context     map[string]interface{} `json:"context"`
}

// AsyncBuildConfig represents configuration for async build execution
type AsyncBuildConfig struct {
	Args          interface{} `json:"args"`           // BuildImageArgs
	PipelineState interface{} `json:"pipeline_state"` // *pipeline.PipelineState
	DockerStage   interface{} `json:"docker_stage"`   // *dockerstage.DockerStage
	RunnerOptions interface{} `json:"runner_options"` // pipeline.RunnerOptions
	JobID         string      `json:"job_id"`
}

// ErrorRouteConfig represents configuration for error routing
type ErrorRouteConfig struct {
	SessionID    string `json:"session_id"`
	SourceTool   string `json:"source_tool"`
	ErrorType    string `json:"error_type"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}
