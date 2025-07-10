//go:build k8s

package infra

import (
	"context"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// initializeKubernetesOperations initializes Kubernetes operations when k8s build tag is enabled
func (c *InfrastructureContainer) initializeKubernetesOperations() error {
	c.logger.Info("Initializing Kubernetes operations", "namespace", c.config.Namespace)

	// Create Kubernetes operations
	kubernetesOps, err := NewKubernetesOperations(c.config.KubeConfig, c.config.Namespace, c.logger)
	if err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to create Kubernetes operations").Wrap(err).Build()
	}

	c.kubernetesOps = kubernetesOps

	c.logger.Info("Kubernetes operations initialized successfully")
	return nil
}

// checkKubernetesHealth checks Kubernetes health when k8s build tag is enabled
func (c *InfrastructureContainer) checkKubernetesHealth(ctx context.Context) error {
	if c.kubernetesOps == nil {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityHigh).
			Message("Kubernetes operations not initialized").
			WithLocation().
			Build()
	}

	// Test Kubernetes connection by getting server version
	serverVersion, err := c.kubernetesOps.client.Discovery().ServerVersion()
	if err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to get Kubernetes server version").Wrap(err).Build()
	}

	c.logger.Debug("Kubernetes health check passed", "version", serverVersion.String())
	return nil
}
