package api

import (
	"time"
)

// ============================================================================
// Typed Analyze Tool
// ============================================================================

// TypedAnalyzeInput represents strongly typed input for repository analysis
type TypedAnalyzeInput struct {
	SessionID            string            `json:"session_id"`
	RepoURL              string            `json:"repo_url"`
	Branch               string            `json:"branch,omitempty"`
	LanguageHint         string            `json:"language_hint,omitempty"`
	IncludeDependencies  bool              `json:"include_dependencies"`
	IncludeSecurityScan  bool              `json:"include_security_scan"`
	IncludeBuildAnalysis bool              `json:"include_build_analysis"`
	CustomOptions        map[string]string `json:"custom_options,omitempty"`
}

// TypedAnalyzeOutput represents strongly typed output from repository analysis
type TypedAnalyzeOutput struct {
	Success              bool            `json:"success"`
	SessionID            string          `json:"session_id"`
	Language             string          `json:"language"`
	Framework            string          `json:"framework"`
	Dependencies         []Dependency    `json:"dependencies,omitempty"`
	SecurityIssues       []SecurityIssue `json:"security_issues,omitempty"`
	BuildRecommendations []string        `json:"build_recommendations,omitempty"`
	AnalysisMetrics      AnalysisMetrics `json:"analysis_metrics"`
	ErrorMsg             string          `json:"error,omitempty"`
}

// AnalysisMetrics contains metrics about the analysis
type AnalysisMetrics struct {
	FilesAnalyzed  int           `json:"files_analyzed"`
	LinesOfCode    int           `json:"lines_of_code"`
	AnalysisTime   time.Duration `json:"analysis_time"`
	CodeComplexity float64       `json:"code_complexity,omitempty"`
	TestCoverage   float64       `json:"test_coverage,omitempty"`
}

// TypedAnalyzeTool is the fully typed interface for analysis tools
type TypedAnalyzeTool interface {
	TypedTool[TypedAnalyzeInput, AnalysisContext, TypedAnalyzeOutput, AnalysisDetails]
}

// ============================================================================
// Typed Build Tool
// ============================================================================

// TypedBuildInput represents strongly typed input for image building
type TypedBuildInput struct {
	SessionID     string            `json:"session_id"`
	Image         string            `json:"image"`
	Dockerfile    string            `json:"dockerfile"`
	ContextPath   string            `json:"context"`
	BuildArgs     map[string]string `json:"build_args,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	NoCache       bool              `json:"no_cache"`
	Platform      string            `json:"platform,omitempty"`
	CustomOptions map[string]string `json:"custom_options,omitempty"`
}

// TypedBuildOutput represents strongly typed output from image building
type TypedBuildOutput struct {
	Success      bool         `json:"success"`
	SessionID    string       `json:"session_id"`
	ImageID      string       `json:"image_id"`
	Digest       string       `json:"digest"`
	Tags         []string     `json:"tags"`
	BuildMetrics BuildMetrics `json:"build_metrics"`
	ErrorMsg     string       `json:"error,omitempty"`
}

// BuildMetrics contains metrics about the build
type BuildMetrics struct {
	BuildTime  time.Duration `json:"build_time"`
	ImageSize  int64         `json:"image_size"`
	LayerCount int           `json:"layer_count"`
	BaseImage  string        `json:"base_image"`
	CacheUsed  bool          `json:"cache_used"`
}

// TypedBuildTool is the fully typed interface for build tools
type TypedBuildTool interface {
	TypedTool[TypedBuildInput, BuildContext, TypedBuildOutput, BuildDetails]
}

// ============================================================================
// Typed Deploy Tool
// ============================================================================

// TypedDeployInput represents strongly typed input for deployment
type TypedDeployInput struct {
	SessionID     string            `json:"session_id"`
	Manifests     []string          `json:"manifests"`
	Namespace     string            `json:"namespace,omitempty"`
	DryRun        bool              `json:"dry_run"`
	Wait          bool              `json:"wait"`
	Timeout       time.Duration     `json:"timeout,omitempty"`
	CustomOptions map[string]string `json:"custom_options,omitempty"`
}

// TypedDeployOutput represents strongly typed output from deployment
type TypedDeployOutput struct {
	Success         bool             `json:"success"`
	SessionID       string           `json:"session_id"`
	DeployedObjects []DeployedObject `json:"deployed_objects"`
	DeployMetrics   DeployMetrics    `json:"deploy_metrics"`
	ErrorMsg        string           `json:"error,omitempty"`
}

// DeployedObject represents a deployed Kubernetes object
type DeployedObject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Status    string `json:"status"`
}

// DeployMetrics contains metrics about the deployment
type DeployMetrics struct {
	DeployTime       time.Duration `json:"deploy_time"`
	ObjectsCreated   int           `json:"objects_created"`
	ObjectsUpdated   int           `json:"objects_updated"`
	ObjectsDeleted   int           `json:"objects_deleted"`
	RollbackRequired bool          `json:"rollback_required"`
}

// TypedDeployTool is the fully typed interface for deployment tools
type TypedDeployTool interface {
	TypedTool[TypedDeployInput, DeployContext, TypedDeployOutput, DeployDetails]
}

// ============================================================================
// Typed Scan Tool
// ============================================================================

// TypedScanInput represents strongly typed input for security scanning
type TypedScanInput struct {
	SessionID     string            `json:"session_id"`
	Target        string            `json:"target"`
	ScanType      ScanType          `json:"scan_type"`
	Severity      []string          `json:"severity,omitempty"`
	IgnoreCVEs    []string          `json:"ignore_cves,omitempty"`
	CustomOptions map[string]string `json:"custom_options,omitempty"`
}

// ScanType represents different types of security scans
type ScanType string

const (
	ScanTypeImage      ScanType = "image"
	ScanTypeFilesystem ScanType = "filesystem"
	ScanTypeRepository ScanType = "repository"
	ScanTypeKubernetes ScanType = "kubernetes"
)

// TypedScanOutput represents strongly typed output from security scanning
type TypedScanOutput struct {
	Success          bool            `json:"success"`
	SessionID        string          `json:"session_id"`
	Vulnerabilities  []Vulnerability `json:"vulnerabilities"`
	ScanMetrics      ScanMetrics     `json:"scan_metrics"`
	ComplianceStatus map[string]bool `json:"compliance_status,omitempty"`
	ErrorMsg         string          `json:"error,omitempty"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	FixedIn     string   `json:"fixed_in,omitempty"`
	Description string   `json:"description"`
	CVSSScore   float64  `json:"cvss_score,omitempty"`
	References  []string `json:"references,omitempty"`
}

// ScanMetrics contains metrics about the scan
type ScanMetrics struct {
	ScanTime        time.Duration `json:"scan_time"`
	PackagesScanned int           `json:"packages_scanned"`
	VulnCount       int           `json:"vuln_count"`
	CriticalCount   int           `json:"critical_count"`
	HighCount       int           `json:"high_count"`
	MediumCount     int           `json:"medium_count"`
	LowCount        int           `json:"low_count"`
}

// ScanContext provides context for scan operations
type ScanContext struct {
	ExecutionContext

	// PolicyFile to use for compliance checking
	PolicyFile string `json:"policy_file,omitempty"`

	// FailOnSeverity causes scan to fail if vulnerabilities exceed this level
	FailOnSeverity string `json:"fail_on_severity,omitempty"`

	// OfflineMode runs scan without network access
	OfflineMode bool `json:"offline_mode"`
}

// ScanDetails provides details for scan operations
type ScanDetails struct {
	ExecutionDetails

	// DatabaseVersion of vulnerability database used
	DatabaseVersion string `json:"database_version"`

	// DatabaseUpdated when the database was last updated
	DatabaseUpdated time.Time `json:"database_updated"`

	// ScanEngine used for scanning
	ScanEngine string `json:"scan_engine"`
}

// TypedScanTool is the fully typed interface for security scanning tools
type TypedScanTool interface {
	TypedTool[TypedScanInput, ScanContext, TypedScanOutput, ScanDetails]
}

// ============================================================================
// Tool Factory Functions
// ============================================================================

// NewTypedAnalyzeTool creates a new typed analyze tool from a generic implementation
func NewTypedAnalyzeTool(impl TypedTool[TypedAnalyzeInput, AnalysisContext, TypedAnalyzeOutput, AnalysisDetails]) TypedAnalyzeTool {
	return impl
}

// NewTypedBuildTool creates a new typed build tool from a generic implementation
func NewTypedBuildTool(impl TypedTool[TypedBuildInput, BuildContext, TypedBuildOutput, BuildDetails]) TypedBuildTool {
	return impl
}

// NewTypedDeployTool creates a new typed deploy tool from a generic implementation
func NewTypedDeployTool(impl TypedTool[TypedDeployInput, DeployContext, TypedDeployOutput, DeployDetails]) TypedDeployTool {
	return impl
}

// NewTypedScanTool creates a new typed scan tool from a generic implementation
func NewTypedScanTool(impl TypedTool[TypedScanInput, ScanContext, TypedScanOutput, ScanDetails]) TypedScanTool {
	return impl
}
