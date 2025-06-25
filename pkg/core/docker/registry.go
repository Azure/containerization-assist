package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/mcp/utils"
	"github.com/rs/zerolog"
)

// RegistryManager provides mechanical Docker registry operations
type RegistryManager struct {
	clients *clients.Clients
	logger  zerolog.Logger
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(clients *clients.Clients, logger zerolog.Logger) *RegistryManager {
	return &RegistryManager{
		clients: clients,
		logger:  logger.With().Str("component", "docker_registry_manager").Logger(),
	}
}

// PushOptions contains options for pushing images
type PushOptions struct {
	Registry   string
	Repository string
	Tag        string
	RetryCount int
	Timeout    time.Duration
}

// RegistryPushResult contains the result of a registry push operation
type RegistryPushResult struct {
	Success  bool                   `json:"success"`
	ImageRef string                 `json:"image_ref"`
	Registry string                 `json:"registry"`
	Output   string                 `json:"output"`
	Duration time.Duration          `json:"duration"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Error    *RegistryError         `json:"error,omitempty"`
}

// PullResult contains the result of a pull operation
type PullResult struct {
	Success  bool                   `json:"success"`
	ImageRef string                 `json:"image_ref"`
	Registry string                 `json:"registry"`
	Output   string                 `json:"output"`
	Duration time.Duration          `json:"duration"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Error    *RegistryError         `json:"error,omitempty"`
}

// RegistryError provides detailed registry error information
type RegistryError struct {
	Type     string                 `json:"type"` // "auth_error", "network_error", "not_found", "push_error"
	Message  string                 `json:"message"`
	ImageRef string                 `json:"image_ref"`
	Registry string                 `json:"registry"`
	Output   string                 `json:"output"`
	Context  map[string]interface{} `json:"context"`
}

// PushImage pushes a Docker image to a registry
func (rm *RegistryManager) PushImage(ctx context.Context, imageRef string, options PushOptions) (*RegistryPushResult, error) {
	startTime := time.Now()

	result := &RegistryPushResult{
		ImageRef: imageRef,
		Registry: rm.extractRegistry(imageRef),
		Context:  make(map[string]interface{}),
	}

	rm.logger.Info().
		Str("image_ref", imageRef).
		Str("registry", result.Registry).
		Msg("Starting Docker push")

	// Validate inputs
	if err := rm.validatePushInputs(imageRef, options); err != nil {
		result.Error = &RegistryError{
			Type:     "validation_error",
			Message:  err.Error(),
			ImageRef: imageRef,
			Registry: result.Registry,
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Set timeout context if specified
	pushCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		pushCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Perform the push
	output, err := rm.clients.Docker.Push(pushCtx, imageRef)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		// Sanitize error and output to remove sensitive information
		sanitizedError, sanitizedOutput := utils.SanitizeRegistryError(err.Error(), output)

		rm.logger.Error().
			Str("error", sanitizedError).
			Str("output", sanitizedOutput).
			Msg("Docker push failed")

		errorType := rm.categorizeError(err, output)
		errorContext := map[string]interface{}{
			"options":  options,
			"duration": result.Duration.Seconds(),
		}

		// Add authentication guidance if this is an auth error
		if errorType == "auth_error" {
			errorContext["auth_guidance"] = utils.GetAuthErrorGuidance(result.Registry)
		}

		result.Error = &RegistryError{
			Type:     errorType,
			Message:  fmt.Sprintf("Docker push failed: %s", sanitizedError),
			ImageRef: imageRef,
			Registry: result.Registry,
			Output:   sanitizedOutput,
			Context:  errorContext,
		}
		return result, nil
	}

	// Push succeeded
	result.Success = true
	result.Context = map[string]interface{}{
		"push_time": result.Duration.Seconds(),
		"registry":  result.Registry,
	}

	rm.logger.Info().
		Str("image_ref", imageRef).
		Str("registry", result.Registry).
		Dur("duration", result.Duration).
		Msg("Docker push completed successfully")

	return result, nil
}

// PullImage pulls a Docker image from a registry
func (rm *RegistryManager) PullImage(ctx context.Context, imageRef string) (*PullResult, error) {
	startTime := time.Now()

	result := &PullResult{
		ImageRef: imageRef,
		Registry: rm.extractRegistry(imageRef),
		Context:  make(map[string]interface{}),
	}

	rm.logger.Info().
		Str("image_ref", imageRef).
		Str("registry", result.Registry).
		Msg("Starting Docker pull")

	// Validate inputs
	if err := rm.validatePullInputs(imageRef); err != nil {
		result.Error = &RegistryError{
			Type:     "validation_error",
			Message:  err.Error(),
			ImageRef: imageRef,
			Registry: result.Registry,
			Context: map[string]interface{}{
				"imageRef": imageRef,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Perform the pull using docker command directly
	cmd := fmt.Sprintf("docker pull %s", imageRef)
	output, err := rm.executeDockerCommand(ctx, cmd)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		// Sanitize error and output to remove sensitive information
		sanitizedError, sanitizedOutput := utils.SanitizeRegistryError(err.Error(), output)

		rm.logger.Error().
			Str("error", sanitizedError).
			Str("output", sanitizedOutput).
			Msg("Docker pull failed")

		errorType := rm.categorizePullError(err, output)
		errorContext := map[string]interface{}{
			"imageRef": imageRef,
			"duration": result.Duration.Seconds(),
		}

		// Add authentication guidance if this is an auth error
		if errorType == "auth_error" {
			errorContext["auth_guidance"] = utils.GetAuthErrorGuidance(result.Registry)
		}

		result.Error = &RegistryError{
			Type:     errorType,
			Message:  fmt.Sprintf("Docker pull failed: %s", sanitizedError),
			ImageRef: imageRef,
			Registry: result.Registry,
			Output:   sanitizedOutput,
			Context:  errorContext,
		}
		return result, nil
	}

	// Pull succeeded
	result.Success = true
	result.Context = map[string]interface{}{
		"pull_time": result.Duration.Seconds(),
		"registry":  result.Registry,
	}

	rm.logger.Info().
		Str("image_ref", imageRef).
		Str("registry", result.Registry).
		Dur("duration", result.Duration).
		Msg("Docker pull completed successfully")

	return result, nil
}

// TagResult contains the result of a tag operation
type TagResult struct {
	Success     bool                   `json:"success"`
	SourceImage string                 `json:"source_image"`
	TargetImage string                 `json:"target_image"`
	Output      string                 `json:"output"`
	Duration    time.Duration          `json:"duration"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Error       *RegistryError         `json:"error,omitempty"`
}

// TagImage tags a Docker image with a new name
func (rm *RegistryManager) TagImage(ctx context.Context, sourceImage, targetImage string) (*TagResult, error) {
	startTime := time.Now()

	result := &TagResult{
		SourceImage: sourceImage,
		TargetImage: targetImage,
		Context:     make(map[string]interface{}),
	}

	rm.logger.Info().
		Str("source", sourceImage).
		Str("target", targetImage).
		Msg("Tagging Docker image")

	// Validate inputs
	if err := rm.validateTagInputs(sourceImage, targetImage); err != nil {
		result.Error = &RegistryError{
			Type:     "validation_error",
			Message:  err.Error(),
			ImageRef: sourceImage,
			Context: map[string]interface{}{
				"source_image": sourceImage,
				"target_image": targetImage,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Perform the tag
	// Perform the tag using docker command directly
	cmd := fmt.Sprintf("docker tag %s %s", sourceImage, targetImage)
	output, err := rm.executeDockerCommand(ctx, cmd)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		rm.logger.Error().
			Str("error", err.Error()).
			Str("output", output).
			Msg("Docker tag failed")

		result.Error = &RegistryError{
			Type:     "tag_error",
			Message:  fmt.Sprintf("Docker tag failed: %s", err.Error()),
			ImageRef: sourceImage,
			Output:   output,
			Context: map[string]interface{}{
				"source_image": sourceImage,
				"target_image": targetImage,
				"duration":     result.Duration.Seconds(),
			},
		}
		return result, nil
	}

	// Tag succeeded
	result.Success = true
	result.Context = map[string]interface{}{
		"tag_time":     result.Duration.Seconds(),
		"source_image": sourceImage,
		"target_image": targetImage,
	}

	rm.logger.Info().
		Str("source", sourceImage).
		Str("target", targetImage).
		Dur("duration", result.Duration).
		Msg("Docker tag completed successfully")

	return result, nil
}

// ValidateRegistryAccess checks if the registry is accessible
func (rm *RegistryManager) ValidateRegistryAccess(ctx context.Context, registry string) error {
	// This is a basic validation - in practice, you might want to do a test push/pull
	if registry == "" {
		return fmt.Errorf("registry URL is required")
	}

	// Basic URL validation
	if !strings.Contains(registry, ".") {
		return fmt.Errorf("registry URL appears to be invalid: %s", registry)
	}

	return nil
}

// Helper methods

func (rm *RegistryManager) validatePushInputs(imageRef string, options PushOptions) error {
	if imageRef == "" {
		return fmt.Errorf("image reference is required")
	}

	if !strings.Contains(imageRef, ":") {
		return fmt.Errorf("image reference should include a tag: %s", imageRef)
	}

	return nil
}

func (rm *RegistryManager) extractRegistry(imageRef string) string {
	// Extract registry from image reference
	// Examples:
	// "myregistry.azurecr.io/myapp:latest" -> "myregistry.azurecr.io"
	// "docker.io/myapp:latest" -> "docker.io"
	// "myapp:latest" -> "docker.io" (default)

	parts := strings.Split(imageRef, "/")
	if len(parts) == 1 {
		return "docker.io" // Default registry
	}

	// Check if first part looks like a registry (contains dots)
	if strings.Contains(parts[0], ".") {
		return parts[0]
	}

	return "docker.io" // Default registry
}

func (rm *RegistryManager) categorizeError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Authentication errors
	if strings.Contains(errStr, "unauthorized") || strings.Contains(outputStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") || strings.Contains(outputStr, "authentication") ||
		strings.Contains(errStr, "denied") || strings.Contains(outputStr, "denied") {
		return "auth_error"
	}

	// Network errors
	if strings.Contains(errStr, "network") || strings.Contains(outputStr, "network") ||
		strings.Contains(errStr, "timeout") || strings.Contains(outputStr, "timeout") ||
		strings.Contains(errStr, "connection") || strings.Contains(outputStr, "connection") {
		return "network_error"
	}

	// Not found errors
	if strings.Contains(errStr, "not found") || strings.Contains(outputStr, "not found") ||
		strings.Contains(errStr, "does not exist") || strings.Contains(outputStr, "does not exist") {
		return "not_found"
	}

	// Default to generic push error
	return "push_error"
}

func (rm *RegistryManager) categorizePullError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Authentication errors
	if strings.Contains(errStr, "unauthorized") || strings.Contains(outputStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") || strings.Contains(outputStr, "authentication") ||
		strings.Contains(errStr, "denied") || strings.Contains(outputStr, "denied") {
		return "auth_error"
	}

	// Network errors
	if strings.Contains(errStr, "network") || strings.Contains(outputStr, "network") ||
		strings.Contains(errStr, "timeout") || strings.Contains(outputStr, "timeout") ||
		strings.Contains(errStr, "connection") || strings.Contains(outputStr, "connection") {
		return "network_error"
	}

	// Not found errors (specific to pulls)
	if strings.Contains(errStr, "not found") || strings.Contains(outputStr, "not found") ||
		strings.Contains(errStr, "does not exist") || strings.Contains(outputStr, "does not exist") ||
		strings.Contains(errStr, "manifest unknown") || strings.Contains(outputStr, "manifest unknown") ||
		strings.Contains(errStr, "repository does not exist") || strings.Contains(outputStr, "repository does not exist") {
		return "not_found"
	}

	// Default to generic pull error
	return "pull_error"
}

func (rm *RegistryManager) validatePullInputs(imageRef string) error {
	if imageRef == "" {
		return fmt.Errorf("image reference is required")
	}

	// Basic image reference validation
	if strings.HasPrefix(imageRef, "-") || strings.HasSuffix(imageRef, "-") {
		return fmt.Errorf("invalid image reference format: %s", imageRef)
	}

	return nil
}

func (rm *RegistryManager) validateTagInputs(sourceImage, targetImage string) error {
	if sourceImage == "" {
		return fmt.Errorf("source image is required")
	}

	if targetImage == "" {
		return fmt.Errorf("target image is required")
	}

	// Validate both image references
	for _, img := range []string{sourceImage, targetImage} {
		if strings.HasPrefix(img, "-") || strings.HasSuffix(img, "-") {
			return fmt.Errorf("invalid image reference format: %s", img)
		}
	}

	// Check if source and target are the same
	if sourceImage == targetImage {
		return fmt.Errorf("source and target images cannot be the same")
	}

	return nil
}

// NormalizeImageRef ensures an image reference is properly formatted for a registry
func (rm *RegistryManager) NormalizeImageRef(imageName, registry, tag string) string {
	if tag == "" {
		tag = "latest"
	}

	if registry == "" {
		return fmt.Sprintf("%s:%s", imageName, tag)
	}

	return fmt.Sprintf("%s/%s:%s", registry, imageName, tag)
}

// executeDockerCommand executes a docker command using the shell
func (rm *RegistryManager) executeDockerCommand(ctx context.Context, cmd string) (string, error) {
	// Use shell to execute docker command
	return rm.executeShellCommand(ctx, "sh", "-c", cmd)
}

// executeShellCommand executes a shell command
func (rm *RegistryManager) executeShellCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
