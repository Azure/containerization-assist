// Package service provides LLM configuration options for runtime customization
package service

import (
	"time"

	domainsampling "github.com/Azure/containerization-assist/pkg/mcp/domain/sampling"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/ai_ml/sampling"
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
		MaxTokens:     c.MaxTokens,
		Temperature:   c.Temperature,
		RetryAttempts: c.RetryAttempts,
		BaseBackoff:   c.BaseBackoff,
		MaxBackoff:    c.MaxBackoff,
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
