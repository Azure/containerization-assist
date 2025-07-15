// Package application provides examples of LLM configuration usage
package application

import (
	"time"
)

// ExampleLLMConfigUsage demonstrates how to use LLM configuration options
func ExampleLLMConfigUsage() {
	// Example 1: Using comprehensive LLM configuration
	server1 := NewServerWithOptions(
		WithLLMConfig(LLMConfig{
			MaxTokens:        1024,
			Temperature:      0.7,
			TopP:             float32Ptr(0.9),
			FrequencyPenalty: float32Ptr(0.2),
			PresencePenalty:  float32Ptr(0.1),
			StreamingEnabled: true,
			RequestTimeout:   30 * time.Second,
			RetryAttempts:    3,
			TokenBudget:      5000,
		}),
	)
	_ = server1

	// Example 2: Using individual parameter configuration
	server2 := NewServerWithOptions(
		WithTemperature(0.5),
		WithMaxTokens(2048),
		WithTopP(0.8),
		WithFrequencyPenalty(0.3),
		WithStreamingEnabled(true),
	)
	_ = server2

	// Example 3: Using preset configurations
	server3 := NewServerWithOptions(
		WithCreativeGeneration(), // High temperature, diverse output
	)
	_ = server3

	server4 := NewServerWithOptions(
		WithConservativeGeneration(), // Low temperature, focused output
	)
	_ = server4

	server5 := NewServerWithOptions(
		WithFastGeneration(), // Optimized for speed
	)
	_ = server5

	// Example 4: Combining presets with custom overrides
	server6 := NewServerWithOptions(
		WithBalancedGeneration(),
		WithMaxTokens(4096),                // Override max tokens
		WithRequestTimeout(45*time.Second), // Override timeout
	)
	_ = server6

	// Example 5: Advanced configuration with stop sequences and seed
	server7 := NewServerWithOptions(
		WithSeed(42), // For reproducible outputs
		WithStopSequences([]string{"END", "STOP", "DONE"}),
		WithRetryConfig(5, 10000), // 5 attempts, 10k token budget
		WithBackoffConfig(100*time.Millisecond, 5*time.Second),
	)
	_ = server7

	// Example 6: Prompt configuration with hot-reload
	server8 := NewServerWithOptions(
		WithPromptDir("/path/to/custom/templates"),
		WithHotReloadEnabled(true),
		WithPromptOverrideAllowed(true),
	)
	_ = server8
}

// ExampleLLMConfigValidation demonstrates configuration validation
func ExampleLLMConfigValidation() {
	// Create a configuration
	config := LLMConfig{
		MaxTokens:   1024,
		Temperature: 0.7,
		TopP:        float32Ptr(0.9),
	}

	// Convert to infrastructure format
	samplingConfig := config.ToSamplingConfig()
	_ = samplingConfig

	// Convert to advanced parameters
	advancedParams := config.ToAdvancedParams()
	_ = advancedParams

	// Validate configuration (this would be done internally)
	if err := samplingConfig.Validate(); err != nil {
		// Handle validation error
		panic(err)
	}
}
