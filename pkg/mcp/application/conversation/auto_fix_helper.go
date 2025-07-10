package conversation

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// AutoFixHelper provides automatic error recovery and fixing capabilities
type AutoFixHelper struct {
	logger        *slog.Logger
	fixes         map[string]FixStrategy
	sessionStore  services.SessionStore
	sessionState  services.SessionState
	fileAccess    services.FileAccessService
	fixHistory    map[string][]FixAttempt
	chainExecutor *FixChainExecutor
}

// FixStrategy defines a function that attempts to fix an error
type FixStrategy func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error)

// FixAttempt represents an attempt to fix an error
type FixAttempt struct {
	ToolName   string    `json:"tool_name"`
	Error      string    `json:"error"`
	Strategy   string    `json:"strategy"`
	Successful bool      `json:"successful"`
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"session_id"`
}

// SessionContext represents session context for auto-fix decisions
type SessionContext struct {
	SessionID    string                 `json:"session_id"`
	Language     string                 `json:"language,omitempty"`
	Framework    string                 `json:"framework,omitempty"`
	Tools        []string               `json:"tools,omitempty"`
	RecentErrors []string               `json:"recent_errors,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// NewAutoFixHelper creates a new auto-fix helper
func NewAutoFixHelper(
	logger *slog.Logger,
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	fileAccess services.FileAccessService,
) *AutoFixHelper {
	helper := &AutoFixHelper{
		logger:       logger,
		fixes:        make(map[string]FixStrategy),
		sessionStore: sessionStore,
		sessionState: sessionState,
		fileAccess:   fileAccess,
		fixHistory:   make(map[string][]FixAttempt),
	}

	// Register common fix strategies
	helper.registerCommonFixes()

	// Initialize fix chain executor
	helper.chainExecutor = NewFixChainExecutor(logger, helper)

	return helper
}

// registerCommonFixes registers common error fix strategies
func (h *AutoFixHelper) registerCommonFixes() {
	// Fix for missing Dockerfile
	h.fixes["dockerfile_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "Dockerfile not found") || strings.Contains(err.Error(), "dockerfile not found") {
			h.logger.Info("Attempting to fix missing Dockerfile error")

			// Try common Dockerfile locations
			buildArgs, ok := args.(*BuildArgs)
			if !ok {
				// Try to extract from map
				if argsMap, ok := args.(map[string]interface{}); ok {
					buildArgs = &BuildArgs{}
					if dp, ok := argsMap["dockerfile_path"].(string); ok {
						buildArgs.DockerfilePath = dp
					}
					if cp, ok := argsMap["context_path"].(string); ok {
						buildArgs.ContextPath = cp
					}
				} else {
					return nil, err
				}
			}

			// Try alternative Dockerfile names
			alternatives := []string{"dockerfile", "Dockerfile.dev", "docker/Dockerfile", ".dockerfile"}
			originalPath := buildArgs.DockerfilePath

			for _, alt := range alternatives {
				buildArgs.DockerfilePath = alt
				h.logger.Debug("Trying alternative Dockerfile path",
					slog.String("path", alt))

				// Create new tool input with updated args
				toolInput := api.ToolInput{
					Data: map[string]interface{}{
						"dockerfile_path": alt,
						"context_path":    buildArgs.ContextPath,
					},
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed Dockerfile location",
						slog.String("original", originalPath),
						slog.String("fixed", alt))
					return result, nil
				}
			}
		}
		return nil, err
	}

	// Fix for missing build context
	h.fixes["context_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "context path does not exist") || strings.Contains(err.Error(), "context path error") {
			h.logger.Info("Attempting to fix missing build context error")

			buildArgs, ok := args.(*BuildArgs)
			if !ok {
				// Try to extract from map
				if argsMap, ok := args.(map[string]interface{}); ok {
					buildArgs = &BuildArgs{}
					if cp, ok := argsMap["context_path"].(string); ok {
						buildArgs.ContextPath = cp
					}
				} else {
					return nil, err
				}
			}

			// Try current directory as context
			originalContext := buildArgs.ContextPath
			buildArgs.ContextPath = "."

			// Create new tool input with updated args
			toolInput := api.ToolInput{
				Data: map[string]interface{}{
					"context_path": ".",
				},
			}

			if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
				h.logger.Info("Auto-fixed build context to current directory",
					slog.String("original", originalContext))
				return result, nil
			}
		}
		return nil, err
	}

	// Fix for authentication errors
	h.fixes["auth_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "authentication required") || strings.Contains(err.Error(), "unauthorized") {
			h.logger.Warn("Authentication required, cannot auto-fix",
				slog.String("error", err.Error()))

			// Return a helpful error message
			return nil, errors.NewError().
				Code(errors.FILE_PERMISSION_DENIED).
				Type(errors.ErrTypePermission).
				Message("authentication required: please ensure Docker is logged in to the registry").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for network errors
	h.fixes["network_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "network") || strings.Contains(err.Error(), "connection") {
			h.logger.Info("Network error detected, suggesting retry")

			// Could implement retry logic here
			return nil, errors.NewError().
				Code(errors.NETWORK_TIMEOUT).
				Type(errors.ErrTypeNetwork).
				Messagef("network error: %w. Please check your internet connection and try again", err).
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for permission errors
	h.fixes["permission_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "access denied") {
			h.logger.Warn("Permission error detected",
				slog.String("error", err.Error()))

			return nil, errors.NewError().
				Code(errors.FILE_PERMISSION_DENIED).
				Type(errors.ErrTypePermission).
				Message("permission denied: ensure you have the necessary permissions to perform this operation").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for disk space errors
	h.fixes["disk_space_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "no space left on device") || strings.Contains(err.Error(), "disk full") {
			h.logger.Error("Disk space error detected",
				slog.String("error", err.Error()))

			return nil, errors.NewError().
				Code(errors.SYSTEM_ERROR).
				Type(errors.ErrTypeInternal).
				Message("insufficient disk space: please free up disk space and try again. Consider running 'docker system prune'").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// NEW PHASE 4 FIX STRATEGIES

	// Fix for invalid port errors
	h.fixes["invalid_port"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "invalid port") || strings.Contains(err.Error(), "port out of range") {
			h.logger.Info("Invalid port error detected, trying common ports")

			// Try common ports based on tool type and language context
			commonPorts := []int{8080, 3000, 5000, 8000, 9000, 80, 443}

			for _, port := range commonPorts {
				h.logger.Debug("Trying alternative port", slog.Int("port", port))

				// Create new tool input with updated port
				toolInput := api.ToolInput{
					Data: h.updateArgsWithPort(args, port),
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed port configuration",
						slog.Int("fixed_port", port))
					return result, nil
				}
			}
		}
		return nil, err
	}

	// Fix for missing dependency errors
	h.fixes["missing_dependency"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "package not found") ||
			strings.Contains(err.Error(), "module not found") ||
			strings.Contains(err.Error(), "dependency not found") {
			h.logger.Info("Missing dependency error detected")

			// Extract package name from error and suggest installation
			suggestion := h.extractPackageSuggestion(err.Error())

			return nil, errors.NewError().
				Code(errors.RESOURCE_NOT_FOUND).
				Type(errors.ErrTypeIO).
				Messagef("missing dependency detected: %s. %s", err.Error(), suggestion).
				Suggestion(suggestion).
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for Dockerfile syntax errors
	h.fixes["dockerfile_syntax_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "dockerfile parse error") ||
			strings.Contains(err.Error(), "syntax error") ||
			strings.Contains(err.Error(), "unknown instruction") {
			h.logger.Info("Dockerfile syntax error detected, applying common fixes")

			// Apply common Dockerfile syntax fixes
			if h.isGenerateDockerfileTool(tool) {
				return h.retryWithFixedDockerfileOptions(ctx, tool, args, err)
			}

			return nil, errors.NewError().
				Code(errors.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("dockerfile syntax error: %w. Common fixes: check instruction spelling, ensure proper formatting, verify base image exists", err).
				Suggestion("Use 'generate_dockerfile' tool to create a valid Dockerfile").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for resource limit errors
	h.fixes["resource_limits"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "memory limit") ||
			strings.Contains(err.Error(), "cpu limit") ||
			strings.Contains(err.Error(), "resource limit") {
			h.logger.Info("Resource limit error detected, adjusting limits")

			// Suggest reduced resource requirements
			return nil, errors.NewError().
				Code(errors.RESOURCE_EXHAUSTED).
				Type(errors.ErrTypeResource).
				Messagef("resource limit exceeded: %w. Try reducing memory/CPU limits or optimize your application", err).
				Suggestion("Consider using multi-stage builds, smaller base images, or reducing resource requirements").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for health check failures
	h.fixes["health_check_failure"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "health check failed") ||
			strings.Contains(err.Error(), "health endpoint") {
			h.logger.Info("Health check failure detected, trying alternative strategies")

			// Try different health check approaches
			alternatives := []string{
				"/health",
				"/healthz",
				"/ping",
				"/status",
				"/api/health",
			}

			for _, endpoint := range alternatives {
				h.logger.Debug("Trying alternative health endpoint", slog.String("endpoint", endpoint))

				toolInput := api.ToolInput{
					Data: h.updateArgsWithHealthEndpoint(args, endpoint),
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed health check endpoint",
						slog.String("endpoint", endpoint))
					return result, nil
				}
			}

			return nil, errors.NewError().
				Code(errors.SYSTEM_UNAVAILABLE).
				Type(errors.ErrTypeIO).
				Messagef("health check failed: %w. Consider implementing a health endpoint or adjusting health check configuration", err).
				Suggestion("Add a /health endpoint to your application or use a simpler health check like 'CMD exit 0'").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for image not found errors
	h.fixes["image_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "image not found") ||
			strings.Contains(err.Error(), "pull access denied") ||
			strings.Contains(err.Error(), "repository does not exist") {
			h.logger.Info("Image not found error detected, trying alternative base images")

			// Try alternative base images based on language
			alternatives := h.getAlternativeBaseImages(args)

			for _, image := range alternatives {
				h.logger.Debug("Trying alternative base image", slog.String("image", image))

				toolInput := api.ToolInput{
					Data: h.updateArgsWithBaseImage(args, image),
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed base image",
						slog.String("image", image))
					return result, nil
				}
			}

			return nil, errors.NewError().
				Code(errors.RESOURCE_NOT_FOUND).
				Type(errors.ErrTypeContainer).
				Messagef("base image not found: %w. Consider using a more common base image like 'ubuntu:latest' or 'alpine:latest'", err).
				Suggestion("Use 'docker search <image>' to find available images").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for port already in use errors
	h.fixes["port_in_use"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "port already in use") ||
			strings.Contains(err.Error(), "address already in use") {
			h.logger.Info("Port in use error detected, finding alternative port")

			// Try ports in higher ranges to avoid conflicts
			alternativePorts := []int{8081, 8082, 8083, 3001, 3002, 5001, 5002, 9001, 9002}

			for _, port := range alternativePorts {
				h.logger.Debug("Trying alternative port", slog.Int("port", port))

				toolInput := api.ToolInput{
					Data: h.updateArgsWithPort(args, port),
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed port conflict",
						slog.Int("new_port", port))
					return result, nil
				}
			}

			return nil, errors.NewError().
				Code(errors.RESOURCE_LOCKED).
				Type(errors.ErrTypeNetwork).
				Messagef("port conflict: %w. Stop other services using the port or use a different port", err).
				Suggestion("Use 'netstat -tlnp | grep <port>' to find what's using the port").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for timeout errors
	h.fixes["timeout_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "deadline exceeded") {
			h.logger.Info("Timeout error detected")

			return nil, errors.NewError().
				Code(errors.TIMEOUT).
				Type(errors.ErrTypeTimeout).
				Messagef("operation timed out: %w. Try again or increase timeout settings", err).
				Suggestion("Check network connectivity and consider increasing timeout values").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for registry authentication errors
	h.fixes["registry_auth_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "registry") &&
			(strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "authentication")) {
			h.logger.Warn("Registry authentication error detected")

			return nil, errors.NewError().
				Code(errors.FILE_PERMISSION_DENIED).
				Type(errors.ErrTypePermission).
				Messagef("registry authentication failed: %w", err).
				Suggestion("Run 'docker login <registry>' to authenticate with the container registry").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for manifest generation errors
	h.fixes["manifest_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "manifest") &&
			(strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "generation failed")) {
			h.logger.Info("Manifest generation error detected, applying fixes")

			// Try with simpler manifest configuration
			if h.isManifestTool(tool) {
				return h.retryWithSimplifiedManifest(ctx, tool, args, err)
			}

			return nil, errors.NewError().
				Code(errors.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("manifest generation failed: %w. Using simplified configuration", err).
				Suggestion("Try with basic deployment configuration or check image name format").
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for deployment errors
	h.fixes["deployment_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "deployment failed") ||
			strings.Contains(err.Error(), "pod failed") ||
			strings.Contains(err.Error(), "imagepullbackoff") {
			h.logger.Info("Deployment error detected")

			suggestion := "Check image name and registry access. Ensure the image was pushed successfully"
			if strings.Contains(err.Error(), "imagepullbackoff") {
				suggestion = "Image pull failed. Verify image exists and registry is accessible"
			}

			return nil, errors.NewError().
				Code(errors.SYSTEM_ERROR).
				Type(errors.ErrTypeIO).
				Messagef("deployment failed: %w", err).
				Suggestion(suggestion).
				WithLocation().
				Build()
		}
		return nil, err
	}

	// Fix for build cache errors
	h.fixes["build_cache_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "cache") &&
			(strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "corrupted")) {
			h.logger.Info("Build cache error detected, clearing cache")

			return nil, errors.NewError().
				Code(errors.SYSTEM_ERROR).
				Type(errors.ErrTypeInternal).
				Messagef("build cache error: %w. Clear Docker build cache with 'docker builder prune'", err).
				Suggestion("Run 'docker builder prune' to clear build cache and try again").
				WithLocation().
				Build()
		}
		return nil, err
	}
}

// AttemptFix attempts to fix an error using registered strategies with context awareness
func (h *AutoFixHelper) AttemptFix(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	if err == nil {
		return nil, nil
	}

	sessionID := h.extractSessionID(args)
	h.logger.Debug("Attempting context-aware auto-fix",
		slog.String("session_id", sessionID),
		slog.String("error", err.Error()),
		slog.String("tool", tool.Name()))

	// Get session context for smarter decisions
	sessionCtx, contextErr := h.buildSessionContext(ctx, sessionID)
	if contextErr != nil {
		h.logger.Warn("Failed to build session context, using basic fixes", "error", contextErr)
		return h.attemptBasicFix(ctx, tool, args, err)
	}

	// Check if this error would benefit from a fix chain
	if h.shouldUseFixChain(sessionCtx, tool, err) {
		chainResult, chainErr := h.chainExecutor.ExecuteChain(ctx, tool, args, err)
		if chainErr == nil && chainResult != nil && chainResult.Success {
			h.recordFixAttempt(sessionID, tool.Name(), err.Error(), "chain", true)
			h.logger.Info("Fix chain successful",
				slog.String("chain", chainResult.ChainName),
				slog.Duration("duration", chainResult.TotalDuration))
			return chainResult.FinalResult, nil
		} else if chainResult != nil {
			h.logger.Warn("Fix chain failed",
				slog.String("chain", chainResult.ChainName),
				slog.String("reason", chainResult.FailureReason))
		}
	}

	// Try context-aware fixes first
	result, fixErr := h.attemptContextAwareFix(ctx, tool, args, err, sessionCtx)
	if fixErr == nil && result != nil {
		h.recordFixAttempt(sessionID, tool.Name(), err.Error(), "context-aware", true)
		return result, nil
	}

	// Fall back to basic fixes
	result, fixErr = h.attemptBasicFix(ctx, tool, args, err)
	if fixErr == nil && result != nil {
		h.recordFixAttempt(sessionID, tool.Name(), err.Error(), "basic", true)
		return result, nil
	}

	// Record failed attempt
	h.recordFixAttempt(sessionID, tool.Name(), err.Error(), "all", false)

	// No fix worked, return original error
	h.logger.Debug("No auto-fix strategy succeeded",
		slog.String("error", err.Error()))
	return nil, err
}

// RegisterFix registers a custom fix strategy
func (h *AutoFixHelper) RegisterFix(name string, strategy FixStrategy) {
	h.fixes[name] = strategy
}

// HasFix checks if a fix strategy exists for the given name
func (h *AutoFixHelper) HasFix(name string) bool {
	_, exists := h.fixes[name]
	return exists
}

// ListFixes returns the names of all registered fix strategies
func (h *AutoFixHelper) ListFixes() []string {
	fixes := make([]string, 0, len(h.fixes))
	for name := range h.fixes {
		fixes = append(fixes, name)
	}
	return fixes
}

// Context-aware helper methods for fix strategies

// updateArgsWithPort updates arguments with a new port number
func (h *AutoFixHelper) updateArgsWithPort(args interface{}, port int) map[string]interface{} {
	data := make(map[string]interface{})

	// Copy existing args
	if argsMap, ok := args.(map[string]interface{}); ok {
		for k, v := range argsMap {
			data[k] = v
		}
	}

	// Update port-related fields
	data["port"] = port
	if _, exists := data["image_name"]; exists {
		// For container operations, might need to update exposed ports
		data["exposed_port"] = port
	}

	return data
}

// updateArgsWithHealthEndpoint updates arguments with a new health check endpoint
func (h *AutoFixHelper) updateArgsWithHealthEndpoint(args interface{}, endpoint string) map[string]interface{} {
	data := make(map[string]interface{})

	// Copy existing args
	if argsMap, ok := args.(map[string]interface{}); ok {
		for k, v := range argsMap {
			data[k] = v
		}
	}

	// Update health check related fields
	data["health_endpoint"] = endpoint
	data["health_path"] = endpoint

	return data
}

// updateArgsWithBaseImage updates arguments with a new base image
func (h *AutoFixHelper) updateArgsWithBaseImage(args interface{}, image string) map[string]interface{} {
	data := make(map[string]interface{})

	// Copy existing args
	if argsMap, ok := args.(map[string]interface{}); ok {
		for k, v := range argsMap {
			data[k] = v
		}
	}

	// Update base image related fields
	data["base_image"] = image
	if _, exists := data["language"]; exists {
		// For Dockerfile generation, update base image
		data["base_image"] = image
	}

	return data
}

// getAlternativeBaseImages returns alternative base images based on language context
func (h *AutoFixHelper) getAlternativeBaseImages(args interface{}) []string {
	var language string

	if argsMap, ok := args.(map[string]interface{}); ok {
		if lang, ok := argsMap["language"].(string); ok {
			language = lang
		}
	}

	switch language {
	case "go":
		return []string{"golang:alpine", "golang:1.21-alpine", "alpine:latest"}
	case "javascript", "typescript":
		return []string{"node:alpine", "node:18-alpine", "node:16-alpine"}
	case "python":
		return []string{"python:alpine", "python:3.11-slim", "python:3.10-slim"}
	case "java":
		return []string{"openjdk:alpine", "openjdk:17-alpine", "amazoncorretto:17-alpine"}
	default:
		return []string{"alpine:latest", "ubuntu:22.04", "debian:bullseye-slim"}
	}
}

// extractPackageSuggestion extracts package name from error and provides installation suggestion
func (h *AutoFixHelper) extractPackageSuggestion(errorMsg string) string {
	errorLower := strings.ToLower(errorMsg)

	// Try to extract package name from common error patterns
	if strings.Contains(errorLower, "package") {
		// Look for package name patterns
		if strings.Contains(errorLower, "not found") {
			return "Check package name spelling and ensure it's listed in your dependency file (package.json, requirements.txt, go.mod, etc.)"
		}
	}

	if strings.Contains(errorLower, "module") {
		return "Ensure the module is installed and available in the module path"
	}

	return "Install the missing dependency using your package manager (npm install, pip install, go get, etc.)"
}

// isGenerateDockerfileTool checks if the tool is the Dockerfile generation tool
func (h *AutoFixHelper) isGenerateDockerfileTool(tool api.Tool) bool {
	return tool.Name() == "generate_dockerfile"
}

// isManifestTool checks if the tool is a manifest generation tool
func (h *AutoFixHelper) isManifestTool(tool api.Tool) bool {
	return tool.Name() == "generate_manifests" || strings.Contains(tool.Name(), "manifest")
}

// retryWithFixedDockerfileOptions retries Dockerfile generation with fixed options
func (h *AutoFixHelper) retryWithFixedDockerfileOptions(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	h.logger.Info("Retrying Dockerfile generation with simplified options")

	data := make(map[string]interface{})

	// Copy existing args
	if argsMap, ok := args.(map[string]interface{}); ok {
		for k, v := range argsMap {
			data[k] = v
		}
	}

	// Apply fixes for common Dockerfile issues
	data["multi_stage"] = false // Simplify to single-stage
	data["optimize"] = false    // Disable advanced optimizations

	// Use more conservative base images
	if language, ok := data["language"].(string); ok {
		switch language {
		case "go":
			data["base_image"] = "golang:alpine"
		case "javascript", "typescript":
			data["base_image"] = "node:alpine"
		case "python":
			data["base_image"] = "python:alpine"
		case "java":
			data["base_image"] = "openjdk:alpine"
		}
	}

	toolInput := api.ToolInput{Data: data}
	return tool.Execute(ctx, toolInput)
}

// retryWithSimplifiedManifest retries manifest generation with simplified configuration
func (h *AutoFixHelper) retryWithSimplifiedManifest(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	h.logger.Info("Retrying manifest generation with simplified configuration")

	data := make(map[string]interface{})

	// Copy existing args
	if argsMap, ok := args.(map[string]interface{}); ok {
		for k, v := range argsMap {
			data[k] = v
		}
	}

	// Simplify manifest configuration
	data["replicas"] = 1          // Single replica
	data["strategy"] = "Recreate" // Simple deployment strategy
	delete(data, "resources")     // Remove resource limits
	delete(data, "affinity")      // Remove affinity rules
	delete(data, "tolerations")   // Remove tolerations

	// Use default namespace if not specified
	if _, exists := data["namespace"]; !exists {
		data["namespace"] = "default"
	}

	toolInput := api.ToolInput{Data: data}
	return tool.Execute(ctx, toolInput)
}

// shouldUseFixChain determines if a fix chain should be used based on session context
func (h *AutoFixHelper) shouldUseFixChain(sessionCtx *SessionContext, tool api.Tool, err error) bool {
	if sessionCtx == nil {
		return false
	}

	// Use fix chain for complex errors with multiple failures
	if history, exists := h.fixHistory[sessionCtx.SessionID]; exists {
		failureCount := 0
		for _, attempt := range history {
			if attempt.ToolName == tool.Name() && !attempt.Successful {
				failureCount++
			}
		}

		// Use chain if we have multiple failures (3 or more)
		if failureCount >= 3 {
			return true
		}
	}

	// Use chain for dockerfile syntax errors or complex build errors
	errorMsg := strings.ToLower(err.Error())
	if strings.Contains(errorMsg, "dockerfile") && strings.Contains(errorMsg, "syntax") {
		return true
	}

	if strings.Contains(errorMsg, "build failed") && strings.Contains(errorMsg, "multiple") {
		return true
	}

	return false
}

// shouldSkipRepeatedFix determines if we should skip a repeated fix attempt
func (h *AutoFixHelper) shouldSkipRepeatedFix(sessionCtx *SessionContext, toolName string, errorType string) bool {
	if sessionCtx == nil {
		return false
	}

	if history, exists := h.fixHistory[sessionCtx.SessionID]; exists {
		failureCount := 0
		for _, attempt := range history {
			if attempt.ToolName == toolName && strings.Contains(attempt.Error, errorType) && !attempt.Successful {
				failureCount++
			}
		}

		// Skip if we've already failed 3 times for this specific error type
		return failureCount >= 3
	}

	return false
}

// Helper methods for session context and fix attempts

// buildSessionContext builds session context for smart decision making
func (h *AutoFixHelper) buildSessionContext(ctx context.Context, sessionID string) (*SessionContext, error) {
	sessionCtx := &SessionContext{
		SessionID: sessionID,
		Metadata:  make(map[string]interface{}),
	}

	// Get session metadata if available
	if metadata, err := h.sessionState.GetSessionMetadata(ctx, sessionID); err == nil {
		sessionCtx.Metadata = metadata

		// Extract language and framework from metadata
		if lang, ok := metadata["language"].(string); ok {
			sessionCtx.Language = lang
		}
		if framework, ok := metadata["framework"].(string); ok {
			sessionCtx.Framework = framework
		}
		if tools, ok := metadata["tools"].([]string); ok {
			sessionCtx.Tools = tools
		}
	}

	// Populate recent errors from fix history
	if history, exists := h.fixHistory[sessionID]; exists {
		recentErrors := make([]string, 0, len(history))
		for _, attempt := range history {
			if len(recentErrors) < 10 { // Limit to last 10 errors
				recentErrors = append(recentErrors, attempt.Error)
			}
		}
		sessionCtx.RecentErrors = recentErrors
	}

	return sessionCtx, nil
}

// recordFixAttempt records a fix attempt for tracking
func (h *AutoFixHelper) recordFixAttempt(sessionID, toolName, errorMsg, strategy string, successful bool) {
	if h.fixHistory == nil {
		h.fixHistory = make(map[string][]FixAttempt)
	}

	attempt := FixAttempt{
		ToolName:   toolName,
		Error:      errorMsg,
		Strategy:   strategy,
		Successful: successful,
		Timestamp:  time.Now(),
		SessionID:  sessionID,
	}

	h.fixHistory[sessionID] = append(h.fixHistory[sessionID], attempt)

	// Keep only last 50 attempts per session to avoid memory leaks
	if len(h.fixHistory[sessionID]) > 50 {
		h.fixHistory[sessionID] = h.fixHistory[sessionID][len(h.fixHistory[sessionID])-50:]
	}
}

// extractSessionID extracts session ID from arguments
func (h *AutoFixHelper) extractSessionID(args interface{}) string {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if sessionID, ok := argsMap["session_id"].(string); ok {
			return sessionID
		}
	}

	// Try to extract from other argument types
	if buildArgs, ok := args.(*BuildArgs); ok && buildArgs != nil {
		// Assuming BuildArgs has a SessionID field, if not we'll use a default
		return "default-session"
	}

	return "default-session"
}

// attemptBasicFix attempts basic fixes using individual strategies
func (h *AutoFixHelper) attemptBasicFix(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	// Try each fix strategy
	for name, strategy := range h.fixes {
		h.logger.Debug("Trying fix strategy", slog.String("strategy", name))

		if result, fixErr := strategy(ctx, tool, args, err); fixErr == nil && result != nil {
			h.logger.Info("Auto-fix successful", slog.String("strategy", name))
			return result, nil
		}
	}

	return nil, err
}

// attemptContextAwareFix attempts context-aware fixes based on session history
func (h *AutoFixHelper) attemptContextAwareFix(ctx context.Context, tool api.Tool, args interface{}, err error, sessionCtx *SessionContext) (interface{}, error) {
	if sessionCtx == nil {
		return h.attemptBasicFix(ctx, tool, args, err)
	}

	errorMsg := strings.ToLower(err.Error())
	toolName := tool.Name()

	// Check if we should skip repeated fixes
	if h.shouldSkipRepeatedFix(sessionCtx, toolName, errorMsg) {
		h.logger.Info("Skipping repeated fix attempt",
			slog.String("tool", toolName),
			slog.String("error_type", errorMsg))
		return nil, err
	}

	// Try language-specific fixes first if we know the language
	if sessionCtx.Language != "" {
		if result, fixErr := h.tryLanguageSpecificFixes(ctx, tool, args, err, sessionCtx.Language); fixErr == nil && result != nil {
			return result, nil
		}
	}

	// Fall back to basic fixes
	return h.attemptBasicFix(ctx, tool, args, err)
}

// tryLanguageSpecificFixes attempts fixes specific to the detected language
func (h *AutoFixHelper) tryLanguageSpecificFixes(ctx context.Context, tool api.Tool, args interface{}, err error, language string) (interface{}, error) {
	errorMsg := strings.ToLower(err.Error())

	// Language-specific port mappings
	languagePorts := map[string][]int{
		"go":         {8080, 3000, 5000},
		"javascript": {3000, 8080, 5000},
		"python":     {5000, 8000, 8080},
		"java":       {8080, 8090, 9000},
	}

	if strings.Contains(errorMsg, "port") {
		if ports, exists := languagePorts[language]; exists {
			for _, port := range ports {
				toolInput := api.ToolInput{
					Data: h.updateArgsWithPort(args, port),
				}
				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Language-specific port fix successful",
						slog.String("language", language),
						slog.Int("port", port))
					return result, nil
				}
			}
		}
	}

	return nil, err
}

// BuildArgs represents build arguments for dockerfile operations
type BuildArgs struct {
	DockerfilePath string
	ContextPath    string
	ImageName      string
	SessionID      string
}

// GetFixChainStatus returns the current status of fix chains
func (h *AutoFixHelper) GetFixChainStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Available chains
	availableChains := map[string]string{
		"dockerfile_build":      "Handles Dockerfile build errors with multi-step fixes",
		"dependency_resolution": "Resolves dependency and package issues",
		"manifest_generation":   "Fixes Kubernetes manifest generation problems",
		"image_registry":        "Handles image registry and push/pull issues",
	}
	status["available_chains"] = availableChains

	// Calculate usage statistics
	totalAttempts := 0
	successfulAttempts := 0

	for _, history := range h.fixHistory {
		for _, attempt := range history {
			if attempt.Strategy == "chain" {
				totalAttempts++
				if attempt.Successful {
					successfulAttempts++
				}
			}
		}
	}

	usageStats := map[string]interface{}{
		"total_chain_attempts": totalAttempts,
		"successful_chains":    successfulAttempts,
		"success_rate":         h.calculateSuccessRate(successfulAttempts, totalAttempts),
	}
	status["usage_stats"] = usageStats

	// Chain availability status
	chainStatus := map[string]interface{}{
		"enabled": h.chainExecutor != nil,
		"healthy": h.chainExecutor != nil, // Simple health check
	}
	status["chain_status"] = chainStatus

	return status
}

// calculateSuccessRate calculates the success rate as a percentage
func (h *AutoFixHelper) calculateSuccessRate(successful int, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(successful) / float64(total)) * 100.0
}
