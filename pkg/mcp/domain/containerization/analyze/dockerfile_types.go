package analyze

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// ============================================================================
// Dockerfile Validation Type Definitions
// ============================================================================

// This file contains all type definitions for Dockerfile validation functionality.
// These types support comprehensive Dockerfile analysis including syntax validation,
// best practices checking, security analysis, and optimization recommendations.

// ============================================================================
// Input Arguments
// ============================================================================

// AtomicValidateDockerfileArgs defines input parameters for Dockerfile validation.
// This structure supports both file-based and content-based validation with
// extensive configuration options for different validation aspects.
type AtomicValidateDockerfileArgs struct {
	types.BaseToolArgs

	// Session ID for state correlation
	SessionID string `json:"session_id" validate:"required,session_id" description:"Session ID for state correlation"`

	// Dockerfile source - either path or content
	DockerfilePath    string `json:"dockerfile_path,omitempty" validate:"omitempty,secure_path" description:"Path to Dockerfile (default: session workspace/Dockerfile)"`
	DockerfileContent string `json:"dockerfile_content,omitempty" validate:"omitempty" description:"Dockerfile content to validate (alternative to path)"`

	// Validation tools and configuration
	UseHadolint       bool     `json:"use_hadolint,omitempty" description:"Use Hadolint for advanced validation"`
	Severity          string   `json:"severity,omitempty" validate:"omitempty,oneof=info warning error" description:"Minimum severity to report (info, warning, error)"`
	IgnoreRules       []string `json:"ignore_rules,omitempty" validate:"omitempty" description:"Hadolint rules to ignore (e.g., DL3008, DL3009)"`
	TrustedRegistries []string `json:"trusted_registries,omitempty" validate:"omitempty,dive,registry_url" description:"List of trusted registries for base image validation"`

	// Validation scope flags
	CheckSecurity      bool `json:"check_security,omitempty" description:"Perform security-focused checks"`
	CheckOptimization  bool `json:"check_optimization,omitempty" description:"Check for image size optimization opportunities"`
	CheckBestPractices bool `json:"check_best_practices,omitempty" description:"Validate against Docker best practices"`

	// Output options
	IncludeSuggestions bool `json:"include_suggestions,omitempty" description:"Include remediation suggestions"`
	GenerateFixes      bool `json:"generate_fixes,omitempty" description:"Generate corrected Dockerfile"`
}

// ============================================================================
// Output Results
// ============================================================================

// AtomicValidateDockerfileResult represents the comprehensive validation result.
// This structure provides detailed validation findings, analysis results, and
// actionable recommendations for improving the Dockerfile.
type AtomicValidateDockerfileResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult

	// Validation status
	Valid             bool    `json:"valid"`
	ValidationScore   float64 `json:"validation_score"`
	DockerfileContent string  `json:"dockerfile_content,omitempty"`

	// Validation findings
	Errors           []DockerfileError         `json:"errors,omitempty"`
	Warnings         []DockerfileWarning       `json:"warnings,omitempty"`
	Suggestions      []DockerfileSuggestion    `json:"suggestions,omitempty"`
	BestPractices    []BestPracticeViolation   `json:"best_practices,omitempty"`
	SecurityIssues   []DockerfileSecurityIssue `json:"security_issues,omitempty"`
	OptimizationTips []OptimizationTip         `json:"optimization_tips,omitempty"`

	// Analysis results
	BaseImageAnalysis    *BaseImageAnalysis `json:"base_image_analysis,omitempty"`
	LayerAnalysis        *LayerAnalysis     `json:"layer_analysis,omitempty"`
	SecurityAnalysis     *SecurityAnalysis  `json:"security_analysis,omitempty"`
	BestPracticesSummary map[string]int     `json:"best_practices_summary,omitempty"`

	// Generated content
	CorrectedDockerfile string              `json:"corrected_dockerfile,omitempty"`
	Recommendations     []Recommendation    `json:"recommendations,omitempty"`
	ImprovementMetrics  *ImprovementMetrics `json:"improvement_metrics,omitempty"`
}

// ============================================================================
// Validation Finding Types
// ============================================================================

// DockerfileError represents a validation error in the Dockerfile.
// Errors indicate issues that must be fixed for the Dockerfile to work correctly.
type DockerfileError struct {
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
	Instruction string `json:"instruction,omitempty"`
	Fix         string `json:"fix,omitempty"`
}

// DockerfileWarning represents a validation warning in the Dockerfile.
// Warnings indicate potential issues that should be addressed but don't prevent functionality.
type DockerfileWarning struct {
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
	Instruction string `json:"instruction,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// DockerfileSuggestion represents an improvement suggestion.
// Suggestions are optional improvements that can enhance the Dockerfile.
type DockerfileSuggestion struct {
	Line        int    `json:"line,omitempty"`
	Category    string `json:"category"`
	Message     string `json:"message"`
	Improvement string `json:"improvement"`
	Impact      string `json:"impact,omitempty"`
}

// BestPracticeViolation represents a Docker best practice violation.
// These violations indicate deviations from recommended Docker patterns.
type BestPracticeViolation struct {
	Line        int    `json:"line,omitempty"`
	Practice    string `json:"practice"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Reference   string `json:"reference,omitempty"`
}

// DockerfileSecurityIssue represents a security concern in the Dockerfile.
// Security issues require immediate attention to prevent vulnerabilities.
type DockerfileSecurityIssue struct {
	Line        int    `json:"line,omitempty"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Remediation string `json:"remediation"`
	CWE         string `json:"cwe,omitempty"`
}

// OptimizationTip represents an optimization opportunity.
// These tips help reduce image size and improve build performance.
type OptimizationTip struct {
	Line            int    `json:"line,omitempty"`
	Type            string `json:"type"`
	Description     string `json:"description"`
	CurrentImpact   string `json:"current_impact"`
	PotentialSaving string `json:"potential_saving,omitempty"`
	Implementation  string `json:"implementation"`
}

// ============================================================================
// Analysis Result Types
// ============================================================================

// BaseImageAnalysis provides detailed analysis of the base image.
// This includes security status, age, size, and recommendations.
type BaseImageAnalysis struct {
	Image             string         `json:"image"`
	Tag               string         `json:"tag"`
	Digest            string         `json:"digest,omitempty"`
	Official          bool           `json:"official"`
	Trusted           bool           `json:"trusted"`
	Age               *time.Duration `json:"age,omitempty"`
	Size              int64          `json:"size,omitempty"`
	Vulnerabilities   int            `json:"vulnerabilities,omitempty"`
	LastUpdated       *time.Time     `json:"last_updated,omitempty"`
	AlternativeImages []string       `json:"alternative_images,omitempty"`
	SecurityScore     float64        `json:"security_score"`
	Recommendations   []string       `json:"recommendations,omitempty"`
}

// LayerAnalysis provides insights into Dockerfile layers.
// This helps identify opportunities for layer optimization and caching improvements.
type LayerAnalysis struct {
	TotalLayers        int           `json:"total_layers"`
	CacheableLayers    int           `json:"cacheable_layers"`
	NonCacheableLayers int           `json:"non_cacheable_layers"`
	EstimatedSize      int64         `json:"estimated_size,omitempty"`
	LayerDetails       []LayerDetail `json:"layer_details,omitempty"`
	OptimizationTips   []string      `json:"optimization_tips,omitempty"`
}

// LayerDetail provides information about a specific layer.
type LayerDetail struct {
	Index       int    `json:"index"`
	Instruction string `json:"instruction"`
	Cacheable   bool   `json:"cacheable"`
	Size        int64  `json:"size,omitempty"`
	Impact      string `json:"impact,omitempty"`
}

// SecurityAnalysis provides comprehensive security assessment.
// This includes various security checks and their results.
type SecurityAnalysis struct {
	Score            float64           `json:"score"`
	RunAsRoot        bool              `json:"run_as_root"`
	ExposedPorts     []int             `json:"exposed_ports,omitempty"`
	SensitiveData    []string          `json:"sensitive_data,omitempty"`
	InsecurePackages []string          `json:"insecure_packages,omitempty"`
	HardcodedSecrets bool              `json:"hardcoded_secrets"`
	SecurityFeatures map[string]bool   `json:"security_features"`
	ComplianceStatus map[string]string `json:"compliance_status,omitempty"`
}

// ============================================================================
// Recommendation and Improvement Types
// ============================================================================

// Recommendation provides actionable improvement suggestions.
// Each recommendation includes priority and expected impact.
type Recommendation struct {
	Priority    string `json:"priority"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"`
	Example     string `json:"example,omitempty"`
}

// ImprovementMetrics quantifies potential improvements.
// This helps prioritize optimization efforts based on expected benefits.
type ImprovementMetrics struct {
	SecurityScoreImprovement   float64 `json:"security_score_improvement"`
	SizereductionPotential     int64   `json:"size_reduction_potential"`
	LayerReductionPotential    int     `json:"layer_reduction_potential"`
	BuildTimeImprovement       string  `json:"build_time_improvement,omitempty"`
	CacheEfficiencyImprovement float64 `json:"cache_efficiency_improvement"`
}

// ============================================================================
// Internal Types
// ============================================================================

// ValidationContext holds the context for validation operations.
// This is used internally to pass state between validation functions.
type ValidationContext struct {
	Content      string
	Lines        []string
	Instructions []DockerfileInstruction
	Errors       []DockerfileError
	Warnings     []DockerfileWarning
	Suggestions  []DockerfileSuggestion
}

// DockerfileInstruction represents a parsed Dockerfile instruction.
// This is used internally for analysis and validation.
type DockerfileInstruction struct {
	Line       int
	Type       string
	Arguments  []string
	RawContent string
}
