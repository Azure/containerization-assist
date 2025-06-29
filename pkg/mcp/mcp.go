// Package mcp provides unified interfaces and types for the MCP server.
// This package contains only interface definitions and types.
// Implementations are in internal packages.
//
// According to the interface unification architecture, this package should NOT
// import from internal packages to avoid import cycles.
package mcp

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// Note: With unified interfaces, import cycles have been resolved.
// Use github.com/Azure/container-kit/pkg/mcp/internal/core.NewServer for server creation.

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() core.ServerConfig {
	homeDir, _ := os.UserHomeDir()
	defaultWorkspace := filepath.Join(homeDir, ".container-kit", "workspace")
	defaultStore := filepath.Join(homeDir, ".container-kit", "sessions.db")

	return core.ServerConfig{
		// Session management
		WorkspaceDir:      defaultWorkspace,
		MaxSessions:       100,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1 << 30,  // 1GB
		TotalDiskLimit:    10 << 30, // 10GB

		// Storage
		StorePath: defaultStore,

		// Transport
		TransportType: "stdio",
		HTTPAddr:      "localhost",
		HTTPPort:      8080,

		// Features
		SandboxEnabled: false,

		// Logging
		LogLevel:       "info",
		LogHTTPBodies:  false,
		MaxBodyLogSize: 1 << 20, // 1MB

		// Cleanup
		CleanupInterval: 1 * time.Hour,

		// Job Management
		MaxWorkers: 10,
		JobTTL:     1 * time.Hour,

		// OpenTelemetry defaults
		EnableOTEL:      false,
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "dev",
		Environment:     "development",
		TraceSampleRate: 1.0,
	}
}
