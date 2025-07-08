package api

import (
	"time"

	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

type ValidationError = validation.Error
type ValidationWarning = validation.Warning
type ValidationMetadata = validation.Metadata
type ValidationResult = validation.Result
type ManifestValidationResult = validation.Result
type BuildValidationResult = validation.Result

// NewError creates a new validation error
func NewError(code, message string, errorType validation.ErrorType, severity validation.ErrorSeverity) *ValidationError {
	return validation.NewError(code, message, errorType, severity)
}

// TypedScanInput represents typed scan input (stub for compatibility)
type TypedScanInput struct {
	Target   string   `json:"target"`
	ScanType string   `json:"scan_type"`
	Severity []string `json:"severity"`
}

// TypedScanOutput represents typed scan output (stub for compatibility)
type TypedScanOutput struct {
	Success         bool            `json:"success"`
	SessionID       string          `json:"session_id,omitempty"`
	ErrorMsg        string          `json:"error_msg,omitempty"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Summary         string          `json:"summary"`
	ScanMetrics     ScanMetrics     `json:"scan_metrics"`
}

// ScanDetails represents scan details (stub for compatibility)
type ScanDetails struct {
	ExecutionDetails ExecutionDetails `json:"execution_details"`
}

// ExecutionDetails represents execution details (stub for compatibility)
type ExecutionDetails struct {
	StartTime     string        `json:"start_time"`
	EndTime       string        `json:"end_time"`
	Duration      string        `json:"duration"`
	ResourcesUsed ResourceUsage `json:"resources_used"`
}

// ResourceUsage represents resource usage (stub for compatibility)
type ResourceUsage struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// Vulnerability represents a vulnerability (stub for compatibility)
type Vulnerability struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	CVSS        float64  `json:"cvss"`
	CVSSScore   float64  `json:"cvss_score"`
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	FixedIn     string   `json:"fixed_in"`
	References  []string `json:"references,omitempty"`
}

// ScanMetrics represents scan metrics (stub for compatibility)
type ScanMetrics struct {
	TotalVulnerabilities int            `json:"total_vulnerabilities"`
	BySeverity           map[string]int `json:"by_severity"`
	CriticalCount        int            `json:"critical_count"`
	HighCount            int            `json:"high_count"`
}

// TypedToolOutput represents typed tool output (stub for compatibility)
type TypedToolOutput[TData any, TDetails any] struct {
	Success bool     `json:"success"`
	Data    TData    `json:"data"`
	Details TDetails `json:"details"`
	Error   string   `json:"error,omitempty"`
}

// ScanContext represents scan context (stub for compatibility)
type ScanContext struct {
	RequestID      string `json:"request_id"`
	FailOnSeverity string `json:"fail_on_severity,omitempty"`
}

// TypedToolInput represents typed tool input (stub for compatibility)
type TypedToolInput[TData any, TContext any] struct {
	SessionID string   `json:"session_id"`
	Data      TData    `json:"data"`
	Context   TContext `json:"context"`
}

// TypedDeployInput represents typed deploy input (stub for compatibility)
type TypedDeployInput struct {
	Namespace   string            `json:"namespace"`
	Manifests   []string          `json:"manifests"`
	DryRun      bool              `json:"dry_run"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TypedDeployOutput represents typed deploy output (stub for compatibility)
type TypedDeployOutput struct {
	Success        bool     `json:"success"`
	DeploymentName string   `json:"deployment_name"`
	ServiceName    string   `json:"service_name"`
	Namespace      string   `json:"namespace"`
	Endpoints      []string `json:"endpoints"`
	Summary        string   `json:"summary"`
}

// DeployContext represents deploy context (stub for compatibility)
type DeployContext struct {
	RequestID      string `json:"request_id"`
	RollbackOnFail bool   `json:"rollback_on_fail"`
}

// DeployDetails represents deploy details (stub for compatibility)
type DeployDetails struct {
	ExecutionDetails ExecutionDetails `json:"execution_details"`
	ResourcesCreated []string         `json:"resources_created"`
	ResourcesUpdated []string         `json:"resources_updated"`
}

// TypedToolSchema represents typed tool schema (stub for compatibility)
type TypedToolSchema[TInput any, TContext any, TOutput any, TDetails any] struct {
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	InputExample  TypedToolInput[TInput, TContext]   `json:"input_example"`
	OutputExample TypedToolOutput[TOutput, TDetails] `json:"output_example"`
}

// AnalyzeInput represents typed analyze input
type AnalyzeInput struct {
	SessionID           string                 `json:"session_id"`
	RepoURL             string                 `json:"repo_url"`
	Path                string                 `json:"path,omitempty"`
	Branch              string                 `json:"branch,omitempty"`
	Language            string                 `json:"language,omitempty"`
	LanguageHint        string                 `json:"language_hint,omitempty"`
	Framework           string                 `json:"framework,omitempty"`
	IncludeDependencies bool                   `json:"include_dependencies,omitempty"`
	IncludeSecurityScan bool                   `json:"include_security_scan,omitempty"`
	CustomOptions       map[string]string      `json:"custom_options,omitempty"`
	Context             map[string]interface{} `json:"context,omitempty"`
}

// GetSessionID implements ToolInputConstraint
func (a *AnalyzeInput) GetSessionID() string {
	return a.SessionID
}

// Validate implements basic validation
func (a *AnalyzeInput) Validate() error {
	if a.SessionID == "" {
		return ErrorInvalidInput
	}
	if a.RepoURL == "" && a.Path == "" {
		return ErrorInvalidInput
	}
	return nil
}

// GetContext returns execution context
func (a *AnalyzeInput) GetContext() map[string]interface{} {
	if a.Context == nil {
		return make(map[string]interface{})
	}
	return a.Context
}

// AnalyzeOutput represents typed analyze output
type AnalyzeOutput struct {
	Success              bool                   `json:"success"`
	SessionID            string                 `json:"session_id,omitempty"`
	Language             string                 `json:"language,omitempty"`
	Framework            string                 `json:"framework,omitempty"`
	Dependencies         []Dependency           `json:"dependencies,omitempty"`
	SecurityIssues       []SecurityIssue        `json:"security_issues,omitempty"`
	BuildRecommendations []string               `json:"build_recommendations,omitempty"`
	AnalysisTime         time.Duration          `json:"analysis_time,omitempty"`
	FilesAnalyzed        int                    `json:"files_analyzed,omitempty"`
	ErrorMsg             string                 `json:"error_msg,omitempty"`
	Repository           interface{}            `json:"repository,omitempty"`
	Dockerfile           interface{}            `json:"dockerfile,omitempty"`
	Summary              string                 `json:"summary,omitempty"`
	Data                 map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess implements ToolOutputConstraint
func (a *AnalyzeOutput) IsSuccess() bool {
	return a.Success
}

// GetData implements ToolOutputConstraint
func (a *AnalyzeOutput) GetData() interface{} {
	if a.Data == nil {
		return make(map[string]interface{})
	}
	return a.Data
}

// GetError implements ToolOutputConstraint
func (a *AnalyzeOutput) GetError() string {
	return a.ErrorMsg
}

// Dependency represents a project dependency
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // direct, indirect, dev, test, etc.
}

// SecurityIssue represents a security issue found during analysis
type SecurityIssue struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	FixVersion  string `json:"fix_version,omitempty"`
	Type        string `json:"type,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Fix         string `json:"fix,omitempty"`
}
