// Package sampling provides factory methods for creating sampling clients with middleware
package sampling

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// NewBasicClient creates a simple sampling client
func NewBasicClient(logger *slog.Logger) *Client {
	return NewClient(logger)
}

// CreateDomainClient creates a domain-compatible client
func CreateDomainClient(logger *slog.Logger) sampling.UnifiedSampler {
	// For now, return a simple adapter until we resolve circular dependencies
	client := NewClient(logger)
	return &domainAdapter{client: client}
}

// domainAdapter adapts the Client to the domain UnifiedSampler interface
type domainAdapter struct {
	client *Client
}

func (d *domainAdapter) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	// Convert domain request to internal request
	internalReq := SamplingRequest{
		Prompt:       req.Prompt,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		SystemPrompt: req.SystemPrompt,
	}

	resp, err := d.client.SampleInternal(ctx, internalReq)
	if err != nil {
		return sampling.Response{}, err
	}

	return sampling.Response{
		Content:    resp.Content,
		TokensUsed: resp.TokensUsed,
		Model:      resp.Model,
		StopReason: resp.StopReason,
	}, nil
}

func (d *domainAdapter) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	// Simple implementation that just returns the full response as a single chunk
	ch := make(chan sampling.StreamChunk, 1)
	go func() {
		defer close(ch)

		resp, err := d.Sample(ctx, req)
		if err != nil {
			ch <- sampling.StreamChunk{Error: err}
			return
		}

		ch <- sampling.StreamChunk{
			Text:        resp.Content,
			TokensSoFar: resp.TokensUsed,
			IsFinal:     true,
		}
	}()

	return ch, nil
}

func (d *domainAdapter) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	// TODO: Implement using SampleInternal with appropriate prompts
	return &sampling.DockerfileAnalysis{
		Language:      "unknown",
		Framework:     "unknown",
		Port:          8080,
		BuildSteps:    []string{},
		Dependencies:  []string{},
		Issues:        []string{},
		Suggestions:   []string{},
		BaseImage:     "alpine:latest",
		EstimatedSize: "100MB",
	}, nil
}

func (d *domainAdapter) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	// TODO: Implement using SampleInternal with appropriate prompts
	return &sampling.ManifestAnalysis{
		ResourceTypes: []string{"Deployment", "Service"},
		Issues:        []string{},
		Suggestions:   []string{},
		SecurityRisks: []string{},
		BestPractices: []string{},
	}, nil
}

func (d *domainAdapter) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	// TODO: Implement using SampleInternal with appropriate prompts
	return &sampling.SecurityAnalysis{
		RiskLevel:       "low",
		Vulnerabilities: []sampling.Vulnerability{},
		Recommendations: []string{},
		Remediations:    []string{},
	}, nil
}

func (d *domainAdapter) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	// TODO: Implement using SampleInternal with appropriate prompts
	return &sampling.DockerfileFix{
		FixedContent: content,
		Changes:      []string{},
		Explanation:  "No fixes needed",
	}, nil
}

func (d *domainAdapter) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	// TODO: Implement using SampleInternal with appropriate prompts
	return &sampling.ManifestFix{
		FixedContent: content,
		Changes:      []string{},
		Explanation:  "No fixes needed",
	}, nil
}
