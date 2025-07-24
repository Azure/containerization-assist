package sampling

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// SampleWithProgress performs streaming sampling and updates progress in real-time.
// This demonstrates the integration between sampling and progress tracking.
func (c *Client) SampleWithProgress(
	ctx context.Context,
	req SamplingRequest,
	tracker *progress.Tracker,
	step int,
	baseMessage string,
) (*SamplingResponse, error) {
	logger := c.logger.With("op", "SampleWithProgress", "step", step)

	// Start streaming
	chunks, err := c.SampleStream(ctx, req)
	if err != nil {
		tracker.Error(step, fmt.Sprintf("%s failed to start", baseMessage), err)
		return nil, err
	}

	var fullContent string
	var totalTokens int
	var model string

	// Process stream chunks and update progress
	for chunk := range chunks {
		if chunk.Error != nil {
			tracker.Error(step, fmt.Sprintf("%s failed", baseMessage), chunk.Error)
			return nil, chunk.Error
		}

		fullContent += chunk.Text
		totalTokens = chunk.TokensSoFar
		model = chunk.Model

		if chunk.IsFinal {
			// Final update
			meta := map[string]interface{}{
				"tokens_used":    totalTokens,
				"model":          model,
				"content_length": len(fullContent),
			}
			tracker.Update(step, fmt.Sprintf("%s completed (%d tokens)", baseMessage, totalTokens), meta)
			break
		} else {
			// Intermediate update showing streaming progress
			meta := map[string]interface{}{
				"tokens_so_far": totalTokens,
				"model":         model,
				"streaming":     true,
			}
			message := fmt.Sprintf("%s... (%d tokens)", baseMessage, totalTokens)
			tracker.Update(step, message, meta)
		}
	}

	logger.Info("Streaming sampling completed",
		"tokens_used", totalTokens,
		"model", model,
		"content_length", len(fullContent))

	return &SamplingResponse{
		Content:    fullContent,
		TokensUsed: totalTokens,
		Model:      model,
		StopReason: "stop", // Assume completion
	}, nil
}

// SampleWithProgressAndRetry combines streaming, progress tracking, and simple retry logic.
func (c *Client) SampleWithProgressAndRetry(
	ctx context.Context,
	req SamplingRequest,
	tracker *progress.Tracker,
	step int,
	baseMessage string,
	maxRetries int,
) (*SamplingResponse, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		message := baseMessage
		if attempt > 1 {
			message = fmt.Sprintf("%s (attempt %d/%d)", baseMessage, attempt, maxRetries)
		}

		resp, err := c.SampleWithProgress(ctx, req, tracker, step, message)
		if err == nil {
			// Success
			return resp, nil
		}

		// Check if we should retry
		if attempt < maxRetries {
			// Continue retrying
			c.logger.Warn("Sampling attempt failed, retrying",
				"attempt", attempt,
				"max_retries", maxRetries,
				"error", err)
			continue
		} else {
			// Final attempt failed
			c.logger.Error("All sampling attempts failed",
				"attempt", attempt,
				"max_retries", maxRetries,
				"error", err)
			return nil, err
		}
	}

	return nil, fmt.Errorf("all %d attempts failed", maxRetries)
}
