// Package sampling provides configurable sampling client creation
package sampling

import (
	"log/slog"
)

// ConfigurableClientOptions holds options for creating a configurable sampling client
type ConfigurableClientOptions struct {
	Config *Config
	Logger *slog.Logger
}

// NewConfigurableClient creates a sampling client with the provided configuration
func NewConfigurableClient(opts ConfigurableClientOptions) (*Client, error) {
	if opts.Config == nil {
		// Fall back to environment-based configuration
		return NewClientFromEnv(opts.Logger)
	}

	// Validate the configuration
	if err := opts.Config.Validate(); err != nil {
		return nil, err
	}

	// Create client with custom configuration
	client := NewClient(opts.Logger, WithConfig(*opts.Config))

	return client, nil
}

// NewClientWithConfig creates a sampling client with the given configuration
func NewClientWithConfig(logger *slog.Logger, config Config) (*Client, error) {
	return NewConfigurableClient(ConfigurableClientOptions{
		Config: &config,
		Logger: logger,
	})
}

// NewClientFromEnvWithOverrides creates a client from environment variables with optional overrides
func NewClientFromEnvWithOverrides(logger *slog.Logger, overrides *Config) (*Client, error) {
	// Start with environment configuration
	envConfig := LoadFromEnv()

	// Apply overrides if provided
	if overrides != nil {
		if overrides.MaxTokens > 0 {
			envConfig.MaxTokens = overrides.MaxTokens
		}
		if overrides.Temperature > 0 {
			envConfig.Temperature = overrides.Temperature
		}
		if overrides.RetryAttempts > 0 {
			envConfig.RetryAttempts = overrides.RetryAttempts
		}
		if overrides.TokenBudget > 0 {
			envConfig.TokenBudget = overrides.TokenBudget
		}
		if overrides.BaseBackoff > 0 {
			envConfig.BaseBackoff = overrides.BaseBackoff
		}
		if overrides.MaxBackoff > 0 {
			envConfig.MaxBackoff = overrides.MaxBackoff
		}
		if overrides.RequestTimeout > 0 {
			envConfig.RequestTimeout = overrides.RequestTimeout
		}
		// StreamingEnabled is a boolean, so we need to check if it's explicitly set
		envConfig.StreamingEnabled = overrides.StreamingEnabled
	}

	return NewClientWithConfig(logger, envConfig)
}
