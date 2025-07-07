package tools

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// BuildToolResult represents the result of Docker build operations
type BuildToolResult struct {
	types.BaseToolResponse
	ImageRef    string            `json:"image_ref,omitempty"`
	ImageID     string            `json:"image_id,omitempty"`
	ImageSize   int64             `json:"image_size,omitempty"`
	BuildTime   time.Duration     `json:"build_time,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	BuildLogs   []string          `json:"build_logs,omitempty"`
	CacheUsed   bool              `json:"cache_used,omitempty"`
	Layers      []string          `json:"layers,omitempty"`
	BuildArgs   map[string]string `json:"build_args,omitempty"`
	Platform    string            `json:"platform,omitempty"`
	Pushed      bool              `json:"pushed,omitempty"`
	RegistryURL string            `json:"registry_url,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r BuildToolResult) IsSuccess() bool {
	return r.Success
}

// GetData implements ToolOutput interface
func (r BuildToolResult) GetData() interface{} {
	return r
}

// DeployToolResult represents the result of Kubernetes deployment operations
type DeployToolResult struct {
	types.BaseToolResponse
	Namespace        string            `json:"namespace,omitempty"`
	DeploymentName   string            `json:"deployment_name,omitempty"`
	ServiceName      string            `json:"service_name,omitempty"`
	IngressName      string            `json:"ingress_name,omitempty"`
	Replicas         int32             `json:"replicas,omitempty"`
	ReadyReplicas    int32             `json:"ready_replicas,omitempty"`
	ServiceEndpoints []string          `json:"service_endpoints,omitempty"`
	IngressURL       string            `json:"ingress_url,omitempty"`
	ManifestPaths    []string          `json:"manifest_paths,omitempty"`
	DeploymentTime   time.Duration     `json:"deployment_time,omitempty"`
	Status           string            `json:"status,omitempty"`
	Events           []string          `json:"events,omitempty"`
	PodStatus        map[string]string `json:"pod_status,omitempty"`
	ResourcesCreated []string          `json:"resources_created,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r DeployToolResult) IsSuccess() bool {
	return r.Success
}

// ScanToolResult represents the result of security scanning operations
type ScanToolResult struct {
	types.BaseToolResponse
	ScanType         string             `json:"scan_type,omitempty"`
	Target           string             `json:"target,omitempty"`
	Scanner          string             `json:"scanner,omitempty"`
	ScanTime         time.Duration      `json:"scan_time,omitempty"`
	TotalFindings    int                `json:"total_findings,omitempty"`
	Vulnerabilities  VulnerabilityStats `json:"vulnerabilities,omitempty"`
	Secrets          SecretStats        `json:"secrets,omitempty"`
	Compliance       ComplianceStats    `json:"compliance,omitempty"`
	Findings         []SecurityFinding  `json:"findings,omitempty"`
	SBOMGenerated    bool               `json:"sbom_generated,omitempty"`
	SBOMPath         string             `json:"sbom_path,omitempty"`
	ReportPath       string             `json:"report_path,omitempty"`
	PolicyViolations []PolicyViolation  `json:"policy_violations,omitempty"`
	ScanMetadata     map[string]string  `json:"scan_metadata,omitempty"`
}

// VulnerabilityStats represents vulnerability scan statistics
type VulnerabilityStats struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// SecretStats represents secret scan statistics
type SecretStats struct {
	Total            int      `json:"total"`
	TruePositives    int      `json:"true_positives"`
	FalsePositives   int      `json:"false_positives"`
	SecretsFound     []string `json:"secrets_found,omitempty"`
	FilesScanned     int      `json:"files_scanned"`
	FilesWithSecrets int      `json:"files_with_secrets"`
}

// ComplianceStats represents compliance scan statistics
type ComplianceStats struct {
	Total     int      `json:"total"`
	Passed    int      `json:"passed"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Score     float64  `json:"score"`
	Standards []string `json:"standards,omitempty"`
}

// SecurityFinding represents a security finding
type SecurityFinding struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // vulnerability, secret, compliance
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	FixedIn     string `json:"fixed_in,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	CVSS        string `json:"cvss,omitempty"`
	CWE         string `json:"cwe,omitempty"`
	Reference   string `json:"reference,omitempty"`
}

// PolicyViolation represents a policy violation
type PolicyViolation struct {
	PolicyID    string `json:"policy_id"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r ScanToolResult) IsSuccess() bool {
	return r.Success
}

// AnalyzeToolResult represents the result of repository analysis operations
type AnalyzeToolResult struct {
	types.BaseToolResponse
	RepositoryPath       string                     `json:"repository_path,omitempty"`
	RepositoryInfo       *core.RepositoryInfo       `json:"repository_info,omitempty"`
	DockerfileInfo       *core.DockerfileInfo       `json:"dockerfile_info,omitempty"`
	BuildRecommendations *core.BuildRecommendations `json:"build_recommendations,omitempty"`
	SecurityAnalysis     *SecurityAnalysisResult    `json:"security_analysis,omitempty"`
	DependencyAnalysis   *DependencyAnalysisResult  `json:"dependency_analysis,omitempty"`
	AnalysisTime         time.Duration              `json:"analysis_time,omitempty"`
	FilesAnalyzed        int                        `json:"files_analyzed,omitempty"`
	IssuesFound          int                        `json:"issues_found,omitempty"`
}

// SecurityAnalysisResult represents security analysis results
type SecurityAnalysisResult struct {
	SecretsFound         int      `json:"secrets_found"`
	VulnerabilitiesFound int      `json:"vulnerabilities_found"`
	SecurityIssues       []string `json:"security_issues,omitempty"`
	Recommendations      []string `json:"recommendations,omitempty"`
}

// DependencyAnalysisResult represents dependency analysis results
type DependencyAnalysisResult struct {
	TotalDependencies      int               `json:"total_dependencies"`
	OutdatedDependencies   int               `json:"outdated_dependencies"`
	VulnerableDependencies int               `json:"vulnerable_dependencies"`
	Dependencies           map[string]string `json:"dependencies,omitempty"`
	Updates                map[string]string `json:"updates,omitempty"`
	SecurityAdvisories     []string          `json:"security_advisories,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r AnalyzeToolResult) IsSuccess() bool {
	return r.Success
}

// GetData implements ToolOutput interface
func (r AnalyzeToolResult) GetData() interface{} {
	return r
}

// GenerateManifestsResult represents the result of manifest generation
type GenerateManifestsResult struct {
	types.BaseToolResponse
	ManifestPaths    []string          `json:"manifest_paths,omitempty"`
	OutputDirectory  string            `json:"output_directory,omitempty"`
	ResourcesCreated []string          `json:"resources_created,omitempty"`
	GenerationTime   time.Duration     `json:"generation_time,omitempty"`
	ImageRef         string            `json:"image_ref,omitempty"`
	Namespace        string            `json:"namespace,omitempty"`
	ServiceGenerated bool              `json:"service_generated,omitempty"`
	IngressGenerated bool              `json:"ingress_generated,omitempty"`
	ConfigGenerated  map[string]string `json:"config_generated,omitempty"`
	ValidationErrors []string          `json:"validation_errors,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r GenerateManifestsResult) IsSuccess() bool {
	return r.Success
}

// SessionResult represents basic session operation results
type SessionResult struct {
	types.BaseToolResponse
	SessionID    string            `json:"session_id,omitempty"`
	WorkspaceDir string            `json:"workspace_dir,omitempty"`
	State        map[string]string `json:"state,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r SessionResult) IsSuccess() bool {
	return r.Success
}

// ============================================================================
// Additional Atomic Tool Results for Type Safety
// ============================================================================

// AtomicAnalyzeRepositoryResult represents the result of atomic repository analysis
type AtomicAnalyzeRepositoryResult struct {
	types.BaseToolResponse
	types.BaseAIContextResult
	SessionID            string                     `json:"session_id"`
	WorkspaceDir         string                     `json:"workspace_dir"`
	RepoURL              string                     `json:"repo_url"`
	Branch               string                     `json:"branch"`
	CloneDir             string                     `json:"clone_dir"`
	RepositoryInfo       *core.RepositoryInfo       `json:"repository_info,omitempty"`
	DockerfileInfo       *core.DockerfileInfo       `json:"dockerfile_info,omitempty"`
	BuildRecommendations *core.BuildRecommendations `json:"build_recommendations,omitempty"`
	AnalysisTime         time.Duration              `json:"analysis_time,omitempty"`
	FilesAnalyzed        int                        `json:"files_analyzed,omitempty"`
	IssuesFound          int                        `json:"issues_found,omitempty"`
	CloneSuccess         bool                       `json:"clone_success"`
	GitMetadata          map[string]string          `json:"git_metadata,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r AtomicAnalyzeRepositoryResult) IsSuccess() bool {
	return r.Success
}

// AtomicBuildImageResult represents the result of atomic image building
type AtomicBuildImageResult struct {
	types.BaseToolResponse
	types.BaseAIContextResult
	SessionID      string            `json:"session_id"`
	WorkspaceDir   string            `json:"workspace_dir"`
	ImageName      string            `json:"image_name"`
	ImageTag       string            `json:"image_tag"`
	FullImageRef   string            `json:"full_image_ref"`
	DockerfilePath string            `json:"dockerfile_path"`
	BuildContext   string            `json:"build_context"`
	Platform       string            `json:"platform"`
	ImageID        string            `json:"image_id,omitempty"`
	ImageSize      int64             `json:"image_size,omitempty"`
	BuildTime      time.Duration     `json:"build_time,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	BuildLogs      []string          `json:"build_logs,omitempty"`
	CacheUsed      bool              `json:"cache_used,omitempty"`
	Layers         []string          `json:"layers,omitempty"`
	BuildArgs      map[string]string `json:"build_args,omitempty"`
	Pushed         bool              `json:"pushed,omitempty"`
	RegistryURL    string            `json:"registry_url,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r AtomicBuildImageResult) IsSuccess() bool {
	return r.Success
}

// AtomicDeployKubernetesResult represents the result of atomic Kubernetes deployment
type AtomicDeployKubernetesResult struct {
	types.BaseToolResponse
	types.BaseAIContextResult
	SessionID        string            `json:"session_id"`
	WorkspaceDir     string            `json:"workspace_dir"`
	ImageRef         string            `json:"image_ref"`
	AppName          string            `json:"app_name"`
	Namespace        string            `json:"namespace"`
	ManifestPath     string            `json:"manifest_path,omitempty"`
	DeploymentName   string            `json:"deployment_name,omitempty"`
	ServiceName      string            `json:"service_name,omitempty"`
	IngressName      string            `json:"ingress_name,omitempty"`
	Replicas         int32             `json:"replicas,omitempty"`
	ReadyReplicas    int32             `json:"ready_replicas,omitempty"`
	ServiceEndpoints []string          `json:"service_endpoints,omitempty"`
	IngressURL       string            `json:"ingress_url,omitempty"`
	ManifestPaths    []string          `json:"manifest_paths,omitempty"`
	DeploymentTime   time.Duration     `json:"deployment_time,omitempty"`
	Status           string            `json:"status,omitempty"`
	Events           []string          `json:"events,omitempty"`
	PodStatus        map[string]string `json:"pod_status,omitempty"`
	ResourcesCreated []string          `json:"resources_created,omitempty"`
	HealthStatus     string            `json:"health_status,omitempty"`
	ValidationResult string            `json:"validation_result,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r AtomicDeployKubernetesResult) IsSuccess() bool {
	return r.Success
}

// AtomicScanImageSecurityResult represents the result of atomic image security scanning
type AtomicScanImageSecurityResult struct {
	types.BaseToolResponse
	types.BaseAIContextResult
	SessionID        string             `json:"session_id"`
	WorkspaceDir     string             `json:"workspace_dir"`
	ImageRef         string             `json:"image_ref"`
	ScanType         string             `json:"scan_type,omitempty"`
	Scanner          string             `json:"scanner,omitempty"`
	ScanTime         time.Duration      `json:"scan_time,omitempty"`
	TotalFindings    int                `json:"total_findings,omitempty"`
	Vulnerabilities  VulnerabilityStats `json:"vulnerabilities,omitempty"`
	Secrets          SecretStats        `json:"secrets,omitempty"`
	Compliance       ComplianceStats    `json:"compliance,omitempty"`
	Findings         []SecurityFinding  `json:"findings,omitempty"`
	SBOMGenerated    bool               `json:"sbom_generated,omitempty"`
	SBOMPath         string             `json:"sbom_path,omitempty"`
	ReportPath       string             `json:"report_path,omitempty"`
	PolicyViolations []PolicyViolation  `json:"policy_violations,omitempty"`
	ScanMetadata     map[string]string  `json:"scan_metadata,omitempty"`
	HasSecrets       bool               `json:"has_secrets"`
	ScanError        string             `json:"scan_error,omitempty"`
}

// IsSuccess returns whether the operation was successful
func (r AtomicScanImageSecurityResult) IsSuccess() bool {
	return r.Success
}
