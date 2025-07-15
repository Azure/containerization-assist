// Package kubernetes provides infrastructure implementations for Kubernetes operations
package kubernetes

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/common/runner"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	infraerrors "github.com/Azure/container-kit/pkg/mcp/infrastructure/core"
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
		// Create structured error for better handling
		infraErr := infraerrors.NewInfrastructureError(
			"delete_deployment",
			"kubernetes",
			"Failed to delete Kubernetes deployment",
			err,
			infraerrors.IsResourceNotFound(err), // Recoverable if resource not found
		).WithContext("namespace", namespace).
			WithContext("name", name).
			WithContext("output", string(out))

		// Check if this is a recoverable error (resource doesn't exist)
		if infraerrors.IsResourceNotFound(err) {
			m.logger.Debug("Kubernetes deployment not found, treating as success",
				"namespace", namespace,
				"name", name,
				"error", err,
				"output", out)
			return nil // Resource not existing is acceptable for deletion
		}

		// Log structured error and return it for non-recoverable cases
		infraErr.LogWithContext(m.logger)
		return infraErr
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
		// Create structured error for better handling
		infraErr := infraerrors.NewInfrastructureError(
			"delete_service",
			"kubernetes",
			"Failed to delete Kubernetes service",
			err,
			infraerrors.IsResourceNotFound(err), // Recoverable if resource not found
		).WithContext("namespace", namespace).
			WithContext("name", name).
			WithContext("output", string(out))

		// Check if this is a recoverable error (resource doesn't exist)
		if infraerrors.IsResourceNotFound(err) {
			m.logger.Debug("Kubernetes service not found, treating as success",
				"namespace", namespace,
				"name", name,
				"error", err,
				"output", out)
			return nil // Resource not existing is acceptable for deletion
		}

		// Log structured error and return it for non-recoverable cases
		infraErr.LogWithContext(m.logger)
		return infraErr
	}

	m.logger.Info("Service deleted successfully", "namespace", namespace, "name", name)
	return nil
}
