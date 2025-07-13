// Package trace provides tracing middleware for the sampling infrastructure
package trace

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracedSampler wraps a UnifiedSampler with OpenTelemetry tracing
type TracedSampler struct {
	next   sampling.UnifiedSampler
	tracer trace.Tracer
}

// New creates a new tracing middleware wrapper
func New(next sampling.UnifiedSampler, tracer trace.Tracer) *TracedSampler {
	return &TracedSampler{
		next:   next,
		tracer: tracer,
	}
}

// Sample wraps the Sample method with tracing
func (t *TracedSampler) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	ctx, span := t.tracer.Start(ctx, "sampling.Sample",
		trace.WithAttributes(
			attribute.Int("sampling.max_tokens", int(req.MaxTokens)),
			attribute.Float64("sampling.temperature", float64(req.Temperature)),
			attribute.Bool("sampling.stream", req.Stream),
		),
	)
	defer span.End()

	resp, err := t.next.Sample(ctx, req)
	if err != nil {
		span.RecordError(err)
	} else {
		span.SetAttributes(
			attribute.String("sampling.model", resp.Model),
			attribute.Int("sampling.tokens_used", resp.TokensUsed),
		)
	}
	return resp, err
}

// Stream wraps the Stream method with tracing
func (t *TracedSampler) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	ctx, span := t.tracer.Start(ctx, "sampling.Stream",
		trace.WithAttributes(
			attribute.Int("sampling.max_tokens", int(req.MaxTokens)),
			attribute.Float64("sampling.temperature", float64(req.Temperature)),
		),
	)

	ch, err := t.next.Stream(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// Wrap the channel to end span when streaming completes
	wrappedCh := make(chan sampling.StreamChunk)
	go func() {
		defer span.End()
		defer close(wrappedCh)

		var totalTokens int
		for chunk := range ch {
			totalTokens = chunk.TokensSoFar
			if chunk.Error != nil {
				span.RecordError(chunk.Error)
			}
			wrappedCh <- chunk
		}
		span.SetAttributes(attribute.Int("sampling.total_tokens", totalTokens))
	}()

	return wrappedCh, nil
}

// AnalyzeDockerfile wraps the method with tracing
func (t *TracedSampler) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	var result *sampling.DockerfileAnalysis
	err := t.traceAnalysis(ctx, "dockerfile", len(content), func(ctx context.Context) error {
		var err error
		result, err = t.next.AnalyzeDockerfile(ctx, content)
		return err
	})
	return result, err
}

// AnalyzeKubernetesManifest wraps the method with tracing
func (t *TracedSampler) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	var result *sampling.ManifestAnalysis
	err := t.traceAnalysis(ctx, "kubernetes-manifest", len(content), func(ctx context.Context) error {
		var err error
		result, err = t.next.AnalyzeKubernetesManifest(ctx, content)
		return err
	})
	return result, err
}

// AnalyzeSecurityScan wraps the method with tracing
func (t *TracedSampler) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	var result *sampling.SecurityAnalysis
	err := t.traceAnalysis(ctx, "security-scan", len(scanResults), func(ctx context.Context) error {
		var err error
		result, err = t.next.AnalyzeSecurityScan(ctx, scanResults)
		return err
	})
	return result, err
}

// FixDockerfile wraps the method with tracing
func (t *TracedSampler) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	var result *sampling.DockerfileFix
	err := t.traceFix(ctx, "dockerfile", len(content), len(issues), func(ctx context.Context) error {
		var err error
		result, err = t.next.FixDockerfile(ctx, content, issues)
		return err
	})
	return result, err
}

// FixKubernetesManifest wraps the method with tracing
func (t *TracedSampler) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	var result *sampling.ManifestFix
	err := t.traceFix(ctx, "kubernetes-manifest", len(content), len(issues), func(ctx context.Context) error {
		var err error
		result, err = t.next.FixKubernetesManifest(ctx, content, issues)
		return err
	})
	return result, err
}

// traceAnalysis is a helper for analysis operations
func (t *TracedSampler) traceAnalysis(ctx context.Context, contentType string, contentSize int, fn func(context.Context) error) error {
	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("sampling.Analyze%s", contentType),
		trace.WithAttributes(
			attribute.String(tracing.AttrSamplingContentType, contentType),
			attribute.Int(tracing.AttrSamplingContentSize, contentSize),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

// traceFix is a helper for fix operations
func (t *TracedSampler) traceFix(ctx context.Context, contentType string, contentSize int, issueCount int, fn func(context.Context) error) error {
	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("sampling.Fix%s", contentType),
		trace.WithAttributes(
			attribute.String(tracing.AttrSamplingContentType, contentType),
			attribute.Int(tracing.AttrSamplingContentSize, contentSize),
			attribute.Int("sampling.issue_count", issueCount),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

// Ensure TracedSampler implements UnifiedSampler
var _ sampling.UnifiedSampler = (*TracedSampler)(nil)
