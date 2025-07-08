//go:build docker

package infra

import (
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/core/docker"
)

// initializeDockerOperations initializes Docker operations when docker build tag is enabled
func (c *InfrastructureContainer) initializeDockerOperations() error {
	c.logger.Info("Initializing Docker operations", "host", c.config.DockerHost)
	
	// Create Docker client
	dockerClient, err := docker.NewClient(docker.ClientConfig{
		Host:     c.config.DockerHost,
		TLS:      c.config.DockerTLS,
		CertPath: c.config.DockerCerts,
	})
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	
	// Create Docker operations
	c.dockerOps = NewDockerOperations(dockerClient, c.logger)
	
	c.logger.Info("Docker operations initialized successfully")
	return nil
}

// checkDockerHealth checks Docker health when docker build tag is enabled
func (c *InfrastructureContainer) checkDockerHealth(ctx context.Context) error {
	if c.dockerOps == nil {
		return fmt.Errorf("Docker operations not initialized")
	}
	
	// Test Docker connection by getting version info
	versionInfo, err := c.dockerOps.client.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker version: %w", err)
	}
	
	c.logger.Debug("Docker health check passed", "version", versionInfo.Version)
	return nil
}