package domain

import (
	"time"

	"github.com/Azure/container-kit/pkg/core/git"
)

// SessionState represents session state information
type SessionState struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at"`

	WorkspaceDir string `json:"workspace_dir"`

	RepositoryAnalyzed bool                `json:"repository_analyzed"`
	RepositoryInfo     *git.RepositoryInfo `json:"repository_info,omitempty"`
	RepoURL            string              `json:"repo_url"`

	DockerfileGenerated bool   `json:"dockerfile_generated"`
	DockerfilePath      string `json:"dockerfile_path"`
	ImageBuilt          bool   `json:"image_built"`
	ImageRef            string `json:"image_ref"`
	ImagePushed         bool   `json:"image_pushed"`

	ManifestsGenerated  bool     `json:"manifests_generated"`
	ManifestPaths       []string `json:"manifest_paths"`
	DeploymentValidated bool     `json:"deployment_validated"`

	CurrentStage string   `json:"current_stage"`
	Status       string   `json:"status"`
	Stage        string   `json:"stage"`
	Errors       []string `json:"errors"`

	SecurityScan *SecurityScanResult `json:"security_scan,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`

	TypedMetadata *SessionMetadata `json:"typed_metadata,omitempty"`
}

// SessionMetadata represents type-safe session metadata
type SessionMetadata struct {
	RepositoryAnalysis *RepositoryAnalysisMetadata `json:"repository_analysis,omitempty"`

	BuildHistory []BuildMetadata `json:"build_history,omitempty"`

	DeploymentHistory []DeploymentMetadata `json:"deployment_history,omitempty"`

	SecurityScans []SecurityScanMetadata `json:"security_scans,omitempty"`

	ConversationHistory []ConversationTurn `json:"conversation_history,omitempty"`

	PerformanceMetrics map[string]string `json:"performance_metrics,omitempty"`

	UserProperties map[string]string `json:"user_properties,omitempty"`
}

// RepositoryAnalysisMetadata represents repository analysis metadata
type RepositoryAnalysisMetadata struct {
	URL          string            `json:"url"`
	Branch       string            `json:"branch"`
	CommitHash   string            `json:"commit_hash,omitempty"`
	AnalyzedAt   time.Time         `json:"analyzed_at"`
	Language     string            `json:"language"`
	Framework    string            `json:"framework"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
}

// BuildMetadata represents build operation metadata
type BuildMetadata struct {
	ImageRef   string            `json:"image_ref"`
	BuildTime  time.Time         `json:"build_time"`
	Duration   time.Duration     `json:"duration"`
	Success    bool              `json:"success"`
	Platform   string            `json:"platform,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// DeploymentMetadata represents deployment operation metadata
type DeploymentMetadata struct {
	ImageRef      string            `json:"image_ref"`
	Namespace     string            `json:"namespace"`
	AppName       string            `json:"app_name"`
	DeployedAt    time.Time         `json:"deployed_at"`
	Success       bool              `json:"success"`
	ManifestPaths []string          `json:"manifest_paths,omitempty"`
	Resources     []string          `json:"resources,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
}

// SecurityScanMetadata represents security scan metadata
type SecurityScanMetadata struct {
	ImageRef         string            `json:"image_ref"`
	ScanType         string            `json:"scan_type"`
	Scanner          string            `json:"scanner"`
	ScannedAt        time.Time         `json:"scanned_at"`
	TotalFindings    int               `json:"total_findings"`
	CriticalFindings int               `json:"critical_findings"`
	HighFindings     int               `json:"high_findings"`
	ReportPath       string            `json:"report_path,omitempty"`
	Properties       map[string]string `json:"properties,omitempty"`
}

// ConversationTurn represents a conversation turn metadata
type ConversationTurn struct {
	TurnID      string             `json:"turn_id"`
	Timestamp   time.Time          `json:"timestamp"`
	UserMessage string             `json:"user_message"`
	ToolCalls   []ToolCallMetadata `json:"tool_calls,omitempty"`
	Properties  map[string]string  `json:"properties,omitempty"`
}

// ToolCallMetadata represents tool call metadata
type ToolCallMetadata struct {
	ToolName   string            `json:"tool_name"`
	Success    bool              `json:"success"`
	Duration   time.Duration     `json:"duration"`
	Properties map[string]string `json:"properties,omitempty"`
}

// SecurityScanResult represents security scan results
type SecurityScanResult struct {
	Success            bool               `json:"success"`
	HasVulnerabilities bool               `json:"has_vulnerabilities"`
	ScannedAt          time.Time          `json:"scanned_at"`
	ImageRef           string             `json:"image_ref"`
	Scanner            string             `json:"scanner"`
	Vulnerabilities    VulnerabilityCount `json:"vulnerabilities"`
	CriticalCount      int                `json:"critical_count"`
	HighCount          int                `json:"high_count"`
	MediumCount        int                `json:"medium_count"`
	LowCount           int                `json:"low_count"`
	VulnerabilityList  []string           `json:"vulnerability_list"`
	ScanTime           time.Time          `json:"scan_time"`
}

// VulnerabilityCount represents counts of vulnerabilities by severity
type VulnerabilityCount struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// SecurityFinding represents a security vulnerability finding
type SecurityFinding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixedIn     string `json:"fixed_in,omitempty"`
}

// SessionInfo represents basic session information
type SessionInfo struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionFilter represents filters for session listing
type SessionFilter struct {
	UserID        string     `json:"user_id,omitempty"`
	Status        string     `json:"status,omitempty"`
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
}

// MCPRequest represents an MCP protocol request
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP protocol response
type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SessionManagerStats represents session management statistics
type SessionManagerStats struct {
	ActiveSessions    int       `json:"active_sessions"`
	TotalSessions     int       `json:"total_sessions"`
	FailedSessions    int       `json:"failed_sessions"`
	ExpiredSessions   int       `json:"expired_sessions"`
	SessionsWithJobs  int       `json:"sessions_with_jobs"`
	AverageSessionAge float64   `json:"average_session_age_minutes"`
	SessionErrors     int       `json:"session_errors_last_hour"`
	TotalDiskUsage    int64     `json:"total_disk_usage_bytes"`
	ServerStartTime   time.Time `json:"server_start_time"`
}

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalDiskUsage int64 `json:"total_disk_usage"`
	SessionCount   int   `json:"session_count"`
	TotalFiles     int   `json:"total_files"`
	DiskLimit      int64 `json:"disk_limit"`
}
