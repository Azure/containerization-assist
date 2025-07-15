// Package container provides infrastructure implementations for container operations
package container

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	infraerrors "github.com/Azure/container-kit/pkg/mcp/infrastructure/core"
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
		// Create structured error for better handling
		infraErr := infraerrors.NewInfrastructureError(
			"remove_image",
			"docker",
			"Failed to remove Docker image",
			err,
			infraerrors.IsImageNotFound(err), // Recoverable if image not found
		).WithContext("image_ref", imageRef).
			WithContext("output", string(out))

		// Check if this is a recoverable error (image doesn't exist)
		if infraerrors.IsImageNotFound(err) {
			m.logger.Debug("Docker image not found, treating as success",
				"image_ref", imageRef,
				"error", err,
				"output", out)
			return nil // Image not existing is acceptable for removal
		}

		// Log structured error and return it for non-recoverable cases
		infraErr.LogWithContext(m.logger)
		return infraErr
	}

	m.logger.Info("Docker image removed successfully", "image_ref", imageRef)
	return nil
}
