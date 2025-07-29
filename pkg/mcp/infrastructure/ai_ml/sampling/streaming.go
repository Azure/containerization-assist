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
	c.logger.Debug("Starting streaming sampling",
		"max_tokens", req.MaxTokens,
		"temperature", req.Temperature,
		"prompt_length", len(req.Prompt))
	logger := c.logger.With("op", "SampleStream")

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
			logger.Debug("Streaming sampling retry attempt", "attempt", attempt+1)

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
						logger.Error("Context cancelled", "error", ctx.Err())
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
					logger.Error("Context cancelled", "error", ctx.Err())
				}
				return
			}

			// Handle error
			if !IsRetryable(err) {
				logger.Error("Streaming sampling error", "error", err)
				lastErr = err
				break
			}

			lastErr = err
			backoff := c.calculateBackoff(attempt)
			logger.Warn("stream attempt failed – backing off",
				"attempt", attempt+1, "err", err, "backoff", backoff)

			logger.Debug("Applying backoff before streaming retry",
				"error", err.Error(),
				"attempt", attempt+1,
				"backoff", backoff.String())

			// Respect context during backoff
			select {
			case <-ctx.Done():
				logger.Error("Context cancelled", "error", ctx.Err())
				return
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}

		// All attempts failed, send error chunk
		if lastErr != nil {
			logger.Error("Streaming sampling failed after all retries", "error", lastErr)
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
		"temperature", req.Temperature,
		"streaming", req.Stream)

	// Log advanced parameters for streaming (when MCP streaming is implemented,
	// these would be passed to the actual MCP streaming API)
	if req.TopP != nil {
		c.logger.Debug("Streaming with TopP parameter", "top_p", *req.TopP)
	}
	if req.FrequencyPenalty != nil {
		c.logger.Debug("Streaming with FrequencyPenalty parameter", "frequency_penalty", *req.FrequencyPenalty)
	}
	if req.PresencePenalty != nil {
		c.logger.Debug("Streaming with PresencePenalty parameter", "presence_penalty", *req.PresencePenalty)
	}
	if len(req.StopSequences) > 0 {
		c.logger.Debug("Streaming with StopSequences parameter", "stop_sequences", req.StopSequences)
	}
	if req.Seed != nil {
		c.logger.Debug("Streaming with Seed parameter", "seed", *req.Seed)
	}
	if len(req.LogitBias) > 0 {
		c.logger.Debug("Streaming with LogitBias parameter", "logit_bias_count", len(req.LogitBias))
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
