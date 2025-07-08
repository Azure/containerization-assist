package constants

import "time"

// Configuration constants (non-conflicting)
const (
	// DefaultMaxConnections per service
	DefaultMaxConnections = 100

	// DefaultCacheSize entries
	DefaultCacheSize = 10000

	// DefaultPort for HTTP server
	DefaultPort = 8080

	// DefaultProfilingPort for profiling server
	DefaultProfilingPort = 6060

	// DefaultMetricsPort for metrics server
	DefaultMetricsPort = 9090

	// DefaultMaxConcurrentSessions for server
	DefaultMaxConcurrentSessions = 10

	// DefaultMaxConcurrentDocker operations
	DefaultMaxConcurrentDocker = 3
)

// Size constants
const (
	// DefaultMaxRequestSize in bytes (10MB)
	DefaultMaxRequestSize = 10 * 1024 * 1024

	// DefaultMaxPreferenceSize in bytes
	DefaultMaxPreferenceSize = 1024 * 1024

	// DefaultCacheTTL for analyzer cache
	DefaultCacheTTL = 1 * time.Hour

	// DefaultMaxAnalysisTime for analysis operations
	DefaultMaxAnalysisTime = 5 * time.Minute

	// DefaultDockerBuildTimeout for Docker builds
	DefaultDockerBuildTimeout = 10 * time.Minute

	// DefaultDockerPushTimeout for Docker pushes
	DefaultDockerPushTimeout = 5 * time.Minute

	// DefaultDockerPullTimeout for Docker pulls
	DefaultDockerPullTimeout = 5 * time.Minute

	// DefaultDockerTimeout for general Docker operations
	DefaultDockerTimeout = 60 * time.Second
)

// Default values
const (
	// DefaultHost for server
	DefaultHost = "localhost"

	// DefaultLogLevel for logging
	DefaultLogLevel = "info"

	// DefaultTransportType for MCP transport
	DefaultTransportType = "stdio"

	// DefaultDockerRegistry for Docker operations
	DefaultDockerRegistry = "docker.io"

	// DefaultServiceName for server identification
	DefaultServiceName = "mcp-server"

	// DefaultServiceVersion for server identification
	DefaultServiceVersion = "1.0.0"

	// DefaultWorkspaceBase for workspaces
	DefaultWorkspaceBase = "/tmp/mcp-workspaces"

	// DefaultReplacementChar for sanitization
	DefaultReplacementChar = "-"
)
