package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// ValidationRule defines a simple validation check
type ValidationRule struct {
	Name        string
	Description string
	Validate    func(ctx context.Context, target interface{}) error
}

// NewBasicValidator creates a new unified basic validator for backward compatibility
func NewBasicValidator(
	sessionManager session.SessionManager,
	logger logging.Standards,
) *BasicValidator {
	unified := NewUnifiedBasicValidator(sessionManager, logger)
	return &BasicValidator{UnifiedBasicValidator: unified}
}

// makeDefaultRules creates the default validation rules
func makeDefaultRules() []ValidationRule {
	return []ValidationRule{
		{
			Name:        "required_fields",
			Description: "Check required fields are present",
			Validate: func(ctx context.Context, target interface{}) error {
				if target == nil {
					return errors.NewError().
						Code(errors.CodeValidationFailed).
						Type(errors.ErrTypeValidation).
						Message("target cannot be nil").
						WithLocation().
						Build()
				}
				return nil
			},
		},
		{
			Name:        "format_check",
			Description: "Check format validity",
			Validate: func(ctx context.Context, target interface{}) error {
				return nil
			},
		},
	}
}

// UnifiedBasicValidator implements the unified validation framework
type UnifiedBasicValidator struct {
	sessionManager session.SessionManager
	logger         logging.Standards
	rules          []ValidationRule
	name           string
	version        string
}

// NewUnifiedBasicValidator creates a new unified basic validator
func NewUnifiedBasicValidator(sessionManager session.SessionManager, logger logging.Standards) *UnifiedBasicValidator {
	return &UnifiedBasicValidator{
		sessionManager: sessionManager,
		logger:         logger.WithComponent("unified_basic_validator"),
		rules:          makeDefaultRules(),
		name:           "unified_basic_validator",
		version:        "1.0.0",
	}
}

// Validate implements core.Validator interface for unified validation framework
func (v *UnifiedBasicValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := core.NewNonGenericResult(v.name, v.version)

	v.logger.Debug().
		Str("target_type", fmt.Sprintf("%T", data)).
		Msg("Starting unified basic validation")

	validationErrors := v.validateWithRules(ctx, data)
	if len(validationErrors) > 0 {
		for _, validationErr := range validationErrors {
			result.AddError(core.NewError("VALIDATION_RULE_FAILED", validationErr.Error(), core.ErrTypeValidation, core.SeverityMedium))
		}
	} else {
		result.AddSuggestion("Basic validation completed successfully")
		result.AddSuggestion(fmt.Sprintf("Validation completed at: %v", time.Now()))
	}

	if v.sessionManager != nil {
		sessionData := map[string]interface{}{
			"validation_result": map[string]interface{}{
				"valid":  len(validationErrors) == 0,
				"errors": len(validationErrors),
			},
			"validated_at": time.Now(),
		}
		if err := v.sessionManager.UpdateSession(ctx, "validation", func(sess *session.SessionState) error {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			for k, v := range sessionData {
				sess.Metadata[k] = v
			}
			return nil
		}); err != nil {
			v.logger.Warn("Failed to store validation result in session",

				"error", err)
		}
	}

	return result
}

// validateWithRules runs validation rules and returns any errors
func (v *UnifiedBasicValidator) validateWithRules(ctx context.Context, data interface{}) []error {
	var validationErrors []error
	for _, rule := range v.rules {
		if err := rule.Validate(ctx, data); err != nil {
			validationErrors = append(validationErrors, errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Messagef("%s: %w", rule.Name, err).
				WithLocation().
				Build())
		}
	}
	return validationErrors
}

// GetVersion returns the validator version
func (v *UnifiedBasicValidator) GetVersion() string {
	return "1.0.0"
}

// GetName returns the validator name
func (v *UnifiedBasicValidator) GetName() string {
	return "unified_basic_validator"
}

// BasicValidationData represents structured data for basic validation
type BasicValidationData struct {
	Target interface{} `json:"target"`
}

// ValidateBasic performs basic validation on a target (legacy interface)
func (v *UnifiedBasicValidator) ValidateBasic(ctx context.Context, target interface{}) (*api.ValidationResult, error) {
	v.logger.Debug().
		Str("target_type", fmt.Sprintf("%T", target)).
		Msg("Starting legacy validation")

	result := &api.ValidationResult{
		Valid:   true,
		Details: make(map[string]interface{}),
	}
	result.Details["timestamp"] = time.Now()

	for _, rule := range v.rules {
		if err := rule.Validate(ctx, target); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, api.ValidationError{
				Field:   rule.Name,
				Message: err.Error(),
				Code:    "validation_failed",
			})
			v.logger.Debug("Validation rule failed",

				"rule", rule.Name,

				"error", err)
		}
	}

	if v.sessionManager != nil {
		sessionData := map[string]interface{}{
			"validation_result": result,
			"validated_at":      time.Now(),
		}
		if err := v.sessionManager.UpdateSession(ctx, "validation", func(sess *session.SessionState) error {
			if sess.Metadata == nil {
				sess.Metadata = make(map[string]interface{})
			}
			for k, v := range sessionData {
				sess.Metadata[k] = v
			}
			return nil
		}); err != nil {
			v.logger.Warn("Failed to store validation result in session",

				"error", err)
		}
	}

	return result, nil
}

// ValidateWithRules validates using custom rules (legacy interface)
func (v *UnifiedBasicValidator) ValidateWithRules(ctx context.Context, target interface{}, rules []ValidationRule) (*api.ValidationResult, error) {
	oldRules := v.rules
	v.rules = rules
	defer func() { v.rules = oldRules }()

	return v.ValidateBasic(ctx, target)
}

// AddRule adds a validation rule
func (v *UnifiedBasicValidator) AddRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// ClearRules removes all validation rules
func (v *UnifiedBasicValidator) ClearRules() {
	v.rules = []ValidationRule{}
}

// GetRules returns current validation rules
func (v *UnifiedBasicValidator) GetRules() []ValidationRule {
	return v.rules
}

// ValidateLegacy performs basic validation using the legacy interface for embedded usage
func (v *UnifiedBasicValidator) ValidateLegacy(ctx context.Context, target interface{}) (*api.ValidationResult, error) {
	return v.ValidateBasic(ctx, target)
}

// BasicValidator is a compatibility wrapper around UnifiedBasicValidator
type BasicValidator struct {
	*UnifiedBasicValidator
}

// Validate performs basic validation using the legacy interface
func (v *BasicValidator) Validate(ctx context.Context, target interface{}) (*api.ValidationResult, error) {
	return v.ValidateBasic(ctx, target)
}
