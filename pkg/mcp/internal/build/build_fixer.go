package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
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

// AtomicDockerBuildOperation implements FixableOperation for Docker builds
type AtomicDockerBuildOperation struct {
	tool           *AtomicBuildImageTool
	args           AtomicBuildImageArgs
	session        *sessiontypes.SessionState
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
		return &types.RichError{
			Code:     "DOCKERFILE_NOT_FOUND",
			Type:     "dockerfile_error",
			Severity: "High",
			Message:  fmt.Sprintf("Dockerfile not found at %s", op.dockerfilePath),
			Context: types.ErrorContext{
				Operation: "docker_build",
				Stage:     "pre_build_validation",
				Component: "dockerfile",
				Metadata: types.NewErrorMetadata("", "build_image", "dockerfile_validation").
					WithBuildContext(&types.BuildMetadata{
						DockerfilePath:   op.dockerfilePath,
						BuildContextPath: op.buildContext,
					}),
			},
		}
	}

	// Get full image reference
	imageTag := op.tool.getImageTag(op.args.ImageTag)
	fullImageRef := fmt.Sprintf("%s:%s", op.args.ImageName, imageTag)

	// Execute the Docker build via pipeline adapter
	buildResult, err := op.tool.pipelineAdapter.BuildDockerImage(
		op.session.SessionID,
		fullImageRef,
		op.dockerfilePath,
	)

	if err != nil {
		op.logger.Warn().Err(err).Msg("Docker build failed")
		return err
	}

	if buildResult == nil || !buildResult.Success {
		errorMsg := "unknown error"
		if buildResult != nil && buildResult.Error != nil {
			errorMsg = buildResult.Error.Message
		}
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("docker build failed: %s", errorMsg), "build_error")
	}

	op.logger.Info().
		Str("image_name", fullImageRef).
		Msg("Docker build completed successfully")

	return nil
}

// GetFailureAnalysis analyzes why the Docker build failed
func (op *AtomicDockerBuildOperation) GetFailureAnalysis(ctx context.Context, err error) (*types.RichError, error) {
	op.logger.Debug().Err(err).Msg("Analyzing Docker build failure")

	// If it's already a RichError, return it
	if richErr, ok := err.(*types.RichError); ok {
		return richErr, nil
	}

	// Analyze the error message to categorize the failure
	errorMsg := err.Error()

	if strings.Contains(errorMsg, "no such file or directory") {
		return &types.RichError{
			Code:     "FILE_NOT_FOUND",
			Type:     "dockerfile_error",
			Severity: "High",
			Message:  errorMsg,
			Context: types.ErrorContext{
				Operation: "docker_build",
				Stage:     "file_access",
				Component: "dockerfile",
				Metadata: types.NewErrorMetadata("", "build_image", "file_access").
					WithBuildContext(&types.BuildMetadata{
						DockerfilePath:   op.dockerfilePath,
						BuildContextPath: op.buildContext,
					}).
					AddCustom("suggested_fix", "Check file paths in Dockerfile"),
			},
		}, nil
	}

	if strings.Contains(errorMsg, "unable to find image") {
		return &types.RichError{
			Code:     "BASE_IMAGE_NOT_FOUND",
			Type:     "dependency_error",
			Severity: "High",
			Message:  errorMsg,
			Context: types.ErrorContext{
				Operation: "docker_build",
				Stage:     "base_image",
				Component: "dockerfile",
				Metadata: types.NewErrorMetadata("", "build_image", "base_image").
					AddCustom("suggested_fix", "Update base image tag or use a different base image"),
			},
		}, nil
	}

	if strings.Contains(errorMsg, "package not found") || strings.Contains(errorMsg, "command not found") {
		return &types.RichError{
			Code:     "PACKAGE_INSTALL_FAILED",
			Type:     "dependency_error",
			Severity: "Medium",
			Message:  errorMsg,
			Context: types.ErrorContext{
				Operation: "docker_build",
				Stage:     "package_install",
				Component: "package_manager",
				Metadata: types.NewErrorMetadata("", "build_image", "package_install").
					AddCustom("suggested_fix", "Update package names or installation commands"),
			},
		}, nil
	}

	// Default categorization
	return &types.RichError{
		Code:     "BUILD_FAILED",
		Type:     "build_error",
		Severity: "High",
		Message:  errorMsg,
		Context: types.ErrorContext{
			Operation: "docker_build",
			Stage:     "build_execution",
			Component: "docker",
		},
	}, nil
}

// PrepareForRetry applies fixes and prepares for the next build attempt
func (op *AtomicDockerBuildOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().
		Str("fix_strategy", fixAttempt.FixStrategy.Name).
		Msg("Preparing for retry after fix")

	// Apply fix based on the strategy type
	switch fixAttempt.FixStrategy.Type {
	case "dockerfile":
		return op.applyDockerfileFix(ctx, fixAttempt)
	case "dependency":
		return op.applyDependencyFix(ctx, fixAttempt)
	case "config":
		return op.applyConfigFix(ctx, fixAttempt)
	default:
		op.logger.Warn().
			Str("fix_type", fixAttempt.FixStrategy.Type).
			Msg("Unknown fix type, applying generic fix")
		return op.applyGenericFix(ctx, fixAttempt)
	}
}

// applyDockerfileFix applies fixes to the Dockerfile
func (op *AtomicDockerBuildOperation) applyDockerfileFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if fixAttempt.FixedContent == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "no fixed Dockerfile content provided", "missing_content")
	}

	// Backup the original Dockerfile
	backupPath := op.dockerfilePath + ".backup"
	if err := op.backupFile(op.dockerfilePath, backupPath); err != nil {
		op.logger.Warn().Err(err).Msg("Failed to backup Dockerfile")
	}

	// Write the fixed Dockerfile
	err := os.WriteFile(op.dockerfilePath, []byte(fixAttempt.FixedContent), 0644)
	if err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to write fixed Dockerfile: %v", err), "file_error")
	}

	op.logger.Info().
		Str("dockerfile_path", op.dockerfilePath).
		Msg("Applied Dockerfile fix")

	return nil
}

// applyDependencyFix applies dependency-related fixes
func (op *AtomicDockerBuildOperation) applyDependencyFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().Msg("Applying dependency fix")

	// Apply file changes specified in the fix strategy
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply dependency fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied dependency file change")
	}

	// Execute any commands specified in the fix strategy
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Dependency fix command would be executed")
		// Note: Command execution could be implemented here if needed
		// Currently focusing on file-based fixes which are more common
	}

	return nil
}

// applyConfigFix applies configuration-related fixes
func (op *AtomicDockerBuildOperation) applyConfigFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().Msg("Applying configuration fix")

	// Apply file changes specified in the fix strategy
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply config fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied configuration file change")
	}

	// Execute any commands specified in the fix strategy
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Configuration fix command would be executed")
		// Note: Command execution could be implemented here if needed
		// Currently focusing on file-based fixes which are more common
	}

	return nil
}

// applyGenericFix applies generic fixes
func (op *AtomicDockerBuildOperation) applyGenericFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().Msg("Applying generic fix")

	// If there's fixed content, treat it as a Dockerfile fix
	if fixAttempt.FixedContent != "" {
		return op.applyDockerfileFix(ctx, fixAttempt)
	}

	// Apply file changes specified in the fix strategy
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply generic fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied generic file change")
	}

	// Execute any commands specified in the fix strategy
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Generic fix command would be executed")
		// Note: Command execution could be implemented here if needed
		// Currently focusing on file-based fixes which are more common
	}

	// If no file changes or commands, this might be a no-op fix
	if len(fixAttempt.FixStrategy.FileChanges) == 0 && len(fixAttempt.FixStrategy.Commands) == 0 {
		op.logger.Info().Msg("Generic fix completed (no specific changes needed)")
	}

	return nil
}

// applyFileChange applies a single file change from a fix strategy
func (op *AtomicDockerBuildOperation) applyFileChange(change mcptypes.FileChange) error {
	// Ensure the directory exists for the target file
	dir := filepath.Dir(change.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create directory %s: %v", dir, err), "filesystem_error")
	}

	switch change.Operation {
	case "create":
		// Create a new file
		err := os.WriteFile(change.FilePath, []byte(change.NewContent), 0644)
		if err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create file: %v", err), "file_error")
		}

	case "update":
		// Backup the original file if it exists
		if _, err := os.Stat(change.FilePath); err == nil {
			backupPath := change.FilePath + ".backup"
			if err := op.backupFile(change.FilePath, backupPath); err != nil {
				op.logger.Warn().Err(err).Str("file", change.FilePath).Msg("Failed to backup file")
			}
		}

		// Write the updated content
		err := os.WriteFile(change.FilePath, []byte(change.NewContent), 0644)
		if err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to update file: %v", err), "file_error")
		}

	case "delete":
		// Delete the file
		if err := os.Remove(change.FilePath); err != nil && !os.IsNotExist(err) {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to delete file: %v", err), "file_error")
		}

	default:
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("unsupported file operation: %s", change.Operation), "invalid_operation")
	}

	return nil
}

// backupFile creates a backup of a file
func (op *AtomicDockerBuildOperation) backupFile(source, backup string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(backup, data, 0644)
}
