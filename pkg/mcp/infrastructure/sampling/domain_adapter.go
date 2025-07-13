package sampling

import (
	"context"

	domain "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// DomainAdapter wraps the infrastructure sampler to implement the unified domain interface
type DomainAdapter struct {
	sampler domain.UnifiedSampler
}

// NewDomainAdapter creates a new domain adapter
func NewDomainAdapter(client *Client) *DomainAdapter {
	// Create a unified sampler with middleware
	sampler := CreateDomainClient(client.logger)
	return &DomainAdapter{sampler: sampler}
}

// Ensure DomainAdapter implements the unified interface
var _ domain.UnifiedSampler = (*DomainAdapter)(nil)

// Sample delegates to the wrapped sampler
func (d *DomainAdapter) Sample(ctx context.Context, req domain.Request) (domain.Response, error) {
	return d.sampler.Sample(ctx, req)
}

// Stream delegates to the wrapped sampler
func (d *DomainAdapter) Stream(ctx context.Context, req domain.Request) (<-chan domain.StreamChunk, error) {
	return d.sampler.Stream(ctx, req)
}

// AnalyzeDockerfile delegates to the wrapped sampler
func (d *DomainAdapter) AnalyzeDockerfile(ctx context.Context, content string) (*domain.DockerfileAnalysis, error) {
	return d.sampler.AnalyzeDockerfile(ctx, content)
}

// AnalyzeKubernetesManifest delegates to the wrapped sampler
func (d *DomainAdapter) AnalyzeKubernetesManifest(ctx context.Context, content string) (*domain.ManifestAnalysis, error) {
	return d.sampler.AnalyzeKubernetesManifest(ctx, content)
}

// AnalyzeSecurityScan delegates to the wrapped sampler
func (d *DomainAdapter) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*domain.SecurityAnalysis, error) {
	return d.sampler.AnalyzeSecurityScan(ctx, scanResults)
}

// FixDockerfile delegates to the wrapped sampler
func (d *DomainAdapter) FixDockerfile(ctx context.Context, content string, issues []string) (*domain.DockerfileFix, error) {
	return d.sampler.FixDockerfile(ctx, content, issues)
}

// FixKubernetesManifest delegates to the wrapped sampler
func (d *DomainAdapter) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*domain.ManifestFix, error) {
	return d.sampler.FixKubernetesManifest(ctx, content, issues)
}
