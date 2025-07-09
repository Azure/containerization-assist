//go:build k8s

package infra

import (
	"context"
	"fmt"
)

// initializeKubernetesOperations initializes Kubernetes operations when k8s build tag is enabled
func (c *InfrastructureContainer) initializeKubernetesOperations() error {
	c.logger.Info("Initializing Kubernetes operations", "namespace", c.config.Namespace)

	// Create Kubernetes operations
	kubernetesOps, err := NewKubernetesOperations(c.config.KubeConfig, c.config.Namespace, c.logger)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes operations: %w", err)
	}

	c.kubernetesOps = kubernetesOps

	c.logger.Info("Kubernetes operations initialized successfully")
	return nil
}

// checkKubernetesHealth checks Kubernetes health when k8s build tag is enabled
func (c *InfrastructureContainer) checkKubernetesHealth(ctx context.Context) error {
	if c.kubernetesOps == nil {
		return fmt.Errorf("Kubernetes operations not initialized")
	}

	// Test Kubernetes connection by getting server version
	serverVersion, err := c.kubernetesOps.client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes server version: %w", err)
	}

	c.logger.Debug("Kubernetes health check passed", "version", serverVersion.String())
	return nil
}
