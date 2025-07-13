// Package validation provides pre-flight checks and validation utilities
package validation

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// PreflightCheck represents a single validation check
type PreflightCheck struct {
	Name        string
	Description string
	Required    bool
	CheckFunc   func(context.Context) error
}

// PreflightResult contains the results of all preflight checks
type PreflightResult struct {
	Success  bool
	Checks   []CheckResult
	Warnings []string
	Errors   []string
}

// CheckResult represents the result of a single check
type CheckResult struct {
	Name     string
	Success  bool
	Message  string
	Duration time.Duration
}

// PreflightValidator performs pre-execution validation checks
type PreflightValidator struct {
	logger *slog.Logger
	checks []PreflightCheck
}

// NewPreflightValidator creates a new preflight validator
func NewPreflightValidator(logger *slog.Logger) *PreflightValidator {
	return &PreflightValidator{
		logger: logger.With("component", "preflight-validator"),
		checks: getDefaultChecks(),
	}
}

// ValidateAll runs all preflight checks
func (v *PreflightValidator) ValidateAll(ctx context.Context) (*PreflightResult, error) {
	v.logger.Info("Starting preflight validation checks")

	result := &PreflightResult{
		Success:  true,
		Checks:   make([]CheckResult, 0, len(v.checks)),
		Warnings: []string{},
		Errors:   []string{},
	}

	for _, check := range v.checks {
		checkResult := v.runCheck(ctx, check)
		result.Checks = append(result.Checks, checkResult)

		if !checkResult.Success {
			if check.Required {
				result.Success = false
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", check.Name, checkResult.Message))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", check.Name, checkResult.Message))
			}
		}

		v.logger.Info("Preflight check completed",
			"check", check.Name,
			"success", checkResult.Success,
			"duration", checkResult.Duration,
			"message", checkResult.Message)
	}

	v.logger.Info("Preflight validation completed",
		"success", result.Success,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings))

	if !result.Success {
		return result, errors.New(errors.CodeValidationFailed, "preflight",
			fmt.Sprintf("Preflight validation failed: %d errors", len(result.Errors)), nil)
	}

	return result, nil
}

// runCheck executes a single check
func (v *PreflightValidator) runCheck(ctx context.Context, check PreflightCheck) CheckResult {
	start := time.Now()

	// Run check with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := check.CheckFunc(checkCtx)
	duration := time.Since(start)

	if err != nil {
		return CheckResult{
			Name:     check.Name,
			Success:  false,
			Message:  err.Error(),
			Duration: duration,
		}
	}

	return CheckResult{
		Name:     check.Name,
		Success:  true,
		Message:  "OK",
		Duration: duration,
	}
}

// getDefaultChecks returns the standard set of preflight checks
func getDefaultChecks() []PreflightCheck {
	return []PreflightCheck{
		{
			Name:        "Docker",
			Description: "Check if Docker daemon is available",
			Required:    true,
			CheckFunc:   checkDocker,
		},
		{
			Name:        "Docker Daemon",
			Description: "Check if Docker daemon is running",
			Required:    true,
			CheckFunc:   checkDockerDaemon,
		},
		{
			Name:        "Disk Space",
			Description: "Check available disk space",
			Required:    false,
			CheckFunc:   checkDiskSpace,
		},
		{
			Name:        "Kubectl",
			Description: "Check if kubectl is installed",
			Required:    false,
			CheckFunc:   checkKubectl,
		},
		{
			Name:        "Kind",
			Description: "Check if kind is installed",
			Required:    false,
			CheckFunc:   checkKind,
		},
		{
			Name:        "Git",
			Description: "Check if git is installed",
			Required:    false,
			CheckFunc:   checkGit,
		},
		{
			Name:        "Network",
			Description: "Check network connectivity",
			Required:    false,
			CheckFunc:   checkNetwork,
		},
	}
}

// checkDocker verifies Docker is installed
func checkDocker(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Docker not found in PATH: %v", err)
	}

	// Check version
	version := strings.TrimSpace(string(output))
	if !strings.Contains(version, "Docker version") {
		return fmt.Errorf("unexpected Docker version output: %s", version)
	}

	return nil
}

// checkDockerDaemon verifies Docker daemon is running
func checkDockerDaemon(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon not running or not accessible: %v", err)
	}
	return nil
}

// checkDiskSpace verifies sufficient disk space
func checkDiskSpace(ctx context.Context) error {
	// This is a simplified check - in production you'd want more sophisticated logic
	const minSpaceGB = 5

	// For now, just check if we can create a temp file
	tempFile, err := os.CreateTemp("", "preflight-disk-check-*")
	if err != nil {
		return fmt.Errorf("insufficient disk space or permissions: %v", err)
	}
	tempFile.Close()
	os.Remove(tempFile.Name())

	return nil
}

// checkKubectl verifies kubectl is installed
func checkKubectl(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--client", "--short")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl not found in PATH: %v", err)
	}
	return nil
}

// checkKind verifies kind is installed
func checkKind(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kind", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind not found in PATH: %v", err)
	}
	return nil
}

// checkGit verifies git is installed
func checkGit(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git not found in PATH: %v", err)
	}
	return nil
}

// checkNetwork verifies basic network connectivity
func checkNetwork(ctx context.Context) error {
	// Try to resolve a common domain
	cmd := exec.CommandContext(ctx, "nslookup", "github.com")
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "nslookup", "github.com")
	} else {
		// Use getent on Linux/Mac which is more reliable
		cmd = exec.CommandContext(ctx, "getent", "hosts", "github.com")
	}

	if err := cmd.Run(); err != nil {
		// Fallback to ping
		pingCmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", "8.8.8.8")
		if runtime.GOOS == "windows" {
			pingCmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "2000", "8.8.8.8")
		}
		if pingErr := pingCmd.Run(); pingErr != nil {
			return fmt.Errorf("network connectivity check failed: %v", err)
		}
	}

	return nil
}

// AddCustomCheck allows adding custom validation checks
func (v *PreflightValidator) AddCustomCheck(check PreflightCheck) {
	v.checks = append(v.checks, check)
}

// ValidateRequired runs only required checks
func (v *PreflightValidator) ValidateRequired(ctx context.Context) (*PreflightResult, error) {
	requiredChecks := []PreflightCheck{}
	for _, check := range v.checks {
		if check.Required {
			requiredChecks = append(requiredChecks, check)
		}
	}

	originalChecks := v.checks
	v.checks = requiredChecks
	defer func() { v.checks = originalChecks }()

	return v.ValidateAll(ctx)
}
