// Package config - Configuration constants consolidation
// This file consolidates configuration constants from scattered locations
package config

import "time"

// ============================================================================
// Server Configuration Constants
// ============================================================================

const (
	// Default server ports
	DefaultHTTPPort      = 8090
	DefaultMetricsPort   = 9090
	DefaultProfilingPort = 6060
	DefaultHealthPort    = 8091

	// Default session limits
	DefaultMaxSessions           = 10
	DefaultMaxConcurrentSessions = 10
	DefaultSessionTTL            = 24 * time.Hour
	DefaultMaxDiskPerSession     = 1024 * 1024 * 1024      // 1GB
	DefaultTotalDiskLimit        = 10 * 1024 * 1024 * 1024 // 10GB

	// Default worker limits
	DefaultMaxWorkers = 5
	DefaultJobTTL     = time.Hour

	// Default timeouts
	DefaultReadTimeout      = 30 * time.Second
	DefaultWriteTimeout     = 30 * time.Second
	DefaultIdleTimeout      = 60 * time.Second
	DefaultShutdownTimeout  = 30 * time.Second
	DefaultKeepAliveTimeout = 3 * time.Minute

	// Default request limits
	DefaultMaxConcurrentRequests = 100
	DefaultMaxRequestSize        = 10 * 1024 * 1024 // 10MB
	DefaultMaxHeaderSize         = 8192
	DefaultRateLimit             = 100 // requests per minute

	// Default cleanup
	DefaultCleanupInterval = time.Hour
)

// ============================================================================
// Docker Configuration Constants
// ============================================================================

const (
	// Docker operation timeouts
	DefaultDockerBuildTimeout = 10 * time.Minute
	DefaultDockerPushTimeout  = 5 * time.Minute
	DefaultDockerPullTimeout  = 5 * time.Minute
	DefaultDockerStartTimeout = 30 * time.Second
	DefaultDockerStopTimeout  = 10 * time.Second

	// Docker operation limits
	DefaultMaxConcurrentDocker = 3
	DefaultDockerMemoryLimit   = 2 * 1024 * 1024 * 1024 // 2GB
	DefaultDockerCPULimit      = 2.0                    // 2 cores

	// Docker registry defaults
	DefaultRegistryTimeout    = 30 * time.Second
	DefaultRegistryRetries    = 3
	DefaultRegistryRetryDelay = 5 * time.Second
)

// ============================================================================
// Kubernetes Configuration Constants
// ============================================================================

const (
	// Kubernetes operation timeouts
	DefaultKubernetesDeployTimeout  = 5 * time.Minute
	DefaultKubernetesDeleteTimeout  = 2 * time.Minute
	DefaultKubernetesRolloutTimeout = 10 * time.Minute

	// Kubernetes defaults
	DefaultKubernetesNamespace = "default"
	DefaultKubernetesReplicas  = 1

	// Kubernetes resource limits
	DefaultPodMemoryLimit   = "512Mi"
	DefaultPodCPULimit      = "500m"
	DefaultPodMemoryRequest = "256Mi"
	DefaultPodCPURequest    = "250m"
)

// ============================================================================
// Security Scanning Constants
// ============================================================================

const (
	// Security scan timeouts
	DefaultScanTimeout = 5 * time.Minute
	DefaultScanRetries = 2

	// Security scan limits
	DefaultMaxVulnerabilities = 1000
	DefaultMaxSecrets         = 100
	DefaultMaxFileSize        = 50 * 1024 * 1024 // 50MB

	// Security scan defaults
	DefaultScanSeverityThreshold = "MEDIUM"
	DefaultScanIncludeFixable    = true
)

// ============================================================================
// Analysis Configuration Constants
// ============================================================================

const (
	// Analysis timeouts
	DefaultMaxAnalysisTime   = 5 * time.Minute
	DefaultRepositoryTimeout = 2 * time.Minute

	// Analysis cache settings
	DefaultCacheTTL     = time.Hour
	DefaultMaxCacheSize = 100000

	// Analysis limits
	DefaultMaxFileAnalysis   = 1000
	DefaultMaxDirectoryDepth = 10
	DefaultMaxRepositorySize = 1024 * 1024 * 1024 // 1GB
)

// ============================================================================
// Observability Configuration Constants
// ============================================================================

const (
	// OpenTelemetry defaults
	DefaultOTELServiceName    = "container-kit-mcp"
	DefaultOTELServiceVersion = "dev"
	DefaultOTELEnvironment    = "development"

	// Metrics collection intervals
	DefaultMetricsInterval     = 30 * time.Second
	DefaultHealthCheckInterval = 30 * time.Second
	DefaultPerformanceInterval = 10 * time.Second
	DefaultGCStatsInterval     = 30 * time.Second

	// Metrics thresholds
	DefaultMaxResponseTime = 30 * time.Second
	DefaultMaxMemoryUsage  = 1024 * 1024 * 1024 // 1GB
	DefaultMaxCPUUsage     = 80.0               // 80%

	// Metrics timeouts
	DefaultMetricsTimeout     = 10 * time.Second
	DefaultHealthCheckTimeout = 5 * time.Second
	DefaultTracingTimeout     = 10 * time.Second
)

// ============================================================================
// Logging Configuration Constants
// ============================================================================

const (
	// Default log settings
	DefaultLogLevel       = "info"
	DefaultLogFormat      = "json"
	DefaultLogOutput      = "stdout"
	DefaultMaxBodyLogSize = 4096

	// Log rotation settings
	DefaultLogMaxSize    = 100 // MB
	DefaultLogMaxAge     = 30  // days
	DefaultLogMaxBackups = 3
	DefaultLogCompress   = true
)

// ============================================================================
// Transport Configuration Constants
// ============================================================================

const (
	// Transport types
	TransportTypeStdio = "stdio"
	TransportTypeHTTP  = "http"
	TransportTypeWS    = "websocket"

	// Default transport settings
	DefaultTransportType = TransportTypeStdio
	DefaultHTTPAddr      = "localhost"
)

// ============================================================================
// Infrastructure Configuration Constants
// ============================================================================

const (
	// Infrastructure constants
	ShutdownGracePeriod   = 30 * time.Second
	DefaultWorkerPoolSize = 10
	DefaultTimeout        = 5 * time.Minute
	MaxGoroutines         = 100
	DefaultCacheSize      = 10000
)

// ============================================================================
// Database Configuration Constants
// ============================================================================

const (
	// Database port defaults
	PostgresPort = 5432
	MySQLPort    = 3306
	RedisPort    = 6379
	MongoDBPort  = 27017
)

// ============================================================================
// Validation Constants
// ============================================================================

const (
	// Validation limits
	MaxNameLength        = 255
	MaxDescriptionLength = 1000
	MaxPasswordLength    = 128
	MaxURLLength         = 2048
	MaxPathLength        = 4096
	MaxTagLength         = 128
	MaxLabelLength       = 63
	MaxAnnotationLength  = 256

	// Validation patterns
	NamePattern      = `^[a-zA-Z0-9][a-zA-Z0-9._-]*$`
	LabelPattern     = `^[a-zA-Z0-9][a-zA-Z0-9._-]*$`
	TagPattern       = `^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`
	NamespacePattern = `^[a-z0-9][a-z0-9-]*$`
	ImageNamePattern = `^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$`

	// Port ranges
	MinPort = 1
	MaxPort = 65535

	// Common port defaults
	HTTPPort       = 80
	HTTPSPort      = 443
	SSHPort        = 22
	PostgreSQLPort = 5432
)

// ============================================================================
// Error Message Constants
// ============================================================================

const (
	// Common error messages
	ErrRequired           = "field is required"
	ErrInvalidFormat      = "field has invalid format"
	ErrTooLong            = "field value is too long"
	ErrTooShort           = "field value is too short"
	ErrOutOfRange         = "field value is out of valid range"
	ErrInvalidChoice      = "field value is not a valid choice"
	ErrConflict           = "field value conflicts with existing configuration"
	ErrNotFound           = "referenced resource not found"
	ErrPermissionDenied   = "insufficient permissions"
	ErrTimeout            = "operation timed out"
	ErrConnectionFailed   = "connection failed"
	ErrInvalidCredentials = "invalid credentials provided"
)

// ============================================================================
// Feature Flags
// ============================================================================

const (
	// Feature flag names
	FeatureSandbox        = "sandbox"
	FeatureHTTPTransport  = "http_transport"
	FeatureMetrics        = "metrics"
	FeatureProfiling      = "profiling"
	FeatureHealthChecks   = "health_checks"
	FeatureRateLimit      = "rate_limit"
	FeatureCORS           = "cors"
	FeatureAuthentication = "authentication"
	FeatureAuthorization  = "authorization"
)

// ============================================================================
// Environment Variable Names
// ============================================================================

const (
	// Server environment variables
	EnvWorkspaceDir  = "MCP_WORKSPACE_DIR"
	EnvStorePath     = "MCP_STORE_PATH"
	EnvTransportType = "MCP_TRANSPORT_TYPE"
	EnvHTTPAddr      = "MCP_HTTP_ADDR"
	EnvHTTPPort      = "MCP_HTTP_PORT"
	EnvLogLevel      = "MCP_LOG_LEVEL"
	EnvEnvironment   = "MCP_ENVIRONMENT"

	// OpenTelemetry environment variables
	EnvOTELEnabled        = "OTEL_ENABLED"
	EnvOTELEndpoint       = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvOTELServiceName    = "OTEL_SERVICE_NAME"
	EnvOTELServiceVersion = "OTEL_SERVICE_VERSION"
	EnvOTELEnvironment    = "OTEL_ENVIRONMENT"

	// Feature flags environment variables
	EnvFeaturePrefix = "MCP_FEATURE_"
)
