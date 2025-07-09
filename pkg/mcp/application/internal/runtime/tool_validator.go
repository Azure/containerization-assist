package runtime

import (
	"context"
)

type ToolValidator struct {
	*BaseValidatorImpl
	toolName string
}

func NewToolValidator(toolName string) *ToolValidator {
	return &ToolValidator{
		BaseValidatorImpl: NewBaseValidator("tool_validator_"+toolName, "1.0.0"),
		toolName:          toolName,
	}
}
func (v *ToolValidator) Validate(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	return v.ValidateTool(ctx, input, options)
}
func (v *ToolValidator) ValidateTool(ctx context.Context, input interface{}, options ValidationOptions) (*RuntimeValidationResult, error) {
	result := v.BaseValidatorImpl.CreateResult()

	return result, nil
}
func (v *ToolValidator) GetToolName() string {
	return v.toolName
}
