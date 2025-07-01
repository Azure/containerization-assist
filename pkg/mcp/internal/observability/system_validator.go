package observability

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// SystemValidator handles system-level checks (Docker, Kubernetes, disk space, etc.)
type SystemValidator struct {
	logger zerolog.Logger
}

// NewSystemValidator creates a new system validator
func NewSystemValidator(logger zerolog.Logger) *SystemValidator {
	return &SystemValidator{
		logger: logger,
	}
}

// CheckDockerDaemon checks if Docker daemon is running
func (sv *SystemValidator) CheckDockerDaemon(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Docker daemon not accessible: %v", err)
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return fmt.Errorf("error")
	}

	sv.logger.Debug().Str("docker_version", version).Msg("Docker daemon check passed")
	return nil
}

// CheckDockerDiskSpace checks available disk space for Docker
func (sv *SystemValidator) CheckDockerDiskSpace(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.DockerRootDir}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Docker root directory: %v", err)
	}

	dockerRoot := strings.TrimSpace(string(output))
	if dockerRoot == "" {
		dockerRoot = "/var/lib/docker"
	}

	cmd = exec.CommandContext(ctx, "df", "-BG", dockerRoot)
	output, err = cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(ctx, "df", "-BG", "/")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("error")
		}
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("error")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return fmt.Errorf("error")
	}

	availStr := strings.TrimSuffix(fields[3], "G")
	availGB, err := strconv.Atoi(availStr)
	if err != nil {
		return fmt.Errorf("error")
	}

	const minSpaceGB = 5
	if availGB < minSpaceGB {
		return fmt.Errorf("error")
	}

	sv.logger.Debug().Int("available_gb", availGB).Msg("Docker disk space check passed")
	return nil
}

// CheckDiskSpace checks general available disk space
func (sv *SystemValidator) CheckDiskSpace(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "df", "-h", "/var/lib/docker")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(ctx, "df", "-h", "/")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("error")
		}
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("error")
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "100%") || strings.Contains(outputStr, "99%") || strings.Contains(outputStr, "98%") {
		return fmt.Errorf("error")
	}

	return nil
}

// CheckKubernetesContext checks if kubectl has a valid context configured
func (sv *SystemValidator) CheckKubernetesContext(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error")
	}

	context := strings.TrimSpace(string(output))
	if context == "" {
		return fmt.Errorf("error")
	}

	sv.logger.Debug().Str("context", context).Msg("Kubernetes context check passed")
	return nil
}

// CheckKubernetesConnectivity checks connectivity to Kubernetes cluster
func (sv *SystemValidator) CheckKubernetesConnectivity(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--short", "--output=json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "connection refused") || strings.Contains(stderr, "no such host") {
				return fmt.Errorf("%s", stderr)
			}
		}
		cmd = exec.CommandContext(ctx, "kubectl", "get", "nodes", "--no-headers")
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	sv.logger.Debug().Str("output", string(output)).Msg("Kubernetes connectivity check passed")
	return nil
}

// CheckRequiredTools checks if required CLI tools are installed
func (sv *SystemValidator) CheckRequiredTools(ctx context.Context) error {
	requiredTools := []string{"docker", "kubectl"}
	missingTools := []string{}

	for _, tool := range requiredTools {
		cmd := exec.CommandContext(ctx, "which", tool)
		if err := cmd.Run(); err != nil {
			missingTools = append(missingTools, tool)
		}
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("missing tools: %s", strings.Join(missingTools, ", "))
	}

	sv.logger.Debug().Msg("Required tools check passed")
	return nil
}

// CheckGitInstalled checks if git is installed
func (sv *SystemValidator) CheckGitInstalled(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	version := strings.TrimSpace(string(output))
	sv.logger.Debug().Str("git_version", version).Msg("Git check passed")
	return nil
}
