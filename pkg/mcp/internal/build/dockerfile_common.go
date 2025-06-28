package build

import (
	"github.com/Azure/container-kit/pkg/core/docker"
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

// Note: ValidationResult, ValidationError, ValidationWarning, and SecurityIssue
// are defined in common.go to avoid duplication
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
type DockerfileValidator interface {
	Validate(content string, options ValidationOptions) (*ValidationResult, error)
}

// DockerfileAnalyzer defines the interface for specific Dockerfile analysis types
type DockerfileAnalyzer interface {
	Analyze(lines []string, context ValidationContext) interface{}
}

// DockerfileFixer defines the interface for generating fixes
type DockerfileFixer interface {
	GenerateFixes(content string, result *ValidationResult) (string, []string)
}

// ConvertCoreResult converts core docker validation result to our result type
func ConvertCoreResult(coreResult *docker.ValidationResult) *ValidationResult {
	result := &ValidationResult{
		Valid:    coreResult.Valid,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
	}
	// Convert errors
	for _, err := range coreResult.Errors {
		result.Errors = append(result.Errors, ValidationError{
			Line:    err.Line,
			Column:  err.Column,
			Message: err.Message,
			Rule:    err.Type,
		})
		// Note: CriticalIssues field removed from ValidationResult
	}
	// Convert warnings
	for _, warn := range coreResult.Warnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    warn.Line,
			Column:  0, // Column not available in core docker warning
			Message: warn.Message,
			Rule:    warn.Type,
		})
	}
	// Note: Context field removed from ValidationResult
	// Note: TotalIssues field removed from ValidationResult
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
