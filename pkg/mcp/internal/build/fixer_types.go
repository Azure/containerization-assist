package build

import (
	"fmt"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// BuildFixerError represents a structured build error
type BuildFixerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Stage   string `json:"stage"`
	Type    string `json:"type"`
}

func (e *BuildFixerError) Error() string {
	return fmt.Sprintf("[%s] %s (stage: %s, type: %s)", e.Code, e.Message, e.Stage, e.Type)
}

// BuildFixerOptions contains build configuration options
type BuildFixerOptions struct {
	NetworkTimeout    int           `json:"network_timeout"`
	NetworkRetries    int           `json:"network_retries"`
	NetworkRetryDelay time.Duration `json:"network_retry_delay"`
	ForceRootUser     bool          `json:"force_root_user"`
	NoCache           bool          `json:"no_cache"`
	ForceRM           bool          `json:"force_rm"`
	Squash            bool          `json:"squash"`
}

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

// BuildStrategyRecommendation represents different build strategies
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

// BuildFixerPerformanceAnalysis provides build performance insights for the fixer
type BuildFixerPerformanceAnalysis struct {
	BuildTime       time.Duration `json:"build_time"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	CacheEfficiency string        `json:"cache_efficiency"`
	ImageSize       string        `json:"image_size"`
	Optimizations   []string      `json:"optimizations"`
	Bottlenecks     []string      `json:"bottlenecks"`
}

// BuildRecoveryStrategy defines the interface for build recovery strategies
type BuildRecoveryStrategy interface {
	GetName() string
	GetDescription() string
	CanHandle(err error, buildResult *coredocker.BuildResult) bool
	AttemptRecovery(err error, buildResult *coredocker.BuildResult, options BuildFixerOptions) (*BuildFix, error)
	GetPriority() int
	GetCategory() string
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
			Evidence:    []string{"Non-zero exit status", "Command execution error"},
		})
	case strings.Contains(errStr, "disk") || strings.Contains(errStr, "space"):
		causes = append(causes, FailureCause{
			Category:    "resources",
			Description: "Insufficient disk space or storage quota exceeded",
			Likelihood:  "medium",
			Evidence:    []string{"Disk space error", "Storage quota exceeded"},
		})
	}

	return causes
}

// generateSuggestedFixes provides actionable fixes for build failures
func (t *AtomicBuildImageTool) generateSuggestedFixes(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildFix {
	fixes := []BuildFix{}

	switch {
	case strings.Contains(errStr, "no such file"):
		fixes = append(fixes, BuildFix{
			Type:          "file_fix",
			Title:         "Fix missing file",
			Description:   "Ensure all required files are in the build context",
			Commands:      []string{"ls -la", "find . -name '*.txt'"},
			Priority:      "high",
			EstimatedTime: "5 minutes",
		})
	case strings.Contains(errStr, "permission denied"):
		fixes = append(fixes, BuildFix{
			Type:          "permission_fix",
			Title:         "Fix permission issues",
			Description:   "Update file permissions or run as root user",
			Commands:      []string{"chmod +x script.sh", "USER root"},
			Priority:      "medium",
			EstimatedTime: "2 minutes",
		})
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		fixes = append(fixes, BuildFix{
			Type:          "network_fix",
			Title:         "Fix network connectivity",
			Description:   "Retry with network configuration or use alternative mirrors",
			Commands:      []string{"apt-get update --allow-releaseinfo-change", "pip install --index-url https://pypi.org/simple/"},
			Priority:      "medium",
			EstimatedTime: "10 minutes",
		})
	case strings.Contains(errStr, "disk") || strings.Contains(errStr, "space"):
		fixes = append(fixes, BuildFix{
			Type:          "space_fix",
			Title:         "Free up disk space",
			Description:   "Clean up build cache and temporary files",
			Commands:      []string{"docker system prune -f", "rm -rf /tmp/*"},
			Priority:      "high",
			EstimatedTime: "3 minutes",
		})
	}

	return fixes
}

// generateAlternativeStrategies provides alternative build approaches
func (t *AtomicBuildImageTool) generateAlternativeStrategies(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []BuildStrategyRecommendation {
	strategies := []BuildStrategyRecommendation{}

	switch {
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Multi-stage build with caching",
			Description: "Use multi-stage builds to cache dependencies separately",
			Benefits:    []string{"Better caching", "Reduced network dependency", "Faster rebuilds"},
			Drawbacks:   []string{"More complex Dockerfile", "Initial setup time"},
			Complexity:  "medium",
			Example:     "FROM node:16 AS deps\nCOPY package*.json ./\nRUN npm ci --only=production",
		})
	case strings.Contains(errStr, "permission"):
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Rootless build strategy",
			Description: "Configure build to work without root privileges",
			Benefits:    []string{"Better security", "Consistent permissions", "Reduced attack surface"},
			Drawbacks:   []string{"Setup complexity", "Some tools may not work"},
			Complexity:  "high",
			Example:     "USER 1001:1001\nWORKDIR /app\nCOPY --chown=1001:1001 . .",
		})
	case strings.Contains(errStr, "disk") || strings.Contains(errStr, "space"):
		strategies = append(strategies, BuildStrategyRecommendation{
			Name:        "Optimized image size strategy",
			Description: "Use alpine images and multi-stage builds to minimize size",
			Benefits:    []string{"Smaller images", "Less disk usage", "Faster transfers"},
			Drawbacks:   []string{"May need different packages", "Compatibility issues"},
			Complexity:  "medium",
			Example:     "FROM alpine:3.18\nRUN apk add --no-cache python3",
		})
	}

	return strategies
}

// analyzePerformanceImpact analyzes the performance implications of build failures
func (t *AtomicBuildImageTool) analyzePerformanceImpact(buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) *BuildFixerPerformanceAnalysis {
	analysis := &BuildFixerPerformanceAnalysis{
		BuildTime:       5 * time.Minute, // Default estimate
		CacheHitRate:    0.5,             // Default estimate
		CacheEfficiency: "medium",
		ImageSize:       "unknown",
		Optimizations:   []string{},
		Bottlenecks:     []string{},
	}

	if buildResult != nil {
		analysis.BuildTime = buildResult.Duration
		if buildResult.Duration > 10*time.Minute {
			analysis.Bottlenecks = append(analysis.Bottlenecks, "Long build time detected")
		}
		// Note: ImageSize field not available in current BuildResult struct
		// if buildResult.ImageSize > 1000*1024*1024 { // > 1GB
		//	analysis.Bottlenecks = append(analysis.Bottlenecks, "Large image size")
		//	analysis.Optimizations = append(analysis.Optimizations, "Consider multi-stage builds")
		// }
	}

	return analysis
}

// identifySecurityImplications identifies potential security issues in build failures
func (t *AtomicBuildImageTool) identifySecurityImplications(errStr string, buildResult *coredocker.BuildResult, result *AtomicBuildImageResult) []string {
	implications := []string{}

	switch {
	case strings.Contains(errStr, "permission"):
		implications = append(implications, "Running as root may introduce security risks")
		implications = append(implications, "Consider using least privilege principle")
	case strings.Contains(errStr, "network"):
		implications = append(implications, "Network failures might expose build to insecure fallbacks")
		implications = append(implications, "Ensure package sources are verified and trusted")
	case strings.Contains(errStr, "authentication"):
		implications = append(implications, "Authentication failures may indicate credential exposure")
		implications = append(implications, "Review credential management practices")
	}

	return implications
}

// createBuildErrorContextForAnalysis creates context for build errors to aid in debugging
func createBuildErrorContextForAnalysis(err error, buildResult *coredocker.BuildResult, dockerfilePath string) map[string]interface{} {
	context := map[string]interface{}{
		"error_message":   err.Error(),
		"dockerfile_path": dockerfilePath,
		"timestamp":       time.Now().UTC(),
	}

	if buildResult != nil {
		context["build_duration"] = buildResult.Duration
		// Note: ImageSize field not available in current BuildResult struct
		// context["image_size"] = buildResult.ImageSize
		context["logs"] = buildResult.Logs
	}

	return context
}
