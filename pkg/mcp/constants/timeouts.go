package constants

import "time"

// Timeout constants for various operations throughout the Container Kit MCP system
const (
	// DefaultTimeout is the standard timeout for most operations
	DefaultTimeout = 30 * time.Second

	// BuildTimeout is the timeout for Docker build operations
	BuildTimeout = 300 * time.Second // 5 minutes

	// DeployTimeout is the timeout for Kubernetes deployment operations
	DeployTimeout = 120 * time.Second // 2 minutes

	// ValidationTimeout is the timeout for validation operations
	ValidationTimeout = 10 * time.Second

	// ScanTimeout is the timeout for security scanning operations
	ScanTimeout = 180 * time.Second // 3 minutes

	// AnalysisTimeout is the timeout for repository analysis operations
	AnalysisTimeout = 60 * time.Second // 1 minute

	// HealthCheckTimeout is the timeout for health check operations
	HealthCheckTimeout = 15 * time.Second

	// ConnectionTimeout is the timeout for network connections
	ConnectionTimeout = 10 * time.Second

	// ShutdownTimeout is the timeout for graceful shutdown
	ShutdownTimeout = 30 * time.Second

	// SessionTimeout is the timeout for session operations
	SessionTimeout = 25 * time.Second

	// RetryDelay is the default delay between retry attempts
	RetryDelay = 5 * time.Second

	// ShortRetryDelay is the delay for quick retry operations
	ShortRetryDelay = 1 * time.Second

	// LongRetryDelay is the delay for expensive retry operations
	LongRetryDelay = 15 * time.Second

	// DatabaseOperationTimeout is the timeout for database operations
	DatabaseOperationTimeout = 20 * time.Second

	// FileOperationTimeout is the timeout for file I/O operations
	FileOperationTimeout = 30 * time.Second

	// RegistryTimeout is the timeout for container registry operations
	RegistryTimeout = 90 * time.Second

	// ContextTimeout is the timeout for context operations
	ContextTimeout = 5 * time.Second
)
