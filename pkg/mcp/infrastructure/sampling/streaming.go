package sampling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StreamChunk represents a partial response from streaming sampling.
type StreamChunk struct {
	Text        string
	IsFinal     bool
	TokensSoFar int
	Model       string
	Error       error
}

// SampleStream performs sampling and streams partial output through a channel.
// The returned channel is closed exactly once when streaming completes or fails.
func (c *Client) SampleStream(
	ctx context.Context,
	req SamplingRequest,
) (<-chan StreamChunk, error) {
	ctx, span := tracing.StartSpan(ctx, "sampling.sampleStream")
	span.SetAttributes(
		attribute.String(tracing.AttrComponent, "sampling"),
		attribute.Bool("sampling.streaming", true),
		attribute.Int("sampling.max_tokens", int(req.MaxTokens)),
		attribute.Float64("sampling.temperature", float64(req.Temperature)),
		attribute.Int("sampling.prompt_length", len(req.Prompt)),
	)
	logger := c.logger.With("op", "SampleStream")

	// Check if we have MCP server context
	if server.ServerFromContext(ctx) == nil {
		span.End()
		return nil, errors.New("no MCP server in context – cannot perform streaming sampling")
	}

	// Set defaults if not provided
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	if req.Temperature == 0 {
		req.Temperature = c.temperature
	}
	req.Stream = true // Force streaming mode

	out := make(chan StreamChunk, 32)
	go func() {
		defer span.End()
		defer close(out)

		var lastErr error
		for attempt := 0; attempt < c.retryAttempts; attempt++ {
			span.SetAttributes(attribute.Int(tracing.AttrSamplingRetryAttempt, attempt+1))

			// Try streaming call
			stream, err := c.callMCPStream(ctx, req)
			if err == nil {
				// Successfully got stream, process chunks
				var tokenCount int
				for delta := range stream {
					tokenCount += estimateTokens(delta)
					chunk := StreamChunk{
						Text:        delta,
						TokensSoFar: tokenCount,
						Model:       "mcp-streaming",
					}
					select {
					case out <- chunk:
					case <-ctx.Done():
						span.RecordError(ctx.Err())
						return
					}
				}

				// Send final chunk
				final := StreamChunk{
					IsFinal:     true,
					TokensSoFar: tokenCount,
					Model:       "mcp-streaming",
				}
				select {
				case out <- final:
				case <-ctx.Done():
					span.RecordError(ctx.Err())
				}
				return
			}

			// Handle error
			if !isRetryable(err) {
				span.RecordError(err)
				lastErr = err
				break
			}

			lastErr = err
			backoff := c.calculateBackoff(attempt)
			logger.Warn("stream attempt failed – backing off",
				"attempt", attempt+1, "err", err, "backoff", backoff)

			span.AddEvent("retry.backoff", trace.WithAttributes(
				attribute.String("error", err.Error()),
				attribute.Int("attempt", attempt+1),
				attribute.String("backoff", backoff.String()),
			))

			// Respect context during backoff
			select {
			case <-ctx.Done():
				span.RecordError(ctx.Err())
				return
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}

		// All attempts failed, send error chunk
		if lastErr != nil {
			span.RecordError(lastErr)
			logger.Error("all streaming attempts failed", "err", lastErr)
			errorChunk := StreamChunk{
				IsFinal: true,
				Error:   lastErr,
			}
			select {
			case out <- errorChunk:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}

// callMCPStream attempts to establish a streaming connection via MCP.
// Returns a channel of text deltas, or an error if the stream cannot be established.
func (c *Client) callMCPStream(
	ctx context.Context,
	req SamplingRequest,
) (<-chan string, error) {
	c.logger.Debug("MCP streaming not yet implemented, using fallback stream",
		"prompt_length", len(req.Prompt),
		"max_tokens", req.MaxTokens,
		"streaming", req.Stream)

	// For now, simulate streaming by chunking the fallback response
	// This allows the streaming interface to work while MCP streaming is developed
	out := make(chan string, 10)

	go func() {
		defer close(out)

		response := fmt.Sprintf("AI ASSISTANCE REQUESTED (STREAMING): %s", req.Prompt)

		// Simulate streaming by sending chunks
		chunkSize := 20
		for i := 0; i < len(response); i += chunkSize {
			end := i + chunkSize
			if end > len(response) {
				end = len(response)
			}
			chunk := response[i:end]

			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}

			// Simulate network delay
			time.Sleep(50 * time.Millisecond)
		}
	}()

	return out, nil
}
