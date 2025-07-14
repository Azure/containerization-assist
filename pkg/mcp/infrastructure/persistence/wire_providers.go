// Package persistence provides Wire providers for state persistence
package persistence

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideStateStore creates a state store instance
func ProvideStateStore(workspaceDir string, logger *slog.Logger) workflow.StateStore {
	return NewFileStateStore(workspaceDir, logger)
}
