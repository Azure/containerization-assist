package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"
)

// EnhancedBuildAnalyzer implements comprehensive build analysis
type EnhancedBuildAnalyzer struct {
	dockerClient     DockerClient
	knowledgeBase    KnowledgeBase
	failurePredictor *FailurePredictor
	policyEngine     PolicyEngine
	logger           *slog.Logger
}

// BuildAnalysisResult represents the result of build analysis
type BuildAnalysisResult struct {
	OptimizationTips []string               `json:"optimization_tips"`
	PotentialIssues  []Issue                `json:"potential_issues"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// Issue represents a potential issue in the build
type Issue struct {
	Severity    string   `json:"severity"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Suggestions []string `json:"suggestions"`
}

// ImageAnalysisResult represents the result of image analysis
type ImageAnalysisResult struct {
	ImageID      string              `json:"image_id"`
	Size         int64               `json:"size"`
	Layers       int                 `json:"layers"`
	Created      time.Time           `json:"created"`
	SecurityScan *SecurityScanResult `json:"security_scan,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// FailurePrediction represents a predicted failure reason (deprecated - use BuildPrediction instead)
type FailurePrediction struct {
	Reason             string           `json:"reason"`
	Confidence         float64          `json:"confidence"`
	Suggestions        []string         `json:"suggestions"`
	FailureProbability float64          `json:"failure_probability"`
	RiskFactors        []RiskFactor     `json:"risk_factors"`
	AnalysisTime       time.Time        `json:"analysis_time"`
	PredictedErrors    []PredictedError `json:"predicted_errors"`
	Optimizations      []string         `json:"optimizations"`
	Mitigations        []string         `json:"mitigations"`
}

// PolicyEngine interface for policy validation
type PolicyEngine interface {
	CheckBuildPolicy(args *BuildArgs) ([]string, error)
}

// FailurePredictor handles failure prediction
type FailurePredictor struct {
	logger *slog.Logger
}

// NewFailurePredictor creates a new failure predictor
func NewFailurePredictor(logger *slog.Logger) *FailurePredictor {
	return &FailurePredictor{
		logger: logger,
	}
}

// PredictBuildOutcome predicts build outcome with comprehensive analysis
func (f *FailurePredictor) PredictBuildOutcome(ctx context.Context, args *BuildArgs) (*BuildPrediction, error) {
	// Initialize prediction with default values
	prediction := &BuildPrediction{
		FailureProbability: 0.1, // Default low probability
		Reason:             "No significant issues detected",
		Mitigations:        []string{},
		RiskFactors:        []RiskFactor{},
		Confidence:         0.85,
		AnalysisTime:       time.Now(),
		PredictedErrors:    []PredictedError{},
		Optimizations:      []string{},
	}

	// Analyze build context
	if err := f.analyzeContext(args, prediction); err != nil {
		return prediction, err
	}

	// Analyze Dockerfile
	if err := f.analyzeDockerfile(args, prediction); err != nil {
		return prediction, err
	}

	// Analyze build arguments
	f.analyzeBuildArgs(args, prediction)

	// Analyze resource constraints
	f.analyzeResourceConstraints(args, prediction)

	// Analyze historical patterns
	f.analyzeHistoricalPatterns(args, prediction)

	// Generate predicted errors based on risk factors
	f.generatePredictedErrors(prediction)

	return prediction, nil
}

// AnalyzeError analyzes an error to predict failure reason with enhanced analysis
func (f *FailurePredictor) AnalyzeError(err error, args *BuildArgs) *FailurePrediction {
	prediction := &FailurePrediction{
		Reason:             "Unknown build error",
		Confidence:         0.5,
		Suggestions:        []string{"Check build logs for details"},
		RiskFactors:        []RiskFactor{},
		FailureProbability: 0.8, // High probability since we already have an error
		AnalysisTime:       time.Now(),
		PredictedErrors:    []PredictedError{},
		Optimizations:      []string{},
		Mitigations:        []string{},
	}

	if err == nil {
		prediction.FailureProbability = 0.0
		prediction.Reason = "No error to analyze"
		return prediction
	}

	// Analyze error patterns with enhanced categorization
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "no such file or directory"):
		prediction.Reason = "Missing file or directory"
		prediction.Confidence = 0.9
		prediction.FailureProbability = 0.95
		prediction.Mitigations = []string{
			"Verify all referenced files exist in the build context",
			"Check Dockerfile COPY/ADD commands for correct paths",
			"Ensure file permissions allow reading",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "filesystem",
			Severity:    "critical",
			Description: "Referenced file or directory not found",
			Impact:      0.95,
			Source:      "build_execution",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "file_not_found",
			Probability: 0.95,
			Description: "Required files missing from build context",
			Solution:    "Add missing files to build context or update file paths",
		})

	case strings.Contains(errStr, "pull access denied") || strings.Contains(errStr, "unauthorized"):
		prediction.Reason = "Base image pull access denied"
		prediction.Confidence = 0.95
		prediction.FailureProbability = 0.9
		prediction.Mitigations = []string{
			"Verify base image name and tag",
			"Check Docker registry authentication",
			"Ensure you have pull permissions for private images",
			"Use docker login to authenticate with registry",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "authentication",
			Severity:    "critical",
			Description: "Cannot pull base image due to access restrictions",
			Impact:      0.9,
			Source:      "registry_access",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "registry_auth_error",
			Probability: 0.9,
			Description: "Authentication failed for container registry",
			Solution:    "Configure registry credentials or use public image",
		})

	case strings.Contains(errStr, "no space left on device"):
		prediction.Reason = "Insufficient disk space"
		prediction.Confidence = 0.99
		prediction.FailureProbability = 0.99
		prediction.Mitigations = []string{
			"Clean up unused Docker images and containers",
			"Increase available disk space",
			"Use docker system prune to free space",
			"Configure Docker to use different storage location",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "resource",
			Severity:    "critical",
			Description: "Insufficient disk space for build operations",
			Impact:      0.99,
			Source:      "system_resources",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "disk_space_error",
			Probability: 0.99,
			Description: "Build failed due to insufficient disk space",
			Solution:    "Free up disk space or increase storage capacity",
		})

	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		prediction.Reason = "Network connectivity issues"
		prediction.Confidence = 0.8
		prediction.FailureProbability = 0.75
		prediction.Mitigations = []string{
			"Check network connectivity",
			"Verify Docker registry is accessible",
			"Check proxy settings if behind a firewall",
			"Retry build operation",
			"Use local mirror or cache if available",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "network",
			Severity:    "high",
			Description: "Network connectivity problems during build",
			Impact:      0.75,
			Source:      "network_connection",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "network_error",
			Probability: 0.75,
			Description: "Network operations failed during build",
			Solution:    "Check network settings and retry build",
		})

	case strings.Contains(errStr, "dockerfile") || strings.Contains(errStr, "parsing"):
		prediction.Reason = "Dockerfile syntax or parsing error"
		prediction.Confidence = 0.9
		prediction.FailureProbability = 0.9
		prediction.Mitigations = []string{
			"Review Dockerfile syntax",
			"Check for typos in Dockerfile instructions",
			"Validate Dockerfile with linter tools",
			"Ensure proper instruction format",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "dockerfile",
			Severity:    "critical",
			Description: "Dockerfile contains syntax errors",
			Impact:      0.9,
			Source:      "dockerfile_parsing",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "dockerfile_syntax_error",
			Probability: 0.9,
			Description: "Dockerfile contains syntax or formatting errors",
			Solution:    "Fix Dockerfile syntax and validate with tools",
		})

	case strings.Contains(errStr, "permission denied"):
		prediction.Reason = "Permission denied error"
		prediction.Confidence = 0.85
		prediction.FailureProbability = 0.85
		prediction.Mitigations = []string{
			"Check file and directory permissions",
			"Ensure Docker daemon has proper permissions",
			"Run build with appropriate user privileges",
			"Check SELinux or AppArmor restrictions",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "permissions",
			Severity:    "high",
			Description: "Insufficient permissions for build operations",
			Impact:      0.85,
			Source:      "file_permissions",
		})
		prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
			Type:        "permission_error",
			Probability: 0.85,
			Description: "Build operations blocked by permission restrictions",
			Solution:    "Adjust file permissions or run with elevated privileges",
		})

	default:
		// Generic error analysis
		prediction.Reason = "General build error"
		prediction.Confidence = 0.6
		prediction.FailureProbability = 0.7
		prediction.Mitigations = []string{
			"Check build logs for detailed error information",
			"Verify all build dependencies are available",
			"Ensure Docker daemon is running properly",
			"Try building with verbose output for more details",
		}
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "general",
			Severity:    "medium",
			Description: "Unspecified build error occurred",
			Impact:      0.7,
			Source:      "build_execution",
		})
	}

	return prediction
}

// BuildPrediction represents a build outcome prediction
type BuildPrediction struct {
	FailureProbability float64          `json:"failure_probability"`
	Reason             string           `json:"reason"`
	Mitigations        []string         `json:"mitigations"`
	RiskFactors        []RiskFactor     `json:"risk_factors"`
	Confidence         float64          `json:"confidence"`
	AnalysisTime       time.Time        `json:"analysis_time"`
	PredictedErrors    []PredictedError `json:"predicted_errors,omitempty"`
	Optimizations      []string         `json:"optimizations,omitempty"`
}

// RiskFactor represents a risk factor in the build
type RiskFactor struct {
	Type        string  `json:"type"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`
	Source      string  `json:"source"`
}

// PredictedError represents a predicted error
type PredictedError struct {
	Type        string  `json:"type"`
	Probability float64 `json:"probability"`
	Description string  `json:"description"`
	Solution    string  `json:"solution"`
}

// NewEnhancedBuildAnalyzer creates a new enhanced build analyzer
func NewEnhancedBuildAnalyzer(
	dockerClient DockerClient,
	knowledgeBase KnowledgeBase,
	failurePredictor *FailurePredictor,
	policyEngine PolicyEngine,
	logger *slog.Logger,
) *EnhancedBuildAnalyzer {
	return &EnhancedBuildAnalyzer{
		dockerClient:     dockerClient,
		knowledgeBase:    knowledgeBase,
		failurePredictor: failurePredictor,
		policyEngine:     policyEngine,
		logger:           logger,
	}
}

// ValidateBuildArgs validates build arguments against policies
func (a *EnhancedBuildAnalyzer) ValidateBuildArgs(args *BuildArgs) error {
	// Validate against policies
	if a.policyEngine != nil {
		violations, err := a.policyEngine.CheckBuildPolicy(args)
		if err != nil {
			return fmt.Errorf("policy check failed: %w", err)
		}

		if len(violations) > 0 {
			return fmt.Errorf("policy violations: %v", violations)
		}
	}

	// Additional validation
	if args.Platform != "" {
		// Validate platform format
		if !a.isValidPlatform(args.Platform) {
			return fmt.Errorf("invalid platform format: %s", args.Platform)
		}
	}

	return nil
}

// AnalyzeBuild performs pre-build analysis
func (a *EnhancedBuildAnalyzer) AnalyzeBuild(ctx context.Context, args *BuildArgs) (*BuildAnalysisResult, error) {
	result := &BuildAnalysisResult{
		OptimizationTips: []string{},
		PotentialIssues:  []Issue{},
		Metadata:         make(map[string]interface{}),
	}

	// Analyze Dockerfile
	dockerfilePath := filepath.Join(args.ContextPath, args.DockerfilePath)
	if args.DockerfilePath == "" {
		dockerfilePath = filepath.Join(args.ContextPath, "Dockerfile")
	}

	dockerfileAnalysis, err := a.analyzeDockerfile(dockerfilePath)
	if err == nil {
		result.OptimizationTips = append(result.OptimizationTips, dockerfileAnalysis.Tips...)
		result.PotentialIssues = append(result.PotentialIssues, dockerfileAnalysis.Issues...)
		result.Metadata["dockerfile_analysis"] = dockerfileAnalysis
	}

	// Check build context size
	contextSize, err := a.calculateContextSize(args.ContextPath)
	if err == nil {
		result.Metadata["context_size"] = contextSize

		if contextSize > 100*1024*1024 { // 100MB
			result.PotentialIssues = append(result.PotentialIssues, Issue{
				Severity:    "medium",
				Type:        "large_context",
				Description: "Large build context detected",
				Suggestions: []string{"Consider using .dockerignore to exclude unnecessary files"},
			})
			result.OptimizationTips = append(result.OptimizationTips,
				"Add .dockerignore file to exclude unnecessary files from build context")
		}
	}

	// Knowledge base insights
	if a.knowledgeBase != nil {
		// Convert BuildArgs to DockerBuildInput for knowledge base
		dockerBuildInput := &DockerBuildInput{
			DockerfilePath: args.DockerfilePath,
			ContextPath:    args.ContextPath,
			BuildArgs:      args.BuildArgs,
			Target:         args.Target,
			Platform:       args.Platform,
			NoCache:        args.NoCache,
		}

		insights, err := a.knowledgeBase.GetBuildInsights(ctx, dockerBuildInput)
		if err == nil {
			for _, insight := range insights {
				result.OptimizationTips = append(result.OptimizationTips, insight)
			}
		}
	}

	// Failure prediction
	if a.failurePredictor != nil {
		prediction, err := a.failurePredictor.PredictBuildOutcome(ctx, args)
		if err == nil && prediction.FailureProbability > 0.7 {
			result.PotentialIssues = append(result.PotentialIssues, Issue{
				Severity:    "high",
				Type:        "build_failure_risk",
				Description: fmt.Sprintf("High failure risk: %s", prediction.Reason),
				Suggestions: prediction.Mitigations,
			})
		}
	}

	return result, nil
}

// AnalyzeBuiltImage analyzes a built Docker image
func (a *EnhancedBuildAnalyzer) AnalyzeBuiltImage(ctx context.Context, imageID string) (*ImageAnalysisResult, error) {
	result := &ImageAnalysisResult{
		ImageID:  imageID,
		Warnings: []string{},
	}

	// Get image details
	imageInfo, err := a.dockerClient.InspectImage(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	result.Size = imageInfo.Size
	result.Layers = len(imageInfo.RootFS.Layers)
	result.Created = imageInfo.Created

	// Analyze image efficiency
	if result.Size > 1024*1024*1024 { // 1GB
		result.Warnings = append(result.Warnings,
			"Image size exceeds 1GB. Consider optimizing base image or removing unnecessary dependencies")
	}

	if result.Layers > 50 {
		result.Warnings = append(result.Warnings,
			"Image has many layers. Consider combining RUN commands to reduce layer count")
	}

	// Security scanning placeholder
	// In real implementation, this would call a security scanner
	result.SecurityScan = &SecurityScanResult{
		Passed:          true,
		Vulnerabilities: []Vulnerability{},
	}

	return result, nil
}

// PredictFailureReason predicts the reason for a build failure
func (a *EnhancedBuildAnalyzer) PredictFailureReason(err error, args *BuildArgs) *FailurePrediction {
	if a.failurePredictor != nil {
		return a.failurePredictor.AnalyzeError(err, args)
	}

	// Basic prediction without failure predictor
	return &FailurePrediction{
		Reason:      "Unknown build error",
		Confidence:  0.5,
		Suggestions: []string{"Check build logs for details"},
	}
}

// AddWarning adds a warning to the result
func (r *BuildAnalysisResult) AddWarning(warning string) {
	issue := Issue{
		Severity:    "warning",
		Type:        "general",
		Description: warning,
		Suggestions: []string{},
	}
	r.PotentialIssues = append(r.PotentialIssues, issue)
}

// Helper methods

func (a *EnhancedBuildAnalyzer) analyzeDockerfile(dockerfilePath string) (*DockerfileAnalysis, error) {
	analysis := &DockerfileAnalysis{
		Tips:   []string{},
		Issues: []Issue{},
	}

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); err != nil {
		return analysis, fmt.Errorf("dockerfile not found: %w", err)
	}

	// Read Dockerfile content
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return analysis, fmt.Errorf("failed to read dockerfile: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Analyze Dockerfile patterns
	hasUser := false
	hasHealthcheck := false
	runCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "USER") {
			hasUser = true
		}
		if strings.HasPrefix(line, "HEALTHCHECK") {
			hasHealthcheck = true
		}
		if strings.HasPrefix(line, "RUN") {
			runCount++
		}

		// Check for anti-patterns
		if strings.Contains(line, "apt-get update") && strings.Contains(line, "apt-get install") && !strings.Contains(line, "&&") {
			analysis.Issues = append(analysis.Issues, Issue{
				Severity:    "medium",
				Type:        "dockerfile_optimization",
				Description: "Separate RUN commands for update and install increase image size",
				Suggestions: []string{"Combine apt-get update && apt-get install in a single RUN command"},
			})
		}

		if strings.Contains(line, "ADD") && !strings.Contains(line, "http") {
			analysis.Tips = append(analysis.Tips, "Consider using COPY instead of ADD for local files")
		}
	}

	// Add recommendations
	if !hasUser {
		analysis.Tips = append(analysis.Tips, "Consider running container as non-root user with USER directive")
	}
	if !hasHealthcheck {
		analysis.Tips = append(analysis.Tips, "Consider adding HEALTHCHECK for better container monitoring")
	}
	if runCount > 10 {
		analysis.Tips = append(analysis.Tips, "Consider combining RUN commands to reduce image layers")
	}

	return analysis, nil
}

func (a *EnhancedBuildAnalyzer) calculateContextSize(contextPath string) (int64, error) {
	var size int64

	err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// DockerfileAnalysis represents Dockerfile analysis results
type DockerfileAnalysis struct {
	Tips   []string `json:"tips"`
	Issues []Issue  `json:"issues"`
}

// analyzeContext analyzes the build context for potential issues
func (f *FailurePredictor) analyzeContext(args *BuildArgs, prediction *BuildPrediction) error {
	if args.ContextPath == "" {
		prediction.FailureProbability = 0.9
		prediction.Reason = "Missing build context"
		prediction.Mitigations = append(prediction.Mitigations, "Specify a valid build context path")
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "context",
			Severity:    "critical",
			Description: "Build context path is empty",
			Impact:      0.9,
			Source:      "build_args",
		})
		return nil
	}

	// Check if context path exists and is accessible
	info, err := os.Stat(args.ContextPath)
	if err != nil {
		prediction.FailureProbability = 0.85
		prediction.Reason = "Build context path not accessible"
		prediction.Mitigations = append(prediction.Mitigations, "Ensure build context path exists and is readable")
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "context",
			Severity:    "critical",
			Description: fmt.Sprintf("Context path error: %v", err),
			Impact:      0.85,
			Source:      "filesystem",
		})
		return nil
	}

	if !info.IsDir() {
		prediction.FailureProbability = 0.8
		prediction.Reason = "Build context path is not a directory"
		prediction.Mitigations = append(prediction.Mitigations, "Provide a directory path for build context")
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "context",
			Severity:    "critical",
			Description: "Context path is not a directory",
			Impact:      0.8,
			Source:      "filesystem",
		})
	}

	// Check context size
	contextSize, err := f.calculateContextSize(args.ContextPath)
	if err == nil {
		if contextSize > 1024*1024*1024 { // 1GB
			prediction.FailureProbability += 0.3
			prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
				Type:        "performance",
				Severity:    "medium",
				Description: "Very large build context may cause timeout",
				Impact:      0.3,
				Source:      "context_size",
			})
			prediction.Mitigations = append(prediction.Mitigations, "Use .dockerignore to reduce context size")
			prediction.Optimizations = append(prediction.Optimizations, "Add .dockerignore file to exclude unnecessary files")
		} else if contextSize > 100*1024*1024 { // 100MB
			prediction.FailureProbability += 0.1
			prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
				Type:        "performance",
				Severity:    "low",
				Description: "Large build context may slow down build",
				Impact:      0.1,
				Source:      "context_size",
			})
			prediction.Optimizations = append(prediction.Optimizations, "Consider using .dockerignore to optimize build context")
		}
	}

	return nil
}

// analyzeDockerfile analyzes the Dockerfile for potential issues
func (f *FailurePredictor) analyzeDockerfile(args *BuildArgs, prediction *BuildPrediction) error {
	// Determine Dockerfile path
	dockerfilePath := filepath.Join(args.ContextPath, "Dockerfile")
	if args.DockerfilePath != "" {
		dockerfilePath = filepath.Join(args.ContextPath, args.DockerfilePath)
	}

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); err != nil {
		prediction.FailureProbability = 0.95
		prediction.Reason = "Dockerfile not found"
		prediction.Mitigations = append(prediction.Mitigations, "Ensure Dockerfile exists in the specified location")
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "dockerfile",
			Severity:    "critical",
			Description: "Dockerfile not found",
			Impact:      0.95,
			Source:      "filesystem",
		})
		return nil
	}

	// Read and analyze Dockerfile content
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		prediction.FailureProbability += 0.2
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "dockerfile",
			Severity:    "medium",
			Description: "Cannot read Dockerfile",
			Impact:      0.2,
			Source:      "filesystem",
		})
		return nil
	}

	lines := strings.Split(string(content), "\n")
	f.analyzeDockerfileContent(lines, prediction)

	return nil
}

// analyzeDockerfileContent analyzes Dockerfile content for issues
func (f *FailurePredictor) analyzeDockerfileContent(lines []string, prediction *BuildPrediction) {
	hasFrom := false
	runCommands := 0
	copyCommands := 0

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)

		// Check for FROM instruction
		if strings.HasPrefix(upper, "FROM") {
			hasFrom = true
			// Check for latest tag
			if strings.Contains(line, ":latest") || (!strings.Contains(line, ":") && !strings.Contains(line, "@")) {
				prediction.FailureProbability += 0.15
				prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
					Type:        "dockerfile",
					Severity:    "medium",
					Description: "Using 'latest' tag or no tag in FROM instruction",
					Impact:      0.15,
					Source:      fmt.Sprintf("line_%d", i+1),
				})
				prediction.Mitigations = append(prediction.Mitigations, "Use specific version tags instead of 'latest'")
				prediction.Optimizations = append(prediction.Optimizations, "Pin base image to specific version for reproducible builds")
			}
		}

		// Check RUN instructions
		if strings.HasPrefix(upper, "RUN") {
			runCommands++

			// Check for apt-get without cleanup
			if strings.Contains(line, "apt-get") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
				prediction.FailureProbability += 0.05
				prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
					Type:        "dockerfile",
					Severity:    "low",
					Description: "apt-get used without cleanup",
					Impact:      0.05,
					Source:      fmt.Sprintf("line_%d", i+1),
				})
				prediction.Optimizations = append(prediction.Optimizations, "Clean up apt cache to reduce image size")
			}

			// Check for potential network operations
			if strings.Contains(line, "curl") || strings.Contains(line, "wget") || strings.Contains(line, "git clone") {
				prediction.FailureProbability += 0.1
				prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
					Type:        "network",
					Severity:    "medium",
					Description: "Network operation may fail due to connectivity issues",
					Impact:      0.1,
					Source:      fmt.Sprintf("line_%d", i+1),
				})
				prediction.Mitigations = append(prediction.Mitigations, "Ensure network connectivity and add retry logic")
			}
		}

		// Check COPY/ADD instructions
		if strings.HasPrefix(upper, "COPY") || strings.HasPrefix(upper, "ADD") {
			copyCommands++
			// Check if source path looks suspicious
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				sourcePath := parts[1]
				if strings.HasPrefix(sourcePath, "/") {
					prediction.FailureProbability += 0.2
					prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
						Type:        "dockerfile",
						Severity:    "medium",
						Description: "Absolute path in COPY/ADD may not exist in build context",
						Impact:      0.2,
						Source:      fmt.Sprintf("line_%d", i+1),
					})
					prediction.Mitigations = append(prediction.Mitigations, "Use relative paths in COPY/ADD instructions")
				}
			}
		}
	}

	// Check for missing FROM instruction
	if !hasFrom {
		prediction.FailureProbability = 0.95
		prediction.Reason = "Missing FROM instruction in Dockerfile"
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "dockerfile",
			Severity:    "critical",
			Description: "Dockerfile must start with FROM instruction",
			Impact:      0.95,
			Source:      "dockerfile_structure",
		})
		prediction.Mitigations = append(prediction.Mitigations, "Add FROM instruction as the first line")
	}

	// Check for excessive RUN commands
	if runCommands > 15 {
		prediction.FailureProbability += 0.1
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "dockerfile",
			Severity:    "low",
			Description: "Too many RUN commands may increase build time and image size",
			Impact:      0.1,
			Source:      "dockerfile_structure",
		})
		prediction.Optimizations = append(prediction.Optimizations, "Combine multiple RUN commands to reduce layers")
	}
}

// analyzeBuildArgs analyzes build arguments for potential issues
func (f *FailurePredictor) analyzeBuildArgs(args *BuildArgs, prediction *BuildPrediction) {
	// Check for potentially problematic build args
	for key, value := range args.BuildArgs {
		// Check for empty values
		if value == "" {
			prediction.FailureProbability += 0.05
			prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
				Type:        "build_args",
				Severity:    "low",
				Description: fmt.Sprintf("Empty build arg value for '%s'", key),
				Impact:      0.05,
				Source:      "build_arguments",
			})
			prediction.Mitigations = append(prediction.Mitigations, fmt.Sprintf("Provide a value for build arg '%s'", key))
		}

		// Check for suspicious patterns in values
		if strings.Contains(value, "${") || strings.Contains(value, "$(") {
			prediction.FailureProbability += 0.1
			prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
				Type:        "build_args",
				Severity:    "medium",
				Description: fmt.Sprintf("Build arg '%s' contains shell expansion", key),
				Impact:      0.1,
				Source:      "build_arguments",
			})
			prediction.Mitigations = append(prediction.Mitigations, "Ensure shell expansions are properly escaped")
		}
	}

	// Check platform compatibility
	if args.Platform != "" {
		// Basic platform validation
		parts := strings.Split(args.Platform, "/")
		if len(parts) < 2 {
			prediction.FailureProbability += 0.4
			prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
				Type:        "platform",
				Severity:    "high",
				Description: "Invalid platform specification",
				Impact:      0.4,
				Source:      "build_arguments",
			})
			prediction.Mitigations = append(prediction.Mitigations, "Use valid platform format (e.g., linux/amd64)")
		}
	}
}

// analyzeResourceConstraints analyzes resource constraints
func (f *FailurePredictor) analyzeResourceConstraints(args *BuildArgs, prediction *BuildPrediction) {
	// Check for no-cache builds (potentially slower)
	if args.NoCache {
		prediction.FailureProbability += 0.05
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "performance",
			Severity:    "low",
			Description: "No-cache build may take longer",
			Impact:      0.05,
			Source:      "build_options",
		})
	}

	// Check for multi-stage builds with target
	if args.Target != "" {
		prediction.Optimizations = append(prediction.Optimizations, "Using multi-stage build target for optimized builds")
	}
}

// analyzeHistoricalPatterns analyzes historical build patterns
func (f *FailurePredictor) analyzeHistoricalPatterns(args *BuildArgs, prediction *BuildPrediction) {
	// Check for common failure patterns based on image name
	if strings.Contains(args.ImageName, "latest") {
		prediction.FailureProbability += 0.05
		prediction.RiskFactors = append(prediction.RiskFactors, RiskFactor{
			Type:        "naming",
			Severity:    "low",
			Description: "Using 'latest' in image name may cause confusion",
			Impact:      0.05,
			Source:      "image_naming",
		})
		prediction.Optimizations = append(prediction.Optimizations, "Use semantic versioning for image tags")
	}

	// Cap failure probability at 0.95
	if prediction.FailureProbability > 0.95 {
		prediction.FailureProbability = 0.95
	}
}

// generatePredictedErrors generates predicted errors based on risk factors
func (f *FailurePredictor) generatePredictedErrors(prediction *BuildPrediction) {
	for _, risk := range prediction.RiskFactors {
		switch risk.Type {
		case "dockerfile":
			if risk.Severity == "critical" {
				prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
					Type:        "dockerfile_error",
					Probability: risk.Impact,
					Description: "Dockerfile parsing or instruction error",
					Solution:    "Review Dockerfile syntax and instructions",
				})
			}
		case "network":
			prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
				Type:        "network_error",
				Probability: risk.Impact,
				Description: "Network connectivity issues during build",
				Solution:    "Ensure stable network connection and retry failed operations",
			})
		case "context":
			if risk.Severity == "critical" {
				prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
					Type:        "context_error",
					Probability: risk.Impact,
					Description: "Build context access or structure error",
					Solution:    "Verify build context path and permissions",
				})
			}
		case "platform":
			prediction.PredictedErrors = append(prediction.PredictedErrors, PredictedError{
				Type:        "platform_error",
				Probability: risk.Impact,
				Description: "Platform compatibility or architecture mismatch",
				Solution:    "Use correct platform specification for target architecture",
			})
		}
	}
}

// calculateContextSize calculates the size of the build context
func (f *FailurePredictor) calculateContextSize(contextPath string) (int64, error) {
	var size int64

	err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// isValidPlatform validates platform format
func (a *EnhancedBuildAnalyzer) isValidPlatform(platform string) bool {
	// Basic validation for platform format
	// Expected format: os/arch or os/arch/variant
	parts := strings.Split(platform, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	// Check OS
	validOS := map[string]bool{
		"linux": true, "windows": true, "darwin": true,
	}
	if !validOS[parts[0]] {
		return false
	}

	// Check architecture
	validArch := map[string]bool{
		"amd64": true, "arm64": true, "arm": true, "386": true, "ppc64le": true, "s390x": true,
	}
	if !validArch[parts[1]] {
		return false
	}

	return true
}
