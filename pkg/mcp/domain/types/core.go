// Package types - Core domain type definitions
// This file contains fundamental types for the Container Kit MCP system
package types

import (
	"sync"
	"time"
)

// Workflow Stage Constants
const (
	StageInit     = "init"
	StageAnalysis = "analysis"
	StageBuild    = "build"
	StageDeploy   = "deploy"
	StageComplete = "complete"
)

// Common string constants
const (
	UnknownString     = "unknown"
	DefaultRegistry   = "docker.io"
	AtomicToolVersion = "1.0.0"
)

// Validation constants
const (
	ValidationModeInline = "inline"
)

// Known registries for validation
var KnownRegistries = []string{
	"docker.io",
	"gcr.io",
	"quay.io",
	"ghcr.io",
}

// ============================================================================
// Cache Types - Consolidated from pkg/mcp/internal/pipeline/cache_types.go
// ============================================================================

// CacheManager provides simple in-memory caching functionality
type CacheManager struct {
	SessionManager interface{} // Will be properly typed after session consolidation
	Logger         interface{} // Will be properly typed after logging consolidation

	// Local cache storage
	Cache      map[string]*CacheEntry
	CacheMutex sync.RWMutex

	// Cache configuration
	Config CacheConfig

	// Performance monitoring
	Metrics      *CacheMetrics
	MetricsMutex sync.RWMutex

	// Background cleanup
	ShutdownCh chan struct{}
}

// CacheEntry represents a cached value with metadata
type CacheEntry struct {
	Key        string        `json:"key"`
	Value      interface{}   `json:"value"`
	CreatedAt  time.Time     `json:"created_at"`
	ExpiresAt  time.Time     `json:"expires_at"`
	AccessedAt time.Time     `json:"accessed_at"`
	TTL        time.Duration `json:"ttl"`
}

// CacheConfig defines cache configuration
type CacheConfig struct {
	MaxSize         int           `json:"max_size"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	Enabled         bool          `json:"enabled"`
}

// CacheMetrics provides cache performance metrics
type CacheMetrics struct {
	Hits      int64     `json:"hits"`
	Misses    int64     `json:"misses"`
	Evictions int64     `json:"evictions"`
	Size      int       `json:"size"`
	LastReset time.Time `json:"last_reset"`
}

// HitRate calculates the cache hit rate
func (m *CacheMetrics) HitRate() float64 {
	total := m.Hits + m.Misses
	if total == 0 {
		return 0
	}
	return float64(m.Hits) / float64(total)
}

// ============================================================================
// Session Types - Consolidated from various session type files
// ============================================================================

// SessionData represents core session information
type SessionData struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastAccessed time.Time              `json:"last_accessed"`
	Status       SessionStatus          `json:"status"`
	Labels       map[string]string      `json:"labels,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// SessionStatus represents the status of a session
type SessionStatus string

const (
	SessionStatusActive     SessionStatus = "active"
	SessionStatusInactive   SessionStatus = "inactive"
	SessionStatusExpired    SessionStatus = "expired"
	SessionStatusTerminated SessionStatus = "terminated"
)

// SessionFilter provides filtering criteria for sessions
type SessionFilter struct {
	Status        []SessionStatus   `json:"status,omitempty"`
	UserID        string            `json:"user_id,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	CreatedAfter  *time.Time        `json:"created_after,omitempty"`
	CreatedBefore *time.Time        `json:"created_before,omitempty"`
	Limit         int               `json:"limit,omitempty"`
}

// ============================================================================
// Repository Analysis Types - Consolidated from analyze_types.go
// ============================================================================

// CloneOptions represents options for cloning a repository
type CloneOptions struct {
	RepoURL   string `json:"repo_url"`
	Branch    string `json:"branch"`
	Shallow   bool   `json:"shallow"`
	TargetDir string `json:"target_dir"`
	SessionID string `json:"session_id"`
}

// CloneResult wraps the git clone result with additional metadata
type CloneResult struct {
	Success    bool          `json:"success"`
	RepoPath   string        `json:"repo_path"`
	Branch     string        `json:"branch"`
	CommitHash string        `json:"commit_hash"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
}

// AnalysisOptions represents options for analyzing a repository
type AnalysisOptions struct {
	RepoPath     string `json:"repo_path"`
	Context      string `json:"context"`
	LanguageHint string `json:"language_hint"`
	SessionID    string `json:"session_id"`
	SkipTests    bool   `json:"skip_tests,omitempty"`
	MaxDepth     int    `json:"max_depth,omitempty"`
}

// AnalysisResult represents the result of repository analysis
type AnalysisResult struct {
	Success       bool                   `json:"success"`
	Language      string                 `json:"language"`
	Framework     string                 `json:"framework,omitempty"`
	Dependencies  []string               `json:"dependencies,omitempty"`
	BuildCommands []string               `json:"build_commands,omitempty"`
	TestCommands  []string               `json:"test_commands,omitempty"`
	Dockerfile    string                 `json:"dockerfile,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Error         string                 `json:"error,omitempty"`
}

// ============================================================================
// Pipeline Types - Core pipeline structures
// ============================================================================

// PipelineStage represents a single stage in a pipeline
type PipelineStage struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Status      StageStatus            `json:"status"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// StageStatus represents the status of a pipeline stage
type StageStatus string

const (
	StagePending   StageStatus = "pending"
	StageRunning   StageStatus = "running"
	StageCompleted StageStatus = "completed"
	StageFailed    StageStatus = "failed"
	StageSkipped   StageStatus = "skipped"
)

// PipelineResult represents the result of a pipeline execution
type PipelineResult struct {
	ID          string                 `json:"id"`
	Status      StageStatus            `json:"status"`
	Stages      []PipelineStage        `json:"stages"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ============================================================================
// Extended Session Types - Consolidated from core/session_types.go
// ============================================================================

// SessionState represents comprehensive session state information
type SessionState struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at"`

	WorkspaceDir string `json:"workspace_dir"`

	// Repository state
	RepositoryAnalyzed bool            `json:"repository_analyzed"`
	RepositoryInfo     *RepositoryInfo `json:"repository_info,omitempty"`
	RepoURL            string          `json:"repo_url"`

	// Build state
	DockerfileGenerated bool   `json:"dockerfile_generated"`
	DockerfilePath      string `json:"dockerfile_path"`
	ImageBuilt          bool   `json:"image_built"`
	ImageRef            string `json:"image_ref"`
	ImagePushed         bool   `json:"image_pushed"`

	// Deployment state
	ManifestsGenerated  bool     `json:"manifests_generated"`
	ManifestPaths       []string `json:"manifest_paths"`
	DeploymentValidated bool     `json:"deployment_validated"`

	// Workflow state
	CurrentStage string   `json:"current_stage"`
	Status       string   `json:"status"`
	Stage        string   `json:"stage"`
	Errors       []string `json:"github.com/Azure/container-kit/pkg/mcp/domain/errors"`

	// Security state
	SecurityScan *SecurityScanResult `json:"security_scan,omitempty"`

	// Extensible metadata (deprecated - use TypedMetadata for type safety)
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Type-safe metadata structure
	TypedMetadata *SessionMetadata `json:"typed_metadata,omitempty"`
}

// SessionMetadata - CONSOLIDATED: Use SessionMetadata from transport.go (avoiding duplicate)

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

// SecurityScanResult - CONSOLIDATED: Use SecurityScanResult from tool_types.go (avoiding duplicate)

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

// ============================================================================
// Repository Analysis Types - Consolidated from core/analysis_types.go
// ============================================================================

// RepositoryInfo represents repository analysis results
type RepositoryInfo struct {
	Path          string            `json:"path"`
	Type          string            `json:"type"`
	Language      string            `json:"language"`
	Framework     string            `json:"framework"`
	Languages     []string          `json:"languages"`
	Dependencies  map[string]string `json:"dependencies"`
	BuildTools    []string          `json:"build_tools"`
	EntryPoint    string            `json:"entry_point"`
	Port          int               `json:"port"`
	HasDockerfile bool              `json:"has_dockerfile"`
	Metadata      map[string]string `json:"metadata"`
}

// DockerfileInfo represents Dockerfile analysis results
type DockerfileInfo struct {
	Path           string            `json:"path"`
	BaseImage      string            `json:"base_image"`
	ExposedPorts   []int             `json:"exposed_ports"`
	WorkingDir     string            `json:"working_dir"`
	EntryPoint     []string          `json:"entry_point"`
	Cmd            []string          `json:"cmd"`
	HealthCheck    *HealthCheckInfo  `json:"health_check,omitempty"`
	Labels         map[string]string `json:"labels"`
	BuildArgs      map[string]string `json:"build_args"`
	MultiStage     bool              `json:"multi_stage"`
	SecurityIssues []string          `json:"security_issues"`
}

// HealthCheckInfo represents Docker health check configuration
type HealthCheckInfo struct {
	Test     []string      `json:"test"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// BuildRecommendations represents build optimization recommendations
type BuildRecommendations struct {
	OptimizationTips []string          `json:"optimization_tips"`
	SecurityTips     []string          `json:"security_tips"`
	PerformanceTips  []string          `json:"performance_tips"`
	BestPractices    []string          `json:"best_practices"`
	Suggestions      map[string]string `json:"suggestions"`
}

// TokenUsage - CONSOLIDATED: Use TokenUsage from clients.go (avoiding duplicate)
// FixingResult - CONSOLIDATED: This type will be migrated to core types in Phase 3
// BaseAIContextResult - CONSOLIDATED: Use BaseAIContextResult from ai_context_base.go (avoiding duplicate)

// FixingResult represents the result of a fixing operation
type FixingResult struct {
	Success         bool          `json:"success"`
	AttemptsUsed    int           `json:"attempts_used"`
	OriginalError   error         `json:"original_error,omitempty"`
	FinalError      error         `json:"final_error,omitempty"`
	FixApplied      bool          `json:"fix_applied"`
	FixDescription  string        `json:"fix_description,omitempty"`
	AllAttempts     []interface{} `json:"all_attempts"`
	TotalAttempts   int           `json:"total_attempts"`
	Duration        time.Duration `json:"duration"`
	LastAttemptTime time.Time     `json:"last_attempt_time"`
}

// ============================================================================
// Progress and Workflow Types - Consolidated from core/analysis_types.go
// ============================================================================

// ProgressToken represents a unique identifier for a progress stage
type ProgressToken string

// ProgressStage represents the state of a progress stage
type ProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`   // "pending", "running", "completed", "failed"
	Progress    int     `json:"progress"` // 0-100
	Message     string  `json:"message"`
	Weight      float64 `json:"weight"` // Relative weight (0.0-1.0) of this stage in overall progress
}
