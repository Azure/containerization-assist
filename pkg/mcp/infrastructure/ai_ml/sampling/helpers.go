// Package sampling provides helper utilities for LLM operations
package sampling

import (
	"context"
	"strings"
	"unicode/utf8"
)

// GetWorkflowIDFromContext extracts workflow ID from context with multiple fallbacks
func GetWorkflowIDFromContext(ctx context.Context) string {
	// Try common context keys for workflow ID
	keys := []interface{}{
		"workflow_id", "workflowID", "workflow",
		"session_id", "sessionID", "session",
		"request_id", "requestID", "request",
	}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if id, ok := val.(string); ok && id != "" {
				return id
			}
		}
	}

	// Generate a fallback ID based on context if available
	if span := ctx.Value("span"); span != nil {
		return "ctx-derived"
	}

	return "unknown"
}

// GetStepNameFromContext extracts step name from context with multiple fallbacks
func GetStepNameFromContext(ctx context.Context) string {
	// Try common context keys for step name
	keys := []interface{}{
		"step_name", "stepName", "step", "current_step",
		"operation", "action", "task",
	}
	for _, key := range keys {
		if val := ctx.Value(key); val != nil {
			if step, ok := val.(string); ok && step != "" {
				return step
			}
		}
	}
	return "unknown"
}

// EstimateTokenCount provides a conservative token count estimate
func EstimateTokenCount(text string) int {
	// More sophisticated estimation considering:
	// - Word count (1.3 tokens per word on average)
	// - Character count (4 characters per token on average)
	// - Unicode considerations

	charCount := utf8.RuneCountInString(text)
	words := len(splitWords(text))

	// Use the more conservative estimate
	fromChars := charCount / 4
	fromWords := int(float64(words) * 1.3)

	if fromWords > fromChars {
		return fromWords
	}
	return fromChars
}

// splitWords splits text into words for more accurate token estimation
func splitWords(text string) []string {
	if text == "" {
		return nil
	}

	words := make([]string, 0)
	word := make([]rune, 0)

	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if len(word) > 0 {
				words = append(words, string(word))
				word = word[:0]
			}
		} else {
			word = append(word, r)
		}
	}

	if len(word) > 0 {
		words = append(words, string(word))
	}

	return words
}

// truncateText truncates a string to a maximum length with ellipsis
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// Contains checks if a string contains a substring (case-insensitive helper)
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// IsRetryable determines if an error is retryable based on common patterns
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "temporarily") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dns")
}
