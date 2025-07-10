// Package infra provides infrastructure layer components for external integrations
//
// This package implements the infrastructure layer of the three-context architecture,
// handling all external integrations including Docker, Kubernetes, persistence,
// templates, and transport protocols.
//
// Architecture:
//   - Docker Operations (with build tags)
//   - Kubernetes Operations (with build tags)
//   - BoltDB Persistence
//   - Template Management
//   - Transport Protocols (HTTP, stdio)
//
// Build Tags:
//   - docker: Enables Docker operations
//   - k8s: Enables Kubernetes operations
//
// Usage:
//
//	go build -tags docker,k8s ./cmd/mcp-server
package infra

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// DockerOperationsInterface defines the interface for Docker operations
type DockerOperationsInterface interface {
	// Methods would be defined here - for now, just a placeholder
}

// KubernetesOperationsInterface defines the interface for Kubernetes operations
type KubernetesOperationsInterface interface {
	// Methods would be defined here - for now, just a placeholder
}

// InfrastructureContainer provides access to all infrastructure components
type InfrastructureContainer struct {
	// Persistence
	persistence *BoltDBPersistence

	// Templates
	templateService     TemplateService
	dockerfileGenerator *DockerfileGenerator
	manifestGenerator   *ManifestGenerator

	// Docker operations (available with docker build tag)
	dockerOps DockerOperationsInterface

	// Kubernetes operations (available with k8s build tag)
	kubernetesOps KubernetesOperationsInterface

	// Configuration
	config *InfrastructureConfig
	logger *slog.Logger
}

// InfrastructureConfig holds configuration for infrastructure components
type InfrastructureConfig struct {
	// Persistence configuration
	DatabasePath string
	BackupPath   string

	// Docker configuration
	DockerHost  string
	DockerTLS   bool
	DockerCerts string

	// Kubernetes configuration
	KubeConfig string
	Namespace  string

	// Template configuration
	TemplateDir string
	CacheSize   int

	// Transport configuration
	HTTPPort    int
	HTTPHost    string
	EnableHTTPS bool
	CertFile    string
	KeyFile     string
	EnableStdio bool

	// General configuration
	LogLevel    string
	MetricsPort int
	Timeout     time.Duration
}

// DefaultInfrastructureConfig returns default configuration
func DefaultInfrastructureConfig() *InfrastructureConfig {
	return &InfrastructureConfig{
		DatabasePath: "./data/mcp.db",
		BackupPath:   "./backups",
		DockerHost:   "unix:///var/run/docker.sock",
		DockerTLS:    false,
		KubeConfig:   "", // Use in-cluster config by default
		Namespace:    "default",
		TemplateDir:  "./templates",
		CacheSize:    100,
		HTTPPort:     8080,
		HTTPHost:     "0.0.0.0",
		EnableHTTPS:  false,
		EnableStdio:  true,
		LogLevel:     "info",
		MetricsPort:  9090,
		Timeout:      30 * time.Second,
	}
}

// NewInfrastructureContainer creates a new infrastructure container
func NewInfrastructureContainer(config *InfrastructureConfig, logger *slog.Logger) (*InfrastructureContainer, error) {
	if config == nil {
		config = DefaultInfrastructureConfig()
	}

	container := &InfrastructureContainer{
		config: config,
		logger: logger,
	}

	// Initialize persistence
	if err := container.initializePersistence(); err != nil {
		return nil, errors.NewError().Code(errors.CodeInternalError).Message("failed to initialize persistence").Cause(err).Build()
	}

	// Initialize templates
	if err := container.initializeTemplates(); err != nil {
		return nil, errors.NewError().Code(errors.CodeInternalError).Message("failed to initialize templates").Cause(err).Build()
	}

	// Initialize Docker operations (if build tag is enabled)
	if err := container.initializeDockerOperations(); err != nil {
		logger.Warn("Docker operations not available", "error", err)
	}

	// Initialize Kubernetes operations (if build tag is enabled)
	if err := container.initializeKubernetesOperations(); err != nil {
		logger.Warn("Kubernetes operations not available", "error", err)
	}

	logger.Info("Infrastructure container initialized successfully")
	return container, nil
}

// initializePersistence initializes the persistence layer
func (c *InfrastructureContainer) initializePersistence() error {
	persistence, err := NewBoltDBPersistence(c.config.DatabasePath, c.logger)
	if err != nil {
		return errors.NewError().Code(errors.CodeInternalError).Message("failed to create BoltDB persistence").Cause(err).Build()
	}

	c.persistence = persistence
	return nil
}

// initializeTemplates initializes the template management
func (c *InfrastructureContainer) initializeTemplates() error {
	c.templateService = NewTemplateService(c.logger)
	c.dockerfileGenerator = NewDockerfileGenerator(c.logger)
	c.manifestGenerator = NewManifestGenerator(c.logger)
	return nil
}

// initializeDockerOperations initializes Docker operations (build tag dependent)
func (c *InfrastructureContainer) initializeDockerOperations() error {
	// This will be implemented with build tags
	// For now, return an error indicating Docker is not available
	return errors.NewError().Code(errors.CodeResourceNotFound).Message("Docker operations not available (build without docker tag)").Build()
}

// initializeKubernetesOperations initializes Kubernetes operations (build tag dependent)
func (c *InfrastructureContainer) initializeKubernetesOperations() error {
	// This will be implemented with build tags
	// For now, return an error indicating Kubernetes is not available
	return errors.NewError().Code(errors.CodeResourceNotFound).Message("Kubernetes operations not available (build without k8s tag)").Build()
}

// Service interface implementations for dependency injection

// Persistence returns the persistence service
func (c *InfrastructureContainer) Persistence() services.Persistence {
	return c.persistence
}

// TemplateService returns the template service
func (c *InfrastructureContainer) TemplateService() TemplateService {
	return c.templateService
}

// DockerfileGenerator returns the Dockerfile generator
func (c *InfrastructureContainer) DockerfileGenerator() *DockerfileGenerator {
	return c.dockerfileGenerator
}

// ManifestGenerator returns the manifest generator
func (c *InfrastructureContainer) ManifestGenerator() *ManifestGenerator {
	return c.manifestGenerator
}

// DockerOperations returns Docker operations (if available)
func (c *InfrastructureContainer) DockerOperations() DockerOperationsInterface {
	return c.dockerOps
}

// KubernetesOperations returns Kubernetes operations (if available)
func (c *InfrastructureContainer) KubernetesOperations() KubernetesOperationsInterface {
	return c.kubernetesOps
}

// Health check and monitoring

// HealthCheck performs a health check on all infrastructure components
func (c *InfrastructureContainer) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		Overall:    "healthy",
		Components: make(map[string]ComponentHealth),
		Timestamp:  time.Now(),
	}

	// Check persistence health
	if err := c.checkPersistenceHealth(ctx); err != nil {
		status.Components["persistence"] = ComponentHealth{
			Status: "unhealthy",
			Error:  err.Error(),
		}
		status.Overall = "unhealthy"
	} else {
		status.Components["persistence"] = ComponentHealth{
			Status: "healthy",
		}
	}

	// Check templates health
	if err := c.checkTemplatesHealth(ctx); err != nil {
		status.Components["templates"] = ComponentHealth{
			Status: "unhealthy",
			Error:  err.Error(),
		}
		status.Overall = "degraded"
	} else {
		status.Components["templates"] = ComponentHealth{
			Status: "healthy",
		}
	}

	// Check Docker health (if available)
	if c.dockerOps != nil {
		if err := c.checkDockerHealth(ctx); err != nil {
			status.Components["docker"] = ComponentHealth{
				Status: "unhealthy",
				Error:  err.Error(),
			}
			status.Overall = "degraded"
		} else {
			status.Components["docker"] = ComponentHealth{
				Status: "healthy",
			}
		}
	}

	// Check Kubernetes health (if available)
	if c.kubernetesOps != nil {
		if err := c.checkKubernetesHealth(ctx); err != nil {
			status.Components["kubernetes"] = ComponentHealth{
				Status: "unhealthy",
				Error:  err.Error(),
			}
			status.Overall = "degraded"
		} else {
			status.Components["kubernetes"] = ComponentHealth{
				Status: "healthy",
			}
		}
	}

	return status, nil
}

// HealthStatus represents the health status of infrastructure components
type HealthStatus struct {
	Overall    string                     `json:"overall"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  time.Time                  `json:"timestamp"`
}

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	LastCheck time.Time         `json:"last_check"`
}

// checkPersistenceHealth checks the health of the persistence layer
func (c *InfrastructureContainer) checkPersistenceHealth(ctx context.Context) error {
	// Test basic database operations
	testKey := "health_check"
	testValue := map[string]string{"timestamp": time.Now().Format(time.RFC3339)}

	if err := c.persistence.Put(ctx, BucketConfiguration, testKey, testValue); err != nil {
		return errors.NewError().Code(errors.CodeIOError).Message("failed to write to database").Cause(err).Build()
	}

	var result map[string]string
	if err := c.persistence.Get(ctx, BucketConfiguration, testKey, &result); err != nil {
		return errors.NewError().Code(errors.CodeIOError).Message("failed to read from database").Cause(err).Build()
	}

	if err := c.persistence.Delete(ctx, BucketConfiguration, testKey); err != nil {
		return errors.NewError().Code(errors.CodeIOError).Message("failed to delete from database").Cause(err).Build()
	}

	return nil
}

// checkTemplatesHealth checks the health of the template system
func (c *InfrastructureContainer) checkTemplatesHealth(ctx context.Context) error {
	// Test template listing
	workflowTemplates, err := c.templateService.ListTemplates(TemplateTypeWorkflow)
	if err != nil {
		return errors.NewError().Code(errors.CodeIOError).Message("failed to list workflow templates").Cause(err).Build()
	}

	if len(workflowTemplates) == 0 {
		return errors.NewError().Code(errors.CodeResourceNotFound).Message("no workflow templates found").Build()
	}

	// Test template rendering
	if len(workflowTemplates) > 0 {
		_, err := c.templateService.RenderTemplate(TemplateRenderParams{
			Name: workflowTemplates[0],
			Type: TemplateTypeWorkflow,
			Variables: map[string]interface{}{
				"TestVar": "test_value",
			},
		})
		if err != nil {
			return errors.NewError().Code(errors.CodeInternalError).Message("failed to render template").Cause(err).Build()
		}
	}

	return nil
}

// checkDockerHealth checks the health of Docker operations
func (c *InfrastructureContainer) checkDockerHealth(ctx context.Context) error {
	// This would be implemented with build tags
	return errors.NewError().Code(errors.CodeResourceNotFound).Message("Docker health check not implemented").Build()
}

// checkKubernetesHealth checks the health of Kubernetes operations
func (c *InfrastructureContainer) checkKubernetesHealth(ctx context.Context) error {
	// This would be implemented with build tags
	return errors.NewError().Code(errors.CodeResourceNotFound).Message("Kubernetes health check not implemented").Build()
}

// Cleanup and shutdown

// Close shuts down all infrastructure components
func (c *InfrastructureContainer) Close() error {
	var errs []error

	// Close persistence
	if c.persistence != nil {
		if err := c.persistence.Close(); err != nil {
			errs = append(errs, errors.NewError().Code(errors.CodeIOError).Message("failed to close persistence").Cause(err).Build())
		}
	}

	// Additional cleanup for other components would go here

	if len(errs) > 0 {
		return errors.NewError().Code(errors.CodeInternalError).Message("errors during shutdown").Context("errors", errs).Build()
	}

	c.logger.Info("Infrastructure container closed successfully")
	return nil
}

// Backup creates a backup of all infrastructure data
func (c *InfrastructureContainer) Backup(ctx context.Context, backupPath string) error {
	c.logger.Info("Creating infrastructure backup", "backup_path", backupPath)

	// Create database backup
	if c.persistence != nil {
		if err := c.persistence.Backup(ctx, backupPath+"/database.db"); err != nil {
			return errors.NewError().Code(errors.CodeIOError).Message("failed to backup database").Cause(err).Build()
		}
	}

	// Additional backup operations would go here

	c.logger.Info("Infrastructure backup completed successfully")
	return nil
}

// GetStats returns statistics about infrastructure components
func (c *InfrastructureContainer) GetStats(ctx context.Context) (*InfrastructureStats, error) {
	stats := &InfrastructureStats{
		Timestamp: time.Now(),
	}

	// Get persistence stats
	if c.persistence != nil {
		persistenceStats, err := c.persistence.Stats(ctx)
		if err != nil {
			return nil, errors.NewError().Code(errors.CodeIOError).Message("failed to get persistence stats").Cause(err).Build()
		}
		stats.Persistence = persistenceStats
	}

	// Get template stats
	stats.Templates = &TemplateStats{
		WorkflowCount:   c.getTemplateCount(TemplateTypeWorkflow),
		ManifestCount:   c.getTemplateCount(TemplateTypeManifest),
		DockerfileCount: c.getTemplateCount(TemplateTypeDockerfile),
		ComponentCount:  c.getTemplateCount(TemplateTypeComponent),
	}

	return stats, nil
}

// InfrastructureStats represents statistics about infrastructure components
type InfrastructureStats struct {
	Timestamp   time.Time         `json:"timestamp"`
	Persistence *PersistenceStats `json:"persistence,omitempty"`
	Templates   *TemplateStats    `json:"templates,omitempty"`
	Docker      *DockerStats      `json:"docker,omitempty"`
	Kubernetes  *KubernetesStats  `json:"kubernetes,omitempty"`
}

// TemplateStats represents template statistics
type TemplateStats struct {
	WorkflowCount   int `json:"workflow_count"`
	ManifestCount   int `json:"manifest_count"`
	DockerfileCount int `json:"dockerfile_count"`
	ComponentCount  int `json:"component_count"`
}

// DockerStats represents Docker statistics
type DockerStats struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version"`
}

// KubernetesStats represents Kubernetes statistics
type KubernetesStats struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version"`
	Namespace string `json:"namespace"`
}

// getTemplateCount gets the count of templates by type
func (c *InfrastructureContainer) getTemplateCount(templateType TemplateType) int {
	templates, err := c.templateService.ListTemplates(templateType)
	if err != nil {
		return 0
	}
	return len(templates)
}
