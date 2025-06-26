package core

import (
	"os"
	"path/filepath"
	"time"
)

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	// Session management
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64

	// Storage
	StorePath string

	// Transport
	TransportType string // "stdio", "http"
	HTTPAddr      string
	HTTPPort      int
	CORSOrigins   []string // CORS allowed origins
	APIKey        string   // API key for authentication
	RateLimit     int      // Requests per minute per IP

	// Features
	SandboxEnabled bool

	// Logging
	LogLevel       string
	LogHTTPBodies  bool  // Log HTTP request/response bodies
	MaxBodyLogSize int64 // Maximum size of bodies to log

	// Cleanup
	CleanupInterval time.Duration

	// Job Management
	MaxWorkers int
	JobTTL     time.Duration

	// OpenTelemetry configuration
	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() ServerConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp directory if home directory cannot be determined
		homeDir = os.TempDir()
	}
	workspaceDir := filepath.Join(homeDir, ".container-kit", "workspaces")
	storePath := filepath.Join(homeDir, ".container-kit", "sessions.db")

	return ServerConfig{
		WorkspaceDir:      workspaceDir,
		MaxSessions:       10,
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

		// OpenTelemetry defaults
		EnableOTEL:      false,
		OTELEndpoint:    "http://localhost:4318/v1/traces",
		OTELHeaders:     make(map[string]string),
		ServiceName:     "container-kit-mcp",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TraceSampleRate: 1.0,
	}
}
