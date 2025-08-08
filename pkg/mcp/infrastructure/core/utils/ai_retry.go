package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/mark3labs/mcp-go/server"
)

// enhancedErrorMessageTemplate is the template for AI assistant error messages.
// This can be customized if needed for different AI assistant behaviors.
const enhancedErrorMessageTemplate = `🤖 AI ASSISTANT: The workflow failed and needs your help.

OPERATION FAILED: "%s" (attempt %d/%d)
ERROR: %s

🔧 REQUIRED ACTION: Please retry the workflow using the individual tools. Start with analyze_repository and follow the tool chain:
- Use analyze_repository first with repo_url and session_id
- Follow the chain hints to generate_dockerfile → build_image → etc.
- Use the SAME session_id across all tool calls to maintain state

📋 TROUBLESHOOTING CHECKLIST:
%s

⚡ IMMEDIATE NEXT STEP: Call analyze_repository tool with repo_url and a unique session_id to start the containerization workflow.`

// WithAIRetry wraps a function with AI-powered retry logic
// This works with external AI assistants (like Claude) using the MCP server
// The AI assistant observes failures through structured error reporting and can retry the workflow
func WithAIRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	// Try to get MCP server from context for enhanced retry with sampling
	if srv := server.ServerFromContext(ctx); srv != nil {
		return WithLLMGuidedRetry(ctx, name, max, fn, logger)
	}

	// Fallback to basic retry logic
	return withBasicAIRetry(ctx, name, max, fn, logger)
}

// WithLLMGuidedRetry uses MCP sampling for intelligent retry logic
func WithLLMGuidedRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	logger.Info("Starting operation with LLM-guided retry", "operation", name, "max_retries", max)

	samplingClient := sampling.NewClient(logger)

	for i := 1; i <= max; i++ {
		logger.Debug("Attempting operation", "operation", name, "attempt", i, "max", max)

		err := fn()
		if err == nil {
			logger.Info("Operation completed successfully", "operation", name, "attempt", i)
			return nil
		}

		logger.Error("Operation failed", "operation", name, "attempt", i, "max", max, "error", err)

		// If this was the last attempt, return enhanced error
		if i == max {
			logger.Error("Operation exhausted all retries", "operation", name, "attempts", max)
			return enhanceErrorForAI(name, err, i, max, logger)
		}

		// Use LLM to analyze the error and suggest fixes
		analysis, analysisErr := samplingClient.AnalyzeError(ctx, err, fmt.Sprintf("Operation: %s, Attempt %d of %d", name, i, max))
		if analysisErr != nil {
			logger.Warn("Failed to get LLM analysis, falling back to pattern-based fixes", "error", analysisErr)
			// Apply pattern-based auto-fixes even without LLM analysis
			fixApplied := applyPatternBasedFixes(ctx, name, err.Error(), logger)
			if fixApplied {
				logger.Info("Pattern-based fixes applied, retrying operation")
				time.Sleep(200 * time.Millisecond)
			}
			continue
		}

		// Log the analysis for visibility
		logger.Info("LLM Error Analysis",
			"operation", name,
			"root_cause", analysis.RootCause,
			"can_auto_fix", analysis.CanAutoFix,
			"fix_steps", len(analysis.FixSteps))

		// Apply LLM-suggested fixes first
		fixApplied := false
		if analysis.CanAutoFix && len(analysis.FixSteps) > 0 {
			logger.Info("Attempting LLM-suggested automated fixes", "fix_count", len(analysis.FixSteps))

			applied, fixErr := applyAIFixSteps(ctx, name, analysis.FixSteps, logger)
			if fixErr != nil {
				logger.Warn("Failed to apply LLM-suggested fixes", "error", fixErr)
			} else if applied {
				logger.Info("LLM fixes applied successfully")
				fixApplied = true
			}
		}

		// If LLM fixes didn't work or weren't applicable, try pattern-based fixes
		if !fixApplied {
			logger.Info("Attempting pattern-based fixes as fallback")
			applied := applyPatternBasedFixes(ctx, name, err.Error(), logger)
			if applied {
				logger.Info("Pattern-based fixes applied successfully")
				fixApplied = true
			}
		}

		// Give fixes time to take effect before retry
		if fixApplied {
			time.Sleep(200 * time.Millisecond)
		}

		// Continue to next retry with LLM insights logged
	}

	return errors.New(errors.CodeOperationFailed, "ai_retry", fmt.Sprintf("%s: exhausted %d retries", name, max), nil)
}

// withBasicAIRetry is the original retry logic without MCP sampling
func withBasicAIRetry(ctx context.Context, name string, max int, fn func() error, logger *slog.Logger) error {
	logger.Info("Starting operation with AI retry", "operation", name, "max_retries", max)

	for i := 1; i <= max; i++ {
		logger.Debug("Attempting operation", "operation", name, "attempt", i, "max", max)

		if err := fn(); err != nil {
			logger.Error("Operation failed", "operation", name, "attempt", i, "max", max, "error", err)

			// If this was the last attempt, return the structured error
			if i == max {
				logger.Error("Operation exhausted all retries", "operation", name, "attempts", max)

				// Return structured error with fix suggestions for the AI assistant
				errorWithSuggestions := enhanceErrorForAI(name, err, i, max, logger)
				return errorWithSuggestions
			}

			// Log structured error information for AI assistant to observe
			logAIObservableError(name, err, i, max, logger)

			// Continue to next attempt - the external AI assistant can observe
			// this failure pattern and make decisions about retrying the workflow
			continue
		}

		// Success!
		logger.Info("Operation completed successfully", "operation", name, "attempt", i)
		return nil
	}

	// This should never be reached due to the logic above
	return errors.New(errors.CodeOperationFailed, "ai_retry", fmt.Sprintf("%s: exhausted %d retries", name, max), nil)
}

type Fix struct {
	Description string `json:"description"`
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	Content     string `json:"content,omitempty"`
}

func logAIObservableError(operation string, err error, attempt, maxAttempts int, logger *slog.Logger) {
	// Log detailed error information in a structured format
	// that the AI assistant can observe and learn from
	suggestions := generateFixSuggestions(operation, err.Error(), logger)

	logger.Warn("🤖 AI ASSISTANT: Workflow step failed but retrying automatically",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"error_type", fmt.Sprintf("%T", err),
		"error_message", err.Error(),
		"retry_available", attempt < maxAttempts,
		"auto_retry_in_progress", true,
		"troubleshooting_checklist", suggestions,
		"ai_instruction", "The workflow is automatically retrying this step. If all retries fail, you will receive explicit instructions to retry the entire workflow.")
}

func enhanceErrorForAI(operation string, originalErr error, attempt, maxAttempts int, logger *slog.Logger) error {
	errorMsg := originalErr.Error()

	// Generate fix suggestions based on error patterns
	fixSuggestions := generateFixSuggestions(operation, errorMsg, logger)

	// Create enhanced error message with explicit instructions for AI assistant
	enhancedMsg := fmt.Sprintf(enhancedErrorMessageTemplate,
		operation, attempt, maxAttempts, errorMsg, fixSuggestions)

	logger.Error("Enhanced error for AI assistant",
		"operation", operation,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"fix_suggestions", fixSuggestions)

	return fmt.Errorf("%s", enhancedMsg)
}

func generateFixSuggestions(operation string, errorMsg string, logger *slog.Logger) string {
	var suggestions []string

	// Analyze the error message for common patterns and suggest fixes
	if containsPattern(errorMsg, "dockerfile", "syntax error", "unknown instruction") {
		suggestions = append(suggestions, "• Check Dockerfile syntax and instruction names")
		suggestions = append(suggestions, "• Verify base image names and tags")
		suggestions = append(suggestions, "• Ensure proper FROM instruction format")
	}

	if containsPattern(errorMsg, "docker build", "failed", "no such file") {
		suggestions = append(suggestions, "• Verify all required files exist in build context")
		suggestions = append(suggestions, "• Check COPY/ADD paths in Dockerfile")
		suggestions = append(suggestions, "• Ensure build context includes necessary files")
	}

	// Java build tool errors
	if containsPattern(errorMsg, "mvn", "command not found", "exit code: 127", "maven") {
		suggestions = append(suggestions, "• Maven is not installed in the Docker image")
		suggestions = append(suggestions, "• Use 'maven:3.9-eclipse-temurin-17' as base image")
		suggestions = append(suggestions, "• Or install Maven in the Dockerfile: RUN apt-get update && apt-get install -y maven")
	}

	if containsPattern(errorMsg, "gradle", "command not found") {
		suggestions = append(suggestions, "• Gradle is not installed in the Docker image")
		suggestions = append(suggestions, "• Use 'gradle:8-jdk17' as base image")
		suggestions = append(suggestions, "• Or install Gradle in the Dockerfile")
	}

	if containsPattern(errorMsg, "kubernetes", "deploy", "image pull") {
		suggestions = append(suggestions, "• Verify image exists in local registry (localhost:5001)")
		suggestions = append(suggestions, "• Check image name and tag format")
		suggestions = append(suggestions, "• Ensure kind cluster can access the image")
	}

	if containsPattern(errorMsg, "deployment", "validation", "pods ready") {
		suggestions = append(suggestions, "• Check pod resource requests and limits")
		suggestions = append(suggestions, "• Verify image pull policy and registry access")
		suggestions = append(suggestions, "• Review pod scheduling constraints and node capacity")
		suggestions = append(suggestions, "• Inspect pod logs for startup errors")
		suggestions = append(suggestions, "• Validate container health checks and readiness probes")
	}

	if containsPattern(errorMsg, "port", "connection", "refused") {
		suggestions = append(suggestions, "• Verify application listens on correct port")
		suggestions = append(suggestions, "• Check port bindings in Dockerfile and K8s manifests")
		suggestions = append(suggestions, "• Ensure no port conflicts with existing services")
	}

	if containsPattern(errorMsg, "permission", "denied", "access") {
		suggestions = append(suggestions, "• Check file permissions in repository")
		suggestions = append(suggestions, "• Verify Docker daemon permissions")
		suggestions = append(suggestions, "• Ensure kubectl has proper cluster access")
	}

	if containsPattern(errorMsg, "kind", "cluster", "not found") {
		suggestions = append(suggestions, "• Ensure kind cluster 'container-kit' exists")
		suggestions = append(suggestions, "• Verify kind and kubectl are installed")
		suggestions = append(suggestions, "• Check cluster connectivity")
	}

	if containsPattern(errorMsg, "git", "clone", "repository") {
		suggestions = append(suggestions, "• Verify repository URL is accessible")
		suggestions = append(suggestions, "• Check network connectivity")
		suggestions = append(suggestions, "• Try different branch (main/master)")
	}

	// Default suggestions if no specific patterns match
	if len(suggestions) == 0 {
		switch operation {
		case "analyze_repository":
			suggestions = append(suggestions, "• Verify repository URL and branch name")
			suggestions = append(suggestions, "• Check network connectivity and access permissions")
		case "generate_dockerfile":
			suggestions = append(suggestions, "• Review detected language and framework")
			suggestions = append(suggestions, "• Check if repository structure matches expectations")
		case "build_image":
			suggestions = append(suggestions, "• Verify Docker daemon is running")
			suggestions = append(suggestions, "• Check Dockerfile content and build context")
		case "deploy_to_k8s":
			suggestions = append(suggestions, "• Verify kind cluster is running")
			suggestions = append(suggestions, "• Check kubectl configuration and permissions")
		default:
			suggestions = append(suggestions, "• Review error details and retry with correct parameters")
			suggestions = append(suggestions, "• Check system prerequisites and dependencies")
		}
	}

	if len(suggestions) == 0 {
		return "No specific suggestions available - review error details"
	}

	return strings.Join(suggestions, "\n")
}

func containsPattern(prompt string, patterns ...string) bool {
	promptLower := strings.ToLower(prompt) // Convert to lowercase for case-insensitive matching
	for _, pattern := range patterns {
		if strings.Contains(promptLower, pattern) {
			return true
		}
	}
	return false
}

type RetryableOperation struct {
	Name       string
	MaxRetries int
	Logger     *slog.Logger
}

func (op *RetryableOperation) Execute(ctx context.Context, fn func() error) error {
	return WithAIRetry(ctx, op.Name, op.MaxRetries, fn, op.Logger)
}

func NewRetryableOperation(name string, maxRetries int, logger *slog.Logger) *RetryableOperation {
	return &RetryableOperation{
		Name:       name,
		MaxRetries: maxRetries,
		Logger:     logger,
	}
}

func applyAIFixSteps(ctx context.Context, operation string, fixSteps []string, logger *slog.Logger) (bool, error) {
	logger.Info("Applying AI-suggested fixes", "operation", operation, "steps", len(fixSteps))

	fixesApplied := false

	for i, step := range fixSteps {
		logger.Debug("Processing fix step", "step", i+1, "description", step)

		applied, err := applySingleFixStep(ctx, step, logger)
		if err != nil {
			logger.Warn("Failed to apply fix step", "step", i+1, "error", err)
			continue
		}

		if applied {
			fixesApplied = true
			logger.Info("Successfully applied fix step", "step", i+1, "description", step)
		}
	}

	return fixesApplied, nil
}

func applySingleFixStep(ctx context.Context, step string, logger *slog.Logger) (bool, error) {
	stepLower := strings.ToLower(step)

	// File-based fixes
	if strings.Contains(stepLower, "dockerfile") && strings.Contains(stepLower, "base image") {
		return applyDockerfileBaseFix(step, logger)
	}

	if strings.Contains(stepLower, "dockerfile") && strings.Contains(stepLower, "maven") {
		return applyMavenDockerfileFix(step, logger)
	}

	if strings.Contains(stepLower, "dockerfile") && strings.Contains(stepLower, "gradle") {
		return applyGradleDockerfileFix(step, logger)
	}

	if strings.Contains(stepLower, "port") && strings.Contains(stepLower, "expose") {
		return applyPortExposeFix(step, logger)
	}

	// Environment/configuration fixes
	if strings.Contains(stepLower, "environment") || strings.Contains(stepLower, "env") {
		return applyEnvironmentFix(step, logger)
	}

	// Path and permission fixes
	if strings.Contains(stepLower, "permission") || strings.Contains(stepLower, "chmod") {
		return applyPermissionFix(step, logger)
	}

	// Log that we couldn't automatically apply this fix
	logger.Debug("Fix step not automatically applicable", "step", step)
	return false, nil
}

func applyDockerfileBaseFix(step string, logger *slog.Logger) (bool, error) {
	dockerfilePath := "./Dockerfile"
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Debug("Dockerfile not found, cannot apply base image fix")
		return false, nil
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to read Dockerfile", err)
	}

	original := string(content)
	updated := original

	// Common base image fixes
	if strings.Contains(step, "maven") {
		// Replace basic java images with Maven-enabled ones
		updated = regexp.MustCompile(`(?i)FROM\s+openjdk:[0-9]+-jdk`).ReplaceAllString(updated, "FROM maven:3.9-eclipse-temurin-17")
		updated = regexp.MustCompile(`(?i)FROM\s+eclipse-temurin:[0-9]+-jdk`).ReplaceAllString(updated, "FROM maven:3.9-eclipse-temurin-17")
	}

	if strings.Contains(step, "gradle") {
		// Replace basic java images with Gradle-enabled ones
		updated = regexp.MustCompile(`(?i)FROM\s+openjdk:[0-9]+-jdk`).ReplaceAllString(updated, "FROM gradle:8-jdk17")
		updated = regexp.MustCompile(`(?i)FROM\s+eclipse-temurin:[0-9]+-jdk`).ReplaceAllString(updated, "FROM gradle:8-jdk17")
	}

	if updated != original {
		err = os.WriteFile(dockerfilePath, []byte(updated), 0644)
		if err != nil {
			return false, errors.New(errors.CodeIoError, "dockerfile", "failed to write updated Dockerfile", err)
		}
		logger.Info("Applied Dockerfile base image fix", "file", dockerfilePath)
		return true, nil
	}

	return false, nil
}

func applyMavenDockerfileFix(step string, logger *slog.Logger) (bool, error) {
	dockerfilePath := "./Dockerfile"
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Debug("Dockerfile not found, cannot apply Maven fix")
		return false, nil
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to read Dockerfile", err)
	}

	original := string(content)

	// Check if Maven is already installed
	if strings.Contains(original, "maven") || strings.Contains(original, "mvn") {
		logger.Debug("Maven already present in Dockerfile")
		return false, nil
	}

	// Add Maven installation after FROM line
	lines := strings.Split(original, "\n")
	var updated []string

	for i, line := range lines {
		updated = append(updated, line)

		// Add Maven installation after FROM line
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM") && i < len(lines)-1 {
			updated = append(updated, "")
			updated = append(updated, "# Install Maven")
			updated = append(updated, "RUN apt-get update && apt-get install -y maven && rm -rf /var/lib/apt/lists/*")
			updated = append(updated, "")
		}
	}

	newContent := strings.Join(updated, "\n")

	err = os.WriteFile(dockerfilePath, []byte(newContent), 0644)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to write updated Dockerfile", err)
	}

	logger.Info("Applied Maven installation fix to Dockerfile", "file", dockerfilePath)
	return true, nil
}

func applyGradleDockerfileFix(step string, logger *slog.Logger) (bool, error) {
	dockerfilePath := "./Dockerfile"
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Debug("Dockerfile not found, cannot apply Gradle fix")
		return false, nil
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to read Dockerfile", err)
	}

	original := string(content)

	// Check if Gradle is already installed
	if strings.Contains(original, "gradle") {
		logger.Debug("Gradle already present in Dockerfile")
		return false, nil
	}

	// Add Gradle installation after FROM line
	lines := strings.Split(original, "\n")
	var updated []string

	for i, line := range lines {
		updated = append(updated, line)

		// Add Gradle installation after FROM line
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM") && i < len(lines)-1 {
			updated = append(updated, "")
			updated = append(updated, "# Install Gradle")
			updated = append(updated, "RUN apt-get update && \\")
			updated = append(updated, "    apt-get install -y wget unzip && \\")
			updated = append(updated, "    wget -q https://services.gradle.org/distributions/gradle-8.0-bin.zip && \\")
			updated = append(updated, "    unzip gradle-8.0-bin.zip -d /opt && \\")
			updated = append(updated, "    ln -s /opt/gradle-8.0/bin/gradle /usr/bin/gradle && \\")
			updated = append(updated, "    rm gradle-8.0-bin.zip && \\")
			updated = append(updated, "    rm -rf /var/lib/apt/lists/*")
			updated = append(updated, "")
		}
	}

	newContent := strings.Join(updated, "\n")

	err = os.WriteFile(dockerfilePath, []byte(newContent), 0644)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to write updated Dockerfile", err)
	}

	logger.Info("Applied Gradle installation fix to Dockerfile", "file", dockerfilePath)
	return true, nil
}

func applyPortExposeFix(step string, logger *slog.Logger) (bool, error) {
	dockerfilePath := "./Dockerfile"
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Debug("Dockerfile not found, cannot apply port expose fix")
		return false, nil
	}

	// Extract port number from the fix step
	portRegex := regexp.MustCompile(`\b(\d{4,5})\b`)
	matches := portRegex.FindStringSubmatch(step)
	port := "8080" // default
	if len(matches) > 1 {
		port = matches[1]
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to read Dockerfile", err)
	}

	original := string(content)

	// Check if EXPOSE is already present
	if strings.Contains(original, "EXPOSE") {
		logger.Debug("EXPOSE directive already present in Dockerfile")
		return false, nil
	}

	// Add EXPOSE directive before CMD/ENTRYPOINT
	lines := strings.Split(original, "\n")
	var updated []string
	exposeAdded := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))

		// Add EXPOSE before CMD or ENTRYPOINT
		if (strings.HasPrefix(trimmed, "CMD") || strings.HasPrefix(trimmed, "ENTRYPOINT")) && !exposeAdded {
			updated = append(updated, "")
			updated = append(updated, fmt.Sprintf("EXPOSE %s", port))
			updated = append(updated, "")
			exposeAdded = true
		}

		updated = append(updated, line)
	}

	// If no CMD/ENTRYPOINT found, add EXPOSE at the end
	if !exposeAdded {
		updated = append(updated, "")
		updated = append(updated, fmt.Sprintf("EXPOSE %s", port))
	}

	newContent := strings.Join(updated, "\n")

	err = os.WriteFile(dockerfilePath, []byte(newContent), 0644)
	if err != nil {
		return false, errors.New(errors.CodeIoError, "dockerfile", "failed to write updated Dockerfile", err)
	}

	logger.Info("Applied port expose fix to Dockerfile", "file", dockerfilePath, "port", port)
	return true, nil
}

func applyEnvironmentFix(step string, logger *slog.Logger) (bool, error) {
	// For now, just log the suggestion - environment fixes are context-specific
	logger.Info("Environment fix suggested (manual intervention required)", "suggestion", step)
	return false, nil
}

func applyPermissionFix(step string, logger *slog.Logger) (bool, error) {
	// Common permission fixes for build scripts
	scriptPaths := []string{"./gradlew", "./mvnw", "./build.sh", "./entrypoint.sh"}

	fixesApplied := false
	for _, path := range scriptPaths {
		if _, err := os.Stat(path); err == nil {
			// Make script executable
			if err := os.Chmod(path, 0755); err == nil {
				logger.Info("Applied permission fix", "file", path, "mode", "0755")
				fixesApplied = true
			}
		}
	}

	return fixesApplied, nil
}

// applyPatternBasedFixes applies automated fixes based on error pattern recognition
// This serves as a fallback when LLM analysis is unavailable
func applyPatternBasedFixes(ctx context.Context, operation string, errorMsg string, logger *slog.Logger) bool {
	// Truncate error message for logging (inline implementation)
	errorPreview := errorMsg
	if len(errorMsg) > 100 {
		errorPreview = errorMsg[:100] + "..."
	}
	logger.Debug("Applying pattern-based auto-fixes", "operation", operation, "error_preview", errorPreview)

	fixesApplied := false
	errorLower := strings.ToLower(errorMsg)

	// Maven build tool fixes
	if strings.Contains(errorLower, "mvn") && (strings.Contains(errorLower, "command not found") || strings.Contains(errorLower, "exit code: 127")) {
		logger.Info("Detected Maven missing error, applying Maven Dockerfile fix")
		if applied, _ := applyMavenDockerfileFix("Install Maven for build", logger); applied {
			fixesApplied = true
		}
	}

	// Gradle build tool fixes
	if strings.Contains(errorLower, "gradle") && strings.Contains(errorLower, "command not found") {
		logger.Info("Detected Gradle missing error, applying Gradle Dockerfile fix")
		if applied, _ := applyGradleDockerfileFix("Install Gradle for build", logger); applied {
			fixesApplied = true
		}
	}

	// Base image fixes for Java projects
	if strings.Contains(errorLower, "dockerfile") && strings.Contains(errorLower, "java") {
		logger.Info("Detected Java Dockerfile issue, applying base image fix")
		if applied, _ := applyDockerfileBaseFix("Update base image for Java Maven", logger); applied {
			fixesApplied = true
		}
	}

	// Port exposure fixes
	if strings.Contains(errorLower, "port") && (strings.Contains(errorLower, "connection") || strings.Contains(errorLower, "refused")) {
		logger.Info("Detected port connection issue, applying port expose fix")
		if applied, _ := applyPortExposeFix("Expose port 8080", logger); applied {
			fixesApplied = true
		}
	}

	// Permission fixes for executable scripts
	if strings.Contains(errorLower, "permission denied") || strings.Contains(errorLower, "not executable") {
		logger.Info("Detected permission issue, applying permission fixes")
		if applied, _ := applyPermissionFix("Fix script permissions", logger); applied {
			fixesApplied = true
		}
	}

	// Kubernetes deployment specific fixes
	if strings.Contains(errorLower, "image pull") && strings.Contains(errorLower, "localhost:5001") {
		logger.Info("Detected image pull issue, applying image registry fix")
		if applied := applyImageRegistryFix(ctx, operation, errorMsg, logger); applied {
			fixesApplied = true
		}
	}

	// Node readiness and scheduling fixes
	if strings.Contains(errorLower, "nodes are available") && strings.Contains(errorLower, "taint") {
		logger.Info("Detected node scheduling issue, applying node readiness fix")
		if applied := applyNodeReadinessFix(ctx, logger); applied {
			fixesApplied = true
		}
	}

	// Pod validation and resource fixes
	if strings.Contains(errorLower, "pods ready") && strings.Contains(operation, "validate") {
		logger.Info("Detected pod readiness issue, applying pod validation fix")
		if applied := applyPodValidationFix(ctx, errorMsg, logger); applied {
			fixesApplied = true
		}
	}

	if fixesApplied {
		logger.Info("Pattern-based auto-fixes completed", "operation", operation)
	} else {
		logger.Debug("No applicable pattern-based fixes found", "operation", operation)
	}

	return fixesApplied
}

func applyImageRegistryFix(ctx context.Context, operation string, errorMsg string, logger *slog.Logger) bool {
	// For kind clusters, ensure image is loaded into kind cluster

	logger.Info("Attempting to load image into kind cluster")

	const registryPrefix = "localhost:5001/"
	const kindClusterName = "container-kit"

	// Extract image name from error message if possible
	// Look for patterns like "localhost:5001/imagename:tag"
	re := regexp.MustCompile(registryPrefix + `([^:\s]+):([^\s]+)`)
	matches := re.FindStringSubmatch(errorMsg)

	if len(matches) >= 3 {
		logger.Warn("Could not extract image name from error message")
		logger.Warn("Manual intervention needed: Load image into kind cluster with 'kind load docker-image'")
		return false
	}

	imageName, imageTag := matches[1], matches[2]
	localImageRef := fmt.Sprintf("%s:%s", imageName, imageTag)
	registryImageRef := fmt.Sprintf("localhost:5001/%s:%s", imageName, imageTag)

	logger.Info("Detected image reference, attempting kind load",
		"local_image", localImageRef,
		"registry_image", registryImageRef)

	// Try to load image into kind cluster
	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", localImageRef, "--name", kindClusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to load image into kind cluster automatically",
			"error", err,
			"output", string(output))
		logger.Warn("Manual intervention needed: Load image into kind cluster with 'kind load docker-image'")
		return false
	}

	logger.Info("Successfully loaded image into kind cluster", "image", localImageRef)
	return true

}

func applyNodeReadinessFix(ctx context.Context, logger *slog.Logger) bool {
	logger.Info("Detected node readiness/taint issue")
	// Check if we can wait for node to become ready or remove taints
	logger.Warn("Manual intervention needed: Check node status and remove taints if necessary")

	// For kind clusters, nodes usually become ready after a short wait
	logger.Info("Applying wait strategy for node readiness")
	time.Sleep(5 * time.Second)
	return true // Return true to indicate we applied a wait strategy
}

func applyPodValidationFix(ctx context.Context, errorMsg string, logger *slog.Logger) bool {
	fixesApplied := false

	// If port is 0, this is a common issue
	if strings.Contains(errorMsg, "Port: 0") {
		logger.Info("Detected port 0 issue, applying port configuration fix")
		if applied, _ := applyPortExposeFix("Set default port 8080", logger); applied {
			fixesApplied = true
		}
	}

	// If image pull is mentioned, try image-related fixes
	if strings.Contains(errorMsg, "Pulling image") {
		logger.Info("Image pull detected, applying image availability fix")
		// Apply a wait strategy for image pull to complete
		logger.Info("Applying wait strategy for image pull completion")
		time.Sleep(3 * time.Second)
		fixesApplied = true
	}

	return fixesApplied
}
