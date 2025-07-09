package types

import "time"

// HTTP Transport Constants
const (
	// DefaultHTTPPort is the default port for HTTP transport
	DefaultHTTPPort = 8080

	// HTTPTimeoutSeconds is the default timeout for HTTP requests and responses
	HTTPTimeoutSeconds = 30 * time.Second

	// HTTPIdleTimeoutSeconds is the default idle timeout for HTTP connections
	HTTPIdleTimeoutSeconds = 120 * time.Second

	// DefaultRateLimitPerMinute is the default rate limit for HTTP requests per minute
	DefaultRateLimitPerMinute = 60

	// CORSMaxAgeSeconds is the maximum age for CORS preflight cache
	CORSMaxAgeSeconds = 300 * time.Second

	// DefaultMaxBodyLogSize is the default maximum size for request/response body logging
	DefaultMaxBodyLogSize = 10 * 1024 // 10KB
)

// Session Management Constants
const (
	// SessionCleanupInterval is the interval for cleaning up expired sessions
	SessionCleanupInterval = 1 * time.Hour
)

// Directory Permission Constants
const (
	// WorkspaceDirectoryPermissions are the default permissions for workspace directories
	WorkspaceDirectoryPermissions = 0o755

	// SecureDirectoryPermissions are the restricted permissions for secure directories
	SecureDirectoryPermissions = 0o750
)

// Resource Limit Constants
const (
	// DefaultMemoryLimit is the default memory limit for containers (512MB)
	DefaultMemoryLimit = 512 * 1024 * 1024

	// DefaultDiskQuota is the default disk quota for containers (1GB)
	DefaultDiskQuota = 1024 * 1024 * 1024

	// DefaultTotalDiskLimit is the default total disk limit (5GB)
	DefaultTotalDiskLimit = 5 * 1024 * 1024 * 1024

	// DefaultCPUQuota is the default CPU quota (100% of one CPU)
	DefaultCPUQuota = 100000

	// RestrictedCPUQuota is the restricted CPU quota (50% of one CPU)
	RestrictedCPUQuota = 50000

	// ResourceAlertThreshold is the threshold for resource usage alerts (80%)
	ResourceAlertThreshold = 0.8

	// ResourceMonitoringInterval is the interval for resource monitoring
	ResourceMonitoringInterval = 100 * time.Millisecond
)

// Retry Configuration Constants
const (
	// DefaultMaxRetryAttempts is the default maximum number of retry attempts
	DefaultMaxRetryAttempts = 3

	// DefaultRetryBackoffMultiplier is the default backoff multiplier for retries
	DefaultRetryBackoffMultiplier = 2.0

	// DefaultMaxRetryDelaySeconds is the default maximum delay between retries
	DefaultMaxRetryDelaySeconds = 30 * time.Second

	// RetryJitterFactor is the factor for adding jitter to retry delays (±25%)
	RetryJitterFactor = 0.5
)

// Buffer and Storage Constants
const (
	// DefaultRingBufferCapacity is the default capacity for ring buffers
	DefaultRingBufferCapacity = 1000

	// GitCloneReservedSpace is the reserved space for git clone operations (100MB)
	GitCloneReservedSpace = 100 * 1024 * 1024

	// ContainerTmpfsSize is the size limit for container tmpfs mounts (100MB)
	ContainerTmpfsSize = 100
)
