// Package kubernetes provides Wire providers for Kubernetes management
package kubernetes

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideDeploymentManager creates a deployment manager instance
func ProvideDeploymentManager(logger *slog.Logger) workflow.DeploymentManager {
	// Use the default command runner
	commandRunner := &runner.DefaultCommandRunner{}
	return NewKubernetesDeploymentManager(commandRunner, logger)
}
