package registry

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

const (
	// DefaultRegistryTimeout is the default timeout for registry connectivity tests
	DefaultRegistryTimeout = 15 * time.Second
)

// CommandExecutor interface abstracts command execution for better testability
type CommandExecutor interface {
	// ExecuteCommand runs a command with the given context and returns output and error
	ExecuteCommand(ctx context.Context, name string, args ...string) ([]byte, error)
	// CommandExists checks if a command exists in PATH
	CommandExists(name string) bool
}

// DefaultCommandExecutor implements CommandExecutor using os/exec
type DefaultCommandExecutor struct{}

// ExecuteCommand runs a command using os/exec
func (d *DefaultCommandExecutor) ExecuteCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// CommandExists checks if a command exists in PATH
func (d *DefaultCommandExecutor) CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// MultiRegistryManager coordinates authentication across multiple registries
type MultiRegistryManager struct {
	config          *MultiRegistryConfig
	providers       []CredentialProvider
	credentialCache map[string]*CachedCredentials
	cacheMutex      sync.RWMutex
	logger          zerolog.Logger
	cmdExecutor     CommandExecutor
}

// MultiRegistryConfig defines configuration for multiple registries
type MultiRegistryConfig struct {
	Registries      map[string]RegistryConfig `json:"registries"`
	DefaultRegistry string                    `json:"default_registry,omitempty"`
	Fallbacks       []string                  `json:"fallbacks,omitempty"`
	CacheTimeout    time.Duration             `json:"cache_timeout,omitempty"`
	MaxRetries      int                       `json:"max_retries,omitempty"`
}

// RegistryConfig contains configuration for a single registry
type RegistryConfig struct {
	URL              string            `json:"url"`
	AuthMethod       string            `json:"auth_method"` // "basic", "oauth", "helper", "keychain"
	Username         string            `json:"username,omitempty"`
	Password         string            `json:"password,omitempty"`
	Token            string            `json:"token,omitempty"`
	CredentialHelper string            `json:"credential_helper,omitempty"`
	Insecure         bool              `json:"insecure,omitempty"`
	Timeout          time.Duration     `json:"timeout,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	FallbackMethods  []string          `json:"fallback_methods,omitempty"`
	RateLimitAware   bool              `json:"rate_limit_aware,omitempty"`
}

// CredentialProvider interface for different authentication methods
type CredentialProvider interface {
	GetCredentials(registry string) (*RegistryCredentials, error)
	IsAvailable() bool
	GetName() string
	GetPriority() int
	Supports(registry string) bool
}

// RegistryCredentials contains authentication credentials
type RegistryCredentials struct {
	Username   string
	Password   string
	Token      string
	ExpiresAt  *time.Time
	Registry   string
	AuthMethod string
	Source     string // Which provider returned these credentials
}

// CachedCredentials wraps credentials with cache metadata
type CachedCredentials struct {
	Credentials *RegistryCredentials
	CachedAt    time.Time
	ExpiresAt   time.Time
}

// NewMultiRegistryManager creates a new multi-registry manager
func NewMultiRegistryManager(config *MultiRegistryConfig, logger zerolog.Logger) *MultiRegistryManager {
	if config.CacheTimeout == 0 {
		config.CacheTimeout = 15 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &MultiRegistryManager{
		config:          config,
		providers:       make([]CredentialProvider, 0),
		credentialCache: make(map[string]*CachedCredentials),
		logger:          logger.With().Str("component", "multi_registry_manager").Logger(),
		cmdExecutor:     &DefaultCommandExecutor{},
	}
}

// SetCommandExecutor sets a custom command executor (primarily for testing)
func (mrm *MultiRegistryManager) SetCommandExecutor(executor CommandExecutor) {
	mrm.cmdExecutor = executor
}

// RegisterProvider adds a credential provider to the manager
func (mrm *MultiRegistryManager) RegisterProvider(provider CredentialProvider) {
	mrm.providers = append(mrm.providers, provider)

	// Sort providers by priority (higher priority first)
	for i := len(mrm.providers) - 1; i > 0; i-- {
		if mrm.providers[i].GetPriority() > mrm.providers[i-1].GetPriority() {
			mrm.providers[i], mrm.providers[i-1] = mrm.providers[i-1], mrm.providers[i]
		}
	}

	mrm.logger.Info().
		Str("provider", provider.GetName()).
		Int("priority", provider.GetPriority()).
		Bool("available", provider.IsAvailable()).
		Msg("Registered credential provider")
}

// GetCredentials retrieves credentials for a specific registry
func (mrm *MultiRegistryManager) GetCredentials(ctx context.Context, registry string) (*RegistryCredentials, error) {
	// Normalize registry URL
	normalizedRegistry := mrm.normalizeRegistry(registry)

	// Check cache first
	if cached := mrm.getCachedCredentials(normalizedRegistry); cached != nil {
		mrm.logger.Debug().
			Str("registry", normalizedRegistry).
			Str("source", "cache").
			Msg("Using cached credentials")
		return cached, nil
	}

	// Try to get credentials from providers
	creds, err := mrm.getCredentialsFromProviders(ctx, normalizedRegistry)
	if err != nil {
		// Try fallback registries if configured
		if fallbackCreds := mrm.tryFallbackRegistries(ctx, normalizedRegistry); fallbackCreds != nil {
			return fallbackCreds, nil
		}
		return nil, types.NewErrorBuilder("registry_auth_failed", "Failed to get credentials for registry", "authentication").
			WithField("registry", normalizedRegistry).
			WithOperation("get_credentials").
			WithStage("credential_retrieval").
			WithRootCause(fmt.Sprintf("All credential providers failed for registry %s: %v", normalizedRegistry, err)).
			WithImmediateStep(1, "Check config", "Verify registry configuration and credentials are valid").
			WithImmediateStep(2, "Test auth", "Use docker login to test registry authentication manually").
			WithImmediateStep(3, "Update providers", "Ensure credential providers are properly configured").
			Build()
	}

	// Cache the credentials
	mrm.cacheCredentials(normalizedRegistry, creds)

	return creds, nil
}

// DetectRegistry automatically detects the registry from an image reference
func (mrm *MultiRegistryManager) DetectRegistry(imageRef string) string {
	// Handle docker.io special case
	if !strings.Contains(imageRef, "/") || (!strings.Contains(imageRef, ".") && !strings.Contains(imageRef, ":")) {
		return "docker.io"
	}

	parts := strings.Split(imageRef, "/")
	if len(parts) > 0 {
		firstPart := parts[0]
		// If first part contains a dot or colon, it's likely a registry
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") {
			return firstPart
		}
	}

	// Default to docker.io for simple image names
	return "docker.io"
}

// ValidateRegistryAccess tests connectivity and authentication with a registry
func (mrm *MultiRegistryManager) ValidateRegistryAccess(ctx context.Context, registry string) error {
	normalizedRegistry := mrm.normalizeRegistry(registry)

	mrm.logger.Info().
		Str("registry", normalizedRegistry).
		Msg("Validating registry access")

	// Get credentials
	creds, err := mrm.GetCredentials(ctx, normalizedRegistry)
	if err != nil {
		return types.NewErrorBuilder("registry_validation_failed", "Registry validation failed - cannot get credentials", "authentication").
			WithField("registry", normalizedRegistry).
			WithOperation("validate_registry_access").
			WithStage("credential_validation").
			WithRootCause(fmt.Sprintf("Unable to retrieve credentials for registry validation: %v", err)).
			WithImmediateStep(1, "Fix auth", "Resolve credential issues before attempting validation").
			WithImmediateStep(2, "Check providers", "Verify credential providers are available and configured").
			Build()
	}

	// Implement actual registry connectivity test
	if err := mrm.testRegistryConnectivity(ctx, normalizedRegistry, creds); err != nil {
		return types.NewErrorBuilder("registry_connectivity_failed", "Registry connectivity test failed", "network").
			WithField("registry", normalizedRegistry).
			WithOperation("validate_registry_access").
			WithStage("connectivity_test").
			WithRootCause(fmt.Sprintf("Unable to connect to registry %s: %v", normalizedRegistry, err)).
			WithImmediateStep(1, "Check network", "Verify network connectivity to registry").
			WithImmediateStep(2, "Test DNS", "Confirm registry hostname resolves correctly").
			WithImmediateStep(3, "Check firewall", "Ensure no firewall rules block registry access").
			Build()
	}

	if creds == nil {
		return types.NewErrorBuilder("no_credentials", "No credentials available for registry", "authentication").
			WithField("registry", normalizedRegistry).
			WithOperation("validate_registry_access").
			WithStage("credential_check").
			WithRootCause(fmt.Sprintf("No valid credentials found for registry %s", normalizedRegistry)).
			WithImmediateStep(1, "Configure auth", "Set up authentication for this registry").
			WithImmediateStep(2, "Use docker login", "Run 'docker login' to authenticate with the registry").
			WithImmediateStep(3, "Check config", "Verify registry is properly configured in settings").
			Build()
	}

	mrm.logger.Info().
		Str("registry", normalizedRegistry).
		Str("auth_method", creds.AuthMethod).
		Str("source", creds.Source).
		Msg("Registry access validated")

	return nil
}

// GetRegistryConfig returns the configuration for a specific registry
func (mrm *MultiRegistryManager) GetRegistryConfig(registry string) (*RegistryConfig, bool) {
	normalizedRegistry := mrm.normalizeRegistry(registry)

	// Check for exact match
	if config, exists := mrm.config.Registries[normalizedRegistry]; exists {
		return &config, true
	}

	// Check for wildcard matches (e.g., "*.dkr.ecr.*.amazonaws.com")
	for pattern, config := range mrm.config.Registries {
		if mrm.matchesPattern(normalizedRegistry, pattern) {
			configCopy := config
			return &configCopy, true
		}
	}

	return nil, false
}

// ClearCache clears the credential cache
func (mrm *MultiRegistryManager) ClearCache() {
	mrm.cacheMutex.Lock()
	defer mrm.cacheMutex.Unlock()

	mrm.credentialCache = make(map[string]*CachedCredentials)
	mrm.logger.Info().Msg("Credential cache cleared")
}

// GetCacheStats returns statistics about the credential cache
func (mrm *MultiRegistryManager) GetCacheStats() map[string]interface{} {
	mrm.cacheMutex.RLock()
	defer mrm.cacheMutex.RUnlock()

	stats := map[string]interface{}{
		"total_entries": len(mrm.credentialCache),
		"entries":       make([]map[string]interface{}, 0, len(mrm.credentialCache)),
	}

	for registry, cached := range mrm.credentialCache {
		entry := map[string]interface{}{
			"registry":    registry,
			"cached_at":   cached.CachedAt,
			"expires_at":  cached.ExpiresAt,
			"auth_method": cached.Credentials.AuthMethod,
			"source":      cached.Credentials.Source,
		}
		stats["entries"] = append(stats["entries"].([]map[string]interface{}), entry)
	}

	return stats
}

// Private helper methods

func (mrm *MultiRegistryManager) normalizeRegistry(registry string) string {
	// Remove protocol if present
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")

	// Handle docker.io special case
	if registry == "docker.io" || registry == "index.docker.io" {
		return "https://index.docker.io/v1/"
	}

	return registry
}

func (mrm *MultiRegistryManager) getCachedCredentials(registry string) *RegistryCredentials {
	mrm.cacheMutex.RLock()
	defer mrm.cacheMutex.RUnlock()

	cached, exists := mrm.credentialCache[registry]
	if !exists {
		return nil
	}

	// Check if cache has expired
	if time.Now().After(cached.ExpiresAt) {
		// Remove expired entry
		delete(mrm.credentialCache, registry)
		return nil
	}

	// Check credential-specific expiration
	if cached.Credentials.ExpiresAt != nil && time.Now().After(*cached.Credentials.ExpiresAt) {
		delete(mrm.credentialCache, registry)
		return nil
	}

	return cached.Credentials
}

func (mrm *MultiRegistryManager) cacheCredentials(registry string, creds *RegistryCredentials) {
	mrm.cacheMutex.Lock()
	defer mrm.cacheMutex.Unlock()

	expiresAt := time.Now().Add(mrm.config.CacheTimeout)

	// Use credential expiration if it's sooner
	if creds.ExpiresAt != nil && creds.ExpiresAt.Before(expiresAt) {
		expiresAt = *creds.ExpiresAt
	}

	mrm.credentialCache[registry] = &CachedCredentials{
		Credentials: creds,
		CachedAt:    time.Now(),
		ExpiresAt:   expiresAt,
	}

	mrm.logger.Debug().
		Str("registry", registry).
		Time("expires_at", expiresAt).
		Msg("Credentials cached")
}

func (mrm *MultiRegistryManager) getCredentialsFromProviders(ctx context.Context, registry string) (*RegistryCredentials, error) {
	var lastErr error

	for _, provider := range mrm.providers {
		if !provider.IsAvailable() || !provider.Supports(registry) {
			continue
		}

		mrm.logger.Debug().
			Str("registry", registry).
			Str("provider", provider.GetName()).
			Msg("Trying credential provider")

		creds, err := provider.GetCredentials(registry)
		if err != nil {
			mrm.logger.Debug().
				Str("registry", registry).
				Str("provider", provider.GetName()).
				Err(err).
				Msg("Provider failed to get credentials")
			lastErr = err
			continue
		}

		if creds != nil {
			creds.Source = provider.GetName()
			mrm.logger.Info().
				Str("registry", registry).
				Str("provider", provider.GetName()).
				Str("auth_method", creds.AuthMethod).
				Msg("Successfully obtained credentials")
			return creds, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, types.NewErrorBuilder("no_provider_auth", "No credential provider could authenticate to registry", "authentication").
		WithField("registry", registry).
		WithField("providers_tried", len(mrm.providers)).
		WithOperation("get_credentials_from_providers").
		WithStage("provider_authentication").
		WithRootCause(fmt.Sprintf("All %d credential providers failed to authenticate to registry %s", len(mrm.providers), registry)).
		WithImmediateStep(1, "Check providers", "Verify credential providers are properly configured and available").
		WithImmediateStep(2, "Test auth", "Test authentication manually with each provider").
		WithImmediateStep(3, "Add provider", "Consider adding additional credential providers").
		Build()
}

func (mrm *MultiRegistryManager) tryFallbackRegistries(ctx context.Context, registry string) *RegistryCredentials {
	// Check if this registry has configured fallbacks
	if config, exists := mrm.GetRegistryConfig(registry); exists && len(config.FallbackMethods) > 0 {
		for _, fallback := range config.FallbackMethods {
			mrm.logger.Debug().
				Str("registry", registry).
				Str("fallback", fallback).
				Msg("Trying fallback authentication method")

			// Try fallback - this is a simplified implementation
			// In a real implementation, you'd try different auth methods
		}
	}

	// Try global fallback registries
	for _, fallbackRegistry := range mrm.config.Fallbacks {
		mrm.logger.Debug().
			Str("original_registry", registry).
			Str("fallback_registry", fallbackRegistry).
			Msg("Trying fallback registry")

		if creds, err := mrm.getCredentialsFromProviders(ctx, fallbackRegistry); err == nil && creds != nil {
			// Modify credentials to point to original registry
			creds.Registry = registry
			return creds
		}
	}

	return nil
}

func (mrm *MultiRegistryManager) matchesPattern(registry, pattern string) bool {
	// Simple wildcard matching - could be enhanced with proper regex
	if !strings.Contains(pattern, "*") {
		return registry == pattern
	}

	// This is a simplified implementation - in production, use proper regex
	return strings.Contains(registry, strings.ReplaceAll(pattern, "*", ""))
}

// testRegistryConnectivity tests connectivity to a registry using Docker API
func (mrm *MultiRegistryManager) testRegistryConnectivity(ctx context.Context, registry string, _ *RegistryCredentials) error {
	// Get timeout from config or use default
	timeout := DefaultRegistryTimeout
	if config, exists := mrm.GetRegistryConfig(registry); exists && config.Timeout > 0 {
		timeout = config.Timeout
	}

	// Create context with timeout for the connectivity test
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	mrm.logger.Debug().
		Str("registry", registry).
		Dur("timeout", timeout).
		Msg("Testing registry connectivity")

	// Check if docker command is available
	if err := mrm.checkDockerAvailability(ctx); err != nil {
		return types.NewErrorBuilder("docker_unavailable", "Docker command not available for registry connectivity test", "system").
			WithOperation("test_registry_connectivity").
			WithStage("docker_check").
			WithRootCause(fmt.Sprintf("Docker CLI is required for registry connectivity tests: %v", err)).
			WithImmediateStep(1, "Install Docker", "Install Docker CLI tools").
			WithImmediateStep(2, "Check PATH", "Ensure Docker is in system PATH").
			WithImmediateStep(3, "Start daemon", "Start Docker daemon if not running").
			Build()
	}

	// Get appropriate test images for the registry
	testImages := mrm.getTestImagesForRegistry(registry)

	var lastErr error
	// Try to connect to the registry using docker manifest inspect
	for _, testImage := range testImages {
		_, err := mrm.cmdExecutor.ExecuteCommand(ctx, "docker", "manifest", "inspect", testImage)
		if err == nil {
			mrm.logger.Info().
				Str("registry", registry).
				Str("test_image", testImage).
				Dur("timeout", timeout).
				Msg("Registry connectivity test passed")
			return nil
		}
		lastErr = err
		mrm.logger.Debug().
			Str("registry", registry).
			Str("test_image", testImage).
			Err(err).
			Msg("Test image failed, trying next")

		// Check for timeout or network-specific errors
		if ctx.Err() == context.DeadlineExceeded {
			return types.NewErrorBuilder("registry_timeout", "Registry connectivity test timed out", "network").
				WithField("registry", registry).
				WithField("timeout", timeout.String()).
				WithOperation("test_registry_connectivity").
				WithStage("connectivity_test").
				WithRootCause(fmt.Sprintf("Registry %s did not respond within %v timeout", registry, timeout)).
				WithImmediateStep(1, "Check network", "Verify network connectivity and latency to registry").
				WithImmediateStep(2, "Increase timeout", "Consider increasing timeout for slow networks").
				WithImmediateStep(3, "Test manually", "Test registry access manually with docker commands").
				Build()
		}
	}

	// Classify the error for better reporting
	if lastErr != nil {
		errStr := lastErr.Error()
		switch {
		case strings.Contains(errStr, "no such host") || strings.Contains(errStr, "name resolution"):
			return types.NewErrorBuilder("registry_dns_failed", "Registry DNS resolution failed", "network").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("dns_resolution").
				WithRootCause(fmt.Sprintf("Cannot resolve hostname for registry %s: %v", registry, lastErr)).
				WithImmediateStep(1, "Check DNS", "Verify DNS settings and connectivity").
				WithImmediateStep(2, "Use IP", "Try using IP address instead of hostname").
				WithImmediateStep(3, "Check hosts", "Verify /etc/hosts file for hostname entries").
				Build()
		case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset"):
			return types.NewErrorBuilder("registry_connection_refused", "Registry connection refused", "network").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("connection_attempt").
				WithRootCause(fmt.Sprintf("Connection to registry %s was refused: %v", registry, lastErr)).
				WithImmediateStep(1, "Check service", "Verify registry service is running and accessible").
				WithImmediateStep(2, "Check port", "Confirm correct port number for registry").
				WithImmediateStep(3, "Check firewall", "Verify firewall rules allow registry connections").
				Build()
		case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
			return types.NewErrorBuilder("registry_connection_timeout", "Registry connection timeout", "network").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("connection_attempt").
				WithRootCause(fmt.Sprintf("Connection to registry %s timed out: %v", registry, lastErr)).
				WithImmediateStep(1, "Check latency", "Test network latency to registry").
				WithImmediateStep(2, "Increase timeout", "Configure longer timeout for slow connections").
				WithImmediateStep(3, "Check proxy", "Verify proxy settings if using corporate network").
				Build()
		case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "authentication"):
			return types.NewErrorBuilder("registry_auth_failed", "Registry authentication failed", "authentication").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("authentication").
				WithRootCause(fmt.Sprintf("Authentication failed for registry %s: %v", registry, lastErr)).
				WithImmediateStep(1, "Check credentials", "Verify username/password or token credentials").
				WithImmediateStep(2, "Refresh token", "Refresh authentication token if expired").
				WithImmediateStep(3, "Test login", "Test login with 'docker login' command").
				Build()
		case strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "access denied"):
			return types.NewErrorBuilder("registry_access_denied", "Registry access denied", "authorization").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("authorization").
				WithRootCause(fmt.Sprintf("Access denied to registry %s: %v", registry, lastErr)).
				WithImmediateStep(1, "Check permissions", "Verify account has permission to access registry").
				WithImmediateStep(2, "Request access", "Request registry access from administrator").
				WithImmediateStep(3, "Check subscription", "Verify registry subscription is active").
				Build()
		default:
			return types.NewErrorBuilder("registry_test_failed", "Registry connectivity test failed", "network").
				WithField("registry", registry).
				WithOperation("test_registry_connectivity").
				WithStage("connectivity_test").
				WithRootCause(fmt.Sprintf("Connectivity test failed for registry %s: %v", registry, lastErr)).
				WithImmediateStep(1, "Check error", "Review specific error details for resolution").
				WithImmediateStep(2, "Test manually", "Test registry access manually with docker CLI").
				WithImmediateStep(3, "Check config", "Verify registry configuration is correct").
				Build()
		}
	}

	// If all test images failed without a specific error, return generic error
	return types.NewErrorBuilder("no_test_images", "Failed to connect to registry - no test images accessible", "registry").
		WithField("registry", registry).
		WithOperation("test_registry_connectivity").
		WithStage("image_access_test").
		WithRootCause(fmt.Sprintf("Registry %s is reachable but no test images are accessible", registry)).
		WithImmediateStep(1, "Check images", "Verify test images exist in the registry").
		WithImmediateStep(2, "Check permissions", "Ensure account has pull permissions for test images").
		WithImmediateStep(3, "Use custom image", "Configure custom test image for this registry").
		Build()
}

// getTestImagesForRegistry returns appropriate test images for different registries
func (mrm *MultiRegistryManager) getTestImagesForRegistry(registry string) []string {
	switch {
	case strings.Contains(registry, "docker.io") || strings.Contains(registry, "index.docker.io"):
		return []string{"docker.io/library/hello-world:latest", "hello-world:latest"}
	case strings.Contains(registry, "ghcr.io"):
		return []string{"ghcr.io/containerbase/base:latest"}
	case strings.Contains(registry, "quay.io"):
		return []string{"quay.io/prometheus/busybox:latest"}
	case strings.Contains(registry, "gcr.io"):
		return []string{"gcr.io/google-containers/pause:latest"}
	case strings.Contains(registry, "mcr.microsoft.com"):
		return []string{"mcr.microsoft.com/hello-world:latest"}
	case strings.Contains(registry, "amazonaws.com"):
		// For AWS ECR, try common base images
		return []string{
			fmt.Sprintf("%s/amazonlinux:latest", registry),
			fmt.Sprintf("%s/alpine:latest", registry),
		}
	case strings.Contains(registry, "azurecr.io"):
		// For Azure Container Registry, try common base images
		return []string{
			fmt.Sprintf("%s/hello-world:latest", registry),
			fmt.Sprintf("%s/alpine:latest", registry),
		}
	default:
		// For unknown registries, try generic approaches
		return []string{
			fmt.Sprintf("%s/hello-world:latest", registry),
			fmt.Sprintf("%s/library/hello-world:latest", registry),
			fmt.Sprintf("%s/alpine:latest", registry),
		}
	}
}

// checkDockerAvailability verifies that the docker command is available and accessible
func (mrm *MultiRegistryManager) checkDockerAvailability(ctx context.Context) error {
	// First check if docker command exists in PATH
	if !mrm.cmdExecutor.CommandExists("docker") {
		return types.NewErrorBuilder("docker_not_found", "Docker command not found in PATH", "system").
			WithOperation("check_docker_availability").
			WithStage("command_check").
			WithRootCause("Docker CLI is not installed or not in system PATH").
			WithImmediateStep(1, "Install Docker", "Install Docker CLI from docker.com").
			WithImmediateStep(2, "Update PATH", "Add Docker installation directory to system PATH").
			WithImmediateStep(3, "Restart shell", "Restart terminal/shell to reload PATH changes").
			Build()
	}

	// Check if docker command is accessible
	output, err := mrm.cmdExecutor.ExecuteCommand(ctx, "docker", "--version")
	if err != nil {
		return types.NewErrorBuilder("docker_not_accessible", "Docker command exists but not accessible", "system").
			WithOperation("check_docker_availability").
			WithStage("command_access").
			WithRootCause(fmt.Sprintf("Docker CLI found but cannot be executed: %v", err)).
			WithImmediateStep(1, "Check permissions", "Verify Docker command has execute permissions").
			WithImmediateStep(2, "Add to group", "Add current user to docker group if needed").
			WithImmediateStep(3, "Run as admin", "Try running with elevated privileges").
			Build()
	}

	// Log docker version for debugging
	version := strings.TrimSpace(string(output))
	mrm.logger.Debug().Str("docker_version", version).Msg("Docker command availability verified")

	// Optionally check if docker daemon is running (quick check)
	_, err = mrm.cmdExecutor.ExecuteCommand(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		mrm.logger.Warn().Err(err).Msg("Docker daemon may not be running - registry connectivity tests may fail")
		// Don't fail here as docker manifest inspect might still work in some scenarios
	}

	return nil
}
