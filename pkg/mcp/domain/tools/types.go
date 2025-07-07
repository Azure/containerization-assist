package tools

import (
	"time"
)

// Legacy interfaces removed - use api.Tool directly

// RegistryOption configures tool registration.
type RegistryOption func(*RegistryConfig)

// RegistryConfig holds tool registration configuration.
type RegistryConfig struct {
	Namespace string
	Tags      []string
	Priority  int
	Enabled   bool
	Metadata  map[string]interface{}
	Timeout   time.Duration
}

// WithNamespace sets the tool namespace.
func WithNamespace(namespace string) RegistryOption {
	return func(c *RegistryConfig) { c.Namespace = namespace }
}

// WithTags adds tags to the tool.
func WithTags(tags ...string) RegistryOption {
	return func(c *RegistryConfig) { c.Tags = append(c.Tags, tags...) }
}

// WithEnabled sets whether the tool is enabled.
func WithEnabled(enabled bool) RegistryOption {
	return func(c *RegistryConfig) { c.Enabled = enabled }
}

// WithPriority sets the tool priority.
func WithPriority(priority int) RegistryOption {
	return func(c *RegistryConfig) { c.Priority = priority }
}

// WithTimeout sets the execution timeout.
func WithTimeout(timeout time.Duration) RegistryOption {
	return func(c *RegistryConfig) { c.Timeout = timeout }
}
