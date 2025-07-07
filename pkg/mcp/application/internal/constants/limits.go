package constants

// Resource limits and constraints for the Container Kit MCP system
const (
	// MaxErrors is the maximum number of validation errors allowed
	MaxErrors = 100

	// MaxSessions is the maximum number of concurrent sessions
	MaxSessions = 1000

	// MaxDiskPerSession is the maximum disk space per session (1GB)
	MaxDiskPerSession = 1 << 30 // 1GB

	// TotalDiskLimit is the total disk space limit (10GB)
	TotalDiskLimit = 10 << 30 // 10GB

	// MaxBodyLogSize is the maximum size of HTTP body to log (1MB)
	MaxBodyLogSize = 1 << 20 // 1MB

	// MaxWorkers is the maximum number of worker goroutines
	MaxWorkers = 10

	// MaxRetries is the maximum number of retry attempts
	MaxRetries = 5

	// MaxRetryAttempts is an alternative max retry constant
	MaxRetryAttempts = 3

	// MaxValidationErrors is the maximum validation errors per operation
	MaxValidationErrors = 50

	// MaxConcurrentBuilds is the maximum number of concurrent Docker builds
	MaxConcurrentBuilds = 3

	// MaxConcurrentScans is the maximum number of concurrent security scans
	MaxConcurrentScans = 5

	// MaxFileSize is the maximum file size for processing (100MB)
	MaxFileSize = 100 << 20 // 100MB

	// MaxLogFileSize is the maximum log file size (50MB)
	MaxLogFileSize = 50 << 20 // 50MB

	// MaxMemoryUsage is the maximum memory usage per operation (2GB)
	MaxMemoryUsage = 2 << 30 // 2GB

	// MaxCacheSize is the maximum cache size (1GB)
	MaxCacheSize = 1 << 30 // 1GB

	// MaxQueueSize is the maximum queue size for async operations
	MaxQueueSize = 1000

	// MaxBatchSize is the maximum batch size for batch operations
	MaxBatchSize = 100

	// MaxConnectionPoolSize is the maximum database connection pool size
	MaxConnectionPoolSize = 20

	// MaxIdleConnections is the maximum number of idle database connections
	MaxIdleConnections = 5

	// MaxItemsPerPage is the maximum items returned per page
	MaxItemsPerPage = 100

	// DefaultPageSize is the default page size for pagination
	DefaultPageSize = 20

	// MaxUploadSize is the maximum upload file size (500MB)
	MaxUploadSize = 500 << 20 // 500MB

	// MaxDownloadSize is the maximum download file size (1GB)
	MaxDownloadSize = 1 << 30 // 1GB
)
