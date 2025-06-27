package retry

import (
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ErrorClassifier categorizes errors for retry and fix strategies
type ErrorClassifier struct {
	patterns map[string][]string
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		patterns: map[string][]string{
			"network": {
				"connection refused", "connection reset", "connection timeout",
				"no route to host", "network unreachable", "dial tcp",
				"timeout", "deadline exceeded", "i/o timeout",
			},
			"resource": {
				"no space left", "disk full", "out of memory",
				"resource temporarily unavailable", "too many open files",
				"port already in use", "address already in use",
			},
			"permission": {
				"permission denied", "access denied", "unauthorized",
				"forbidden", "not allowed", "insufficient privileges",
			},
			"config": {
				"configuration error", "invalid configuration", "config not found",
				"missing required", "invalid format", "parse error",
			},
			"dependency": {
				"not found", "no such file", "command not found",
				"module not found", "package not found", "import error",
			},
			"docker": {
				"docker daemon", "docker engine", "dockerfile",
				"image not found", "build failed", "push failed", "pull failed",
			},
			"kubernetes": {
				"kubectl", "kubernetes", "k8s", "pod", "deployment",
				"service account", "cluster", "node", "namespace",
			},
			"git": {
				"git", "repository", "branch", "commit", "merge conflict",
				"authentication failed", "remote", "clone failed",
			},
			"ai": {
				"model not available", "rate limited", "quota exceeded",
				"api key", "authentication", "token", "openai", "azure openai",
			},
			"validation": {
				"validation failed", "invalid input", "malformed",
				"schema violation", "constraint violation", "format error",
			},
			"temporary": {
				"temporary failure", "try again", "retry", "throttled",
				"rate limit", "service unavailable", "502", "503", "504",
			},
		},
	}
}

// ClassifyError categorizes an error based on its message and type
func (ec *ErrorClassifier) ClassifyError(err error) string {
	if err == nil {
		return "unknown"
	}

	errMsg := strings.ToLower(err.Error())

	// Check if it's an MCP error with category
	if mcpErr, ok := err.(*errors.MCPError); ok {
		switch mcpErr.Category {
		case errors.CategoryNetwork:
			return "network"
		case errors.CategoryResource:
			return "resource"
		case errors.CategoryValidation:
			return "validation"
		case errors.CategoryAuth:
			return "permission"
		case errors.CategoryConfig:
			return "config"
		case errors.CategoryTimeout:
			return "network"
		case errors.CategoryInternal:
			return "internal"
		}
	}

	// Pattern-based classification
	for category, patterns := range ec.patterns {
		for _, pattern := range patterns {
			if strings.Contains(errMsg, pattern) {
				return category
			}
		}
	}

	return "unknown"
}

// IsRetryable determines if an error should be retried
func (ec *ErrorClassifier) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check MCP error retryable flag
	if mcpErr, ok := err.(*errors.MCPError); ok {
		return mcpErr.Retryable
	}

	category := ec.ClassifyError(err)
	retryableCategories := []string{
		"network", "temporary", "resource", "docker", "kubernetes", "git",
	}

	for _, retryable := range retryableCategories {
		if category == retryable {
			return true
		}
	}

	return false
}

// IsFixable determines if an error can potentially be fixed automatically
func (ec *ErrorClassifier) IsFixable(err error) bool {
	if err == nil {
		return false
	}

	category := ec.ClassifyError(err)
	fixableCategories := []string{
		"config", "dependency", "docker", "permission", "validation",
	}

	for _, fixable := range fixableCategories {
		if category == fixable {
			return true
		}
	}

	return false
}

// GetFixPriority returns the priority level for fixing this error type
func (ec *ErrorClassifier) GetFixPriority(err error) int {
	category := ec.ClassifyError(err)

	priorities := map[string]int{
		"validation": 1, // Highest priority - quick fixes
		"config":     2,
		"dependency": 3,
		"permission": 4,
		"docker":     5,
		"kubernetes": 6,
		"network":    7,
		"resource":   8,
		"git":        9,
		"unknown":    10, // Lowest priority
	}

	if priority, exists := priorities[category]; exists {
		return priority
	}
	return 10
}

// AddPattern adds a new error pattern for a category
func (ec *ErrorClassifier) AddPattern(category, pattern string) {
	if ec.patterns[category] == nil {
		ec.patterns[category] = make([]string, 0)
	}
	ec.patterns[category] = append(ec.patterns[category], pattern)
}

// GetCategories returns all available error categories
func (ec *ErrorClassifier) GetCategories() []string {
	categories := make([]string, 0, len(ec.patterns))
	for category := range ec.patterns {
		categories = append(categories, category)
	}
	return categories
}
