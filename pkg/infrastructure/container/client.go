package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
)

// BuildDockerfileContent builds a Docker image from a string containing Dockerfile contents
func BuildDockerfileContent(ctx context.Context, docker DockerClient, dockerfileContent string, targetDir string, registry string, imageName string) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "docker-build-*")
	if err != nil {
		return "", errors.New(errors.CodeIoError, "docker", fmt.Sprintf("failed to create temp directory: %v", err), err)
	}
	defer os.RemoveAll(tmpDir) // Clean up

	// Create temporary Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return "", errors.New(errors.CodeIoError, "docker", fmt.Sprintf("failed to write Dockerfile: %v", err), err)
	}

	registryPrefix := ""
	if registry != "" {
		registryPrefix = registry + "/"
	}

	// Build the image using the temporary Dockerfile
	buildErrors, err := docker.Build(ctx, dockerfilePath, registryPrefix+imageName+":latest", targetDir)

	if err != nil {
		return buildErrors, errors.New(errors.CodeImageBuildFailed, "docker", fmt.Sprintf("docker build failed: %v", err), err)
	}

	return buildErrors, nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func checkDockerRunning(ctx context.Context, docker DockerClient) error {
	if output, err := docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

func PushDockerImage(ctx context.Context, docker DockerClient, image string) error {
	_, err := docker.Push(ctx, image)

	if err != nil {
		return errors.New(errors.CodeImagePushFailed, "docker", fmt.Sprintf("error pushing to registry: %s", err.Error()), err)
	}

	return nil
}
