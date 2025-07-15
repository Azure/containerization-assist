// Package persistence provides unified dependency injection for data persistence services
package persistence

import (
	"fmt"
	"log/slog"
	"path/filepath"

	applicationsession "github.com/Azure/container-kit/pkg/mcp/application/session"
	domainresources "github.com/Azure/container-kit/pkg/mcp/domain/resources"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/resources"
	"github.com/google/wire"
)

// Providers provides all persistence domain dependencies
var Providers = wire.NewSet(
	// Session management
	ProvideSessionManager,

	// State storage
	ProvideStateStore,

	// Resource storage
	ProvideResourceStore,

	// Interface bindings would go here if needed
)

// ProvideSessionManager creates an optimized session manager
func ProvideSessionManager(config workflow.ServerConfig, logger *slog.Logger) (applicationsession.OptimizedSessionManager, error) {
	// Use StorePath from config or default to workspace dir
	dbPath := config.StorePath
	if dbPath == "" {
		dbPath = filepath.Join(config.WorkspaceDir, "sessions.db")
	}

	// Create the adapter that implements OptimizedSessionManager
	adapter, err := applicationsession.NewBoltStoreAdapter(dbPath, logger, config.SessionTTL, config.MaxSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	return adapter, nil
}

// ProvideResourceStore creates a resource store
func ProvideResourceStore(logger *slog.Logger) domainresources.Store {
	return resources.NewStore(logger)
}

// ProvideStateStore creates a workflow state store
func ProvideStateStore(config workflow.ServerConfig, logger *slog.Logger) workflow.StateStore {
	return NewFileStateStore(config.WorkspaceDir, logger)
}
