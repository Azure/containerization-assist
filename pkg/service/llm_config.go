// Package service provides LLM configuration options for runtime customization
package service

import (
	"time"
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
