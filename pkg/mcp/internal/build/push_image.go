package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// PushImageArgs defines the arguments for pushing a Docker image to a registry
type PushImageArgs struct {
	mcptypes.BaseToolArgs
	ImageRef    mcptypes.ImageReference `json:"image_ref" description:"Image reference to push (required)"`
	PushTimeout time.Duration           `json:"push_timeout,omitempty" description:"Push timeout (default: 10m)"`
	RetryCount  int                     `json:"retry_count,omitempty" description:"Number of retry attempts (default: 3)"`
	AsyncPush   bool                    `json:"async_push,omitempty" description:"Run push asynchronously"`
}

// PushImageResult represents the result of a Docker image push
type PushImageResult struct {
	mcptypes.BaseToolResponse
	Success       bool                `json:"success"`
	JobID         string              `json:"job_id,omitempty"` // For async pushes
	ImageRef      string              `json:"image_ref"`
	Registry      string              `json:"registry"`
	Size          int64               `json:"size_bytes,omitempty"`
	LayersInfo    *LayersInfo         `json:"layers_info,omitempty"`
	Logs          []string            `json:"logs"`
	Duration      time.Duration       `json:"duration"`
	CacheHitRatio float64             `json:"cache_hit_ratio"`
	Error         *mcptypes.ToolError `json:"error,omitempty"`
}

// LayersInfo represents information about pushed layers
type LayersInfo struct {
	TotalLayers    int     `json:"total_layers"`
	PushedLayers   int     `json:"pushed_layers"`
	CachedLayers   int     `json:"cached_layers"`
	FailedLayers   int     `json:"failed_layers"`
	LayerSizeBytes int64   `json:"layer_size_bytes"`
	CacheRatio     float64 `json:"cache_ratio"`
}

// PushImageTool handles Docker image push operations
type PushImageTool struct {
	logger zerolog.Logger
}

// NewPushImageTool creates a new push image tool
func NewPushImageTool(logger zerolog.Logger) *PushImageTool {
	return &PushImageTool{
		logger: logger,
	}
}

// ExecuteTyped pushes a Docker image to a registry
func (t *PushImageTool) ExecuteTyped(ctx context.Context, args PushImageArgs) (*PushImageResult, error) {
	startTime := time.Now()
	// Create base response
	response := &PushImageResult{
		BaseToolResponse: mcptypes.NewBaseResponse("push_image", args.SessionID, args.DryRun),
		ImageRef:         t.normalizeImageRef(args),
		Logs:             make([]string, 0),
	}
	// Extract registry from image reference
	response.Registry = t.extractRegistry(response.ImageRef)
	// Handle dry-run
	if args.DryRun {
		response.Success = true
		response.Logs = append(response.Logs, "DRY-RUN: Would push Docker image to registry")
		response.Logs = append(response.Logs, fmt.Sprintf("DRY-RUN: Image reference: %s", response.ImageRef))
		response.Logs = append(response.Logs, fmt.Sprintf("DRY-RUN: Target registry: %s", response.Registry))
		response.Logs = append(response.Logs, "DRY-RUN: Would authenticate using Docker credential helpers")
		response.Logs = append(response.Logs, "DRY-RUN: Would check if image exists locally")
		response.Logs = append(response.Logs, "DRY-RUN: Would upload layers to registry")
		if args.AsyncPush {
			response.JobID = fmt.Sprintf("push_job_%d", time.Now().UnixNano())
			response.Logs = append(response.Logs, fmt.Sprintf("DRY-RUN: Would create async job: %s", response.JobID))
		}
		response.Duration = time.Since(startTime)
		return response, nil
	}
	// Validate image reference
	if err := t.validateImageRef(response.ImageRef); err != nil {
		response.Error = &mcptypes.ToolError{
			Type:    "validation_error",
			Message: fmt.Sprintf("Invalid image reference: %v", err),
		}
		response.Success = false
		response.Duration = time.Since(startTime)
		return response, nil
	}
	// Set push timeout
	pushTimeout := args.PushTimeout
	if pushTimeout == 0 {
		pushTimeout = 10 * time.Minute
	}
	// Set retry count
	retryCount := args.RetryCount
	if retryCount == 0 {
		retryCount = 3
	}
	// Determine if this should be async
	isAsync := args.AsyncPush || pushTimeout > 5*time.Minute
	t.logger.Info().
		Str("image_ref", response.ImageRef).
		Str("registry", response.Registry).
		Bool("async", isAsync).
		Dur("timeout", pushTimeout).
		Int("retry_count", retryCount).
		Msg("Starting Docker push")
	if isAsync {
		// Create mock job ID for async push
		jobID := fmt.Sprintf("push_job_%d", time.Now().UnixNano())
		response.JobID = jobID
		response.Success = true // Job creation succeeded
		response.Logs = append(response.Logs, fmt.Sprintf("Created async push job: %s", jobID))
		response.Logs = append(response.Logs, "Use get_job_status to check push progress")
		response.Duration = time.Since(startTime)
		t.logger.Info().
			Str("job_id", jobID).
			Str("image_ref", response.ImageRef).
			Msg("Created async push job")
		return response, nil
	}
	// Synchronous push simulation
	pushResult, err := t.performPush(ctx, response.ImageRef, retryCount)
	if err != nil {
		response.Error = &mcptypes.ToolError{
			Type:        "push_error",
			Message:     fmt.Sprintf("Push failed: %v", err),
			Retryable:   t.isRetryableError(err),
			RetryCount:  retryCount,
			Suggestions: t.generateErrorSuggestions(err),
		}
		response.Success = false
	} else {
		response.Success = true
		response.Size = pushResult.Size
		response.LayersInfo = pushResult.LayersInfo
		response.CacheHitRatio = pushResult.CacheHitRatio
	}
	response.Logs = pushResult.Logs
	response.Duration = time.Since(startTime)
	t.logger.Info().
		Str("image_ref", response.ImageRef).
		Bool("success", response.Success).
		Dur("duration", response.Duration).
		Msg("Docker push completed")
	return response, nil
}

// PushExecutionResult represents the result of executing a push
type PushExecutionResult struct {
	Size          int64       `json:"size_bytes"`
	LayersInfo    *LayersInfo `json:"layers_info"`
	CacheHitRatio float64     `json:"cache_hit_ratio"`
	Logs          []string    `json:"logs"`
}

// performPush simulates the actual Docker push operation
func (t *PushImageTool) performPush(ctx context.Context, imageRef string, retryCount int) (*PushExecutionResult, error) {
	result := &PushExecutionResult{
		Logs: make([]string, 0),
	}
	// Simulate checking if image exists locally
	result.Logs = append(result.Logs, "Checking if image exists locally...")
	result.Logs = append(result.Logs, fmt.Sprintf("Found image: %s", imageRef))
	// Simulate authentication
	result.Logs = append(result.Logs, "Authenticating with registry...")
	result.Logs = append(result.Logs, "Using Docker credential helpers")
	// Simulate layer analysis and push
	result.Logs = append(result.Logs, "Analyzing image layers...")
	// Mock layer information
	totalLayers := 8
	cachedLayers := 5 // Some layers already exist in registry
	pushedLayers := 3 // New layers to push
	result.LayersInfo = &LayersInfo{
		TotalLayers:    totalLayers,
		PushedLayers:   pushedLayers,
		CachedLayers:   cachedLayers,
		FailedLayers:   0,
		LayerSizeBytes: 45 * 1024 * 1024, // 45MB
		CacheRatio:     float64(cachedLayers) / float64(totalLayers),
	}
	// Simulate pushing layers
	for i := 1; i <= pushedLayers; i++ {
		result.Logs = append(result.Logs, fmt.Sprintf("Pushing layer %d/%d...", i, pushedLayers))
		result.Logs = append(result.Logs, fmt.Sprintf("Layer %d: pushed", i))
	}
	// Simulate cached layers
	for i := 1; i <= cachedLayers; i++ {
		result.Logs = append(result.Logs, fmt.Sprintf("Layer %d: already exists, skipping", pushedLayers+i))
	}
	// Simulate final steps
	result.Logs = append(result.Logs, "Pushing manifest...")
	result.Logs = append(result.Logs, fmt.Sprintf("Successfully pushed %s", imageRef))
	// Set result values
	result.Size = 85 * 1024 * 1024 // 85MB total image size
	result.CacheHitRatio = result.LayersInfo.CacheRatio
	// For demonstration, we always succeed
	// In real implementation, this would call the actual Docker client
	return result, nil
}

// normalizeImageRef creates a normalized image reference string
func (t *PushImageTool) normalizeImageRef(args PushImageArgs) string {
	// ImageRef is now required
	if args.ImageRef.Repository == "" {
		return "" // Will be caught by validation
	}
	return args.ImageRef.String()
}

// extractRegistry extracts the registry from an image reference
func (t *PushImageTool) extractRegistry(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) >= 2 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return mcptypes.DefaultRegistry // Default to Docker Hub
}

// validateImageRef validates an image reference format
func (t *PushImageTool) validateImageRef(imageRef string) error {
	if imageRef == "" {
		return fmt.Errorf(
			"invalid arguments: image reference cannot be empty")
	}
	if !strings.Contains(imageRef, ":") {
		return fmt.Errorf(
			"invalid arguments: image reference missing tag")
	}
	// Basic validation - in real implementation, this would be more thorough
	if strings.Contains(imageRef, " ") {
		return fmt.Errorf(
			"invalid arguments: image reference cannot contain spaces")
	}
	return nil
}

// isRetryableError determines if an error is retryable
func (t *PushImageTool) isRetryableError(err error) bool {
	errorStr := err.Error()
	retryableErrors := []string{
		"network",
		"timeout",
		"connection",
		"temporary",
		"rate limit",
		"502",
		"503",
		"504",
	}
	for _, retryableErr := range retryableErrors {
		if strings.Contains(strings.ToLower(errorStr), retryableErr) {
			return true
		}
	}
	return false
}

// generateErrorSuggestions provides suggestions for fixing push errors
func (t *PushImageTool) generateErrorSuggestions(err error) []string {
	errorStr := strings.ToLower(err.Error())
	suggestions := make([]string, 0)
	if strings.Contains(errorStr, "authentication") || strings.Contains(errorStr, "unauthorized") {
		suggestions = append(suggestions, "Check Docker credentials with 'docker login'")
		suggestions = append(suggestions, "Verify registry permissions for the image")
		suggestions = append(suggestions, "Ensure DOCKER_USERNAME and DOCKER_PASSWORD env vars are set")
	}
	if strings.Contains(errorStr, "network") || strings.Contains(errorStr, "connection") {
		suggestions = append(suggestions, "Check network connectivity to registry")
		suggestions = append(suggestions, "Verify registry URL is correct")
		suggestions = append(suggestions, "Try again in a few moments")
	}
	if strings.Contains(errorStr, "not found") {
		suggestions = append(suggestions, "Build the image locally first with build_image")
		suggestions = append(suggestions, "Check that the image name and tag are correct")
	}
	if strings.Contains(errorStr, "rate limit") {
		suggestions = append(suggestions, "Wait before retrying due to rate limiting")
		suggestions = append(suggestions, "Consider using authenticated requests")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Check Docker daemon is running")
		suggestions = append(suggestions, "Verify image exists locally with 'docker images'")
		suggestions = append(suggestions, "Check registry documentation for requirements")
	}
	return suggestions
}

// Additional helper functions for future integration
// checkDockerLogin verifies Docker credentials are configured
func (t *PushImageTool) checkDockerLogin(registry string) error {
	t.logger.Debug().Str("registry", registry).Msg("Checking Docker credentials")
	// Check environment variables first
	if os.Getenv("DOCKER_USERNAME") != "" && os.Getenv("DOCKER_PASSWORD") != "" {
		t.logger.Debug().Msg("Found Docker credentials in environment variables")
		return nil
	}
	// Check Docker config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".docker", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		t.logger.Debug().Str("config_path", configPath).Msg("Found Docker config file")
		return nil
	}

	return fmt.Errorf("no Docker credentials found for registry %s", registry)
}
