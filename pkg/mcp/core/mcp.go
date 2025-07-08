// Package core provides the core API for the Container Kit MCP (Model Context Protocol) server.
//
// This package exposes a clean, minimal API surface that provides access to all
// containerization tools and orchestration capabilities while keeping implementation
// details hidden in internal packages.
package core

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() ServerConfig {
	homeDir, _ := os.UserHomeDir()
	defaultWorkspace := filepath.Join(homeDir, ".container-kit", "workspace")
	defaultStore := filepath.Join(homeDir, ".container-kit", "sessions.db")

	return ServerConfig{
		WorkspaceDir:      defaultWorkspace,
		MaxSessions:       100,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1 << 30,
		TotalDiskLimit:    10 << 30,
		StorePath:         defaultStore, TransportType: "stdio",
		HTTPAddr: "localhost",
		HTTPPort: 8080, SandboxEnabled: false, LogLevel: "info",
		LogHTTPBodies:  false,
		MaxBodyLogSize: 1 << 20, CleanupInterval: 1 * time.Hour, MaxWorkers: 10,
		JobTTL: 1 * time.Hour, ServiceName: "container-kit-mcp",
		ServiceVersion: "dev",
		Environment:    "development",
	}
}
