// Package metrics provides metrics collection middleware for the sampling infrastructure
package metrics

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// MetricsCollector defines the interface for collecting sampling metrics
type MetricsCollector interface {
	RecordSamplingRequest(ctx context.Context, operation string, success bool, duration time.Duration, contentSize int, responseSize int, validation ValidationResult)
	RecordStreamingRequest(ctx context.Context, operation string, success bool, duration time.Duration, totalTokens int)
}

// ValidationResult contains validation status information
type ValidationResult struct {
	IsValid       bool
	SyntaxValid   bool
	BestPractices bool
	Errors        []string
	Warnings      []string
}

// MetricsSampler wraps a UnifiedSampler with metrics collection
type MetricsSampler struct {
	next      sampling.UnifiedSampler
	collector MetricsCollector
}

// New creates a new metrics middleware wrapper
func New(next sampling.UnifiedSampler, collector MetricsCollector) *MetricsSampler {
	return &MetricsSampler{
		next:      next,
		collector: collector,
	}
}

// Sample wraps the Sample method with metrics collection
func (m *MetricsSampler) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	start := time.Now()

	resp, err := m.next.Sample(ctx, req)

	success := err == nil
	duration := time.Since(start)
	contentSize := len(req.Prompt) + len(req.SystemPrompt)
	responseSize := 0
	if success {
		responseSize = len(resp.Content)
	}

	m.collector.RecordSamplingRequest(ctx, "sample", success, duration, contentSize, responseSize, ValidationResult{IsValid: success})

	return resp, err
}

// Stream wraps the Stream method with metrics collection
func (m *MetricsSampler) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	start := time.Now()

	ch, err := m.next.Stream(ctx, req)
	if err != nil {
		m.collector.RecordStreamingRequest(ctx, "stream", false, time.Since(start), 0)
		return nil, err
	}

	// Wrap the channel to collect metrics when streaming completes
	wrappedCh := make(chan sampling.StreamChunk)
	go func() {
		defer close(wrappedCh)

		var totalTokens int
		var success = true

		for chunk := range ch {
			totalTokens = chunk.TokensSoFar
			if chunk.Error != nil {
				success = false
			}
			wrappedCh <- chunk
		}

		m.collector.RecordStreamingRequest(ctx, "stream", success, time.Since(start), totalTokens)
	}()

	return wrappedCh, nil
}

// AnalyzeDockerfile wraps the method with metrics collection
func (m *MetricsSampler) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	start := time.Now()

	result, err := m.next.AnalyzeDockerfile(ctx, content)

	success := err == nil
	validation := ValidationResult{IsValid: success}
	if result != nil {
		validation.Errors = result.Issues
		validation.Warnings = result.Suggestions
	}

	m.collector.RecordSamplingRequest(ctx, "analyze-dockerfile", success, time.Since(start),
		len(content), 0, validation)

	return result, err
}

// AnalyzeKubernetesManifest wraps the method with metrics collection
func (m *MetricsSampler) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	start := time.Now()

	result, err := m.next.AnalyzeKubernetesManifest(ctx, content)

	success := err == nil
	validation := ValidationResult{IsValid: success}
	if result != nil {
		validation.Errors = result.Issues
		validation.Warnings = result.Suggestions
	}

	m.collector.RecordSamplingRequest(ctx, "analyze-k8s-manifest", success, time.Since(start),
		len(content), 0, validation)

	return result, err
}

// AnalyzeSecurityScan wraps the method with metrics collection
func (m *MetricsSampler) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	start := time.Now()

	result, err := m.next.AnalyzeSecurityScan(ctx, scanResults)

	success := err == nil
	validation := ValidationResult{IsValid: success}

	m.collector.RecordSamplingRequest(ctx, "analyze-security", success, time.Since(start),
		len(scanResults), 0, validation)

	return result, err
}

// FixDockerfile wraps the method with metrics collection
func (m *MetricsSampler) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	start := time.Now()

	result, err := m.next.FixDockerfile(ctx, content, issues)

	success := err == nil
	responseSize := 0
	validation := ValidationResult{IsValid: success}
	if result != nil {
		responseSize = len(result.FixedContent)
		validation.SyntaxValid = true
		validation.BestPractices = true
	}

	m.collector.RecordSamplingRequest(ctx, "fix-dockerfile", success, time.Since(start),
		len(content), responseSize, validation)

	return result, err
}

// FixKubernetesManifest wraps the method with metrics collection
func (m *MetricsSampler) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	start := time.Now()

	result, err := m.next.FixKubernetesManifest(ctx, content, issues)

	success := err == nil
	responseSize := 0
	validation := ValidationResult{IsValid: success}
	if result != nil {
		responseSize = len(result.FixedContent)
		validation.SyntaxValid = true
		validation.BestPractices = true
	}

	m.collector.RecordSamplingRequest(ctx, "fix-k8s-manifest", success, time.Since(start),
		len(content), responseSize, validation)

	return result, err
}

// Ensure MetricsSampler implements UnifiedSampler
var _ sampling.UnifiedSampler = (*MetricsSampler)(nil)
