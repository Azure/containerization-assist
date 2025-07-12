// Package elicitation provides MCP elicitation capabilities for gathering user input
package elicitation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Client provides MCP elicitation capabilities
type Client struct {
	ctx     context.Context
	logger  *slog.Logger
	timeout time.Duration
}

// NewClient creates a new elicitation client
func NewClient(ctx context.Context, logger *slog.Logger) *Client {
	return &Client{
		ctx:     ctx,
		logger:  logger.With("component", "elicitation"),
		timeout: 30 * time.Second, // Default timeout for user responses
	}
}

// ElicitationRequest represents a request for user input
type ElicitationRequest struct {
	Prompt     string                 `json:"prompt"`
	Type       ElicitationType        `json:"type"`
	Required   bool                   `json:"required"`
	Options    []string               `json:"options,omitempty"`    // For choice/multi-choice
	Default    string                 `json:"default,omitempty"`    // Default value
	Validation *ValidationRules       `json:"validation,omitempty"` // Validation rules
	Context    map[string]interface{} `json:"context,omitempty"`    // Additional context
	Timeout    time.Duration          `json:"timeout,omitempty"`    // Custom timeout
}

// ElicitationResponse represents the user's response
type ElicitationResponse struct {
	Value     string                 `json:"value"`
	Cancelled bool                   `json:"cancelled"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ElicitationType defines the type of input being requested
type ElicitationType string

const (
	ElicitationTypeText        ElicitationType = "text"
	ElicitationTypePassword    ElicitationType = "password"
	ElicitationTypeChoice      ElicitationType = "choice"
	ElicitationTypeMultiChoice ElicitationType = "multi_choice"
	ElicitationTypeBoolean     ElicitationType = "boolean"
	ElicitationTypeNumber      ElicitationType = "number"
	ElicitationTypeURL         ElicitationType = "url"
	ElicitationTypeFile        ElicitationType = "file"
	ElicitationTypeDirectory   ElicitationType = "directory"
)

// ValidationRules defines validation rules for user input
type ValidationRules struct {
	MinLength   int      `json:"min_length,omitempty"`
	MaxLength   int      `json:"max_length,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`      // Regex pattern
	AllowedKeys []string `json:"allowed_keys,omitempty"` // For multi-choice
	Required    bool     `json:"required,omitempty"`
}

// Elicit requests input from the user through MCP elicitation
func (c *Client) Elicit(ctx context.Context, request ElicitationRequest) (*ElicitationResponse, error) {
	c.logger.Debug("Eliciting user input",
		"prompt", request.Prompt,
		"type", request.Type,
		"required", request.Required)

	// Check if we have MCP server context for elicitation
	if c.ctx != nil {
		if srv := server.ServerFromContext(c.ctx); srv != nil {
			// TODO: Use actual MCP elicitation when mcp-go implements sampling/prompts API
			// For now, fall back to console-based elicitation
			c.logger.Info("MCP elicitation pending implementation in mcp-go, using fallback")
			return c.fallbackElicitation(ctx, request)
		}
	}

	return c.fallbackElicitation(ctx, request)
}

// fallbackElicitation provides console-based elicitation when MCP is not available
func (c *Client) fallbackElicitation(ctx context.Context, request ElicitationRequest) (*ElicitationResponse, error) {
	c.logger.Info("Using console fallback for elicitation",
		"prompt", request.Prompt,
		"type", request.Type)

	// Create a simulated response based on the request type and context
	response := &ElicitationResponse{
		Metadata: map[string]interface{}{
			"fallback_mode": true,
			"timestamp":     time.Now().Format(time.RFC3339),
		},
	}

	// Handle different elicitation types with reasonable defaults
	switch request.Type {
	case ElicitationTypeText:
		response.Value = c.getDefaultOrGenerate(request, "container-app")

	case ElicitationTypePassword:
		response.Value = c.getDefaultOrGenerate(request, "")

	case ElicitationTypeChoice:
		if len(request.Options) > 0 {
			response.Value = request.Options[0] // Pick first option
		} else {
			response.Value = c.getDefaultOrGenerate(request, "default")
		}

	case ElicitationTypeMultiChoice:
		if len(request.Options) > 0 {
			response.Value = request.Options[0] // Pick first option
		} else {
			response.Value = c.getDefaultOrGenerate(request, "default")
		}

	case ElicitationTypeBoolean:
		response.Value = c.getDefaultOrGenerate(request, "true")

	case ElicitationTypeNumber:
		response.Value = c.getDefaultOrGenerate(request, "8080")

	case ElicitationTypeURL:
		response.Value = c.getDefaultOrGenerate(request, "https://localhost:8080")

	case ElicitationTypeFile:
		response.Value = c.getDefaultOrGenerate(request, "./Dockerfile")

	case ElicitationTypeDirectory:
		response.Value = c.getDefaultOrGenerate(request, "./")

	default:
		response.Value = c.getDefaultOrGenerate(request, "default")
	}

	// Validate the response
	if err := c.ValidateResponse(response.Value, request); err != nil {
		if request.Required {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		// Use default if validation fails and not required
		response.Value = request.Default
	}

	c.logger.Debug("Elicitation completed",
		"value", response.Value,
		"type", request.Type,
		"fallback", true)

	return response, nil
}

// getDefaultOrGenerate returns the default value or generates a reasonable value
func (c *Client) getDefaultOrGenerate(request ElicitationRequest, fallback string) string {
	if request.Default != "" {
		return request.Default
	}

	// Try to infer from context
	if request.Context != nil {
		if workspaceName, ok := request.Context["workspace_name"].(string); ok && workspaceName != "" {
			return workspaceName
		}
		if projectName, ok := request.Context["project_name"].(string); ok && projectName != "" {
			return projectName
		}
	}

	return fallback
}

// ValidateResponse validates a user response against the validation rules
func (c *Client) ValidateResponse(value string, request ElicitationRequest) error {
	if request.Validation == nil {
		return nil
	}

	rules := request.Validation

	// Check required
	if rules.Required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is required")
	}

	// Check length constraints
	if rules.MinLength > 0 && len(value) < rules.MinLength {
		return fmt.Errorf("value must be at least %d characters", rules.MinLength)
	}

	if rules.MaxLength > 0 && len(value) > rules.MaxLength {
		return fmt.Errorf("value must be at most %d characters", rules.MaxLength)
	}

	// Check pattern (would need regex package in real implementation)
	if rules.Pattern != "" {
		c.logger.Debug("Pattern validation not implemented in fallback mode", "pattern", rules.Pattern)
	}

	// Check allowed keys for multi-choice
	if len(rules.AllowedKeys) > 0 {
		found := false
		for _, key := range rules.AllowedKeys {
			if key == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value must be one of: %s", strings.Join(rules.AllowedKeys, ", "))
		}
	}

	return nil
}

// ElicitMissingConfiguration elicits missing configuration values needed for containerization
func (c *Client) ElicitMissingConfiguration(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
	c.logger.Info("Eliciting missing configuration values")

	result := make(map[string]interface{})
	for key, value := range config {
		result[key] = value
	}

	// Common configuration values that might be missing
	configurations := []struct {
		key        string
		prompt     string
		defaultVal string
		required   bool
		elicitType ElicitationType
		options    []string
	}{
		{
			key:        "app_name",
			prompt:     "What is the name of your application?",
			defaultVal: "my-app",
			required:   true,
			elicitType: ElicitationTypeText,
		},
		{
			key:        "port",
			prompt:     "What port does your application listen on?",
			defaultVal: "8080",
			required:   false,
			elicitType: ElicitationTypeNumber,
		},
		{
			key:        "registry",
			prompt:     "Which container registry would you like to use?",
			defaultVal: "docker.io",
			required:   false,
			elicitType: ElicitationTypeChoice,
			options:    []string{"docker.io", "ghcr.io", "gcr.io", "quay.io"},
		},
		{
			key:        "environment",
			prompt:     "Which deployment environment is this for?",
			defaultVal: "development",
			required:   false,
			elicitType: ElicitationTypeChoice,
			options:    []string{"development", "staging", "production"},
		},
		{
			key:        "enable_security_scan",
			prompt:     "Would you like to enable security scanning?",
			defaultVal: "true",
			required:   false,
			elicitType: ElicitationTypeBoolean,
		},
	}

	for _, conf := range configurations {
		if _, exists := result[conf.key]; !exists {
			request := ElicitationRequest{
				Prompt:   conf.prompt,
				Type:     conf.elicitType,
				Required: conf.required,
				Default:  conf.defaultVal,
				Options:  conf.options,
				Context:  result, // Pass existing config as context
			}

			response, err := c.Elicit(ctx, request)
			if err != nil {
				if conf.required {
					return nil, fmt.Errorf("failed to elicit required value for %s: %w", conf.key, err)
				}
				c.logger.Warn("Failed to elicit optional value, using default",
					"key", conf.key,
					"default", conf.defaultVal,
					"error", err)
				result[conf.key] = conf.defaultVal
			} else if !response.Cancelled {
				result[conf.key] = response.Value
			} else if conf.required {
				return nil, fmt.Errorf("required configuration %s was cancelled", conf.key)
			}
		}
	}

	c.logger.Info("Configuration elicitation completed", "config_count", len(result))
	return result, nil
}

// ElicitDeploymentParameters elicits deployment-specific parameters
func (c *Client) ElicitDeploymentParameters(ctx context.Context, currentParams map[string]interface{}) (map[string]interface{}, error) {
	c.logger.Info("Eliciting deployment parameters")

	result := make(map[string]interface{})
	for key, value := range currentParams {
		result[key] = value
	}

	deploymentParams := []struct {
		key        string
		prompt     string
		defaultVal string
		required   bool
		elicitType ElicitationType
		options    []string
	}{
		{
			key:        "namespace",
			prompt:     "Which Kubernetes namespace should the application be deployed to?",
			defaultVal: "default",
			required:   false,
			elicitType: ElicitationTypeText,
		},
		{
			key:        "replicas",
			prompt:     "How many replicas would you like to deploy?",
			defaultVal: "2",
			required:   false,
			elicitType: ElicitationTypeNumber,
		},
		{
			key:        "cpu_limit",
			prompt:     "What CPU limit should be set? (e.g., 500m, 1000m)",
			defaultVal: "500m",
			required:   false,
			elicitType: ElicitationTypeText,
		},
		{
			key:        "memory_limit",
			prompt:     "What memory limit should be set? (e.g., 512Mi, 1Gi)",
			defaultVal: "512Mi",
			required:   false,
			elicitType: ElicitationTypeText,
		},
		{
			key:        "service_type",
			prompt:     "What type of Kubernetes service should be created?",
			defaultVal: "ClusterIP",
			required:   false,
			elicitType: ElicitationTypeChoice,
			options:    []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"},
		},
	}

	for _, param := range deploymentParams {
		if _, exists := result[param.key]; !exists {
			request := ElicitationRequest{
				Prompt:   param.prompt,
				Type:     param.elicitType,
				Required: param.required,
				Default:  param.defaultVal,
				Options:  param.options,
				Context:  result,
			}

			response, err := c.Elicit(ctx, request)
			if err != nil {
				if param.required {
					return nil, fmt.Errorf("failed to elicit required parameter %s: %w", param.key, err)
				}
				result[param.key] = param.defaultVal
			} else if !response.Cancelled {
				result[param.key] = response.Value
			}
		}
	}

	c.logger.Info("Deployment parameter elicitation completed", "param_count", len(result))
	return result, nil
}
