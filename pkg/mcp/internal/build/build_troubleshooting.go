package build

import (
	"fmt"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/rs/zerolog"
)

// BuildTroubleshooter handles build error analysis and fix suggestions
type BuildTroubleshooter struct {
	logger zerolog.Logger
}

// NewBuildTroubleshooter creates a new build troubleshooter
func NewBuildTroubleshooter(logger zerolog.Logger) *BuildTroubleshooter {
	return &BuildTroubleshooter{
		logger: logger.With().Str("component", "build_troubleshooter").Logger(),
	}
}

// AddPushTroubleshootingTips adds troubleshooting tips for push failures
func (t *BuildTroubleshooter) AddPushTroubleshootingTips(result *AtomicBuildImageResult, pushResult *coredocker.RegistryPushResult, registryURL string, err error) {
	if result.BuildContext_Info == nil {
		result.BuildContext_Info = &BuildContextInfo{}
	}

	tips := []string{}

	if err != nil {
		errStr := err.Error()

		if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "authentication") {
			tips = append(tips,
				"Authentication failed - check registry credentials",
				fmt.Sprintf("Run: docker login %s", registryURL),
				"Verify username/password or access token is correct",
			)
		}

		if strings.Contains(errStr, "denied") || strings.Contains(errStr, "forbidden") {
			tips = append(tips,
				"Access denied - check registry permissions",
				"Verify you have push access to the repository",
				"Check if repository exists and you're a collaborator",
			)
		}

		if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
			tips = append(tips,
				"Network issue detected",
				"Check internet connectivity and registry availability",
				"Consider retrying the push operation",
			)
		}

		if strings.Contains(errStr, "manifest") {
			tips = append(tips,
				"Manifest-related error",
				"Check if image tag already exists and is immutable",
				"Try using a different tag or version",
			)
		}
	}

	if len(tips) == 0 {
		tips = append(tips, "Push failed - check registry configuration and connectivity")
	}

	result.BuildContext_Info.NextStepSuggestions = append(result.BuildContext_Info.NextStepSuggestions, tips...)
}

// AddTroubleshootingTips adds general troubleshooting tips based on build errors
func (t *BuildTroubleshooter) AddTroubleshootingTips(result *AtomicBuildImageResult, err error) {
	if result.BuildContext_Info == nil {
		result.BuildContext_Info = &BuildContextInfo{}
	}

	tips := []string{}
	errStr := err.Error()

	// Dockerfile syntax errors
	if strings.Contains(errStr, "dockerfile parse error") || strings.Contains(errStr, "syntax error") {
		tips = append(tips,
			"Dockerfile syntax error detected",
			"Check Dockerfile for proper instruction format",
			"Validate all commands follow Docker syntax rules",
		)
	}

	// Base image issues
	if strings.Contains(errStr, "pull access denied") || strings.Contains(errStr, "repository does not exist") {
		tips = append(tips,
			"Base image not accessible",
			"Verify base image name and tag are correct",
			"Check if base image requires authentication",
		)
	}

	// Build context issues
	if strings.Contains(errStr, "no such file or directory") {
		tips = append(tips,
			"File not found in build context",
			"Verify all COPY/ADD paths exist relative to build context",
			"Check .dockerignore isn't excluding required files",
		)
	}

	// Permission issues
	if strings.Contains(errStr, "permission denied") {
		tips = append(tips,
			"Permission error encountered",
			"Check file permissions in build context",
			"Ensure Docker has access to required files",
		)
	}

	// Network issues
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
		tips = append(tips,
			"Network connectivity issue",
			"Check internet connection for package downloads",
			"Verify proxy settings if behind corporate firewall",
		)
	}

	// Space issues
	if strings.Contains(errStr, "no space left") || strings.Contains(errStr, "disk full") {
		tips = append(tips,
			"Insufficient disk space",
			"Clean up unused Docker images: docker system prune",
			"Check available disk space: df -h",
		)
	}

	if len(tips) == 0 {
		tips = append(tips, "Build failed - check Docker daemon logs for detailed error information")
	}

	result.BuildContext_Info.NextStepSuggestions = append(result.BuildContext_Info.NextStepSuggestions, tips...)
}

// GenerateBuildFailureAnalysis creates comprehensive failure analysis
func (t *BuildTroubleshooter) GenerateBuildFailureAnalysis(err error, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) *BuildFailureAnalysis {
	analysis := &BuildFailureAnalysis{}

	errStr := err.Error()
	analysis.FailureReason = errStr
	analysis.FailureType, analysis.FailureStage = t.classifyFailure(errStr, buildResult)

	// Convert causes to strings
	causes := t.identifyFailureCauses(errStr, buildResult, result)
	analysis.CommonCauses = make([]string, len(causes))
	for i, cause := range causes {
		analysis.CommonCauses[i] = cause.Description
	}

	// Convert fixes to strings
	fixes := t.generateSuggestedFixes(errStr, buildResult, result)
	analysis.SuggestedFixes = make([]string, len(fixes))
	for i, fix := range fixes {
		analysis.SuggestedFixes[i] = fix.Description
	}

	// Convert strategies to strings
	strategies := t.generateAlternativeStrategies(errStr, buildResult, result)
	analysis.AlternativeStrategies = make([]string, len(strategies))
	for i, strategy := range strategies {
		analysis.AlternativeStrategies[i] = strategy.Description
	}

	analysis.SecurityImplications = t.identifySecurityImplications(errStr, buildResult, result)

	if buildResult != nil {
		perfAnalysis := t.analyzePerformanceImpact(buildResult, result)
		analysis.PerformanceImpact = perfAnalysis.CacheEfficiency
	}

	return analysis
}

// classifyFailure categorizes the build failure type and severity
func (t *BuildTroubleshooter) classifyFailure(errStr string, buildResult *coredocker.BuildResult) (string, string) {
	errLower := strings.ToLower(errStr)

	// Determine failure type
	failureType := "unknown"
	if strings.Contains(errLower, "dockerfile") || strings.Contains(errLower, "syntax") {
		failureType = "dockerfile_syntax"
	} else if strings.Contains(errLower, "network") || strings.Contains(errLower, "timeout") || strings.Contains(errLower, "connection") {
		failureType = "network"
	} else if strings.Contains(errLower, "permission") || strings.Contains(errLower, "access denied") {
		failureType = "permissions"
	} else if strings.Contains(errLower, "space") || strings.Contains(errLower, "disk full") {
		failureType = "resources"
	} else if strings.Contains(errLower, "pull") || strings.Contains(errLower, "repository") {
		failureType = "image_access"
	} else if strings.Contains(errLower, "copy") || strings.Contains(errLower, "add") || strings.Contains(errLower, "no such file") {
		failureType = "file_operations"
	}

	// Determine severity
	severity := "medium"
	if strings.Contains(errLower, "critical") || strings.Contains(errLower, "fatal") {
		severity = "high"
	} else if strings.Contains(errLower, "warning") || strings.Contains(errLower, "deprecated") {
		severity = "low"
	}

	return failureType, severity
}

// identifyFailureCauses identifies potential root causes of the build failure
func (t *BuildTroubleshooter) identifyFailureCauses(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []FailureCause {
	causes := []FailureCause{}
	errLower := strings.ToLower(errStr)

	// Dockerfile-related causes
	if strings.Contains(errLower, "dockerfile") || strings.Contains(errLower, "syntax") {
		causes = append(causes, FailureCause{
			Type:        "dockerfile",
			Description: "Dockerfile contains syntax errors or invalid instructions",
			Likelihood:  "high",
			Evidence:    []string{"Syntax error in Dockerfile", "Invalid instruction format"},
		})
	}

	// Network-related causes
	if strings.Contains(errLower, "network") || strings.Contains(errLower, "timeout") {
		causes = append(causes, FailureCause{
			Type:        "network",
			Description: "Network connectivity issues preventing resource access",
			Likelihood:  "high",
			Evidence:    []string{"Network timeout", "Connection refused", "DNS resolution failure"},
		})
	}

	// Permission-related causes
	if strings.Contains(errLower, "permission") || strings.Contains(errLower, "access denied") {
		causes = append(causes, FailureCause{
			Type:        "permissions",
			Description: "Insufficient permissions to access required resources",
			Likelihood:  "high",
			Evidence:    []string{"Permission denied", "Access denied", "Unauthorized"},
		})
	}

	// Base image issues
	if strings.Contains(errLower, "pull") && strings.Contains(errLower, "denied") {
		causes = append(causes, FailureCause{
			Type:        "base_image",
			Description: "Base image is not accessible or does not exist",
			Likelihood:  "high",
			Evidence:    []string{"Pull access denied", "Repository does not exist"},
		})
	}

	// File operation issues
	if strings.Contains(errLower, "no such file") || strings.Contains(errLower, "copy") {
		causes = append(causes, FailureCause{
			Type:        "file_operations",
			Description: "Required files are missing from build context",
			Likelihood:  "high",
			Evidence:    []string{"File not found", "Copy operation failed"},
		})
	}

	// Resource constraints
	if strings.Contains(errLower, "space") || strings.Contains(errLower, "memory") {
		causes = append(causes, FailureCause{
			Type:        "resources",
			Description: "Insufficient system resources (disk space, memory)",
			Likelihood:  "medium",
			Evidence:    []string{"No space left", "Out of memory", "Resource exhausted"},
		})
	}

	return causes
}

// generateSuggestedFixes generates actionable fix suggestions
func (t *BuildTroubleshooter) generateSuggestedFixes(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildFix {
	fixes := []BuildFix{}
	errLower := strings.ToLower(errStr)

	// Dockerfile syntax fixes
	if strings.Contains(errLower, "dockerfile") || strings.Contains(errLower, "syntax") {
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Type:        "dockerfile",
			Description: "Fix Dockerfile syntax errors",
			Commands:    []string{"Review Dockerfile for syntax errors", "Validate instruction format", "Check for typos in commands"},
			Validation:  "Should resolve build failure immediately",
		})
	}

	// Network connectivity fixes
	if strings.Contains(errLower, "network") || strings.Contains(errLower, "timeout") {
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Type:        "network",
			Description: "Resolve network connectivity issues",
			Commands:    []string{"Check internet connection", "Verify DNS resolution", "Test registry connectivity"},
			Validation:  "Will enable resource downloads and base image pulls",
		})
	}

	// Permission fixes
	if strings.Contains(errLower, "permission") {
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Type:        "permissions",
			Description: "Fix file and directory permissions",
			Commands:    []string{"chmod +r required files", "Check Docker daemon permissions", "Verify user access to build context"},
			Validation:  "Will allow Docker to access required files",
		})
	}

	// Base image fixes
	if strings.Contains(errLower, "pull") && strings.Contains(errLower, "denied") {
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Type:        "base_image",
			Description: "Resolve base image access issues",
			Commands:    []string{"Verify base image name and tag", "Check registry authentication", "Use alternative base image"},
			Validation:  "Will allow successful base image download",
		})
	}

	// File operation fixes
	if strings.Contains(errLower, "no such file") {
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Type:        "file_operations",
			Description: "Ensure required files exist in build context",
			Commands:    []string{"Verify file paths in COPY/ADD instructions", "Check .dockerignore exclusions", "Confirm files exist relative to build context"},
			Validation:  "Will allow successful file operations in Dockerfile",
		})
	}

	// Resource constraint fixes
	if strings.Contains(errLower, "space") {
		fixes = append(fixes, BuildFix{
			Priority:    "medium",
			Type:        "resources",
			Description: "Free up disk space",
			Commands:    []string{"docker system prune -f", "rm -rf unnecessary files", "Increase available disk space"},
			Validation:  "Will provide sufficient space for build operations",
		})
	}

	return fixes
}

// generateAlternativeStrategies suggests alternative approaches
func (t *BuildTroubleshooter) generateAlternativeStrategies(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildStrategyRecommendation {
	strategies := []BuildStrategyRecommendation{}
	errLower := strings.ToLower(errStr)

	// Multi-stage build strategy
	if strings.Contains(errLower, "space") || strings.Contains(errLower, "size") {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "multi_stage_build",
			Description: "Use multi-stage builds to reduce final image size",
			Benefits:    []string{"Smaller final image", "Reduced layer count", "Better caching"},
			Complexity:  "medium",
			Example:     "Requires Dockerfile restructuring",
		})
	}

	// Alternative base image strategy
	if strings.Contains(errLower, "pull") || strings.Contains(errLower, "base") {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "alternative_base_image",
			Description: "Switch to a different base image",
			Benefits:    []string{"Avoid access issues", "Potentially smaller size", "Different package managers"},
			Complexity:  "low",
			Example:     "Update FROM instruction in Dockerfile",
		})
	}

	// Buildkit strategy
	if strings.Contains(errLower, "cache") || strings.Contains(errLower, "performance") {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "buildkit",
			Description: "Enable Docker BuildKit for improved builds",
			Benefits:    []string{"Better caching", "Parallel builds", "Advanced features"},
			Complexity:  "low",
			Example:     "Set DOCKER_BUILDKIT=1 environment variable",
		})
	}

	return strategies
}

// analyzePerformanceImpact analyzes how the failure impacts build performance
func (t *BuildTroubleshooter) analyzePerformanceImpact(buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) PerformanceAnalysis {
	analysis := PerformanceAnalysis{
		BuildTime:       buildResult.Duration,
		CacheEfficiency: "unknown",
		CacheHitRate:    0.0,
		ImageSize:       "unknown",
		Optimizations:   []string{},
		Bottlenecks:     []string{},
	}

	// Set default cache efficiency since Steps field is not available
	analysis.CacheEfficiency = "unknown"

	// Add optimization recommendations
	if analysis.CacheEfficiency == "poor" {
		analysis.Optimizations = []string{
			"Reorder Dockerfile instructions to improve layer caching",
			"Move frequently changing instructions (like COPY source code) to the end",
			"Use .dockerignore to reduce build context size",
		}
	}

	if result.BuildContext_Info != nil && result.BuildContext_Info.ContextSize > 100*1024*1024 {
		analysis.Optimizations = append(analysis.Optimizations,
			"Large build context detected - consider reducing context size")
	}

	return analysis
}

// identifySecurityImplications analyzes security aspects of the build failure
func (t *BuildTroubleshooter) identifySecurityImplications(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []string {
	implications := []string{}

	// Permission-related security implications
	if strings.Contains(errStr, "permission") {
		implications = append(implications,
			"Permission errors may indicate overly restrictive or permissive file access",
			"Review file ownership and ensure principle of least privilege")
	}

	// Network-related security implications
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "download") {
		implications = append(implications,
			"Network failures during build may expose dependencies on external resources",
			"Consider vendoring dependencies to reduce supply chain risks")
	}

	// Base image security implications
	if result.BuildContext_Info != nil {
		baseImage := strings.ToLower(result.BuildContext_Info.BaseImage)
		if strings.Contains(baseImage, "latest") {
			implications = append(implications,
				"Using 'latest' tag creates unpredictable builds and potential security vulnerabilities",
				"Pin to specific image versions for reproducible and secure builds")
		}

		if strings.Contains(baseImage, "ubuntu") || strings.Contains(baseImage, "centos") {
			implications = append(implications,
				"Full OS base images have larger attack surface",
				"Consider minimal base images like alpine or distroless")
		}

		// Context-specific implications
		if !result.BuildContext_Info.HasDockerIgnore {
			implications = append(implications,
				"Missing .dockerignore may include sensitive files in image layers",
				"Create .dockerignore to prevent accidental inclusion of secrets")
		}

		if len(result.BuildContext_Info.LargeFilesFound) > 0 {
			implications = append(implications,
				"Large files in build context may contain sensitive data",
				"Review and exclude unnecessary large files from image")
		}
	}

	return implications
}
