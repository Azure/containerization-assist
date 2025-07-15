// Package application provides LLM configuration options for runtime customization
package application

import (
	"time"

	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
)

// LLMConfig contains comprehensive configuration for LLM operations
type LLMConfig struct {
	// Core LLM parameters
	MaxTokens        int32         `json:"max_tokens"`
	Temperature      float32       `json:"temperature"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	StreamingEnabled bool          `json:"streaming_enabled"`

	// Advanced LLM parameters
	TopP             *float32           `json:"top_p,omitempty"`
	FrequencyPenalty *float32           `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float32           `json:"presence_penalty,omitempty"`
	StopSequences    []string           `json:"stop_sequences,omitempty"`
	Seed             *int               `json:"seed,omitempty"`
	LogitBias        map[string]float32 `json:"logit_bias,omitempty"`

	// Infrastructure parameters
	RetryAttempts int           `json:"retry_attempts"`
	TokenBudget   int           `json:"token_budget"`
	BaseBackoff   time.Duration `json:"base_backoff"`
	MaxBackoff    time.Duration `json:"max_backoff"`

	// Prompt configuration
	PromptDir       string `json:"prompt_dir,omitempty"`
	EnableHotReload bool   `json:"enable_hot_reload"`
	AllowOverride   bool   `json:"allow_override"`
}

// DefaultLLMConfig returns the default LLM configuration
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		// Core parameters
		MaxTokens:        2048,
		Temperature:      0.3,
		RequestTimeout:   30 * time.Second,
		StreamingEnabled: false,

		// Advanced parameters (nil = use model defaults)
		TopP:             nil,
		FrequencyPenalty: nil,
		PresencePenalty:  nil,
		StopSequences:    nil,
		Seed:             nil,
		LogitBias:        nil,

		// Infrastructure parameters
		RetryAttempts: 3,
		TokenBudget:   5000,
		BaseBackoff:   200 * time.Millisecond,
		MaxBackoff:    10 * time.Second,

		// Prompt configuration
		PromptDir:       "", // Use embedded templates
		EnableHotReload: false,
		AllowOverride:   false,
	}
}

// ToSamplingConfig converts LLMConfig to infrastructure sampling.Config
func (c LLMConfig) ToSamplingConfig() sampling.Config {
	return sampling.Config{
		MaxTokens:        c.MaxTokens,
		Temperature:      c.Temperature,
		RetryAttempts:    c.RetryAttempts,
		TokenBudget:      c.TokenBudget,
		BaseBackoff:      c.BaseBackoff,
		MaxBackoff:       c.MaxBackoff,
		StreamingEnabled: c.StreamingEnabled,
		RequestTimeout:   c.RequestTimeout,
	}
}

// ToAdvancedParams converts LLMConfig to domain AdvancedParams
func (c LLMConfig) ToAdvancedParams() *domainsampling.AdvancedParams {
	// Only create AdvancedParams if at least one advanced parameter is set
	if c.TopP == nil && c.FrequencyPenalty == nil && c.PresencePenalty == nil &&
		len(c.StopSequences) == 0 && c.Seed == nil && len(c.LogitBias) == 0 {
		return nil
	}

	return &domainsampling.AdvancedParams{
		TopP:             c.TopP,
		FrequencyPenalty: c.FrequencyPenalty,
		PresencePenalty:  c.PresencePenalty,
		StopSequences:    c.StopSequences,
		Seed:             c.Seed,
		LogitBias:        c.LogitBias,
	}
}

// WithLLMConfig creates a server option that configures LLM parameters
func WithLLMConfig(llmConfig LLMConfig) ServerOption {
	return func(cfg *serverConfig) {
		// Store LLM configuration for use by the sampling client
		cfg.llmConfig = &llmConfig
	}
}

// Convenience functions for common LLM configurations

// WithTemperature sets the temperature for LLM responses
func WithTemperature(temperature float32) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.Temperature = temperature
	}
}

// WithMaxTokens sets the maximum number of tokens in LLM responses
func WithMaxTokens(maxTokens int32) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.MaxTokens = maxTokens
	}
}

// WithTopP sets the nucleus sampling parameter
func WithTopP(topP float32) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.TopP = &topP
	}
}

// WithFrequencyPenalty sets the frequency penalty parameter
func WithFrequencyPenalty(penalty float32) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.FrequencyPenalty = &penalty
	}
}

// WithPresencePenalty sets the presence penalty parameter
func WithPresencePenalty(penalty float32) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.PresencePenalty = &penalty
	}
}

// WithStreamingEnabled enables or disables streaming mode
func WithStreamingEnabled(enabled bool) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.StreamingEnabled = enabled
	}
}

// WithSeed sets the random seed for reproducible outputs
func WithSeed(seed int) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.Seed = &seed
	}
}

// WithStopSequences sets the stop sequences for LLM responses
func WithStopSequences(sequences []string) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.StopSequences = sequences
	}
}

// WithRequestTimeout sets the timeout for LLM requests
func WithRequestTimeout(timeout time.Duration) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.RequestTimeout = timeout
	}
}

// WithRetryConfig sets the retry configuration for LLM requests
func WithRetryConfig(attempts int, budget int) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.RetryAttempts = attempts
		cfg.llmConfig.TokenBudget = budget
	}
}

// WithBackoffConfig sets the backoff configuration for retries
func WithBackoffConfig(baseBackoff, maxBackoff time.Duration) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.BaseBackoff = baseBackoff
		cfg.llmConfig.MaxBackoff = maxBackoff
	}
}

// WithPromptDir sets the external prompt template directory
func WithPromptDir(promptDir string) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.PromptDir = promptDir
	}
}

// WithHotReloadEnabled enables or disables hot-reload for prompt templates
func WithHotReloadEnabled(enabled bool) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.EnableHotReload = enabled
	}
}

// WithPromptOverrideAllowed allows external templates to override embedded ones
func WithPromptOverrideAllowed(allowed bool) ServerOption {
	return func(cfg *serverConfig) {
		if cfg.llmConfig == nil {
			defaultConfig := DefaultLLMConfig()
			cfg.llmConfig = &defaultConfig
		}
		cfg.llmConfig.AllowOverride = allowed
	}
}

// Preset configurations for common use cases

// WithConservativeGeneration configures the LLM for conservative, focused generation
func WithConservativeGeneration() ServerOption {
	return WithLLMConfig(LLMConfig{
		MaxTokens:        1024,
		Temperature:      0.1,
		TopP:             float32Ptr(0.8),
		FrequencyPenalty: float32Ptr(0.3),
		PresencePenalty:  float32Ptr(0.2),
		RequestTimeout:   30 * time.Second,
		StreamingEnabled: false,
		RetryAttempts:    3,
		TokenBudget:      5000,
		BaseBackoff:      200 * time.Millisecond,
		MaxBackoff:       10 * time.Second,
	})
}

// WithCreativeGeneration configures the LLM for creative, diverse generation
func WithCreativeGeneration() ServerOption {
	return WithLLMConfig(LLMConfig{
		MaxTokens:        2048,
		Temperature:      0.8,
		TopP:             float32Ptr(0.9),
		FrequencyPenalty: float32Ptr(0.1),
		PresencePenalty:  float32Ptr(0.1),
		RequestTimeout:   45 * time.Second,
		StreamingEnabled: true,
		RetryAttempts:    2,
		TokenBudget:      8000,
		BaseBackoff:      200 * time.Millisecond,
		MaxBackoff:       10 * time.Second,
	})
}

// WithBalancedGeneration configures the LLM for balanced generation (default)
func WithBalancedGeneration() ServerOption {
	return WithLLMConfig(DefaultLLMConfig())
}

// WithFastGeneration configures the LLM for fast, efficient generation
func WithFastGeneration() ServerOption {
	return WithLLMConfig(LLMConfig{
		MaxTokens:        512,
		Temperature:      0.2,
		TopP:             float32Ptr(0.7),
		RequestTimeout:   15 * time.Second,
		StreamingEnabled: false,
		RetryAttempts:    2,
		TokenBudget:      2000,
		BaseBackoff:      100 * time.Millisecond,
		MaxBackoff:       5 * time.Second,
	})
}

// ToPromptConfig converts LLMConfig to prompts.ManagerConfig
func (c LLMConfig) ToPromptConfig() prompts.ManagerConfig {
	return prompts.ManagerConfig{
		TemplateDir:     c.PromptDir,
		EnableHotReload: c.EnableHotReload,
		AllowOverride:   c.AllowOverride,
	}
}

// Helper function to create float32 pointer
func float32Ptr(f float32) *float32 {
	return &f
}
