package dockerfile

import (
	"github.com/Azure/container-copilot/pkg/core/docker"
)

// ValidationContext provides context for validation operations
type ValidationContext struct {
	DockerfilePath    string
	DockerfileContent string
	SessionID         string
	Options           ValidationOptions
}

// ValidationOptions contains configuration for validation
type ValidationOptions struct {
	UseHadolint        bool
	Severity           string
	IgnoreRules        []string
	TrustedRegistries  []string
	CheckSecurity      bool
	CheckOptimization  bool
	CheckBestPractices bool
}

// ValidationResult represents the result of Dockerfile validation
type ValidationResult struct {
	IsValid         bool
	ValidationScore int
	TotalIssues     int
	CriticalIssues  int

	Errors           []ValidationError
	Warnings         []ValidationWarning
	SecurityIssues   []SecurityIssue
	OptimizationTips []OptimizationTip

	BaseImageAnalysis BaseImageAnalysis
	LayerAnalysis     LayerAnalysis
	SecurityAnalysis  SecurityAnalysis

	Suggestions []string
	Context     map[string]interface{}
}

// ValidationError represents a validation error
type ValidationError struct {
	Type          string
	Line          int
	Column        int
	Rule          string
	Message       string
	Instruction   string
	Severity      string
	Fix           string
	Documentation string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Type       string
	Line       int
	Rule       string
	Message    string
	Suggestion string
	Impact     string
}

// SecurityIssue represents a security-related issue
type SecurityIssue struct {
	Type          string
	Line          int
	Severity      string
	Description   string
	Remediation   string
	CVEReferences []string
}

// OptimizationTip represents an optimization suggestion
type OptimizationTip struct {
	Type             string
	Line             int
	Description      string
	Impact           string
	Suggestion       string
	EstimatedSavings string
}

// BaseImageAnalysis provides analysis of the base image
type BaseImageAnalysis struct {
	Image           string
	Registry        string
	IsTrusted       bool
	IsOfficial      bool
	HasKnownVulns   bool
	Alternatives    []string
	Recommendations []string
}

// LayerAnalysis provides analysis of Dockerfile layers
type LayerAnalysis struct {
	TotalLayers      int
	CacheableSteps   int
	ProblematicSteps []ProblematicStep
	Optimizations    []LayerOptimization
}

// ProblematicStep represents a step that could cause issues
type ProblematicStep struct {
	Line        int
	Instruction string
	Issue       string
	Impact      string
}

// LayerOptimization represents a layer optimization opportunity
type LayerOptimization struct {
	Type        string
	Description string
	Before      string
	After       string
	Benefit     string
}

// SecurityAnalysis provides comprehensive security analysis
type SecurityAnalysis struct {
	RunsAsRoot      bool
	ExposedPorts    []int
	HasSecrets      bool
	UsesPackagePin  bool
	SecurityScore   int
	Recommendations []string
}

// Validator defines the interface for Dockerfile validators
type Validator interface {
	Validate(content string, options ValidationOptions) (*ValidationResult, error)
}

// Analyzer defines the interface for specific analysis types
type Analyzer interface {
	Analyze(lines []string, context ValidationContext) interface{}
}

// Fixer defines the interface for generating fixes
type Fixer interface {
	GenerateFixes(content string, result *ValidationResult) (string, []string)
}

// ConvertCoreResult converts core docker validation result to our result type
func ConvertCoreResult(coreResult *docker.ValidationResult) *ValidationResult {
	result := &ValidationResult{
		IsValid:     coreResult.Valid,
		Suggestions: coreResult.Suggestions,
		Context:     make(map[string]interface{}),
		Errors:      make([]ValidationError, 0),
		Warnings:    make([]ValidationWarning, 0),
	}

	// Convert errors
	for _, err := range coreResult.Errors {
		result.Errors = append(result.Errors, ValidationError{
			Type:        err.Type,
			Line:        err.Line,
			Column:      err.Column,
			Message:     err.Message,
			Instruction: err.Instruction,
			Severity:    err.Severity,
		})
		if err.Severity == "error" {
			result.CriticalIssues++
		}
	}

	// Convert warnings
	for _, warn := range coreResult.Warnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:       warn.Type,
			Line:       warn.Line,
			Message:    warn.Message,
			Suggestion: warn.Suggestion,
			Impact:     determineImpact(warn.Type),
		})
	}

	// Copy context
	if coreResult.Context != nil {
		for k, v := range coreResult.Context {
			result.Context[k] = v
		}
	}

	result.TotalIssues = len(result.Errors) + len(result.Warnings)

	return result
}

func determineImpact(warningType string) string {
	switch warningType {
	case "security":
		return "security"
	case "best_practice":
		return "maintainability"
	default:
		return "performance"
	}
}
