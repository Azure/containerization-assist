package core

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/config"
)

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() config.ServerConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp directory if home directory cannot be determined
		homeDir = os.TempDir()
	}
	workspaceDir := filepath.Join(homeDir, ".container-kit", "workspaces")
	storePath := filepath.Join(homeDir, ".container-kit", "sessions.db")

	return config.ServerConfig{
		WorkspaceDir:      workspaceDir,
		MaxSessions:       50,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,      // 1GB
		TotalDiskLimit:    10 * 1024 * 1024 * 1024, // 10GB
		StorePath:         storePath,
		TransportType:     "stdio",
		HTTPAddr:          "localhost",
		HTTPPort:          8080,
		CORSOrigins:       []string{"*"}, // Allow all origins by default
		APIKey:            "",            // No auth by default
		RateLimit:         60,            // 60 requests per minute
		SandboxEnabled:    false,
		LogLevel:          "info",
		CleanupInterval:   1 * time.Hour,
		MaxWorkers:        5,
		JobTTL:            1 * time.Hour,

		// Service identification (these fields already exist in ServerConfig)
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TraceSampleRate: 1.0,
	}
}
