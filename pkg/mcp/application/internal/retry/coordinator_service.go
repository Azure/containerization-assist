package retry

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// CoordinatorService manages retry coordination without global state
type CoordinatorService struct {
	mu          sync.RWMutex
	coordinator *Coordinator
	initOnce    sync.Once
}

// NewCoordinatorService creates a new coordinator service
func NewCoordinatorService() *CoordinatorService {
	return &CoordinatorService{}
}

// Initialize initializes the coordinator service with standard policies
func (c *CoordinatorService) Initialize() {
	c.initOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.coordinator = New()

		// Register standard fix providers
		c.coordinator.RegisterFixProvider("docker", NewDockerFixProvider())
		c.coordinator.RegisterFixProvider("config", NewConfigFixProvider())
		c.coordinator.RegisterFixProvider("dependency", NewDependencyFixProvider())

		// Configure operation-specific policies
		c.setupStandardPolicies()
	})
}

// GetCoordinator returns the coordinator instance, initializing if needed
func (c *CoordinatorService) GetCoordinator() *Coordinator {
	c.Initialize()
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.coordinator
}

// setupStandardPolicies configures retry policies for different operation types
func (c *CoordinatorService) setupStandardPolicies() {
	coordinator := c.coordinator

	// Network operations - aggressive retry with exponential backoff
	coordinator.SetPolicy("network", &Policy{
		MaxAttempts:     5,
		InitialDelay:    time.Second,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: BackoffExponential,
		Multiplier:      2.0,
		Jitter:          true,
		ErrorPatterns: []string{
			"timeout", "deadline exceeded", "connection refused",
			"connection reset", "network unreachable", "dial tcp",
			"i/o timeout", "temporary failure", "service unavailable",
		},
	})

	// Docker operations - moderate retry with linear backoff
	coordinator.SetPolicy("docker", &Policy{
		MaxAttempts:     3,
		InitialDelay:    2 * time.Second,
		MaxDelay:        15 * time.Second,
		BackoffStrategy: BackoffLinear,
		Multiplier:      1.5,
		Jitter:          true,
		ErrorPatterns: []string{
			"docker daemon", "image not found", "build failed",
			"push failed", "pull failed", "container", "docker engine",
		},
	})

	// Kubernetes operations - moderate retry with exponential backoff
	coordinator.SetPolicy("kubernetes", &Policy{
		MaxAttempts:     4,
		InitialDelay:    time.Second,
		MaxDelay:        20 * time.Second,
		BackoffStrategy: BackoffExponential,
		Multiplier:      2.0,
		Jitter:          true,
		ErrorPatterns: []string{
			"kubectl", "kubernetes", "k8s", "pod", "deployment",
			"service account", "cluster", "node", "namespace",
			"api server", "connection refused",
		},
	})

	// Git operations - limited retry with fixed backoff
	coordinator.SetPolicy("git", &Policy{
		MaxAttempts:     3,
		InitialDelay:    2 * time.Second,
		MaxDelay:        10 * time.Second,
		BackoffStrategy: BackoffFixed,
		Multiplier:      1.0,
		Jitter:          false,
		ErrorPatterns: []string{
			"git", "repository", "remote", "clone failed",
			"authentication failed", "connection", "timeout",
		},
	})

	// AI/LLM operations - conservative retry with exponential backoff
	coordinator.SetPolicy("ai", &Policy{
		MaxAttempts:     3,
		InitialDelay:    5 * time.Second,
		MaxDelay:        60 * time.Second,
		BackoffStrategy: BackoffExponential,
		Multiplier:      3.0,
		Jitter:          true,
		ErrorPatterns: []string{
			"rate limited", "quota exceeded", "model not available",
			"api key", "authentication", "token", "openai", "azure openai",
			"too many requests", "503", "502",
		},
	})

	// Build operations - comprehensive retry with linear backoff
	coordinator.SetPolicy("build", &Policy{
		MaxAttempts:     4,
		InitialDelay:    3 * time.Second,
		MaxDelay:        25 * time.Second,
		BackoffStrategy: BackoffLinear,
		Multiplier:      1.5,
		Jitter:          true,
		ErrorPatterns: []string{
			"build failed", "compilation error", "dependency",
			"package not found", "download failed", "temporary",
			"network", "timeout", "resource",
		},
	})

	// Deployment operations - balanced retry with exponential backoff
	coordinator.SetPolicy("deployment", &Policy{
		MaxAttempts:     3,
		InitialDelay:    5 * time.Second,
		MaxDelay:        30 * time.Second,
		BackoffStrategy: BackoffExponential,
		Multiplier:      2.0,
		Jitter:          true,
		ErrorPatterns: []string{
			"deployment failed", "rollout", "timeout", "readiness",
			"liveness", "probe", "health check", "service",
			"ingress", "load balancer",
		},
	})

	// File operations - quick retry with fixed backoff
	coordinator.SetPolicy("file", &Policy{
		MaxAttempts:     2,
		InitialDelay:    500 * time.Millisecond,
		MaxDelay:        2 * time.Second,
		BackoffStrategy: BackoffFixed,
		Multiplier:      1.0,
		Jitter:          false,
		ErrorPatterns: []string{
			"permission denied", "file not found", "directory",
			"resource temporarily unavailable", "no space left",
		},
	})
}

// WithPolicy is a convenience method to retry operations with a specific policy
func (c *CoordinatorService) WithPolicy(ctx context.Context, operationType string, fn func(ctx context.Context) error) error {
	return c.GetCoordinator().Execute(ctx, operationType, fn)
}

// WithFix is a convenience method to retry operations with automatic fixing
func (c *CoordinatorService) WithFix(ctx context.Context, operationType string, fn func(ctx context.Context, retryCtx *Context) error) error {
	return c.GetCoordinator().ExecuteWithFix(ctx, operationType, fn)
}

// NetworkOperation retries network operations with appropriate backoff
func (c *CoordinatorService) NetworkOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "network", fn)
}

// DockerOperation retries Docker operations with fixing capabilities
func (c *CoordinatorService) DockerOperation(ctx context.Context, dockerfilePath string, fn func(ctx context.Context, retryCtx *Context) error) error {
	return c.WithFix(ctx, "docker", func(ctx context.Context, retryCtx *Context) error {
		// Set dockerfile path in context for potential fixes
		retryCtx.Context["dockerfile_path"] = dockerfilePath
		return fn(ctx, retryCtx)
	})
}

// KubernetesOperation retries Kubernetes operations
func (c *CoordinatorService) KubernetesOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "kubernetes", fn)
}

// GitOperation retries Git operations
func (c *CoordinatorService) GitOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "git", fn)
}

// AIOperation retries AI/LLM operations with conservative backoff
func (c *CoordinatorService) AIOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "ai", fn)
}

// BuildOperation retries build operations with fixing capabilities
func (c *CoordinatorService) BuildOperation(ctx context.Context, buildContext map[string]interface{}, fn func(ctx context.Context, retryCtx *Context) error) error {
	return c.WithFix(ctx, "build", func(ctx context.Context, retryCtx *Context) error {
		// Merge build context into retry context
		for k, v := range buildContext {
			retryCtx.Context[k] = v
		}
		return fn(ctx, retryCtx)
	})
}

// DeploymentOperation retries deployment operations
func (c *CoordinatorService) DeploymentOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "deployment", fn)
}

// FileOperation retries file operations
func (c *CoordinatorService) FileOperation(ctx context.Context, fn func(ctx context.Context) error) error {
	return c.WithPolicy(ctx, "file", fn)
}

// IsRetryableError checks if an error should be retried
func (c *CoordinatorService) IsRetryableError(err error) bool {
	coordinator := c.GetCoordinator()
	return coordinator.errorClassifier.IsRetryable(err)
}

// ClassifyError classifies an error
func (c *CoordinatorService) ClassifyError(err error) string {
	coordinator := c.GetCoordinator()
	return coordinator.errorClassifier.ClassifyError(err)
}

// CreateRetryableError creates an error that will be retried by the coordinator
func (c *CoordinatorService) CreateRetryableError(module, message string) error {
	return &errors.MCPError{
		Category:    errors.CategoryNetwork,
		Module:      module,
		Message:     message,
		Retryable:   true,
		Recoverable: true,
	}
}

// CreateNonRetryableError creates an error that will not be retried
func (c *CoordinatorService) CreateNonRetryableError(module, message string) error {
	return &errors.MCPError{
		Category:    errors.CategoryValidation,
		Module:      module,
		Message:     message,
		Retryable:   false,
		Recoverable: false,
	}
}

// Reset resets the coordinator service (useful for testing)
func (c *CoordinatorService) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.coordinator = nil
	c.initOnce = sync.Once{}
}
