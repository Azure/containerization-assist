package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/runner"
)

func TestValidateDockerfileRuntime_GenericFailure(t *testing.T) {
	// Mock Docker client that simulates a container with custom failure patterns
	fakeRunner := &runner.FakeCommandRunner{
		Output: "test-container-id\nApplication starting...\ncustom failure occurred\nProcess terminated",
		ErrStr: "",
	}

	dockerClient := NewDockerCmdRunner(fakeRunner)

	ctx := context.Background()
	imageName := "test-app:latest"
	config := RuntimeValidationConfig{
		StartupTimeout:    100 * time.Millisecond, // Short timeout for testing
		ValidationCommand: nil,
		ExpectedExitCodes: []int{0},
		FailurePatterns:   []string{"custom failure occurred"}, // Custom failure pattern
		SuccessPatterns:   []string{},
	}

	err := ValidateDockerfileRuntime(ctx, dockerClient, imageName, config)

	// Should return an error because of the custom failure pattern
	if err == nil {
		t.Error("Expected error for custom failure pattern, but got nil")
	}

	if !strings.Contains(err.Error(), "custom failure occurred") {
		t.Errorf("Expected error to contain 'custom failure occurred', got: %v", err)
	}
}

func TestValidateDockerfileRuntime_Success(t *testing.T) {
	// Mock Docker client that simulates a successful container run
	fakeRunner := &runner.FakeCommandRunner{
		Output: "test-container-id\nApplication starting...\ncustom startup complete\nService ready",
		ErrStr: "",
	}

	dockerClient := NewDockerCmdRunner(fakeRunner)

	ctx := context.Background()
	imageName := "test-app:latest"
	config := RuntimeValidationConfig{
		StartupTimeout:    100 * time.Millisecond, // Short timeout for testing
		ValidationCommand: nil,
		ExpectedExitCodes: []int{0},
		FailurePatterns:   []string{},
		SuccessPatterns:   []string{"startup complete"}, // Custom success pattern
	}

	err := ValidateDockerfileRuntime(ctx, dockerClient, imageName, config)

	// Should not return an error for successful startup
	if err != nil {
		t.Errorf("Expected no error for successful startup, got: %v", err)
	}
}

func TestValidateDockerfileRuntime_NoPatterns(t *testing.T) {
	// Test with no patterns configured - should always pass
	fakeRunner := &runner.FakeCommandRunner{
		Output: "test-container-id\nSome random log output\nError: something happened\nBut no patterns configured",
		ErrStr: "",
	}

	dockerClient := NewDockerCmdRunner(fakeRunner)

	ctx := context.Background()
	imageName := "test-app:latest"
	config := RuntimeValidationConfig{
		StartupTimeout:    100 * time.Millisecond,
		ValidationCommand: nil,
		ExpectedExitCodes: []int{0},
		FailurePatterns:   []string{}, // No patterns
		SuccessPatterns:   []string{}, // No patterns
	}

	err := ValidateDockerfileRuntime(ctx, dockerClient, imageName, config)

	// Should not return an error when no patterns are configured
	if err != nil {
		t.Errorf("Expected no error when no patterns configured, got: %v", err)
	}
}

func TestValidateDockerfileRuntime_CustomPatterns(t *testing.T) {
	// Test custom failure and success patterns
	fakeRunner := &runner.FakeCommandRunner{
		Output: "test-container-id\nApplication initializing...\nCustom startup complete\nService operational",
		ErrStr: "",
	}

	dockerClient := NewDockerCmdRunner(fakeRunner)

	ctx := context.Background()
	imageName := "test-custom-app:latest"
	config := RuntimeValidationConfig{
		StartupTimeout:    100 * time.Millisecond,
		ValidationCommand: nil,
		ExpectedExitCodes: []int{0},
		FailurePatterns:   []string{"initialization failed", "service error"},
		SuccessPatterns:   []string{"startup complete", "service operational"},
	}

	err := ValidateDockerfileRuntime(ctx, dockerClient, imageName, config)

	// Should not return an error because we find the success pattern
	if err != nil {
		t.Errorf("Expected no error with custom success pattern, got: %v", err)
	}
}
