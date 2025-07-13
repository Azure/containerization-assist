// Package sampling contains domain-level abstractions for AI/LLM sampling.
// This package must NOT import any infrastructure packages.
package sampling

// Request represents a sampling request with all supported parameters.
type Request struct {
	// Core parameters
	Prompt       string
	MaxTokens    int32
	Temperature  float32
	SystemPrompt string
	Stream       bool

	// Advanced parameters (optional)
	Advanced *AdvancedParams

	// Metadata for context
	Metadata map[string]interface{}
}

// AdvancedParams contains optional advanced sampling parameters.
type AdvancedParams struct {
	TopP             *float32           `json:"top_p,omitempty"`
	FrequencyPenalty *float32           `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float32           `json:"presence_penalty,omitempty"`
	StopSequences    []string           `json:"stop_sequences,omitempty"`
	Seed             *int               `json:"seed,omitempty"`
	LogitBias        map[string]float32 `json:"logit_bias,omitempty"`
}

// Response represents a completed sampling response.
type Response struct {
	Content    string
	Model      string
	TokensUsed int
	StopReason string
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	Text        string
	TokensSoFar int
	Model       string
	IsFinal     bool
	Error       error
}
