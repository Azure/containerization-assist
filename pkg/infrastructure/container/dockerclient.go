package container

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
)

type DockerClient interface {
	Version(ctx context.Context) (string, error)
	Info(ctx context.Context) (string, error)
	Build(ctx context.Context, dockerfilePath, imageTag, contextPath string) (string, error)
	Push(ctx context.Context, imageTag string) (string, error)
	Pull(ctx context.Context, imageRef string) (string, error)
	Tag(ctx context.Context, sourceRef, targetRef string) (string, error)
	Inspect(ctx context.Context, imageRef string) (string, error)
	Login(ctx context.Context, registry, username, password string) (string, error)
	LoginWithToken(ctx context.Context, registry, token string) (string, error)
	Logout(ctx context.Context, registry string) (string, error)
	IsLoggedIn(ctx context.Context, registry string) (bool, error)

	// Container management for runtime validation
	RunContainer(ctx context.Context, imageRef string, command []string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	RemoveImage(ctx context.Context, imageRef string) error
	GetContainerLogs(ctx context.Context, containerID string) (string, error)
}

type DockerCmdRunner struct {
	runner            core.CommandRunner
	authCache         map[string]time.Time // Registry -> last successful auth time
	authCacheDuration time.Duration
}

var _ DockerClient = &DockerCmdRunner{}

func NewDockerCmdRunner(runner core.CommandRunner) DockerClient {
	return &DockerCmdRunner{
		runner:            runner,
		authCache:         make(map[string]time.Time),
		authCacheDuration: 30 * time.Minute, // Cache auth for 30 minutes
	}
}

func (d *DockerCmdRunner) Info(ctx context.Context) (string, error) {
	return d.runner.RunCommand("docker", "info")
}

func (d *DockerCmdRunner) Version(ctx context.Context) (string, error) {
	return d.runner.RunCommand("docker", "version")
}

func (d *DockerCmdRunner) Build(ctx context.Context, dockerfilePath, imageTag, contextPath string) (string, error) {
	// The -q flag makes docker output only the image ID to stdout on success
	return d.runner.RunCommand("docker", "build", "-q", "-f", dockerfilePath, "-t", imageTag, contextPath)
}

func (d *DockerCmdRunner) Push(ctx context.Context, image string) (string, error) {
	return d.runner.RunCommand("docker", "push", image)
}

func (d *DockerCmdRunner) Pull(ctx context.Context, imageRef string) (string, error) {
	return d.runner.RunCommand("docker", "pull", imageRef)
}

func (d *DockerCmdRunner) Tag(ctx context.Context, sourceRef, targetRef string) (string, error) {
	return d.runner.RunCommand("docker", "tag", sourceRef, targetRef)
}

func (d *DockerCmdRunner) Inspect(ctx context.Context, imageRef string) (string, error) {
	return d.runner.RunCommand("docker", "inspect", imageRef)
}

func (d *DockerCmdRunner) Login(ctx context.Context, registry, username, password string) (string, error) {
	// Use stdin for password to avoid exposing it in process list
	var args []string
	args = append(args, "login", "--username", username, "--password-stdin")

	if registry != "" {
		args = append(args, registry)
	}

	// Create command and pass password via stdin
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("docker login failed: %v", err), err)
	}

	registryKey := registry
	if registryKey == "" {
		registryKey = "docker.io"
	}
	d.authCache[registryKey] = time.Now()

	return string(output), nil
}

func (d *DockerCmdRunner) LoginWithToken(ctx context.Context, registry, token string) (string, error) {
	// Use token-based authentication (e.g., for registries like ghcr.io)
	var args []string
	args = append(args, "login", "--username", "token", "--password-stdin")

	if registry != "" {
		args = append(args, registry)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(token)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("docker login with token failed: %v", err), err)
	}

	registryKey := registry
	if registryKey == "" {
		registryKey = "docker.io"
	}
	d.authCache[registryKey] = time.Now()

	return string(output), nil
}

func (d *DockerCmdRunner) Logout(ctx context.Context, registry string) (string, error) {
	var output string
	var err error

	if registry != "" {
		output, err = d.runner.RunCommand("docker", "logout", registry)
	} else {
		output, err = d.runner.RunCommand("docker", "logout")
	}

	if err != nil {
		return output, errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("docker logout failed: %v", err), err)
	}

	registryKey := registry
	if registryKey == "" {
		registryKey = "docker.io"
	}
	delete(d.authCache, registryKey)

	return output, nil
}

func (d *DockerCmdRunner) IsLoggedIn(ctx context.Context, registry string) (bool, error) {
	registryKey := registry
	if registryKey == "" {
		registryKey = "docker.io"
	}

	// Check auth cache first
	if authTime, exists := d.authCache[registryKey]; exists {
		if time.Since(authTime) < d.authCacheDuration {
			return true, nil
		}
		// Cache expired, remove it
		delete(d.authCache, registryKey)
	}

	// Try a simple docker info command to check auth status
	// This is a lightweight way to verify authentication
	_, err := d.runner.RunCommand("docker", "system", "info")
	if err != nil {
		return false, nil
	}

	// For now, assume we're logged in if docker is working
	// A more sophisticated check would try to access the specific registry
	return true, nil
}
<<<<<<< HEAD:pkg/infrastructure/container/dockerclient.go
=======

func CheckDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New(errors.CodeFileNotFound, "docker", "docker executable not found in PATH. Please install Docker or ensure it's available in your PATH", nil)
	}
	return nil
}

// Container management methods for runtime validation

func (d *DockerCmdRunner) RunContainer(ctx context.Context, imageRef string, command []string) (string, error) {
	// For simple validation, we can use timeout and the image's default command
	if len(command) == 0 {
		// Run with default entrypoint for a short time to test basic startup
		output, err := d.runner.RunCommand("docker", "run", "--rm", "-d", imageRef)
		if err != nil {
			return "", errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to run container: %v", err), err)
		}

		containerID := strings.TrimSpace(output)
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		return containerID, nil
	}

	// For commands with arguments, join them as a single string
	cmdStr := strings.Join(command, " ")
	output, err := d.runner.RunCommand("docker", "run", "--rm", "-d", imageRef, cmdStr)
	if err != nil {
		return "", errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to run container with command '%s': %v", cmdStr, err), err)
	}

	containerID := strings.TrimSpace(output)
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	return containerID, nil
}

func (d *DockerCmdRunner) StopContainer(ctx context.Context, containerID string) error {
	_, err := d.runner.RunCommand("docker", "stop", containerID)
	if err != nil {
		return errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to stop container %s: %v", containerID, err), err)
	}
	return nil
}

func (d *DockerCmdRunner) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := d.runner.RunCommand("docker", "rm", "-f", containerID)
	if err != nil {
		return errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to remove container %s: %v", containerID, err), err)
	}
	return nil
}

func (d *DockerCmdRunner) RemoveImage(ctx context.Context, imageRef string) error {
	_, err := d.runner.RunCommand("docker", "rmi", "-f", imageRef)
	if err != nil {
		return errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to remove image %s: %v", imageRef, err), err)
	}
	return nil
}

func (d *DockerCmdRunner) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	// Try to get logs with both stdout and stderr
	output, err := d.runner.RunCommand("docker", "logs", containerID)
	if err != nil {
		// For containers that exit immediately, docker logs might still return useful output
		// even if the command "fails". Try to extract any available output.
		if output != "" {
			// We got some output despite the error, return it
			return output, nil
		}

		// No output available, return the error
		return "", errors.New(errors.CodeOperationFailed, "docker", fmt.Sprintf("failed to get logs for container %s: %v", containerID, err), err)
	}
	return output, nil
}
>>>>>>> ed897c91 (current):pkg/core/docker/dockerclient.go
