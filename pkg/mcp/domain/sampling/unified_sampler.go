// Package sampling contains domain-level abstractions for AI/LLM operations.
// KEEP THIS PACKAGE CLEAN: it must not import infrastructure packages.
package sampling

import "context"

// UnifiedSampler is the single contract for all AI/LLM operations.
// It combines core sampling, analysis, and fix functionality into one interface.
// This provides a unified interface for all AI/LLM operations.
type UnifiedSampler interface {
	// Core sampling operations
	Sample(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)

	// Analysis operations
	AnalyzeDockerfile(ctx context.Context, content string) (*DockerfileAnalysis, error)
	AnalyzeKubernetesManifest(ctx context.Context, content string) (*ManifestAnalysis, error)
	AnalyzeSecurityScan(ctx context.Context, scanResults string) (*SecurityAnalysis, error)

	// Fix operations
	FixDockerfile(ctx context.Context, content string, issues []string) (*DockerfileFix, error)
	FixKubernetesManifest(ctx context.Context, content string, issues []string) (*ManifestFix, error)
}
