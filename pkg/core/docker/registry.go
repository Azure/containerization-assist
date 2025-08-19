package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/errors"
)

// RegistryManager provides mechanical Docker registry operations
type RegistryManager struct {
	docker DockerClient
	logger *slog.Logger
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(docker DockerClient, logger *slog.Logger) *RegistryManager {
	return &RegistryManager{
		docker: docker,
		logger: logger.With("component", "docker_registry_manager"),
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

	rm.logger.Info("Starting Docker push",
		"image_ref", imageRef,
		"registry", result.Registry)

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
	output, err := rm.docker.Push(pushCtx, imageRef)
	result.Output = output
	result.Duration = time.Since(startTime)

	if err != nil {
		// Sanitize error and output to remove sensitive information
		sanitizedError, sanitizedOutput := SanitizeRegistryError(err.Error(), output)

		rm.logger.Error("Docker push failed",
			"error", sanitizedError,
			"output", sanitizedOutput)

		errorType := rm.categorizeError(err, output)
		errorContext := map[string]interface{}{
			"options":  options,
			"duration": result.Duration.Seconds(),
		}

		// Add authentication guidance if this is an auth error
		if errorType == "auth_error" {
			errorContext["auth_guidance"] = GetAuthErrorGuidance(result.Registry)
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

	rm.logger.Info("Docker push completed successfully",
		"image_ref", imageRef,
		"registry", result.Registry,
		"duration", result.Duration)

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

	rm.logger.Info("Starting Docker pull",
		"image_ref", imageRef,
		"registry", result.Registry)

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
		sanitizedError, sanitizedOutput := SanitizeRegistryError(err.Error(), output)

		rm.logger.Error("Docker pull failed",
			"error", sanitizedError,
			"output", sanitizedOutput)

		errorType := rm.categorizePullError(err, output)
		errorContext := map[string]interface{}{
			"imageRef": imageRef,
			"duration": result.Duration.Seconds(),
		}

		// Add authentication guidance if this is an auth error
		if errorType == "auth_error" {
			errorContext["auth_guidance"] = GetAuthErrorGuidance(result.Registry)
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

	rm.logger.Info("Docker pull completed successfully",
		"image_ref", imageRef,
		"registry", result.Registry,
		"duration", result.Duration)

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

	rm.logger.Info("Tagging Docker image",
		"source", sourceImage,
		"target", targetImage)

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
		rm.logger.Error("Docker tag failed",
			"error", err.Error(),
			"output", output)

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

	rm.logger.Info("Docker tag completed successfully",
		"source", sourceImage,
		"target", targetImage,
		"duration", result.Duration)

	return result, nil
}

// ValidateRegistryAccess checks if the registry is accessible
func (rm *RegistryManager) ValidateRegistryAccess(ctx context.Context, registry string) error {
	// This is a basic validation - in practice, you might want to do a test push/pull
	if registry == "" {
		return errors.New(errors.CodeValidationFailed, "registry", "registry URL is required", nil)
	}

	if !strings.Contains(registry, ".") {
		return errors.New(errors.CodeValidationFailed, "registry", fmt.Sprintf("registry URL appears to be invalid: %s", registry), nil)
	}

	return nil
}

func (rm *RegistryManager) validatePushInputs(imageRef string, options PushOptions) error {
	if imageRef == "" {
		return fmt.Errorf("image reference is required")
	}

	// Basic image reference validation - check for dash at start/end of image name
	if strings.HasPrefix(imageRef, "-") {
		return fmt.Errorf("invalid image reference format: %s", imageRef)
	}

	if !strings.Contains(imageRef, ":") {
		return fmt.Errorf("image reference should include a tag: %s", imageRef)
	}

	// Extract image name part (before the colon) and validate it
	colonIndex := strings.LastIndex(imageRef, ":")
	if colonIndex > 0 {
		imageName := imageRef[:colonIndex]
		if strings.HasSuffix(imageName, "-") {
			return fmt.Errorf("invalid image reference format: %s", imageRef)
		}
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

	// Check if first part looks like a registry (contains dots or is localhost with port)
	if strings.Contains(parts[0], ".") || strings.HasPrefix(parts[0], "localhost:") {
		return parts[0]
	}

	return "docker.io" // Default registry
}

func (rm *RegistryManager) categorizeError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Authentication errors
	if rm.isAuthError(errStr, outputStr) {
		return "auth_error"
	}

	// Network errors
	if rm.isNetworkError(errStr, outputStr) {
		return "network_error"
	}

	// Not found errors
	if rm.isPushNotFoundError(errStr, outputStr) {
		return "not_found"
	}

	// Default to generic push error
	return "push_error"
}

// Helper method to check if error is a not found error for push operations
func (rm *RegistryManager) isPushNotFoundError(errStr, outputStr string) bool {
	notFoundPatterns := []string{
		"not found",
		"does not exist",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(errStr, pattern) || strings.Contains(outputStr, pattern) {
			return true
		}
	}
	return false
}

func (rm *RegistryManager) categorizePullError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Check for not found errors first (more specific than generic "denied")
	if rm.isNotFoundError(errStr, outputStr) {
		return "not_found"
	}

	// Authentication errors
	if rm.isAuthError(errStr, outputStr) {
		return "auth_error"
	}

	// Network errors
	if rm.isNetworkError(errStr, outputStr) {
		return "network_error"
	}

	// Default to generic pull error
	return "pull_error"
}

// Helper method to check if error is a not found error
func (rm *RegistryManager) isNotFoundError(errStr, outputStr string) bool {
	notFoundPatterns := []string{
		"not found",
		"does not exist",
		"manifest unknown",
		"repository does not exist",
	}

	for _, pattern := range notFoundPatterns {
		if strings.Contains(errStr, pattern) || strings.Contains(outputStr, pattern) {
			return true
		}
	}
	return false
}

// Helper method to check if error is an authentication error
func (rm *RegistryManager) isAuthError(errStr, outputStr string) bool {
	authPatterns := []string{
		"unauthorized",
		"authentication",
		"denied",
	}

	for _, pattern := range authPatterns {
		if strings.Contains(errStr, pattern) || strings.Contains(outputStr, pattern) {
			return true
		}
	}
	return false
}

// Helper method to check if error is a network error
func (rm *RegistryManager) isNetworkError(errStr, outputStr string) bool {
	networkPatterns := []string{
		"network",
		"timeout",
		"connection",
	}

	for _, pattern := range networkPatterns {
		if strings.Contains(errStr, pattern) || strings.Contains(outputStr, pattern) {
			return true
		}
	}
	return false
}

func (rm *RegistryManager) validatePullInputs(imageRef string) error {
	if imageRef == "" {
		return fmt.Errorf("image reference is required")
	}

	// Basic image reference validation - check for dash at start of image reference
	if strings.HasPrefix(imageRef, "-") {
		return fmt.Errorf("invalid image reference format: %s", imageRef)
	}

	// For images with tags (containing ":"), check if image name ends with dash
	if strings.Contains(imageRef, ":") {
		colonIndex := strings.LastIndex(imageRef, ":")
		if colonIndex > 0 {
			imageName := imageRef[:colonIndex]
			if strings.HasSuffix(imageName, "-") {
				return fmt.Errorf("invalid image reference format: %s", imageRef)
			}
		}
	} else {
		// For images without tags, check if the entire string ends with dash
		if strings.HasSuffix(imageRef, "-") {
			return fmt.Errorf("invalid image reference format: %s", imageRef)
		}
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
		if strings.HasPrefix(img, "-") {
			return fmt.Errorf("invalid image reference format: %s", img)
		}

		// For images with tags (containing ":"), check if image name ends with dash
		if strings.Contains(img, ":") {
			colonIndex := strings.LastIndex(img, ":")
			if colonIndex > 0 {
				imageName := img[:colonIndex]
				if strings.HasSuffix(imageName, "-") {
					return fmt.Errorf("invalid image reference format: %s", img)
				}
			}
		} else {
			// For images without tags, check if the entire string ends with dash
			if strings.HasSuffix(img, "-") {
				return fmt.Errorf("invalid image reference format: %s", img)
			}
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
