package clients

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/container-kit/pkg/logger"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// buildDockerfileContent builds a Docker image from a string containing Dockerfile contents
func (c *Clients) BuildDockerfileContent(ctx context.Context, dockerfileContent string, targetDir string, registry string, imageName string) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "docker-build-*")
	if err != nil {
		return "", mcperrors.NewError().Messagef("failed to create temp directory: %v", err).WithLocation().Build()
	}
	defer os.RemoveAll(tmpDir) // Clean up

	// Create temporary Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return "", mcperrors.NewError().Messagef("failed to write Dockerfile: %v", err).WithLocation().Build()
	}

	registryPrefix := ""
	if registry != "" {
		registryPrefix = registry + "/"
	}

	// Build the image using the temporary Dockerfile
	logger.Infof("building docker image with tag '%s%s:latest'", registryPrefix, imageName)
	buildErrors, err := c.Docker.Build(ctx, dockerfilePath, registryPrefix+imageName+":latest", targetDir)

	if err != nil {
		return buildErrors, mcperrors.NewError().Messagef("docker build failed: %v", err).WithLocation().Build()
	}

	logger.Info("built docker image")
	return buildErrors, nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func (c *Clients) checkDockerRunning(ctx context.Context) error {
	if output, err := c.Docker.Info(ctx); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

func (c *Clients) PushDockerImage(ctx context.Context, image string) error {
	output, err := c.Docker.Push(ctx, image)
	logger.Infof("Output: %s", output)

	if err != nil {
		logger.Errorf("Registry push failed with error: %s", err.Error())
		return mcperrors.NewError().Messagef("error pushing to registry: %s", err.Error()).WithLocation().Build()
	}

	return nil
}
