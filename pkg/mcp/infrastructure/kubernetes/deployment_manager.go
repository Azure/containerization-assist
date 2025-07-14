// Package kubernetes provides infrastructure implementations for Kubernetes operations
package kubernetes

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// KubernetesDeploymentManager implements the workflow.DeploymentManager interface using kubectl
type KubernetesDeploymentManager struct {
	runner runner.CommandRunner
	logger *slog.Logger
}

// NewKubernetesDeploymentManager creates a new Kubernetes deployment manager
func NewKubernetesDeploymentManager(runner runner.CommandRunner, logger *slog.Logger) workflow.DeploymentManager {
	return &KubernetesDeploymentManager{
		runner: runner,
		logger: logger.With("component", "k8s-deployment-manager"),
	}
}

// DeleteDeployment removes a Kubernetes deployment
func (m *KubernetesDeploymentManager) DeleteDeployment(ctx context.Context, namespace, name string) error {
	m.logger.Info("Deleting Kubernetes deployment", "namespace", namespace, "name", name)

	// Use --ignore-not-found=true to handle non-existent deployments gracefully
	out, err := m.runner.RunWithOutput(ctx, "kubectl", "delete", "deployment", name, "-n", namespace, "--ignore-not-found=true")
	if err != nil {
		// Log the error but don't fail - deployment might not exist
		m.logger.Warn("Failed to delete deployment",
			"namespace", namespace,
			"name", name,
			"error", err,
			"output", out)
		return nil // Ignore errors as per original behavior
	}

	m.logger.Info("Deployment deleted successfully", "namespace", namespace, "name", name)
	return nil
}

// DeleteService removes a Kubernetes service
func (m *KubernetesDeploymentManager) DeleteService(ctx context.Context, namespace, name string) error {
	m.logger.Info("Deleting Kubernetes service", "namespace", namespace, "name", name)

	// Use --ignore-not-found=true to handle non-existent services gracefully
	out, err := m.runner.RunWithOutput(ctx, "kubectl", "delete", "service", name, "-n", namespace, "--ignore-not-found=true")
	if err != nil {
		// Log the error but don't fail - service might not exist
		m.logger.Warn("Failed to delete service",
			"namespace", namespace,
			"name", name,
			"error", err,
			"output", out)
		return nil // Ignore errors as per original behavior
	}

	m.logger.Info("Service deleted successfully", "namespace", namespace, "name", name)
	return nil
}
