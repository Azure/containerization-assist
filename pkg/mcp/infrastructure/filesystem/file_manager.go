// Package filesystem provides infrastructure implementations for file operations
package filesystem

import (
	"context"
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// FileSystemManager implements the workflow.FileManager interface
type FileSystemManager struct {
	logger *slog.Logger
}

// NewFileSystemManager creates a new file system manager
func NewFileSystemManager(logger *slog.Logger) workflow.FileManager {
	return &FileSystemManager{
		logger: logger.With("component", "filesystem-manager"),
	}
}

// RemoveFile removes a file if it exists
func (m *FileSystemManager) RemoveFile(ctx context.Context, path string) error {
	m.logger.Info("Removing file", "path", path)

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		m.logger.Warn("Failed to remove file", "path", path, "error", err)
		return err
	}

	if err == nil {
		m.logger.Info("File removed successfully", "path", path)
	} else {
		m.logger.Info("File does not exist, skipping removal", "path", path)
	}

	return nil
}
