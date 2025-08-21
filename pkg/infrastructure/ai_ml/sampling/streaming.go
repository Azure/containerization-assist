package sampling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/server"
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
	// Log streaming sampling parameters

	// Check if we have MCP server context
	if server.ServerFromContext(ctx) == nil {
		// End of streaming operation
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
		defer close(out) // End of streaming operation

		var lastErr error
		for attempt := 0; attempt < c.retryAttempts; attempt++ {

			// Try streaming call
			stream, err := c.callMCPStream(ctx, req)
			if err == nil {
				// Successfully got stream, process chunks
				var tokenCount int
				startTime := time.Now()

				for delta := range stream {
					tokenCount += estimateTokens(delta)
					chunk := StreamChunk{
						Text:        delta,
						TokensSoFar: tokenCount,
						Model:       "mcp-streaming",
					}

					// Emit token-level progress every 10 tokens
					if tokenCount%10 == 0 {
						c.emitTokenProgress(ctx, tokenCount, req.MaxTokens, startTime)
					}

					select {
					case out <- chunk:
					case <-ctx.Done():
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
				}
				return
			}

			// Handle error
			if !IsRetryable(err) {
				lastErr = err
				break
			}

			lastErr = err
			backoff := c.calculateBackoff(attempt)

			// Respect context during backoff
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}

		// All attempts failed, send error chunk
		if lastErr != nil {
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

	// Log advanced parameters for streaming (when MCP streaming is implemented,
	// these would be passed to the actual MCP streaming API)
	if req.TopP != nil {
	}
	if req.FrequencyPenalty != nil {
	}
	if req.PresencePenalty != nil {
	}
	if len(req.StopSequences) > 0 {
	}
	if req.Seed != nil {
	}
	if len(req.LogitBias) > 0 {
	}

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
