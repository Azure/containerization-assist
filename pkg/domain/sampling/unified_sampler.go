// Package sampling contains domain-level abstractions for AI/LLM operations.
// KEEP THIS PACKAGE CLEAN: it must not import infrastructure packages.
package sampling

import "context"

// UnifiedSampler combines core sampling, analysis, and fix functionality into one interface.
type UnifiedSampler interface {
	Sample(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)

	AnalyzeDockerfile(ctx context.Context, content string) (*DockerfileAnalysis, error)
	AnalyzeKubernetesManifest(ctx context.Context, content string) (*ManifestAnalysis, error)
	AnalyzeSecurityScan(ctx context.Context, scanResults string) (*SecurityAnalysis, error)

	FixDockerfile(ctx context.Context, content string, issues []string) (*DockerfileFix, error)
	FixKubernetesManifest(ctx context.Context, content string, issues []string) (*ManifestFix, error)
}
