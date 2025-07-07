package config

import "time"

const (
	DefaultTimeout      = 30 * time.Second
	DefaultConnTimeout  = 10 * time.Second
	DefaultReadTimeout  = 15 * time.Second
	DefaultWriteTimeout = 15 * time.Second
	DefaultIdleTimeout  = 60 * time.Second

	DefaultPollInterval  = 5 * time.Second
	DefaultRetryInterval = 1 * time.Second
	FastPollInterval     = 1 * time.Second
	SlowPollInterval     = 30 * time.Second
	ShutdownGracePeriod  = 30 * time.Second
	HealthCheckInterval  = 10 * time.Second

	CacheCleanupInterval   = 5 * time.Minute
	SessionCleanupInterval = 10 * time.Minute
	MetricsFlushInterval   = 100 * time.Millisecond
	TelemetryInterval      = 30 * time.Second

	P95Target         = 300 * time.Microsecond
	P99Target         = 1 * time.Millisecond
	MaxProcessingTime = 5 * time.Second
)

const (
	DefaultCacheSize         = 1000
	SchemaGeneratorCacheSize = 500
	RefResolverCacheSize     = 1000
	SessionCacheSize         = 100
	ToolRegistryCacheSize    = 200

	DefaultBatchSize  = 100
	DefaultBufferSize = 1024
	MetricsBufferSize = 1000
	EventBufferSize   = 500
	LogBufferSize     = 256

	MaxRequestSize     = 10 * 1024 * 1024
	MaxResponseSize    = 50 * 1024 * 1024
	MaxFileSize        = 100 * 1024 * 1024
	MaxEventHistory    = 1000
	MaxStageHistory    = 50
	MaxSessionsPerUser = 10
)

const (
	DefaultHTTPPort    = 8080
	DefaultMetricsPort = 8081
	DefaultGRPCPort    = 9090
	DefaultDebugPort   = 6060

	PostgresPort = 5432
	RedisPort    = 6379
	MongoDBPort  = 27017
	MySQLPort    = 3306
	HTTPSPort    = 443
	HTTPPort     = 80

	MaxConnections        = 500
	MaxIdleConnections    = 100
	MaxConnectionsPerHost = 50
	MaxConcurrentRequests = 200
)

const (
	MaxRetries          = 3
	MaxRetriesExpensive = 1
	MaxRetriesQuick     = 5
	BackoffMultiplier   = 2.0
	MaxBackoffTime      = 1 * time.Minute
	InitialBackoffTime  = 100 * time.Millisecond

	CircuitBreakerThreshold = 5
	CircuitBreakerTimeout   = 30 * time.Second
	CircuitBreakerHalfOpen  = 2

	RateLimitRequests = 100
	RateLimitWindow   = 1 * time.Minute
	BurstLimit        = 10
)

const (
	DefaultWorkerPoolSize   = 10
	MaxWorkerPoolSize       = 100
	MinWorkerPoolSize       = 1
	TelemetryWorkerPoolSize = 5
	AnalysisWorkerPoolSize  = 3
	BuildWorkerPoolSize     = 2

	MaxGoroutines           = 100
	MaxBackgroundWorkers    = 20
	MaxConcurrentOperations = 50

	MaxOpenFiles   = 1000
	MaxMemoryUsage = 1024 * 1024 * 1024
	MaxCPUUsage    = 80

	TaskQueueSize         = 200
	EventQueueSize        = 1000
	NotificationQueueSize = 100
)

const (
	MaxNameLength        = 255
	MaxDescriptionLength = 1000
	MaxCommentLength     = 500
	MinPasswordLength    = 8
	MaxPasswordLength    = 128

	ToolNamePattern  = `^[a-zA-Z][a-zA-Z0-9_-]*$`
	SessionIDPattern = `^[a-zA-Z0-9-]{36}$`
	UserIDPattern    = `^[a-zA-Z0-9_-]+$`

	MinPort      = 1024
	MaxPort      = 65535
	MinCacheSize = 10
	MaxCacheSize = 10000
	MinWorkers   = 1
	MaxWorkers   = 1000
)

const (
	DevLogLevel       = "debug"
	DevMetricsEnabled = true
	DevTracingEnabled = true
	DevCacheSize      = 100

	ProdLogLevel       = "info"
	ProdMetricsEnabled = true
	ProdTracingEnabled = true
	ProdCacheSize      = 1000

	TestLogLevel       = "warn"
	TestMetricsEnabled = false
	TestTracingEnabled = false
	TestCacheSize      = 50
	TestTimeout        = 5 * time.Second
)

const (
	MaxErrorsBeforeFailure = 10
	ErrorTrackingWindow    = 1 * time.Hour
	CriticalErrorThreshold = 5

	MaxRecoveryAttempts    = 3
	RecoveryBackoffTime    = 5 * time.Second
	ErrorReportingInterval = 1 * time.Minute
)

const (
	MetricsSampleRate  = 1.0
	FastPathSampleRate = 0.01

	TraceSampleRate        = 0.1
	SlowOperationThreshold = 1 * time.Second

	HealthCheckTimeout = 5 * time.Second
	UnhealthyThreshold = 3
	HealthyThreshold   = 2

	AlertCooldownPeriod    = 15 * time.Minute
	CriticalAlertThreshold = 0.95
	WarningAlertThreshold  = 0.80
)

const (
	DefaultFilePermissions = 0644
	DefaultDirPermissions  = 0755
	TempFilePrefix         = "mcp-"
	BackupFileSuffix       = ".backup"

	DefaultConfigPath = "/etc/mcp"
	DefaultDataPath   = "/var/lib/mcp"
	DefaultLogPath    = "/var/log/mcp"
	DefaultTempPath   = "/tmp/mcp"

	MaxLogFileSize    = 100 * 1024 * 1024
	MaxConfigFileSize = 1 * 1024 * 1024
	MaxSchemaFileSize = 5 * 1024 * 1024
)

const (
	DefaultToolTimeout = 5 * time.Minute
	QuickToolTimeout   = 30 * time.Second
	LongToolTimeout    = 15 * time.Minute

	MaxConcurrentTools = 10
	MaxToolHistory     = 100
	ToolCacheExpiry    = 1 * time.Hour

	QuickToolCategory    = "quick"
	StandardToolCategory = "standard"
	LongToolCategory     = "long"
)
