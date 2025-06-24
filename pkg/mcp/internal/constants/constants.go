package constants

import "time"

// Version constants
const (
	// AtomicToolVersion is the version for all atomic tools
	AtomicToolVersion = "1.0.0"
)

// Timeout constants
const (
	// DefaultHealthCheckTimeout is the default timeout for health checks
	DefaultHealthCheckTimeout = 30 * time.Second
	
	// DefaultWaitTimeout is the default timeout for deployment and operation waits
	DefaultWaitTimeout = 5 * time.Minute
	
	// LongOperationTimeout is the timeout for long-running operations
	LongOperationTimeout = 15 * time.Minute
	
	// CacheValidityPeriod is the default cache validity period
	CacheValidityPeriod = time.Hour
)

// Docker constants
const (
	// DefaultDockerRegistry is the default Docker registry
	DefaultDockerRegistry = "docker.io"
	
	// DefaultImageTag is the default Docker image tag
	DefaultImageTag = "latest"
	
	// DefaultPlatform is the default Docker platform
	DefaultPlatform = "linux/amd64"
)

// Kubernetes constants
const (
	// DefaultNamespace is the default Kubernetes namespace
	DefaultNamespace = "default"
	
	// DefaultApplicationPort is the default application port for manifests
	DefaultApplicationPort = 8080
)

// Security constants
const (
	// DefaultSeverityThreshold is the default minimum severity for security scans
	DefaultSeverityThreshold = "HIGH,CRITICAL"
)

// Registry detection patterns
var (
	// KnownRegistries contains patterns for identifying container registries
	KnownRegistries = []string{
		"azurecr.io",
		"gcr.io", 
		"amazonaws.com",
		"quay.io",
		"mcr.microsoft.com",
		"public.ecr.aws",
	}
	
	// DefaultSecretScanExclusions contains default file patterns to exclude from secret scanning
	DefaultSecretScanExclusions = []string{
		"*.git/*",
		"node_modules/*", 
		"vendor/*",
		"*.log",
		"*.tmp",
		"*.cache",
	}
)