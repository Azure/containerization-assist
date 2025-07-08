package build

import (
	validationcore "github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/core"
)

// ConvertToUnifiedResult converts the core validation result to the build format
func ConvertToUnifiedResult(unifiedResult *validationcore.NonGenericResult) *BuildValidationResult {
	result := &core.BuildValidationResult{
		Valid: unifiedResult.Valid,
		Metadata: core.ValidationMetadata{
			ValidatorName:    "converter",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]string),
		},
		Details: make(map[string]interface{}),
	}

	// Convert errors
	for _, err := range unifiedResult.Errors {
		result.Errors = append(result.Errors, core.Error{
			Code:     err.Code,
			Message:  err.Message,
			Severity: core.SeverityHigh,
			Context:  make(map[string]string),
		})
	}

	// Convert warnings
	for _, warn := range unifiedResult.Warnings {
		result.Warnings = append(result.Warnings, core.Warning{
			Code:    warn.Code,
			Message: warn.Message,
			Context: make(map[string]string),
		})
	}

	// Convert suggestions to details
	if len(unifiedResult.Suggestions) > 0 {
		result.Details["suggestions"] = unifiedResult.Suggestions
	}

	// Return the unified result
	return result
}

// ConvertToUnifiedOptions converts old ValidationOptions to new format
func ConvertToUnifiedOptions(oldOptions ValidationOptions) *validationcore.ValidationOptions {
	options := validationcore.ValidationOptions{
		FailFast: false,
		Context:  make(map[string]interface{}),
	}

	// Map severity to fail fast mode
	if oldOptions.Severity == "error" || oldOptions.Severity == "critical" {
		options.FailFast = true
	}

	// Convert specific checks to skip rules
	if !oldOptions.CheckSecurity {
		options.SkipFields = append(options.SkipFields, "security")
	}
	if !oldOptions.CheckBestPractices {
		options.SkipFields = append(options.SkipFields, "best_practices")
	}
	if !oldOptions.CheckOptimization {
		options.SkipFields = append(options.SkipFields, "optimization")
	}

	// Add ignore rules
	options.SkipFields = append(options.SkipFields, oldOptions.IgnoreRules...)

	// Add context data
	if len(oldOptions.TrustedRegistries) > 0 {
		options.Context["trusted_registries"] = "configured"
	}

	return &options
}
