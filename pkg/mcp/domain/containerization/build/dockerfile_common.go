package build

import (
	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	coredocker "github.com/Azure/container-kit/pkg/core/docker"
)

// ValidationContext provides context for validation operations
type ValidationContext struct {
	DockerfilePath    string
	DockerfileContent string
	SessionID         string
	Options           ValidationOptions
}

// Note: BuildValidationResult, ValidationError, ValidationWarning, and SecurityIssue
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
	Validate(content string, options ValidationOptions) (*BuildValidationResult, error)
}

// DockerfileAnalyzer defines the interface for specific Dockerfile analysis types
type DockerfileAnalyzer interface {
	Analyze(lines []string, context ValidationContext) interface{}
}

// DockerfileFixer defines the interface for generating fixes
type DockerfileFixer interface {
	GenerateFixes(content string, result *BuildValidationResult) (string, []string)
}

// ConvertCoreResult converts core docker validation result to our result type
func ConvertCoreResult(coreResult *coredocker.BuildResult) *BuildValidationResult {
	result := &core.BuildResult{
		Valid: coreResult.Success,
	}

	// Convert error from docker build result to new format
	if coreResult.Error != nil {
		result.Errors = append(result.Errors, &core.Error{
			Message:  coreResult.Error.Message,
			Code:     "DOCKERFILE_ERROR",
			Severity: "medium",
		})
		result.Valid = false
	}

	// Note: docker.BuildResult doesn't have warnings field
	// Note: Context field removed from BuildValidationResult
	// Note: TotalIssues field removed from BuildValidationResult
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
