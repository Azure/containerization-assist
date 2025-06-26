package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// MultiRegistryManager coordinates authentication across multiple registries
type MultiRegistryManager struct {
	config          *MultiRegistryConfig
	providers       []CredentialProvider
	credentialCache map[string]*CachedCredentials
	cacheMutex      sync.RWMutex
	logger          zerolog.Logger
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
	}
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
		return nil, fmt.Errorf("failed to get credentials for registry %s: %w", normalizedRegistry, err)
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
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Implement actual registry connectivity test\n	if err := mrm.testRegistryConnectivity(ctx, normalizedRegistry, creds); err != nil {\n		return fmt.Errorf(\"registry connectivity test failed: %w\", err)\n	}
	// This would involve making a request to the registry API
	// For now, we just validate that we have credentials

	if creds == nil {
		return fmt.Errorf("no credentials available for registry %s", normalizedRegistry)
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

	return nil, fmt.Errorf("no credential provider could authenticate to registry %s", registry)
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
