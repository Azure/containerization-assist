package build

import (
	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// ConvertToUnifiedResult converts the core validation result to the build format
func ConvertToUnifiedResult(unifiedResult *core.NonGenericResult) *BuildValidationResult {
	result := &core.BuildResult{
		Valid:       unifiedResult.Valid,
		Errors:      unifiedResult.Errors,
		Warnings:    unifiedResult.Warnings,
		Suggestions: unifiedResult.Suggestions,
	}

	// Return the unified result
	return result
}

// ConvertToUnifiedOptions converts old ValidationOptions to new format
func ConvertToUnifiedOptions(oldOptions ValidationOptions) *core.ValidationOptions {
	options := core.ValidationOptions{
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
