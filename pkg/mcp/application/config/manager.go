// Package config handles server configuration management
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// Manager handles server configuration and validation
type Manager interface {
	GetConfig() workflow.ServerConfig
	ValidateConfig() error
	EnsureDirectories() error
}

// managerImpl implements the configuration manager
type managerImpl struct {
	config workflow.ServerConfig
	logger *slog.Logger
}

// NewManager creates a new configuration manager
func NewManager(config workflow.ServerConfig, logger *slog.Logger) Manager {
	return &managerImpl{
		config: config,
		logger: logger.With("component", "config_manager"),
	}
}

// GetConfig returns the server configuration
func (m *managerImpl) GetConfig() workflow.ServerConfig {
	return m.config
}

// ValidateConfig validates the server configuration
func (m *managerImpl) ValidateConfig() error {
	// Validate transport type
	if m.config.TransportType != "stdio" {
		return fmt.Errorf("unsupported transport type: %s", m.config.TransportType)
	}

	// Validate paths
	if m.config.WorkspaceDir == "" {
		return fmt.Errorf("workspace directory is required")
	}

	if m.config.StorePath == "" {
		return fmt.Errorf("storage path is required")
	}

	// Validate session limits
	if m.config.MaxSessions <= 0 {
		return fmt.Errorf("max sessions must be positive, got %d", m.config.MaxSessions)
	}

	if m.config.SessionTTL <= 0 {
		return fmt.Errorf("session timeout must be positive, got %v", m.config.SessionTTL)
	}

	m.logger.Debug("Configuration validated successfully",
		"transport", m.config.TransportType,
		"workspace", m.config.WorkspaceDir,
		"max_sessions", m.config.MaxSessions)

	return nil
}

// EnsureDirectories creates necessary directories if they don't exist
func (m *managerImpl) EnsureDirectories() error {
	// Create workspace directory
	if err := m.ensureDirectory(m.config.WorkspaceDir, "workspace"); err != nil {
		return err
	}

	// Create storage directory
	if err := m.ensureDirectory(m.config.StorePath, "storage"); err != nil {
		return err
	}

	// Create sessions subdirectory in storage
	sessionsDir := filepath.Join(m.config.StorePath, "sessions")
	if err := m.ensureDirectory(sessionsDir, "sessions storage"); err != nil {
		return err
	}

	// Create resources subdirectory in storage
	resourcesDir := filepath.Join(m.config.StorePath, "resources")
	if err := m.ensureDirectory(resourcesDir, "resources storage"); err != nil {
		return err
	}

	m.logger.Info("All required directories ensured",
		"workspace", m.config.WorkspaceDir,
		"storage", m.config.StorePath)

	return nil
}

// ensureDirectory creates a directory if it doesn't exist
func (m *managerImpl) ensureDirectory(path, description string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve %s path: %w", description, err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", description, err)
	}

	m.logger.Debug("Directory ensured", "type", description, "path", absPath)
	return nil
}

// Builder provides a fluent interface for building configurations
type Builder struct {
	config workflow.ServerConfig
}

// NewBuilder creates a new configuration builder
func NewBuilder() *Builder {
	return &Builder{
		config: workflow.DefaultServerConfig(),
	}
}

// WithWorkspaceDir sets the workspace directory
func (b *Builder) WithWorkspaceDir(dir string) *Builder {
	b.config.WorkspaceDir = dir
	return b
}

// WithStoragePath sets the storage path
func (b *Builder) WithStoragePath(path string) *Builder {
	b.config.StorePath = path
	return b
}

// WithMaxSessions sets the maximum number of sessions
func (b *Builder) WithMaxSessions(max int) *Builder {
	b.config.MaxSessions = max
	return b
}

// WithSessionTimeout sets the session timeout
func (b *Builder) WithSessionTimeout(timeout int) *Builder {
	b.config.SessionTTL = time.Duration(timeout) * time.Second
	return b
}

// WithTransportType sets the transport type
func (b *Builder) WithTransportType(transportType string) *Builder {
	b.config.TransportType = transportType
	return b
}

// Build returns the built configuration
func (b *Builder) Build() workflow.ServerConfig {
	return b.config
}
