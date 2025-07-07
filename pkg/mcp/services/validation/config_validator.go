package validation

import (
	"context"
	"regexp"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// NewConfigValidator creates a new unified configuration validator for backward compatibility
func NewConfigValidator() *UnifiedConfigValidator {
	return NewUnifiedConfigValidator()
}

// UnifiedConfigValidator implements the unified validation framework
type UnifiedConfigValidator struct {
	name    string
	version string
}

// NewUnifiedConfigValidator creates a new unified config validator
func NewUnifiedConfigValidator() *UnifiedConfigValidator {
	return &UnifiedConfigValidator{
		name:    "unified_config_validator",
		version: "1.0.0",
	}
}

// Validate implements core.Validator interface for unified validation framework
func (v *UnifiedConfigValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := core.NewNonGenericResult(v.name, v.version)

	switch input := data.(type) {
	case map[string]interface{}:
		if metadata, ok := input["metadata"]; ok {
			if metadataMap, ok := metadata.(map[string]interface{}); ok {
				if err := v.validateSessionData(metadataMap); err != nil {
					result.AddError(core.NewError("SESSION_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
				} else {
					result.AddSuggestion("Session metadata validation completed successfully")
				}
			}
		}

		if buildArgs, ok := input["build_args"]; ok {
			if argsData, ok := buildArgs.(*api.BuildArgs); ok {
				if err := v.validateBuildData(argsData); err != nil {
					result.AddError(core.NewError("BUILD_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
				} else {
					result.AddSuggestion("Build arguments validation completed successfully")
				}
			}
		}

		if workflow, ok := input["workflow"]; ok {
			if workflowData, ok := workflow.(*api.Workflow); ok {
				if err := v.validateWorkflowData(workflowData); err != nil {
					result.AddError(core.NewError("WORKFLOW_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
				} else {
					result.AddSuggestion("Workflow validation completed successfully")
				}
			}
		}

		if deployConfig, ok := input["deploy_config"]; ok {
			if configData, ok := deployConfig.(*services.DeployConfig); ok {
				if err := v.validateDeployData(configData); err != nil {
					result.AddError(core.NewError("DEPLOY_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
				} else {
					result.AddSuggestion("Deploy configuration validation completed successfully")
				}
			}
		}

	case *api.BuildArgs:
		if err := v.validateBuildData(input); err != nil {
			result.AddError(core.NewError("BUILD_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
		} else {
			result.AddSuggestion("Build arguments validation completed successfully")
		}

	case *api.Workflow:
		if err := v.validateWorkflowData(input); err != nil {
			result.AddError(core.NewError("WORKFLOW_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
		} else {
			result.AddSuggestion("Workflow validation completed successfully")
		}

	case *services.DeployConfig:
		if err := v.validateDeployData(input); err != nil {
			result.AddError(core.NewError("DEPLOY_VALIDATION_FAILED", err.Error(), core.ErrTypeValidation, core.SeverityHigh))
		} else {
			result.AddSuggestion("Deploy configuration validation completed successfully")
		}

	default:
		result.AddError(core.NewError("UNSUPPORTED_INPUT_TYPE", "Unsupported input type for config validation", core.ErrTypeValidation, core.SeverityMedium))
	}

	return result
}

// GetVersion returns the validator version
func (v *UnifiedConfigValidator) GetVersion() string {
	return "1.0.0"
}

// GetName returns the validator name
func (v *UnifiedConfigValidator) GetName() string {
	return "unified_config_validator"
}

// ConfigValidationData represents structured data for config validation
type ConfigValidationData struct {
	SessionMetadata map[string]interface{} `json:"session_metadata,omitempty"`
	BuildArgs       *api.BuildArgs         `json:"build_args,omitempty"`
	Workflow        *api.Workflow          `json:"workflow,omitempty"`
	DeployConfig    *services.DeployConfig `json:"deploy_config,omitempty"`
}

func (v *UnifiedConfigValidator) ValidateSession(metadata map[string]interface{}) error {
	return v.validateSessionData(metadata)
}

func (v *UnifiedConfigValidator) ValidateBuild(args *api.BuildArgs) error {
	return v.validateBuildData(args)
}

func (v *UnifiedConfigValidator) ValidateWorkflow(workflow *api.Workflow) error {
	return v.validateWorkflowData(workflow)
}

func (v *UnifiedConfigValidator) ValidateDeploy(config *services.DeployConfig) error {
	return v.validateDeployData(config)
}

// validateSessionData validates session metadata
func (v *UnifiedConfigValidator) validateSessionData(metadata map[string]interface{}) error {
	if metadata == nil {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("session metadata cannot be nil").Build()
	}

	if name, exists := metadata["name"]; exists {
		if nameStr, ok := name.(string); !ok || nameStr == "" || len(nameStr) > 100 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("session name must be a non-empty string with max 100 characters").
				Context("name", name).Build()
		}
	}

	if description, exists := metadata["description"]; exists {
		if descStr, ok := description.(string); !ok || len(descStr) > 500 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("session description must be a string with max 500 characters").
				Context("description", description).Build()
		}
	}

	if environment, exists := metadata["environment"]; exists {
		if envStr, ok := environment.(string); !ok {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("environment must be a string").
				Context("environment", environment).Build()
		} else if envStr != "" && envStr != "development" && envStr != "staging" && envStr != "production" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("environment must be one of: development, staging, production").
				Context("environment", environment).Build()
		}
	}

	return nil
}

// validateBuildData validates build arguments
func (v *UnifiedConfigValidator) validateBuildData(args *api.BuildArgs) error {
	if args == nil {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("build arguments cannot be nil").Build()
	}

	if args.SessionID == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("session ID is required for build").Build()
	}

	if args.ImageName == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("image name is required for build").Build()
	}

	if args.Context == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("build context is required").Build()
	}

	if len(args.ImageName) == 0 || len(args.ImageName) > 128 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("image name must be between 1 and 128 characters").
			Context("image_name", args.ImageName).Build()
	}

	dockerNamePattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9\-._/])*[a-z0-9]$`)
	if !dockerNamePattern.MatchString(args.ImageName) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("image name contains invalid characters").
			Context("image_name", args.ImageName).Build()
	}

	for i, tag := range args.Tags {
		if tag == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("build tag cannot be empty").
				Context("tag_index", i).
				Context("image_name", args.ImageName).Build()
		}
	}

	return nil
}

// validateWorkflowData validates workflow configuration
func (v *UnifiedConfigValidator) validateWorkflowData(workflow *api.Workflow) error {
	if workflow == nil {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("workflow cannot be nil").Build()
	}

	if workflow.Name == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("workflow name is required").Build()
	}

	if len(workflow.Name) > 100 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow name must not exceed 100 characters").
			Context("workflow_name", workflow.Name).Build()
	}

	namePattern := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9\-_\s]*$`)
	if !namePattern.MatchString(workflow.Name) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow name contains invalid characters").
			Context("workflow_name", workflow.Name).Build()
	}

	if len(workflow.Description) > 1000 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow description must not exceed 1000 characters").
			Context("workflow_name", workflow.Name).Build()
	}

	if len(workflow.Steps) == 0 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow must have at least one step").
			Context("workflow_name", workflow.Name).Build()
	}

	if len(workflow.Steps) > 50 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("workflow cannot have more than 50 steps").
			Context("workflow_name", workflow.Name).
			Context("steps_count", len(workflow.Steps)).Build()
	}

	stepPattern := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9\-_]*$`)
	for i, step := range workflow.Steps {
		if step.ID == "" {
			return errors.NewError().
				Code(errors.CodeMissingParameter).
				Type(errors.ErrTypeValidation).
				Message("workflow step ID is required").
				Context("step_index", i).
				Context("workflow_name", workflow.Name).Build()
		}

		if len(step.ID) > 50 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("workflow step ID must not exceed 50 characters").
				Context("step_index", i).
				Context("step_id", step.ID).
				Context("workflow_name", workflow.Name).Build()
		}

		if !stepPattern.MatchString(step.ID) {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("workflow step ID contains invalid characters").
				Context("step_index", i).
				Context("step_id", step.ID).
				Context("workflow_name", workflow.Name).Build()
		}

		if step.Name == "" {
			return errors.NewError().
				Code(errors.CodeMissingParameter).
				Type(errors.ErrTypeValidation).
				Message("workflow step name is required").
				Context("step_index", i).
				Context("step_id", step.ID).
				Context("workflow_name", workflow.Name).Build()
		}

		if len(step.Name) > 100 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("workflow step name must not exceed 100 characters").
				Context("step_index", i).
				Context("step_name", step.Name).
				Context("workflow_name", workflow.Name).Build()
		}

		if step.Tool == "" {
			return errors.NewError().
				Code(errors.CodeMissingParameter).
				Type(errors.ErrTypeValidation).
				Message("workflow step tool is required").
				Context("step_index", i).
				Context("step_name", step.Name).
				Context("workflow_name", workflow.Name).Build()
		}

		if len(step.Tool) > 50 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("workflow step tool must not exceed 50 characters").
				Context("step_index", i).
				Context("step_tool", step.Tool).
				Context("workflow_name", workflow.Name).Build()
		}

		if !stepPattern.MatchString(step.Tool) {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("workflow step tool contains invalid characters").
				Context("step_index", i).
				Context("step_tool", step.Tool).
				Context("workflow_name", workflow.Name).Build()
		}
	}

	return nil
}

// validateDeployData validates deployment configuration
func (v *UnifiedConfigValidator) validateDeployData(config *services.DeployConfig) error {
	if config == nil {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("deploy configuration cannot be nil").Build()
	}

	if config.Image == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("deployment image is required").Build()
	}

	imagePattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9\-._/])*[a-z0-9]$`)
	if !imagePattern.MatchString(config.Image) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("deployment image contains invalid characters").
			Context("image", config.Image).Build()
	}

	if config.Name == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Message("deployment name is required").Build()
	}

	if len(config.Name) > 63 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("deployment name must not exceed 63 characters").
			Context("deployment_name", config.Name).Build()
	}

	k8sNamePattern := regexp.MustCompile(`^[a-z]([a-z0-9\-])*[a-z0-9]$`)
	if !k8sNamePattern.MatchString(config.Name) {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("deployment name contains invalid characters").
			Context("deployment_name", config.Name).Build()
	}

	if config.Namespace != "" {
		if len(config.Namespace) > 63 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("deployment namespace must not exceed 63 characters").
				Context("namespace", config.Namespace).
				Context("deployment_name", config.Name).Build()
		}

		if !k8sNamePattern.MatchString(config.Namespace) {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("deployment namespace contains invalid characters").
				Context("namespace", config.Namespace).
				Context("deployment_name", config.Name).Build()
		}
	}

	if config.Replicas < 0 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("deployment replicas cannot be negative").
			Context("replicas", config.Replicas).
			Context("deployment_name", config.Name).Build()
	}

	if config.Replicas > 100 {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("deployment replicas cannot exceed 100").
			Context("replicas", config.Replicas).
			Context("deployment_name", config.Name).Build()
	}

	if config.Strategy != "" {
		validStrategies := []string{"RollingUpdate", "Recreate", "BlueGreen"}
		validStrategy := false
		for _, strategy := range validStrategies {
			if config.Strategy == strategy {
				validStrategy = true
				break
			}
		}

		if !validStrategy {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("invalid deployment strategy").
				Context("strategy", config.Strategy).
				Context("valid_strategies", validStrategies).
				Context("deployment_name", config.Name).Build()
		}
	}

	for i, port := range config.Ports {
		if port < 1 || port > 65535 {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("invalid port number").
				Context("port_index", i).
				Context("port", port).
				Context("deployment_name", config.Name).Build()
		}
	}

	return nil
}
