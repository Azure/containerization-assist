// Package registrar handles tool and prompt registration
package registrar

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// RetryConfig holds retry configuration for a tool
type RetryConfig struct {
	MaxRetries      int
	RetryableErrors []string
	BackoffBase     time.Duration
	BackoffMax      time.Duration
}

// DefaultRetryConfigs provides default retry settings per tool
var DefaultRetryConfigs = map[string]RetryConfig{
	"analyze_repository": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"file not found",
			"permission denied",
			"directory not accessible",
			"timeout",
		},
		BackoffBase: 1 * time.Second,
		BackoffMax:  10 * time.Second,
	},
	"generate_dockerfile": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"template error",
			"ai service unavailable",
			"timeout",
			"rate limit",
		},
		BackoffBase: 2 * time.Second,
		BackoffMax:  20 * time.Second,
	},
	"build_image": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"connection reset",
			"timeout",
			"temporary failure",
			"docker daemon not responding",
			"no space left on device",
		},
		BackoffBase: 2 * time.Second,
		BackoffMax:  30 * time.Second,
	},
	"scan_image": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"scanner unavailable",
			"connection timeout",
			"service temporarily unavailable",
			"rate limit exceeded",
		},
		BackoffBase: 3 * time.Second,
		BackoffMax:  30 * time.Second,
	},
	"tag_image": {
		MaxRetries: 2,
		RetryableErrors: []string{
			"docker daemon error",
			"image not found",
			"temporary failure",
		},
		BackoffBase: 1 * time.Second,
		BackoffMax:  5 * time.Second,
	},
	"push_image": {
		MaxRetries: 5,
		RetryableErrors: []string{
			"registry unavailable",
			"authentication failed",
			"network error",
			"connection reset",
			"timeout",
			"503 service unavailable",
			"502 bad gateway",
		},
		BackoffBase: 3 * time.Second,
		BackoffMax:  60 * time.Second,
	},
	"generate_k8s_manifests": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"template generation error",
			"ai service unavailable",
			"timeout",
			"rate limit",
		},
		BackoffBase: 2 * time.Second,
		BackoffMax:  20 * time.Second,
	},
	"prepare_cluster": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"cluster unavailable",
			"connection timeout",
			"authentication error",
			"kubectl error",
			"api server unavailable",
		},
		BackoffBase: 3 * time.Second,
		BackoffMax:  30 * time.Second,
	},
	"deploy_application": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"deployment failed",
			"timeout waiting for pods",
			"cluster resource unavailable",
			"api server error",
			"connection reset",
		},
		BackoffBase: 5 * time.Second,
		BackoffMax:  60 * time.Second,
	},
	"verify_deployment": {
		MaxRetries: 3,
		RetryableErrors: []string{
			"health check failed",
			"pods not ready",
			"service unavailable",
			"connection timeout",
			"endpoint not reachable",
		},
		BackoffBase: 5 * time.Second,
		BackoffMax:  60 * time.Second,
	},
}

// BackoffStrategy defines retry delay calculation
type BackoffStrategy interface {
	GetDelay(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff with jitter
type ExponentialBackoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// GetDelay calculates the delay for a given retry attempt
func (e *ExponentialBackoff) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential delay: base * 2^(attempt-1)
	delay := e.BaseDelay * time.Duration(1<<uint(attempt-1))

	// Cap at max delay
	if delay > e.MaxDelay {
		delay = e.MaxDelay
	}

	// Add jitter (up to 25% of delay)
	jitter := time.Duration(rand.Int63n(int64(delay / 4)))
	return delay + jitter
}

// wrapWithRetry wraps a tool handler with retry logic and backoff
func (tr *ToolRegistrar) wrapWithRetry(
	toolName string,
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get retry parameters from request
		retryNumber := 1
		if rn, ok := args["retryNumber"].(float64); ok {
			retryNumber = int(rn)
		}

		// Get max retries - first from request, then from config
		maxRetries := 3 // default
		if config, exists := DefaultRetryConfigs[toolName]; exists {
			maxRetries = config.MaxRetries
		}
		if mr, ok := args["maxRetries"].(float64); ok {
			maxRetries = int(mr)
		}

		// Log retry attempt if not the first
		if retryNumber > 1 {
			tr.logger.Info("Tool retry attempt",
				"tool", toolName,
				"retryNumber", retryNumber,
				"maxRetries", maxRetries,
				"sessionId", args["session_id"],
			)

			// Calculate and suggest backoff delay
			backoffDelay := tr.calculateBackoffDelay(toolName, retryNumber)
			if backoffDelay > 0 {
				tr.logger.Info("Suggested backoff delay",
					"tool", toolName,
					"delay", backoffDelay.String(),
					"retryNumber", retryNumber,
				)
			}
		}

		// Execute the handler
		result, err := handler(ctx, req)

		// Check if we should suggest retry
		if err != nil && retryNumber < maxRetries {
			if config, exists := DefaultRetryConfigs[toolName]; exists {
				if shouldRetry(err, config.RetryableErrors) {
					tr.logger.Warn("Retryable error detected",
						"tool", toolName,
						"error", err.Error(),
						"retryNumber", retryNumber,
						"nextAttempt", retryNumber+1,
					)
					// Return result suggesting retry with backoff
					return tr.createRetryResult(toolName, retryNumber+1, err, true)
				}
			}
		}

		// If max retries reached, log it
		if err != nil && retryNumber >= maxRetries {
			tr.logger.Error("Max retries reached for tool",
				"tool", toolName,
				"error", err.Error(),
				"retryNumber", retryNumber,
				"maxRetries", maxRetries,
			)
		}

		return result, err
	}
}

// shouldRetry checks if error matches retryable patterns
func shouldRetry(err error, patterns []string) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, pattern := range patterns {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// createRetryResult creates a result suggesting retry with optional backoff information
func (tr *ToolRegistrar) createRetryResult(toolName string, nextRetry int, err error, includeBackoff bool) (*mcp.CallToolResult, error) {
	retryHint := map[string]interface{}{
		"should_retry": true,
		"next_attempt": nextRetry,
		"retry_tool":   toolName,
		"reason":       fmt.Sprintf("Retryable error detected, attempt %d", nextRetry),
	}

	// Add backoff information if requested
	if includeBackoff {
		backoffDelay := tr.calculateBackoffDelay(toolName, nextRetry)
		retryHint["backoff_seconds"] = backoffDelay.Seconds()
		retryHint["backoff_ms"] = backoffDelay.Milliseconds()
		retryHint["reason"] = fmt.Sprintf("Retryable error detected, attempt %d - wait %v before retry", nextRetry, backoffDelay)
	}

	result := map[string]interface{}{
		"success":    false,
		"error":      err.Error(),
		"retry_hint": retryHint,
		"chain_hint": map[string]interface{}{
			"next_tool": toolName, // Retry same tool
			"reason":    fmt.Sprintf("Retrying after error (attempt %d): %s", nextRetry, err.Error()),
		},
	}

	jsonData, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// GetBackoffStrategy returns the backoff strategy for a tool
func GetBackoffStrategy(toolName string) BackoffStrategy {
	config, exists := DefaultRetryConfigs[toolName]
	if !exists {
		// Default backoff if tool not configured
		return &ExponentialBackoff{
			BaseDelay: 2 * time.Second,
			MaxDelay:  30 * time.Second,
		}
	}

	return &ExponentialBackoff{
		BaseDelay: config.BackoffBase,
		MaxDelay:  config.BackoffMax,
	}
}

// calculateBackoffDelay calculates the backoff delay for a retry attempt
func (tr *ToolRegistrar) calculateBackoffDelay(toolName string, retryNumber int) time.Duration {
	config, exists := DefaultRetryConfigs[toolName]
	if !exists {
		// Default backoff
		return time.Duration(retryNumber-1) * 2 * time.Second
	}

	// Use exponential backoff from the config
	backoff := &ExponentialBackoff{
		BaseDelay: config.BackoffBase,
		MaxDelay:  config.BackoffMax,
	}

	return backoff.GetDelay(retryNumber)
}
