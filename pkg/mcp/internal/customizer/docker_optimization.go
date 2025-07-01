package customizer

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Optimizer handles Dockerfile optimization
type Optimizer struct {
	logger zerolog.Logger
}

// NewOptimizer creates a new Dockerfile optimizer
func NewOptimizer(logger zerolog.Logger) *Optimizer {
	return &Optimizer{
		logger: logger.With().Str("component", "dockerfile_optimizer").Logger(),
	}
}

// ApplyOptimization applies optimization strategies to a Dockerfile
func (o *Optimizer) ApplyOptimization(content string, strategy OptimizationStrategy, context *TemplateContext) string {
	switch strategy {
	case OptimizationSize:
		return o.optimizeForSize(content, context)
	case OptimizationSpeed:
		return o.optimizeForSpeed(content, context)
	case OptimizationSecurity:
		return o.optimizeForSecurity(content, context)
	default:
		return content
	}
}

// optimizeForSize optimizes the Dockerfile for minimal image size
func (o *Optimizer) optimizeForSize(content string, context *TemplateContext) string {
	var optimizations []string

	// Suggest Alpine-based images
	if !strings.Contains(content, "alpine") && !strings.Contains(content, "distroless") {
		optimizations = append(optimizations, "# Size optimization: Consider using Alpine Linux or distroless images")
	}

	// Add layer optimization comments
	if !strings.Contains(content, "&&") || strings.Count(content, "RUN") > 5 {
		optimizations = append(optimizations, "# Size optimization: Combine RUN commands to reduce layers")
	}

	// Clean package manager caches
	if context != nil {
		switch context.Language {
		case "Python":
			if !strings.Contains(content, "--no-cache-dir") {
				content = strings.ReplaceAll(content, "pip install", "pip install --no-cache-dir")
			}
		case "JavaScript", "TypeScript":
			if !strings.Contains(content, "npm cache clean") {
				content = o.addCleanupStep(content, "RUN npm cache clean --force")
			}
		}
	}

	// Add cleanup commands
	if !strings.Contains(content, "rm -rf") && !strings.Contains(content, "apt-get clean") {
		optimizations = append(optimizations, "# Size optimization: Add cleanup commands to remove temporary files")
	}

	if len(optimizations) > 0 {
		content = strings.Join(optimizations, "\n") + "\n\n" + content
	}

	o.logger.Debug().
		Str("strategy", "size").
		Int("optimization_count", len(optimizations)).
		Msg("Applied size optimizations")

	return content
}

// optimizeForSpeed optimizes the Dockerfile for faster builds
func (o *Optimizer) optimizeForSpeed(content string, context *TemplateContext) string {
	var optimizations []string

	// Suggest build cache optimization
	if !strings.Contains(content, "--mount=type=cache") {
		optimizations = append(optimizations, "# Speed optimization: Use BuildKit cache mounts for package managers")
	}

	// Order COPY commands for better caching
	if context != nil && (context.Language == "JavaScript" || context.Language == "Python") {
		if !o.hasOptimalCopyOrder(content) {
			optimizations = append(optimizations, "# Speed optimization: Copy dependency files before source code for better caching")
		}
	}

	// Suggest parallel builds
	if context != nil && context.Language == "JavaScript" && !strings.Contains(content, "--parallel") {
		optimizations = append(optimizations, "# Speed optimization: Use parallel builds where supported")
	}

	if len(optimizations) > 0 {
		content = strings.Join(optimizations, "\n") + "\n\n" + content
	}

	o.logger.Debug().
		Str("strategy", "speed").
		Int("optimization_count", len(optimizations)).
		Msg("Applied speed optimizations")

	return content
}

// optimizeForSecurity optimizes the Dockerfile for security
func (o *Optimizer) optimizeForSecurity(content string, context *TemplateContext) string {
	var optimizations []string

	// Add non-root user if not present
	if !strings.Contains(content, "USER") || strings.Contains(content, "USER root") {
		userSection := `
# Security: Run as non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser
USER appuser`
		// Add before the last ENTRYPOINT or CMD
		content = o.insertBeforeLastCommand(content, userSection)
		optimizations = append(optimizations, "Added non-root user")
	}

	// Suggest security scanning
	if !strings.Contains(content, "trivy") && !strings.Contains(content, "scan") {
		optimizations = append(optimizations, "# Security: Consider adding vulnerability scanning in CI/CD")
	}

	// Use specific version tags
	if strings.Contains(content, ":latest") {
		optimizations = append(optimizations, "# Security: Avoid using 'latest' tag, specify exact versions")
		// Replace :latest with a version placeholder that includes the current date
		// This makes it clear that the version needs to be specified
		versionPlaceholder := fmt.Sprintf(":%s-CHANGEME", time.Now().Format("20060102"))
		content = strings.ReplaceAll(content, ":latest", versionPlaceholder+" # SECURITY: Replace with specific version")
	}

	// Minimal base images
	if !strings.Contains(content, "distroless") && !strings.Contains(content, "scratch") {
		optimizations = append(optimizations, "# Security: Consider using distroless or minimal base images")
	}

	if len(optimizations) > 0 {
		header := fmt.Sprintf("# Security optimizations applied: %s\n", strings.Join(optimizations, ", "))
		content = header + content
	}

	o.logger.Debug().
		Str("strategy", "security").
		Int("optimization_count", len(optimizations)).
		Msg("Applied security optimizations")

	return content
}

// hasOptimalCopyOrder checks if dependency files are copied before source code
func (o *Optimizer) hasOptimalCopyOrder(content string) bool {
	lines := strings.Split(content, "\n")
	firstSourceCopy := -1
	firstDepCopy := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "COPY") {
			if strings.Contains(line, "package.json") || strings.Contains(line, "requirements.txt") ||
				strings.Contains(line, "go.mod") || strings.Contains(line, "pom.xml") {
				if firstDepCopy == -1 {
					firstDepCopy = i
				}
			} else if strings.Contains(line, ".") && !strings.Contains(line, "*.") {
				if firstSourceCopy == -1 {
					firstSourceCopy = i
				}
			}
		}
	}

	// Optimal if dependency files are copied before source code
	return firstDepCopy != -1 && (firstSourceCopy == -1 || firstDepCopy < firstSourceCopy)
}

// addCleanupStep adds a cleanup step to the Dockerfile
func (o *Optimizer) addCleanupStep(content, cleanupCmd string) string {
	lines := strings.Split(content, "\n")
	// Find the last RUN command
	lastRunIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "RUN") {
			lastRunIndex = i
			break
		}
	}

	if lastRunIndex != -1 {
		// Insert cleanup after the last RUN command
		lines = append(lines[:lastRunIndex+1], append([]string{cleanupCmd}, lines[lastRunIndex+1:]...)...)
	}

	return strings.Join(lines, "\n")
}

// insertBeforeLastCommand inserts content before the last ENTRYPOINT or CMD
func (o *Optimizer) insertBeforeLastCommand(content, insertion string) string {
	lines := strings.Split(content, "\n")
	insertIndex := len(lines)

	// Find the last ENTRYPOINT or CMD
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "ENTRYPOINT") || strings.HasPrefix(trimmed, "CMD") {
			insertIndex = i
			break
		}
	}

	// Insert the new content
	result := append(lines[:insertIndex], append([]string{insertion}, lines[insertIndex:]...)...)
	return strings.Join(result, "\n")
}

// GenerateOptimizationContext generates optimization recommendations
func (o *Optimizer) GenerateOptimizationContext(content string, context *TemplateContext) *OptimizationContext {
	ctx := &OptimizationContext{
		CurrentSize:       o.estimateImageSize(content),
		OptimizationHints: []string{},
		SecurityIssues:    []string{},
		PerformanceIssues: []string{},
	}

	// Size hints
	if !strings.Contains(content, "alpine") && !strings.Contains(content, "distroless") {
		ctx.OptimizationHints = append(ctx.OptimizationHints, "Use Alpine Linux or distroless base images to reduce size")
	}

	if strings.Count(content, "RUN") > 5 {
		ctx.OptimizationHints = append(ctx.OptimizationHints, "Combine RUN commands to reduce layer count")
	}

	// Security issues
	if !strings.Contains(content, "USER") || strings.Contains(content, "USER root") {
		ctx.SecurityIssues = append(ctx.SecurityIssues, "Running as root user - add non-root user")
	}

	if strings.Contains(content, ":latest") {
		ctx.SecurityIssues = append(ctx.SecurityIssues, "Using 'latest' tags - specify exact versions")
	}

	// Performance issues
	if !o.hasOptimalCopyOrder(content) {
		ctx.PerformanceIssues = append(ctx.PerformanceIssues, "Suboptimal COPY order - copy dependencies before source")
	}

	return ctx
}

// estimateImageSize provides a rough estimate of image size
func (o *Optimizer) estimateImageSize(content string) string {
	if strings.Contains(content, "alpine") {
		return "Small (< 50MB base)"
	} else if strings.Contains(content, "slim") {
		return "Medium (100-200MB base)"
	} else if strings.Contains(content, "distroless") {
		return "Minimal (< 20MB base)"
	}
	return "Large (> 200MB base)"
}

// OptimizationContext provides optimization recommendations
type OptimizationContext struct {
	CurrentSize       string
	OptimizationHints []string
	SecurityIssues    []string
	PerformanceIssues []string
}
