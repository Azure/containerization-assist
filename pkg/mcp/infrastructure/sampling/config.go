// Package sampling provides configuration for the sampling client
package sampling

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the sampling client
type Config struct {
	MaxTokens        int32         `json:"max_tokens" env:"SAMPLING_MAX_TOKENS"`
	Temperature      float32       `json:"temperature" env:"SAMPLING_TEMPERATURE"`
	RetryAttempts    int           `json:"retry_attempts" env:"SAMPLING_RETRY_ATTEMPTS"`
	TokenBudget      int           `json:"token_budget" env:"SAMPLING_TOKEN_BUDGET"`
	BaseBackoff      time.Duration `json:"base_backoff" env:"SAMPLING_BASE_BACKOFF"`
	MaxBackoff       time.Duration `json:"max_backoff" env:"SAMPLING_MAX_BACKOFF"`
	StreamingEnabled bool          `json:"streaming_enabled" env:"SAMPLING_STREAMING_ENABLED"`
	RequestTimeout   time.Duration `json:"request_timeout" env:"SAMPLING_REQUEST_TIMEOUT"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MaxTokens:        2048,
		Temperature:      0.3,
		RetryAttempts:    3,
		TokenBudget:      5000,
		BaseBackoff:      200 * time.Millisecond,
		MaxBackoff:       10 * time.Second,
		StreamingEnabled: false,
		RequestTimeout:   30 * time.Second,
	}
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() Config {
	cfg := DefaultConfig()

	if val := os.Getenv("SAMPLING_MAX_TOKENS"); val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 32); err == nil {
			cfg.MaxTokens = int32(parsed)
		}
	}

	if val := os.Getenv("SAMPLING_TEMPERATURE"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 32); err == nil {
			cfg.Temperature = float32(parsed)
		}
	}

	if val := os.Getenv("SAMPLING_RETRY_ATTEMPTS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.RetryAttempts = parsed
		}
	}

	if val := os.Getenv("SAMPLING_TOKEN_BUDGET"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cfg.TokenBudget = parsed
		}
	}

	if val := os.Getenv("SAMPLING_BASE_BACKOFF"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.BaseBackoff = parsed
		}
	}

	if val := os.Getenv("SAMPLING_MAX_BACKOFF"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.MaxBackoff = parsed
		}
	}

	if val := os.Getenv("SAMPLING_STREAMING_ENABLED"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			cfg.StreamingEnabled = parsed
		}
	}

	if val := os.Getenv("SAMPLING_REQUEST_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			cfg.RequestTimeout = parsed
		}
	}

	return cfg
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	if c.MaxTokens <= 0 {
		return ErrInvalidConfig("max_tokens must be positive")
	}

	if c.Temperature < 0 || c.Temperature > 2 {
		return ErrInvalidConfig("temperature must be between 0 and 2")
	}

	if c.RetryAttempts < 0 {
		return ErrInvalidConfig("retry_attempts must be non-negative")
	}

	if c.TokenBudget <= 0 {
		return ErrInvalidConfig("token_budget must be positive")
	}

	if c.BaseBackoff <= 0 {
		return ErrInvalidConfig("base_backoff must be positive")
	}

	if c.MaxBackoff <= c.BaseBackoff {
		return ErrInvalidConfig("max_backoff must be greater than base_backoff")
	}

	if c.RequestTimeout <= 0 {
		return ErrInvalidConfig("request_timeout must be positive")
	}

	return nil
}

// WithConfig returns an Option that applies the given configuration
func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.maxTokens = cfg.MaxTokens
		c.temperature = cfg.Temperature
		c.retryAttempts = cfg.RetryAttempts
		c.tokenBudget = cfg.TokenBudget
		c.baseBackoff = cfg.BaseBackoff
		c.maxBackoff = cfg.MaxBackoff
		c.streamingEnabled = cfg.StreamingEnabled
		c.requestTimeout = cfg.RequestTimeout
	}
}

// ErrInvalidConfig represents a configuration error
type ErrInvalidConfig string

func (e ErrInvalidConfig) Error() string {
	return "invalid sampling config: " + string(e)
}