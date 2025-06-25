package runtime

// ToolValidatorExtensions provides tool-specific validation utilities
// This file contains extensions and utilities for validation that are specific
// to tools and not part of the core validation types in validator.go

import (
	"context"
)

// ToolValidator provides tool-specific validation functionality
type ToolValidator struct {
	*BaseValidator
	toolName string
}

// NewToolValidator creates a new tool validator
func NewToolValidator(toolName string) *ToolValidator {
	return &ToolValidator{
		BaseValidator: NewBaseValidator("tool_validator_"+toolName, "1.0.0"),
		toolName:      toolName,
	}
}

// ValidateTool performs tool-specific validation
func (v *ToolValidator) ValidateTool(ctx context.Context, input interface{}, options ValidationOptions) (*ValidationResult, error) {
	result := v.CreateResult()

	// Tool-specific validation logic would go here
	// This could include checking tool arguments, dependencies, etc.

	result.CalculateScore()
	return result, nil
}

// GetToolName returns the validated tool name
func (v *ToolValidator) GetToolName() string {
	return v.toolName
}
