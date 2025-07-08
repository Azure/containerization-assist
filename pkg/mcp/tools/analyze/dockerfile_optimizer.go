package analyze

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
)

// DockerfileOptimizer handles Dockerfile optimization and validation
type DockerfileOptimizer struct {
	logger            *slog.Logger
	validator         *coredocker.Validator
	hadolintValidator *coredocker.HadolintValidator
}

// NewDockerfileOptimizer creates a new dockerfile optimizer
func NewDockerfileOptimizer(logger *slog.Logger) *DockerfileOptimizer {
	return &DockerfileOptimizer{
		logger:            logger,
		validator:         coredocker.NewValidator(logger),
		hadolintValidator: coredocker.NewHadolintValidator(logger),
	}
}

// ApplyCustomizations applies customizations to Dockerfile content
func (do *DockerfileOptimizer) ApplyCustomizations(content string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) string {
	lines := strings.Split(content, "\n")

	// Apply build args
	if len(args.BuildArgs) > 0 {
		var newLines []string
		for _, line := range lines {
			newLines = append(newLines, line)
			if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
				newLines = append(newLines, "")
				newLines = append(newLines, "# Build arguments")
				for key, value := range args.BuildArgs {
					newLines = append(newLines, fmt.Sprintf("ARG %s=%s", key, value))
				}
				newLines = append(newLines, "")
			}
		}
		lines = newLines
	}

	// Apply platform
	if args.Platform != "" {
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
				lines[i] = fmt.Sprintf("FROM --platform=%s %s", args.Platform, strings.TrimPrefix(strings.TrimSpace(line), "FROM "))
			}
		}
	}

	// Apply optimizations
	switch args.Optimization {
	case "size":
		lines = do.applySizeOptimizations(lines)
	case "security":
		lines = do.applySecurityOptimizations(lines)
	}

	return strings.Join(lines, "\n")
}

// GenerateHealthCheck generates a health check command
func (do *DockerfileOptimizer) GenerateHealthCheck() string {
	port := 80
	return fmt.Sprintf("HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:%d/health || exit 1", port)
}

// applySizeOptimizations applies size optimization techniques
func (do *DockerfileOptimizer) applySizeOptimizations(lines []string) []string {
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "RUN ") {
			if strings.Contains(trimmed, "apt-get") || strings.Contains(trimmed, "apk") {
				if strings.Contains(trimmed, "apt-get") && !strings.Contains(trimmed, "rm -rf /var/lib/apt/lists/*") {
					line += " && rm -rf /var/lib/apt/lists/*"
				} else if strings.Contains(trimmed, "apk") && !strings.Contains(trimmed, "--no-cache") {
					line = strings.Replace(line, "apk add", "apk add --no-cache", 1)
				}
			}
		}

		result = append(result, line)
	}

	return result
}

// applySecurityOptimizations applies security optimization techniques
func (do *DockerfileOptimizer) applySecurityOptimizations(lines []string) []string {
	var result []string
	addedUser := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !addedUser && (strings.HasPrefix(trimmed, "CMD ") || strings.HasPrefix(trimmed, "ENTRYPOINT ")) {
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
			result = append(result, "")
			addedUser = true
		}

		result = append(result, line)

		if i == len(lines)-1 && !addedUser {
			result = append(result, "")
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
		}
	}

	return result
}

// ExtractBuildSteps extracts build steps from Dockerfile content
func (do *DockerfileOptimizer) ExtractBuildSteps(content string) []string {
	var steps []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "FROM ") ||
			strings.HasPrefix(trimmed, "RUN ") ||
			strings.HasPrefix(trimmed, "COPY ") ||
			strings.HasPrefix(trimmed, "ADD ") ||
			strings.HasPrefix(trimmed, "WORKDIR ") {
			steps = append(steps, trimmed)
		}
	}

	return steps
}

// ExtractExposedPorts extracts exposed ports from Dockerfile content
func (do *DockerfileOptimizer) ExtractExposedPorts(content string) []int {
	var ports []int
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "EXPOSE ") {
			portStr := strings.TrimPrefix(trimmed, "EXPOSE ")
			portStr = strings.TrimSpace(portStr)

			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
				ports = append(ports, port)
			}
		}
	}

	return ports
}

// ExtractBaseImage extracts the base image from Dockerfile content
func (do *DockerfileOptimizer) ExtractBaseImage(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "FROM ") {
			baseImage := strings.TrimPrefix(trimmed, "FROM ")
			if idx := strings.Index(baseImage, " AS "); idx > 0 {
				baseImage = baseImage[:idx]
			}
			return strings.TrimSpace(baseImage)
		}
	}

	return ""
}

// ExtractHealthCheck extracts health check from Dockerfile content
func (do *DockerfileOptimizer) ExtractHealthCheck(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "HEALTHCHECK ") {
			return trimmed
		}
	}

	return ""
}

// ValidateDockerfile validates Dockerfile content
func (do *DockerfileOptimizer) ValidateDockerfile(_ context.Context, content string) *types.BuildValidationResult {
	// Basic validation - create a simple result using the types unified types
	result := types.NewBuildResult()
	if result.Metadata.Context == nil {
		result.Metadata.Context = make(map[string]string)
	}
	result.Metadata.ValidatorName = "dockerfile-optimizer"
	result.Metadata.ValidatorVersion = "1.0.0"

	// Simple validation checks
	if !strings.Contains(content, "FROM") {
		result.Valid = false
		fromError := &types.ValidationError{
			Code:     "DOCKERFILE_MISSING_FROM",
			Message:  "Dockerfile must contain a FROM instruction",
			Field:    "FROM",
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": "1"},
		}
		result.Errors = append(result.Errors, *fromError)
	}

	return result
}

// GenerateOptimizationContext generates optimization context for the Dockerfile
func (do *DockerfileOptimizer) GenerateOptimizationContext(content string, args GenerateDockerfileArgs) *OptimizationContext {
	context := &OptimizationContext{
		OptimizationGoals: []string{},
		SuggestedChanges:  []OptimizationChange{},
		SecurityConcerns:  []SecurityConcern{},
		BestPractices:     []string{},
	}

	// Set optimization goals based on args
	switch args.Optimization {
	case "size":
		context.OptimizationGoals = []string{
			"Minimize final image size",
			"Reduce layer count",
			"Remove unnecessary dependencies",
		}
	case "security":
		context.OptimizationGoals = []string{
			"Run as non-root user",
			"Minimize attack surface",
			"Use official base images",
			"Keep dependencies updated",
		}
	case "speed":
		context.OptimizationGoals = []string{
			"Optimize build cache usage",
			"Parallelize build steps",
			"Minimize rebuild frequency",
		}
	default:
		context.OptimizationGoals = []string{
			"Balance size and security",
			"Follow Docker best practices",
			"Maintain build efficiency",
		}
	}

	// Analyze content for improvements
	lines := strings.Split(content, "\n")

	// Check for multi-stage build opportunities
	fromCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
			fromCount++
		}
	}

	if fromCount == 1 && (strings.Contains(content, "go build") || strings.Contains(content, "npm run build")) {
		context.SuggestedChanges = append(context.SuggestedChanges, OptimizationChange{
			Type:        "Multi-stage build",
			Description: "Use multi-stage build to reduce final image size",
			Impact:      "Can reduce image size by 50-90%",
			Example:     "FROM builder AS build\n# Build steps\nFROM alpine\nCOPY --from=build /app/binary /app/",
		})
	}

	// Check for package manager cleanup
	if strings.Contains(content, "apt-get install") && !strings.Contains(content, "rm -rf /var/lib/apt/lists/*") {
		context.SuggestedChanges = append(context.SuggestedChanges, OptimizationChange{
			Type:        "Package cache cleanup",
			Description: "Clean package manager cache after installation",
			Impact:      "Reduces layer size",
			Example:     "RUN apt-get update && apt-get install -y pkg && rm -rf /var/lib/apt/lists/*",
		})
	}

	// Security checks
	if !strings.Contains(content, "USER ") || strings.Contains(content, "USER root") {
		context.SecurityConcerns = append(context.SecurityConcerns, SecurityConcern{
			Issue:      "Running as root user",
			Severity:   "high",
			Suggestion: "Create and use a non-root user",
			Reference:  "https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#user",
		})
	}

	// Check for COPY vs ADD usage
	if strings.Contains(content, "ADD ") && !strings.Contains(content, ".tar") && !strings.Contains(content, "http") {
		context.SecurityConcerns = append(context.SecurityConcerns, SecurityConcern{
			Issue:      "Using ADD instead of COPY",
			Severity:   "low",
			Suggestion: "Use COPY unless you need ADD's tar extraction or remote URL features",
			Reference:  "https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#add-or-copy",
		})
	}

	// Best practices
	context.BestPractices = []string{
		"Order Dockerfile commands from least to most frequently changing",
		"Combine RUN commands to reduce layers",
		"Use specific image tags instead of 'latest'",
		"Set WORKDIR early in the Dockerfile",
		"Use .dockerignore to exclude unnecessary files",
	}

	return context
}
