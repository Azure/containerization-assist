package docker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RuntimeValidationConfig holds configuration for Dockerfile runtime validation
type RuntimeValidationConfig struct {
	// Timeout for how long to wait for the container to start
	StartupTimeout time.Duration
	// Command to run for validation (if empty, uses image default)
	ValidationCommand []string
	// Expected exit codes (0 = success, others can be acceptable for some apps)
	ExpectedExitCodes []int
	// Custom failure patterns to look for in logs (optional)
	FailurePatterns []string
	// Custom success patterns to look for in logs (optional)
	SuccessPatterns []string
}

// DefaultRuntimeValidationConfig returns sensible defaults for runtime validation
func DefaultRuntimeValidationConfig() RuntimeValidationConfig {
	return RuntimeValidationConfig{
		StartupTimeout:    30 * time.Second,
		ValidationCommand: nil,        // Use image default command
		ExpectedExitCodes: []int{0},   // Only success expected
		FailurePatterns:   []string{}, // No custom patterns by default
		SuccessPatterns:   []string{}, // No custom patterns by default
	}
}

// ValidateDockerfileRuntime performs runtime validation of a built Docker image
// This catches issues that static analysis cannot detect:
// - Missing dependencies at runtime
// - Configuration errors
// - Application startup failures
// - Port binding issues
func ValidateDockerfileRuntime(ctx context.Context, dockerClient DockerClient, imageRef string, config RuntimeValidationConfig) error {
	// Create a temporary validation tag to avoid conflicts
	tempTag := fmt.Sprintf("%s-validation-%d", imageRef, time.Now().Unix())

	// Tag the image for validation
	_, err := dockerClient.Tag(ctx, imageRef, tempTag)
	if err != nil {
		return fmt.Errorf("failed to create validation tag: %w", err)
	}

	// Ensure cleanup happens
	defer func() {
		_ = dockerClient.RemoveImage(ctx, tempTag)
	}()

	// Cast to DockerCmdRunner to access the underlying runner
	dockerCmd, ok := dockerClient.(*DockerCmdRunner)
	if !ok {
		return fmt.Errorf("runtime validation requires DockerCmdRunner")
	}

	// Create a timeout context for the validation
	timeoutCtx, cancel := context.WithTimeout(ctx, config.StartupTimeout)
	defer cancel()

	// Create a channel to capture the result
	type result struct {
		output string
		err    error
	}

	resultCh := make(chan result, 1)

	// Run the docker command in a goroutine
	go func() {
		output, err := dockerCmd.runner.RunCommand("docker", "run", "--rm", tempTag)
		resultCh <- result{output: output, err: err}
	}()

	// Wait for either completion or timeout
	select {
	case res := <-resultCh:
		if res.err != nil {
			// Include the output which should contain the actual error message
			if res.output != "" {
				return fmt.Errorf("container failed to start: %s", res.output)
			}
			return fmt.Errorf("container failed to start: %v", res.err)
		}
	case <-timeoutCtx.Done():
		return fmt.Errorf("container validation timed out after %v. This may indicate the container is hanging or taking too long to start. Try running 'docker run %s' manually to debug", config.StartupTimeout, tempTag)
	}

	return nil
}

// analyzeStartupLogs checks container logs for failure patterns
func analyzeStartupLogs(logs string, config RuntimeValidationConfig, imageRef string) error {
	logsLower := strings.ToLower(logs)

	// Check custom failure patterns only if provided
	for _, pattern := range config.FailurePatterns {
		if strings.Contains(logsLower, strings.ToLower(pattern)) {
			return fmt.Errorf("detected failure pattern '%s' in container logs. "+
				"Container output:\n%s\n\nTry running 'docker run %s' to debug",
				pattern, logs, imageRef)
		}
	}

	// Check custom success patterns only if provided
	for _, pattern := range config.SuccessPatterns {
		if strings.Contains(logsLower, strings.ToLower(pattern)) {
			return nil // Found success pattern, validation passed
		}
	}

	// If no patterns are configured, check for common failure indicators
	commonErrors := []string{
		"error", "exception", "failed", "fatal", "panic", "cannot", "unable to",
		"no such file", "permission denied", "connection refused", "address already in use",
	}

	foundErrors := []string{}
	for _, errorPattern := range commonErrors {
		if strings.Contains(logsLower, errorPattern) {
			foundErrors = append(foundErrors, errorPattern)
		}
	}

	// If we found error indicators and no specific success patterns, report the issue
	if len(foundErrors) > 0 && len(config.SuccessPatterns) == 0 {
		return fmt.Errorf("container logs suggest startup issues (found: %s). "+
			"Container output:\n%s\n\nIf this is expected behavior, you can disable validation with validate_runtime=false",
			strings.Join(foundErrors, ", "), logs)
	}

	// If no patterns are configured and no obvious errors found, validation passes
	return nil
}
