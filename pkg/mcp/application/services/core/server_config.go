package core

import (
	"os"
	"path/filepath"
	"time"
)

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64
	StorePath         string
	TransportType     string
	HTTPAddr          string
	HTTPPort          int
	CORSOrigins       []string
	APIKey            string
	RateLimit         int
	SandboxEnabled    bool
	LogLevel          string
	LogHTTPBodies     bool
	MaxBodyLogSize    int64
	CleanupInterval   time.Duration
	MaxWorkers        int
	JobTTL            time.Duration
	EnableOTEL        bool
	OTELEndpoint      string
	OTELHeaders       map[string]string
	ServiceName       string
	ServiceVersion    string
	Environment       string
	TraceSampleRate   float64
}

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() ServerConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}
	workspaceDir := filepath.Join(homeDir, ".container-kit", "workspaces")
	storePath := filepath.Join(homeDir, ".container-kit", "sessions.db")

	return ServerConfig{
		WorkspaceDir:      workspaceDir,
		MaxSessions:       50,
		SessionTTL:        24 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 1024,
		TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		StorePath:         storePath,
		TransportType:     "stdio",
		HTTPAddr:          "localhost",
		HTTPPort:          8080,
		CORSOrigins:       []string{"*"},
		APIKey:            "",
		RateLimit:         60,
		SandboxEnabled:    false,
		LogLevel:          "info",
		CleanupInterval:   1 * time.Hour,
		MaxWorkers:        5,
		JobTTL:            1 * time.Hour,

		EnableOTEL:      false,
		OTELEndpoint:    "http://localhost:4318/v1/traces",
		OTELHeaders:     make(map[string]string),
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TraceSampleRate: 1.0,
	}
}
