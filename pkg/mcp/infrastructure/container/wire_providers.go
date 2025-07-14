// Package container provides Wire providers for container management
package container

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideContainerManager creates a container manager instance
func ProvideContainerManager(logger *slog.Logger) workflow.ContainerManager {
	// Use the default command runner
	commandRunner := &runner.DefaultCommandRunner{}
	return NewDockerContainerManager(commandRunner, logger)
}
