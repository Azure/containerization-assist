// Package container provides infrastructure implementations for container operations
package container

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// DockerContainerManager implements the workflow.ContainerManager interface using Docker
type DockerContainerManager struct {
	runner runner.CommandRunner
	logger *slog.Logger
}

// NewDockerContainerManager creates a new Docker container manager
func NewDockerContainerManager(runner runner.CommandRunner, logger *slog.Logger) workflow.ContainerManager {
	return &DockerContainerManager{
		runner: runner,
		logger: logger.With("component", "docker-container-manager"),
	}
}

// RemoveImage removes a Docker image by reference
func (m *DockerContainerManager) RemoveImage(ctx context.Context, imageRef string) error {
	m.logger.Info("Removing Docker image", "image_ref", imageRef)

	// Use -f flag to force removal
	out, err := m.runner.RunWithOutput(ctx, "docker", "rmi", "-f", imageRef)
	if err != nil {
		// Log the error but don't fail - image might not exist
		m.logger.Warn("Failed to remove Docker image",
			"image_ref", imageRef,
			"error", err,
			"output", out)
		return nil // Ignore errors as per original behavior
	}

	m.logger.Info("Docker image removed successfully", "image_ref", imageRef)
	return nil
}
