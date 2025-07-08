package core

import "time"

// TypedMap replacements for map[string]interface{} usage patterns

// ConfigMap represents typed configuration data instead of map[string]interface{}
type ConfigMap struct {
	BuildConfig    *BuildConfiguration    `json:"build_config,omitempty"`
	DeployConfig   *DeployConfiguration   `json:"deploy_config,omitempty"`
	SecurityConfig *SecurityConfiguration `json:"security_config,omitempty"`
	SessionConfig  *SessionConfiguration  `json:"session_config,omitempty"`
	RawData        map[string]interface{} `json:"raw_data,omitempty"` // Fallback for unknown data
}

// MetadataMap represents typed metadata instead of map[string]interface{}
type MetadataMap struct {
	ToolMetadata    *ToolMetadataInfo      `json:"tool_metadata,omitempty"`
	ResultMetadata  *ResultMetadataInfo    `json:"result_metadata,omitempty"`
	ContextMetadata *ContextMetadataInfo   `json:"context_metadata,omitempty"`
	SessionMetadata *SessionMetadataInfo   `json:"session_metadata,omitempty"`
	RawData         map[string]interface{} `json:"raw_data,omitempty"` // Fallback for unknown data
}

// ContextMap represents typed context data instead of map[string]interface{}
type ContextMap struct {
	ExecutionContext         *ExecutionContextInfo  `json:"execution_context,omitempty"`
	ConsolidatedErrorContext *ErrorContextInfo      `json:"error_context,omitempty"`
	SessionContext           *SessionContextInfo    `json:"session_context,omitempty"`
	ToolContext              *ToolContextInfo       `json:"tool_context,omitempty"`
	RawData                  map[string]interface{} `json:"raw_data,omitempty"` // Fallback for unknown data
}

// Configuration structures
type BuildConfiguration struct {
	ImageName      string            `json:"image_name,omitempty"`
	ImageTag       string            `json:"image_tag,omitempty"`
	DockerfilePath string            `json:"dockerfile_path,omitempty"`
	BuildContext   string            `json:"build_context,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	NoCache        bool              `json:"no_cache,omitempty"`
}

type DeployConfiguration struct {
	Namespace      string            `json:"namespace,omitempty"`
	AppName        string            `json:"app_name,omitempty"`
	Replicas       int               `json:"replicas,omitempty"`
	Port           int               `json:"port,omitempty"`
	ServiceType    string            `json:"service_type,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	ResourceLimits *ResourceLimits   `json:"resource_limits,omitempty"`
	IncludeIngress bool              `json:"include_ingress,omitempty"`
	WaitForReady   bool              `json:"wait_for_ready,omitempty"`
}

type SecurityConfiguration struct {
	ScanTypes      []string `json:"scan_types,omitempty"`
	VulnTypes      []string `json:"vuln_types,omitempty"`
	SecurityLevel  string   `json:"security_level,omitempty"`
	IgnoreRules    []string `json:"ignore_rules,omitempty"`
	IncludeSecrets bool     `json:"include_secrets,omitempty"`
	OutputFormat   string   `json:"output_format,omitempty"`
	Severity       string   `json:"severity,omitempty"`
}

type SessionConfiguration struct {
	WorkspaceDir  string        `json:"workspace_dir,omitempty"`
	TTL           time.Duration `json:"ttl,omitempty"`
	CleanupOnExit bool          `json:"cleanup_on_exit,omitempty"`
	CacheEnabled  bool          `json:"cache_enabled,omitempty"`
	LogLevel      string        `json:"log_level,omitempty"`
}

// Metadata information structures
type ToolMetadataInfo struct {
	ToolName      string            `json:"tool_name,omitempty"`
	ToolVersion   string            `json:"tool_version,omitempty"`
	Category      string            `json:"category,omitempty"`
	ExecutionTime time.Duration     `json:"execution_time,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
}

type ResultMetadataInfo struct {
	ResultType     string        `json:"result_type,omitempty"`
	GeneratedAt    time.Time     `json:"generated_at,omitempty"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`
	DataSize       int64         `json:"data_size,omitempty"`
	Checksum       string        `json:"checksum,omitempty"`
}

type ContextMetadataInfo struct {
	RequestID     string    `json:"request_id,omitempty"`
	UserID        string    `json:"user_id,omitempty"`
	Timestamp     time.Time `json:"timestamp,omitempty"`
	Environment   string    `json:"environment,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
}

type SessionMetadataInfo struct {
	SessionID      string            `json:"session_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty"`
	LastAccessedAt time.Time         `json:"last_accessed_at,omitempty"`
	SessionState   string            `json:"session_state,omitempty"`
	WorkspaceInfo  *WorkspaceInfo    `json:"workspace_info,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// Context information structures
type ExecutionContextInfo struct {
	ExecutionID   string         `json:"execution_id,omitempty"`
	StartTime     time.Time      `json:"start_time,omitempty"`
	EndTime       time.Time      `json:"end_time,omitempty"`
	Duration      time.Duration  `json:"duration,omitempty"`
	Stage         string         `json:"stage,omitempty"`
	Progress      float64        `json:"progress,omitempty"`
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`
}

type ErrorContextInfo struct {
	ErrorCode     string            `json:"error_code,omitempty"`
	ErrorCategory string            `json:"error_category,omitempty"`
	FailureStage  string            `json:"failure_stage,omitempty"`
	Recoverable   bool              `json:"recoverable,omitempty"`
	Suggestions   []string          `json:"suggestions,omitempty"`
	DebugInfo     map[string]string `json:"debug_info,omitempty"`
}

type SessionContextInfo struct {
	SessionID     string            `json:"session_id,omitempty"`
	WorkspaceDir  string            `json:"workspace_dir,omitempty"`
	ActiveTools   []string          `json:"active_tools,omitempty"`
	SessionState  map[string]string `json:"session_state,omitempty"`
	LastOperation string            `json:"last_operation,omitempty"`
}

type ToolContextInfo struct {
	ToolName           string                 `json:"tool_name,omitempty"`
	ToolVersion        string                 `json:"tool_version,omitempty"`
	OperationType      string                 `json:"operation_type,omitempty"`
	InputSize          int64                  `json:"input_size,omitempty"`
	OutputSize         int64                  `json:"output_size,omitempty"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics,omitempty"`
}

// Supporting structures
type ResourceLimits struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

type ResourceUsage struct {
	CPUUsage    float64 `json:"cpu_usage,omitempty"`
	MemoryUsage int64   `json:"memory_usage,omitempty"`
	DiskUsage   int64   `json:"disk_usage,omitempty"`
	NetworkIO   int64   `json:"network_io,omitempty"`
}

type WorkspaceInfo struct {
	Path         string    `json:"path,omitempty"`
	Size         int64     `json:"size,omitempty"`
	FilesCount   int       `json:"files_count,omitempty"`
	LastModified time.Time `json:"last_modified,omitempty"`
	GitInfo      *GitInfo  `json:"git_info,omitempty"`
}

type GitInfo struct {
	RepoURL       string `json:"repo_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	CommitHash    string `json:"commit_hash,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	IsDirty       bool   `json:"is_dirty,omitempty"`
}

// Utility methods for converting to/from map[string]interface{}

// ToMap converts ConfigMap to map[string]interface{} for backward compatibility
func (cm *ConfigMap) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if cm.BuildConfig != nil {
		result["build_config"] = cm.BuildConfig
	}
	if cm.DeployConfig != nil {
		result["deploy_config"] = cm.DeployConfig
	}
	if cm.SecurityConfig != nil {
		result["security_config"] = cm.SecurityConfig
	}
	if cm.SessionConfig != nil {
		result["session_config"] = cm.SessionConfig
	}

	// Merge raw data
	for k, v := range cm.RawData {
		result[k] = v
	}

	return result
}

// FromMap creates ConfigMap from map[string]interface{} for migration
func ConfigMapFromMap(data map[string]interface{}) *ConfigMap {
	cm := &ConfigMap{
		RawData: make(map[string]interface{}),
	}

	for k, v := range data {
		switch k {
		case "build_config":
			if bc, ok := v.(*BuildConfiguration); ok {
				cm.BuildConfig = bc
			}
		case "deploy_config":
			if dc, ok := v.(*DeployConfiguration); ok {
				cm.DeployConfig = dc
			}
		case "security_config":
			if sc, ok := v.(*SecurityConfiguration); ok {
				cm.SecurityConfig = sc
			}
		case "session_config":
			if sesc, ok := v.(*SessionConfiguration); ok {
				cm.SessionConfig = sesc
			}
		default:
			cm.RawData[k] = v
		}
	}

	return cm
}

// ToMap converts MetadataMap to map[string]interface{} for backward compatibility
func (mm *MetadataMap) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if mm.ToolMetadata != nil {
		result["tool_metadata"] = mm.ToolMetadata
	}
	if mm.ResultMetadata != nil {
		result["result_metadata"] = mm.ResultMetadata
	}
	if mm.ContextMetadata != nil {
		result["context_metadata"] = mm.ContextMetadata
	}
	if mm.SessionMetadata != nil {
		result["session_metadata"] = mm.SessionMetadata
	}

	// Merge raw data
	for k, v := range mm.RawData {
		result[k] = v
	}

	return result
}

// FromMap creates MetadataMap from map[string]interface{} for migration
func MetadataMapFromMap(data map[string]interface{}) *MetadataMap {
	mm := &MetadataMap{
		RawData: make(map[string]interface{}),
	}

	for k, v := range data {
		switch k {
		case "tool_metadata":
			if tm, ok := v.(*ToolMetadataInfo); ok {
				mm.ToolMetadata = tm
			}
		case "result_metadata":
			if rm, ok := v.(*ResultMetadataInfo); ok {
				mm.ResultMetadata = rm
			}
		case "context_metadata":
			if cm, ok := v.(*ContextMetadataInfo); ok {
				mm.ContextMetadata = cm
			}
		case "session_metadata":
			if sm, ok := v.(*SessionMetadataInfo); ok {
				mm.SessionMetadata = sm
			}
		default:
			mm.RawData[k] = v
		}
	}

	return mm
}

// ToMap converts ContextMap to map[string]interface{} for backward compatibility
func (cm *ContextMap) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if cm.ExecutionContext != nil {
		result["execution_context"] = cm.ExecutionContext
	}
	if cm.ConsolidatedErrorContext != nil {
		result["error_context"] = cm.ConsolidatedErrorContext
	}
	if cm.SessionContext != nil {
		result["session_context"] = cm.SessionContext
	}
	if cm.ToolContext != nil {
		result["tool_context"] = cm.ToolContext
	}

	// Merge raw data
	for k, v := range cm.RawData {
		result[k] = v
	}

	return result
}

// FromMap creates ContextMap from map[string]interface{} for migration
func ContextMapFromMap(data map[string]interface{}) *ContextMap {
	cm := &ContextMap{
		RawData: make(map[string]interface{}),
	}

	for k, v := range data {
		switch k {
		case "execution_context":
			if ec, ok := v.(*ExecutionContextInfo); ok {
				cm.ExecutionContext = ec
			}
		case "error_context":
			if ec, ok := v.(*ErrorContextInfo); ok {
				cm.ConsolidatedErrorContext = ec
			}
		case "session_context":
			if sc, ok := v.(*SessionContextInfo); ok {
				cm.SessionContext = sc
			}
		case "tool_context":
			if tc, ok := v.(*ToolContextInfo); ok {
				cm.ToolContext = tc
			}
		default:
			cm.RawData[k] = v
		}
	}

	return cm
}
