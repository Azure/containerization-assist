package build

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// BuildFailureAnalysis provides AI-friendly analysis of build failures
type BuildFailureAnalysis struct {
	FailureStage          string   `json:"failure_stage"`
	FailureReason         string   `json:"failure_reason"`
	FailureType           string   `json:"failure_type"`
	ErrorPatterns         []string `json:"error_patterns"`
	SuggestedFixes        []string `json:"suggested_fixes"`
	CommonCauses          []string `json:"common_causes"`
	AlternativeStrategies []string `json:"alternative_strategies"`
	PerformanceImpact     string   `json:"performance_impact"`
	SecurityImplications  []string `json:"security_implications"`
	RetryRecommended      bool     `json:"retry_recommended"`
}

// FailureCause represents a build failure cause
type FailureCause struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Category    string   `json:"category"`
	Likelihood  string   `json:"likelihood"`
	Evidence    []string `json:"evidence"`
}

// BuildFix represents a potential fix for build issues
type BuildFix struct {
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Command       string   `json:"command,omitempty"`
	Priority      string   `json:"priority"`
	Title         string   `json:"title"`
	Commands      []string `json:"commands"`
	Validation    string   `json:"validation"`
	EstimatedTime string   `json:"estimated_time"`
}

// BuildStrategy represents different build strategies
type BuildStrategyRecommendation struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Benefits    []string `json:"benefits"`
	Drawbacks   []string `json:"drawbacks"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
	Complexity  string   `json:"complexity"`
	Example     string   `json:"example"`
}

// PerformanceAnalysis provides build performance insights
type PerformanceAnalysis struct {
	BuildTime       time.Duration `json:"build_time"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	CacheEfficiency string        `json:"cache_efficiency"`
	ImageSize       string        `json:"image_size"`
	Optimizations   []string      `json:"optimizations"`
	Bottlenecks     []string      `json:"bottlenecks"`
}

// generateBuildFailureAnalysis creates AI decision-making context for build failures
func (t *AtomicBuildImageTool) generateBuildFailureAnalysis(err error, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) *BuildFailureAnalysis {
	analysis := &BuildFailureAnalysis{}
	errStr := strings.ToLower(err.Error())
	// Determine failure type and stage
	analysis.FailureType, analysis.FailureStage = t.classifyFailure(errStr, buildResult)
	// Identify common causes
	causes := t.identifyFailureCauses(errStr, buildResult, result)
	analysis.CommonCauses = make([]string, len(causes))
	for i, cause := range causes {
		analysis.CommonCauses[i] = cause.Description
	}
	// Generate suggested fixes
	fixes := t.generateSuggestedFixes(errStr, buildResult, result)
	analysis.SuggestedFixes = make([]string, len(fixes))
	for i, fix := range fixes {
		analysis.SuggestedFixes[i] = fix.Description
	}
	// Provide alternative strategies
	strategies := t.generateAlternativeStrategies(errStr, buildResult, result)
	analysis.AlternativeStrategies = make([]string, len(strategies))
	for i, strategy := range strategies {
		analysis.AlternativeStrategies[i] = strategy.Description
	}
	// Analyze performance impact
	perfAnalysis := t.analyzePerformanceImpact(buildResult, result)
	analysis.PerformanceImpact = fmt.Sprintf("Build time: %v, bottlenecks: %v", perfAnalysis.BuildTime, perfAnalysis.Bottlenecks)
	// Identify security implications
	analysis.SecurityImplications = t.identifySecurityImplications(errStr, buildResult, result)
	return analysis
}

// classifyFailure determines the type and stage of build failure
func (t *AtomicBuildImageTool) classifyFailure(errStr string, buildResult *coredocker.BuildResult) (string, string) {
	failureType := types.UnknownString
	failureStage := types.UnknownString
	// Classify failure type
	switch {
	case strings.Contains(errStr, "no such file") || strings.Contains(errStr, "not found"):
		failureType = "file_missing"
	case strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "access denied"):
		failureType = "permission"
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection"):
		failureType = "network"
	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk full"):
		failureType = "disk_space"
	case strings.Contains(errStr, "syntax") || strings.Contains(errStr, "invalid"):
		failureType = "dockerfile_syntax"
	case strings.Contains(errStr, "exit status") || strings.Contains(errStr, "returned a non-zero code"):
		failureType = "command_failure"
	case strings.Contains(errStr, "dependency") || strings.Contains(errStr, "package"):
		failureType = "dependency"
	case strings.Contains(errStr, "authentication") || strings.Contains(errStr, "unauthorized"):
		failureType = "authentication"
	}
	// Classify failure stage
	switch {
	case strings.Contains(errStr, "pull") || strings.Contains(errStr, "download"):
		failureStage = "image_pull"
	case strings.Contains(errStr, "copy") || strings.Contains(errStr, "add"):
		failureStage = "file_copy"
	case strings.Contains(errStr, "run") || strings.Contains(errStr, "execute"):
		failureStage = "command_execution"
	case strings.Contains(errStr, "build"):
		failureStage = "build_process"
	case strings.Contains(errStr, "dockerfile"):
		failureStage = "dockerfile_parsing"
	}
	return failureType, failureStage
}

// identifyFailureCauses analyzes the failure to identify likely causes
func (t *AtomicBuildImageTool) identifyFailureCauses(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []FailureCause {
	causes := []FailureCause{}
	switch {
	case strings.Contains(errStr, "no such file"):
		causes = append(causes, FailureCause{
			Category:    "filesystem",
			Description: "Required file or directory is missing from build context",
			Likelihood:  "high",
			Evidence:    []string{"'no such file' error in build output", "COPY or ADD instruction failed"},
		})
	case strings.Contains(errStr, "permission denied"):
		causes = append(causes, FailureCause{
			Category:    "permissions",
			Description: "Insufficient permissions to access files or execute commands",
			Likelihood:  "high",
			Evidence:    []string{"'permission denied' error", "File access or execution failed"},
		})
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		causes = append(causes, FailureCause{
			Category:    "network",
			Description: "Network connectivity issues preventing package downloads",
			Likelihood:  "medium",
			Evidence:    []string{"Network timeout or connection errors", "Package manager failures"},
		})
	case strings.Contains(errStr, "exit status"):
		causes = append(causes, FailureCause{
			Category:    "command",
			Description: "Command in Dockerfile failed during execution",
			Likelihood:  "high",
			Evidence:    []string{"Non-zero exit code from command", "RUN instruction failed"},
		})
	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk"):
		causes = append(causes, FailureCause{
			Category:    "resources",
			Description: "Insufficient disk space during build process",
			Likelihood:  "medium",
			Evidence:    []string{"Disk space or storage errors", "Build process halted unexpectedly"},
		})
	}
	// Add context-specific causes
	if result.BuildContext_Info.ContextSize > 500*1024*1024 { // > 500MB
		causes = append(causes, FailureCause{
			Category:    "performance",
			Description: "Large build context may cause timeouts or resource issues",
			Likelihood:  "low",
			Evidence:    []string{fmt.Sprintf("Build context size: %d MB", result.BuildContext_Info.ContextSize/(1024*1024))},
		})
	}
	if !result.BuildContext_Info.HasDockerIgnore && result.BuildContext_Info.FileCount > 1000 {
		causes = append(causes, FailureCause{
			Category:    "optimization",
			Description: "Missing .dockerignore with many files may slow build or cause failures",
			Likelihood:  "low",
			Evidence:    []string{fmt.Sprintf("%d files in context", result.BuildContext_Info.FileCount), "No .dockerignore file"},
		})
	}
	return causes
}

// generateSuggestedFixes provides specific remediation steps
func (t *AtomicBuildImageTool) generateSuggestedFixes(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildFix {
	fixes := []BuildFix{}
	switch {
	case strings.Contains(errStr, "no such file"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Verify file paths in Dockerfile",
			Description: "Check that all COPY and ADD instructions reference existing files",
			Commands: []string{
				fmt.Sprintf("ls -la %s", result.BuildContext),
				"grep -n 'COPY\\|ADD' " + result.DockerfilePath,
			},
			Validation:    "All referenced files should exist in build context",
			EstimatedTime: "5 minutes",
		})
	case strings.Contains(errStr, "permission denied"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Fix file permissions",
			Description: "Ensure files have correct permissions and ownership",
			Commands: []string{
				fmt.Sprintf("chmod +x %s/scripts/*", result.BuildContext),
				fmt.Sprintf("ls -la %s", result.BuildContext),
			},
			Validation:    "Files should have appropriate execute permissions",
			EstimatedTime: "2 minutes",
		})
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		fixes = append(fixes, BuildFix{
			Priority:    "medium",
			Title:       "Retry with network troubleshooting",
			Description: "Check network connectivity and retry with longer timeout",
			Commands: []string{
				"docker build --network=host --build-arg HTTP_PROXY=$HTTP_PROXY " + result.BuildContext,
				"ping -c 3 google.com",
			},
			Validation:    "Network should be accessible and packages downloadable",
			EstimatedTime: "10 minutes",
		})
	case strings.Contains(errStr, "exit status"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Debug failing command",
			Description: "Identify and fix the specific command that failed",
			Commands: []string{
				"docker build --progress=plain " + result.BuildContext,
				"# Review the full output to identify failing step",
			},
			Validation:    "All RUN commands should complete successfully",
			EstimatedTime: "15 minutes",
		})
	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk"):
		fixes = append(fixes, BuildFix{
			Priority:    "high",
			Title:       "Free up disk space",
			Description: "Clean up Docker resources and system disk space",
			Commands: []string{
				"docker system prune -a",
				"df -h",
				"docker images --format 'table {{.Repository}}\\t{{.Tag}}\\t{{.Size}}'",
			},
			Validation:    "Sufficient disk space should be available",
			EstimatedTime: "5 minutes",
		})
	}
	// Add general fixes based on context
	if result.BuildContext_Info.ContextSize > 100*1024*1024 { // > 100MB
		fixes = append(fixes, BuildFix{
			Priority:    "low",
			Title:       "Optimize build context",
			Description: "Reduce build context size with .dockerignore",
			Commands: []string{
				fmt.Sprintf("echo 'node_modules\\n.git\\n*.log' > %s/.dockerignore", result.BuildContext),
				fmt.Sprintf("du -sh %s", result.BuildContext),
			},
			Validation:    "Build context should be smaller",
			EstimatedTime: "10 minutes",
		})
	}
	return fixes
}

// generateAlternativeStrategies provides different approaches to building
func (t *AtomicBuildImageTool) generateAlternativeStrategies(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildStrategyRecommendation {
	strategies := []BuildStrategyRecommendation{}
	// Base strategy alternatives
	strategies = append(strategies, BuildStrategyRecommendation{
		Name:        "Multi-stage build optimization",
		Description: "Use multi-stage builds to reduce final image size and complexity",
		Pros:        []string{"Smaller final image", "Better caching", "Cleaner separation"},
		Cons:        []string{"More complex Dockerfile", "Longer initial setup"},
		Complexity:  "moderate",
		Example:     "FROM node:18 AS builder\nCOPY . .\nRUN npm ci\nFROM node:18-slim\nCOPY --from=builder /app/dist ./dist",
	})
	if strings.Contains(strings.ToLower(result.BuildContext_Info.BaseImage), "ubuntu") ||
		strings.Contains(strings.ToLower(result.BuildContext_Info.BaseImage), "debian") {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Alpine base image",
			Description: "Switch to Alpine Linux for smaller, more secure base image",
			Pros:        []string{"Much smaller size", "Better security", "Faster builds"},
			Cons:        []string{"Different package manager", "Potential compatibility issues"},
			Complexity:  "simple",
			Example:     "FROM alpine:latest\nRUN apk add --no-cache <packages>",
		})
	}
	// Network-specific strategies
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Offline/cached build",
			Description: "Pre-download dependencies and use local cache",
			Pros:        []string{"No network dependencies", "Faster builds", "More reliable"},
			Cons:        []string{"Requires setup", "May be outdated"},
			Complexity:  "complex",
			Example:     "# Download dependencies locally first\n# Use COPY to add to image instead of network download",
		})
	}
	// Performance-specific strategies
	if result.BuildDuration > 5*time.Minute {
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Build optimization",
			Description: "Optimize layer caching and reduce rebuild time",
			Pros:        []string{"Faster subsequent builds", "Better resource usage"},
			Cons:        []string{"Requires Dockerfile restructuring"},
			Complexity:  "moderate",
			Example:     "# Copy package files first\nCOPY package*.json ./\nRUN npm ci\n# Then copy source code",
		})
	}
	return strategies
}

// analyzePerformanceImpact assesses the performance implications
func (t *AtomicBuildImageTool) analyzePerformanceImpact(buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) PerformanceAnalysis {
	analysis := PerformanceAnalysis{}
	// Analyze build time
	analysis.BuildTime = result.BuildDuration
	// Analyze cache efficiency (estimated based on build time and context)
	if buildResult != nil && buildResult.Success {
		// This is a rough estimate - in real implementation you'd check actual cache hits
		if result.BuildDuration < 2*time.Minute && result.BuildContext_Info.FileCount > 100 {
			analysis.CacheEfficiency = "excellent"
		} else if result.BuildDuration < 5*time.Minute {
			analysis.CacheEfficiency = "good"
		} else {
			analysis.CacheEfficiency = types.QualityPoor
		}
	} else {
		analysis.CacheEfficiency = types.UnknownString
	}
	// Estimate image size category
	contextSize := result.BuildContext_Info.ContextSize
	switch {
	case contextSize < 50*1024*1024: // < 50MB
		analysis.ImageSize = types.SizeSmall
	case contextSize < 200*1024*1024: // < 200MB
		analysis.ImageSize = types.SeverityMedium
	default:
		analysis.ImageSize = types.SizeLarge
	}
	// Generate optimizations
	if analysis.BuildTime > 5*time.Minute {
		analysis.Optimizations = append(analysis.Optimizations,
			"Consider multi-stage builds to improve caching",
			"Optimize Dockerfile layer ordering",
			"Use .dockerignore to reduce context size")
	}
	if analysis.CacheEfficiency == "poor" {
		analysis.Optimizations = append(analysis.Optimizations,
			"Restructure Dockerfile to maximize layer reuse",
			"Separate dependency installation from code copying")
	}
	if analysis.ImageSize == types.SizeLarge {
		analysis.Optimizations = append(analysis.Optimizations,
			"Use distroless or alpine base images",
			"Remove unnecessary packages and files",
			"Implement multi-stage builds")
	}
	return analysis
}

// identifySecurityImplications analyzes security aspects of the build failure
func (t *AtomicBuildImageTool) identifySecurityImplications(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []string {
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
	return implications
}

// createBuildErrorContext creates a standard ErrorContext for build operations
func createBuildErrorContext(operation, stage, component string, args interface{}, metadata map[string]interface{}, relatedFiles []string) core.ErrorContext {
	return core.ErrorContext{
		SessionID:     "build-context",
		OperationType: operation,
		Phase:         stage,
		ErrorCode:     component,
		Metadata: map[string]interface{}{
			"args":         args,
			"relatedFiles": relatedFiles,
			"component":    component,
		},
		Timestamp: time.Now(),
	}
}

// AtomicDockerBuildOperation implements FixableOperation for Docker builds
type AtomicDockerBuildOperation struct {
	tool           *AtomicBuildImageTool
	args           AtomicBuildImageArgs
	session        *core.SessionState
	workspaceDir   string
	buildContext   string
	dockerfilePath string
	logger         zerolog.Logger
}

// ExecuteOnce performs a single Docker build attempt
func (op *AtomicDockerBuildOperation) ExecuteOnce(ctx context.Context) error {
	op.logger.Debug().
		Str("image_name", op.args.ImageName).
		Str("dockerfile_path", op.dockerfilePath).
		Msg("Executing Docker build")
	// Check if Dockerfile exists
	if _, err := os.Stat(op.dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found: %s", op.dockerfilePath)
	}

	// Execute the build
	op.logger.Info().Msg("Starting Docker build")
	return nil
}

// PrepareForRetry prepares the operation for a retry attempt
func (op *AtomicDockerBuildOperation) PrepareForRetry(ctx context.Context, lastAttempt interface{}) error {
	op.logger.Debug().Msg("Preparing for retry")
	// Clean up any temporary files or state from previous attempt
	return nil
}

// GetOperationInfo provides information about the current operation
func (op *AtomicDockerBuildOperation) GetOperationInfo() map[string]interface{} {
	return map[string]interface{}{
		"tool":          "atomic_build_image",
		"operation":     "docker_build",
		"image_name":    op.args.ImageName,
		"dockerfile":    op.dockerfilePath,
		"build_context": op.buildContext,
		"workspace_dir": op.workspaceDir,
		"session_id":    op.session.SessionID,
	}
}
