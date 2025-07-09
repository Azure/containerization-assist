package constants

import "time"

// Retry configuration constants (non-conflicting with existing limits.go)
const (
	// BaseRetryDelay is the initial delay between retries
	BaseRetryDelay = 1 * time.Second

	// NetworkRetryDelay for network errors that need more time
	NetworkRetryDelay = 2 * time.Second

	// ResourceRetryDelay for resource contention that needs longer wait
	ResourceRetryDelay = 5 * time.Second

	// TimeoutRetryDelay for timeout errors
	TimeoutRetryDelay = 3 * time.Second

	// KubernetesRetryDelay for K8s API calls that need reasonable wait
	KubernetesRetryDelay = 2 * time.Second

	// ContainerRetryDelay for container operations that can retry quickly
	ContainerRetryDelay = 1 * time.Second

	// MaxRetryDelay is the maximum delay between retries
	MaxRetryDelay = 30 * time.Second

	// RetryBackoffMultiplier for exponential backoff
	RetryBackoffMultiplier = 2.0
)

// Retry attempt limits by severity
const (
	// CriticalMaxRetries for critical errors (only one retry)
	CriticalMaxRetries = 1

	// HighMaxRetries for high severity errors
	HighMaxRetries = 2

	// MediumMaxRetries for medium severity errors
	MediumMaxRetries = 3

	// LowMaxRetries for low severity errors
	LowMaxRetries = 5
)
