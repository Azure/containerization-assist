// Package core - Core domain type definitions
// This file contains fundamental types for the Container Kit MCP system
package core

import (
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/config"
)

// Workflow Stage Constants
const (
	StageInit     = "init"
	StageAnalysis = "analysis"
	StageBuild    = "build"
	StageDeploy   = "deploy"
	StageComplete = "complete"
)

// ProgressReporter provides progress reporting capabilities for operations
type ProgressReporter interface {
	ReportProgress(current, total int, message string)
	ReportError(err error)
	ReportComplete(message string)
}

// TypedPipelineOperations is defined in interfaces.go

// Common string constants
const (
	UnknownString     = "unknown"
	DefaultRegistry   = "docker.io"
	AtomicToolVersion = "1.0.0"
)

// Note: ScanResult type is defined in base_types.go to avoid duplication

// Validation constants
const (
	ValidationModeInline = "inline"
)

// Type aliases for external access
type ServerConfig = config.ServerConfig

// ConsolidatedConversationConfig represents conversation mode configuration
type ConsolidatedConversationConfig struct {
	EnableTelemetry   bool              `json:"enable_telemetry"`
	TelemetryPort     int               `json:"telemetry_port"`
	PreferencesDBPath string            `json:"preferences_db_path"`
	EnableOTEL        bool              `json:"enable_otel"`
	OTELEndpoint      string            `json:"otel_endpoint"`
	OTELHeaders       map[string]string `json:"otel_headers"`
	ServiceName       string            `json:"service_name"`
	ServiceVersion    string            `json:"service_version"`
	Environment       string            `json:"environment"`
	TraceSampleRate   float64           `json:"trace_sample_rate"`
}

// KnownRegistries is deprecated - use RegistryService instead
// var KnownRegistries = []string{ ... } // REMOVED: Global state eliminated

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

// NOTE: SessionFilter moved to session_types.go to avoid redeclaration

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

// NOTE: SessionState, RepositoryAnalysisMetadata, BuildMetadata, and DeploymentMetadata
// moved to session_types.go to avoid redeclaration

// NOTE: SecurityScanMetadata and ConversationTurn also moved to session_types.go

// NOTE: ToolCallMetadata moved to session_types.go to avoid redeclaration

// SecurityScanResult - CONSOLIDATED: Use SecurityScanResult from tool_types.go (avoiding duplicate)

// NOTE: VulnerabilityCount moved to session_types.go to avoid redeclaration

// NOTE: SecurityFinding moved to session_types.go to avoid redeclaration

// NOTE: SessionManagerStats moved to session_types.go to avoid redeclaration

// NOTE: WorkspaceStats moved to session_types.go to avoid redeclaration

// ============================================================================
// Repository Analysis Types - Consolidated from core/analysis_types.go
// ============================================================================

// NOTE: RepositoryInfo, DockerfileInfo, HealthCheckInfo, BuildRecommendations,
// FixingResult, and ProgressToken moved to analysis_types.go to avoid redeclaration

// NOTE: ProgressStage also moved to analysis_types.go
