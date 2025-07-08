// Package types - Tool-specific type definitions
// This file contains types for containerization tools (scan, analyze, build, deploy)
package core

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// ============================================================================
// Base Response Types
// ============================================================================

// ExtendedToolResponse provides extended response fields for tools that need additional metadata
type ExtendedToolResponse struct {
	BaseToolResponse
	ToolName  string        `json:"tool_name"`
	SessionID string        `json:"session_id"`
	DryRun    bool          `json:"dry_run,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// NewExtendedResponse creates a new extended response with common fields
func NewExtendedResponse(toolName, sessionID string, dryRun bool) ExtendedToolResponse {
	return ExtendedToolResponse{
		BaseToolResponse: BaseToolResponse{
			Success:   false,
			Timestamp: time.Now(),
		},
		ToolName:  toolName,
		SessionID: sessionID,
		DryRun:    dryRun,
	}
}

// ============================================================================
// Security Scanning Types - Consolidated from scan_security_types.go
// ============================================================================

// BaseToolArgs provides common arguments for all tools
type BaseToolArgs struct {
	SessionID string                 `json:"session_id" validate:"required,session_id"`
	Context   map[string]interface{} `json:"context,omitempty"`
	DryRun    bool                   `json:"dry_run,omitempty" description:"If true, only validate without executing"`
}

// AtomicScanImageSecurityArgs defines arguments for atomic security scanning
type AtomicScanImageSecurityArgs struct {
	BaseToolArgs

	// Target image
	ImageName string `json:"image_name" validate:"required,docker_image" description:"Docker image name/tag to scan (e.g., nginx:latest)"`

	// Scanning options
	SeverityThreshold string   `json:"severity_threshold,omitempty" validate:"omitempty,severity" description:"Minimum severity to report (LOW,MEDIUM,HIGH,CRITICAL)"`
	VulnTypes         []string `json:"vuln_types,omitempty" validate:"omitempty,dive,vuln_type" description:"Types of vulnerabilities to scan for (os,library,app)"`
	IncludeFixable    bool     `json:"include_fixable,omitempty" description:"Include only fixable vulnerabilities"`
	MaxResults        int      `json:"max_results,omitempty" validate:"omitempty,min=1,max=10000" description:"Maximum number of vulnerabilities to return"`

	// Output options
	IncludeRemediations bool `json:"include_remediations,omitempty" description:"Include remediation recommendations"`
	GenerateReport      bool `json:"generate_report,omitempty" description:"Generate detailed security report"`
	FailOnCritical      bool `json:"fail_on_critical,omitempty" description:"Fail if critical vulnerabilities found"`
}

// NOTE: SecurityScanResult moved to session_types.go to avoid redeclaration

// SecurityVulnerability represents a security vulnerability
type SecurityVulnerability struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	CVSS        float64 `json:"cvss,omitempty"`
	Package     struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		FixedVersion   string `json:"fixed_version,omitempty"`
		PackageManager string `json:"package_manager,omitempty"`
	} `json:"package"`
	References []string  `json:"references,omitempty"`
	Fixed      bool      `json:"fixed,omitempty"`
	Fix        string    `json:"fix,omitempty"`
	Published  time.Time `json:"published,omitempty"`
}

// ComplianceResult represents a compliance check result
type ComplianceResult struct {
	Standard    string `json:"standard"`
	Control     string `json:"control"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// LicenseInfo represents license information
type LicenseInfo struct {
	Package string `json:"package"`
	License string `json:"license"`
	Type    string `json:"type"`
	Risk    string `json:"risk"`
}

// VulnerabilitySummary provides a summary of vulnerabilities
type VulnerabilitySummary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Total    int `json:"total"`
	Fixable  int `json:"fixable"`
}

// ScanMetadata contains metadata about the scan
type ScanMetadata struct {
	Scanner     string        `json:"scanner"`
	Version     string        `json:"version"`
	ScanTime    time.Time     `json:"scan_time"`
	Duration    time.Duration `json:"duration"`
	ImageSize   int64         `json:"image_size,omitempty"`
	ImageLayers int           `json:"image_layers,omitempty"`
}

// Remediation represents a security remediation recommendation
type Remediation struct {
	VulnerabilityID string `json:"vulnerability_id"`
	Type            string `json:"type"`
	Description     string `json:"description"`
	Command         string `json:"command,omitempty"`
	Priority        int    `json:"priority"`
}

// ============================================================================
// Secret Scanning Types - Consolidated from secrets_types.go
// ============================================================================

// SecretScanArgs defines arguments for secret scanning
type SecretScanArgs struct {
	BaseToolArgs

	Path         string   `json:"path" description:"Path to scan for secrets"`
	Patterns     []string `json:"patterns,omitempty" description:"Custom patterns to scan for"`
	IncludeTests bool     `json:"include_tests,omitempty" description:"Include test files in scan"`
	MaxFileSize  int64    `json:"max_file_size,omitempty" description:"Maximum file size to scan"`
}

// SecretScanResult represents the result of a secret scan
type SecretScanResult struct {
	Secrets  []DetectedSecret `json:"secrets"`
	Summary  SecretSummary    `json:"summary"`
	Metadata ScanMetadata     `json:"metadata"`
}

// DetectedSecret represents a detected secret
type DetectedSecret struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Content     string `json:"content"`
	Confidence  string `json:"confidence"`
	Rule        string `json:"rule"`
	Fingerprint string `json:"fingerprint"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// SecretSummary provides a summary of detected secrets
type SecretSummary struct {
	Total        int            `json:"total"`
	ByType       map[string]int `json:"by_type"`
	ByConfidence map[string]int `json:"by_confidence"`
	Files        []string       `json:"files"`
}

// ============================================================================
// Build Types - Container build types
// ============================================================================

// BuildArgs defines arguments for container build operations
type BuildArgs struct {
	BaseToolArgs

	Context        string            `json:"context" description:"Build context path"`
	Dockerfile     string            `json:"dockerfile,omitempty" description:"Path to Dockerfile"`
	Tags           []string          `json:"tags" description:"Image tags"`
	BuildArgs      map[string]string `json:"build_args,omitempty" description:"Build-time variables"`
	Target         string            `json:"target,omitempty" description:"Target stage for multi-stage builds"`
	NoCache        bool              `json:"no_cache,omitempty" description:"Disable build cache"`
	PullParent     bool              `json:"pull_parent,omitempty" description:"Always pull parent images"`
	Registry       string            `json:"registry,omitempty" description:"Target registry"`
	PushAfterBuild bool              `json:"push_after_build,omitempty" description:"Push image after successful build"`
}

// BuildResult represents the result of a container build
type BuildResult struct {
	Success   bool          `json:"success"`
	ImageID   string        `json:"image_id,omitempty"`
	Tags      []string      `json:"tags"`
	Size      int64         `json:"size,omitempty"`
	Duration  time.Duration `json:"duration"`
	LogOutput string        `json:"log_output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Warnings  []string      `json:"warnings,omitempty"`
	Pushed    bool          `json:"pushed,omitempty"`
	Registry  string        `json:"registry,omitempty"`
}

// ============================================================================
// Deploy Types - Container deployment types
// ============================================================================

// DeployArgs defines arguments for container deployment
type DeployArgs struct {
	BaseToolArgs

	Image       string               `json:"image" description:"Container image to deploy"`
	Name        string               `json:"name" description:"Deployment name"`
	Namespace   string               `json:"namespace,omitempty" description:"Kubernetes namespace"`
	Replicas    int                  `json:"replicas,omitempty" description:"Number of replicas"`
	Ports       []ContainerPort      `json:"ports,omitempty" description:"Exposed ports"`
	Environment map[string]string    `json:"environment,omitempty" description:"Environment variables"`
	Resources   types.ResourceLimits `json:"resources,omitempty" description:"Resource limits"`
	HealthCheck HealthCheck          `json:"health_check,omitempty" description:"Health check configuration"`
	Strategy    string               `json:"strategy,omitempty" description:"Deployment strategy"`
}

// ContainerPort represents a container port configuration
type ContainerPort struct {
	ContainerPort int    `json:"container_port"`
	ServicePort   int    `json:"service_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	Name          string `json:"name,omitempty"`
}

// NOTE: ResourceLimits moved to operation_params.go to avoid redeclaration

// HealthCheck defines health check configuration
type HealthCheck struct {
	Type                string   `json:"type"` // http, tcp, exec
	Path                string   `json:"path,omitempty"`
	Port                int      `json:"port,omitempty"`
	Command             []string `json:"command,omitempty"`
	InitialDelaySeconds int      `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int      `json:"period_seconds,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds,omitempty"`
	FailureThreshold    int      `json:"failure_threshold,omitempty"`
}

// NOTE: DeployResult moved to operation_params.go to avoid redeclaration

// DeploymentReplicas represents replica status
type DeploymentReplicas struct {
	Desired   int `json:"desired"`
	Ready     int `json:"ready"`
	Available int `json:"available"`
	Updated   int `json:"updated"`
}

// DeploymentEvent represents a deployment event
type DeploymentEvent struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================================
// Telemetry and Metrics Types
// ============================================================================

// ToolMetrics represents metrics collected for tool execution
type ToolMetrics struct {
	Tool       string        `json:"tool"`        // Tool name
	Success    bool          `json:"success"`     // Whether execution succeeded
	DryRun     bool          `json:"dry_run"`     // Whether this was a dry run
	Duration   time.Duration `json:"duration"`    // Execution duration
	TokensUsed int           `json:"tokens_used"` // AI tokens consumed (if applicable)
}
