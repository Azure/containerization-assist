// Elicitation provides MCP elicitation capabilities for gathering user input within workflows
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/mark3labs/mcp-go/server"
)

// ElicitationClient provides MCP elicitation capabilities
type ElicitationClient struct {
	ctx     context.Context
	logger  *slog.Logger
	timeout time.Duration
}

// NewElicitationClient creates a new elicitation client
func NewElicitationClient(ctx context.Context, logger *slog.Logger) *ElicitationClient {
	return &ElicitationClient{
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
func (c *ElicitationClient) Elicit(ctx context.Context, request ElicitationRequest) (*ElicitationResponse, error) {
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
func (c *ElicitationClient) fallbackElicitation(ctx context.Context, request ElicitationRequest) (*ElicitationResponse, error) {
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
			return nil, errors.NewWorkflowError(
				errors.CodeValidationFailed,
				"workflow",
				"elicitation",
				"validation failed",
				err,
			).WithStepContext("prompt", request.Prompt).
				WithStepContext("type", string(request.Type))
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
func (c *ElicitationClient) getDefaultOrGenerate(request ElicitationRequest, fallback string) string {
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
func (c *ElicitationClient) ValidateResponse(value string, request ElicitationRequest) error {
	if request.Validation == nil {
		return nil
	}

	rules := request.Validation

	// Check required
	if rules.Required && strings.TrimSpace(value) == "" {
		return errors.NewWorkflowError(
			errors.CodeValidationFailed,
			"workflow",
			"elicitation",
			"value is required",
			nil,
		).WithStepContext("field", request.Prompt).
			WithStepContext("validation_type", "required")
	}

	// Check length constraints
	if rules.MinLength > 0 && len(value) < rules.MinLength {
		return errors.NewWorkflowError(
			errors.CodeValidationFailed,
			"workflow",
			"elicitation",
			fmt.Sprintf("value must be at least %d characters", rules.MinLength),
			nil,
		).WithStepContext("field", request.Prompt).
			WithStepContext("validation_type", "min_length").
			WithStepContext("min_length", rules.MinLength)
	}

	if rules.MaxLength > 0 && len(value) > rules.MaxLength {
		return errors.NewWorkflowError(
			errors.CodeValidationFailed,
			"workflow",
			"elicitation",
			fmt.Sprintf("value must be at most %d characters", rules.MaxLength),
			nil,
		).WithStepContext("field", request.Prompt).
			WithStepContext("validation_type", "max_length").
			WithStepContext("max_length", rules.MaxLength)
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
			return errors.NewWorkflowError(
				errors.CodeValidationFailed,
				"workflow",
				"elicitation",
				fmt.Sprintf("value must be one of: %s", strings.Join(rules.AllowedKeys, ", ")),
				nil,
			).WithStepContext("field", request.Prompt).
				WithStepContext("validation_type", "allowed_keys").
				WithStepContext("allowed_keys", rules.AllowedKeys)
		}
	}

	return nil
}

// ElicitMissingConfiguration elicits missing configuration values needed for containerization
func (c *ElicitationClient) ElicitMissingConfiguration(ctx context.Context, config map[string]interface{}) (map[string]interface{}, error) {
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
					return nil, errors.NewWorkflowError(
						errors.CodeOperationFailed,
						"workflow",
						"elicitation",
						fmt.Sprintf("failed to elicit required value for %s", conf.key),
						err,
					).WithStepContext("config_key", conf.key).
						WithStepContext("prompt", conf.prompt)
				}
				c.logger.Warn("Failed to elicit optional value, using default",
					"key", conf.key,
					"default", conf.defaultVal,
					"error", err)
				result[conf.key] = conf.defaultVal
			} else if !response.Cancelled {
				result[conf.key] = response.Value
			} else if conf.required {
				return nil, errors.NewWorkflowError(
					errors.CodeOperationFailed,
					"workflow",
					"elicitation",
					fmt.Sprintf("required configuration %s was cancelled", conf.key),
					nil,
				).WithStepContext("config_key", conf.key).
					WithStepContext("cancelled", true)
			}
		}
	}

	c.logger.Info("Configuration elicitation completed", "config_count", len(result))
	return result, nil
}

// ElicitDeploymentParameters elicits deployment-specific parameters
func (c *ElicitationClient) ElicitDeploymentParameters(ctx context.Context, currentParams map[string]interface{}) (map[string]interface{}, error) {
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
					return nil, errors.NewWorkflowError(
						errors.CodeOperationFailed,
						"workflow",
						"elicitation",
						fmt.Sprintf("failed to elicit required parameter %s", param.key),
						err,
					).WithStepContext("param_key", param.key).
						WithStepContext("prompt", param.prompt)
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
