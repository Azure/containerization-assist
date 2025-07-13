// Package sampling contains domain-level abstractions for AI/LLM sampling.
// This package must NOT import any infrastructure packages.
package sampling

import "context"

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

// Sampler is the core interface for AI/LLM sampling operations.
type Sampler interface {
	// Sample performs a non-streaming sampling request.
	Sample(ctx context.Context, req Request) (Response, error)

	// Stream initiates a streaming sampling request.
	// The returned channel will be closed when streaming completes or errors.
	Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)
}

// AnalysisSampler extends Sampler with analysis-specific methods.
type AnalysisSampler interface {
	Sampler

	// AnalyzeDockerfile analyzes a Dockerfile for issues.
	AnalyzeDockerfile(ctx context.Context, content string) (*DockerfileAnalysis, error)

	// AnalyzeKubernetesManifest analyzes Kubernetes manifests.
	AnalyzeKubernetesManifest(ctx context.Context, content string) (*ManifestAnalysis, error)

	// AnalyzeSecurityScan analyzes security scan results.
	AnalyzeSecurityScan(ctx context.Context, scanResults string) (*SecurityAnalysis, error)
}

// FixSampler extends Sampler with fix-specific methods.
type FixSampler interface {
	Sampler

	// FixDockerfile attempts to fix issues in a Dockerfile.
	FixDockerfile(ctx context.Context, content string, issues []string) (*DockerfileFix, error)

	// FixKubernetesManifest attempts to fix issues in Kubernetes manifests.
	FixKubernetesManifest(ctx context.Context, content string, issues []string) (*ManifestFix, error)
}
