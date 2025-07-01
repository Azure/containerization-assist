package build

import (
	"context"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
	"github.com/rs/zerolog"
)

// ValidationAdapter bridges the old build package validation types with the new unified validation framework
type ValidationAdapter struct {
	logger            zerolog.Logger
	dockerValidator   *validators.DockerfileValidator
	contextValidator  *validators.ContextValidator
	imageValidator    *validators.ImageValidator
	securityValidator *validators.SecurityValidator
}

// NewValidationAdapter creates a new validation adapter
func NewValidationAdapter(logger zerolog.Logger) *ValidationAdapter {
	return &ValidationAdapter{
		logger:            logger,
		dockerValidator:   validators.NewDockerfileValidator(),
		contextValidator:  validators.NewContextValidator(),
		imageValidator:    validators.NewImageValidator(),
		securityValidator: validators.NewSecurityValidator(),
	}
}

// ConvertToUnifiedResult converts the new ValidationResult to the old format
func ConvertToUnifiedResult(unifiedResult *core.ValidationResult) *ValidationResult {
	result := &ValidationResult{
		Valid:    unifiedResult.Valid,
		Errors:   make([]ValidationError, 0, len(unifiedResult.Errors)),
		Warnings: make([]ValidationWarning, 0, len(unifiedResult.Warnings)),
		Info:     make([]string, 0),
	}

	// Convert errors
	for _, err := range unifiedResult.Errors {
		result.Errors = append(result.Errors, ValidationError{
			Line:    err.Line,
			Column:  err.Column,
			Message: err.Message,
			Rule:    err.Rule,
		})
	}

	// Convert warnings
	for _, warn := range unifiedResult.Warnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    warn.Line,
			Column:  warn.Column,
			Message: warn.Message,
			Rule:    warn.Rule,
		})
	}

	// Add suggestions as info
	result.Info = append(result.Info, unifiedResult.Suggestions...)

	return result
}

// ConvertToUnifiedOptions converts old ValidationOptions to new format
func ConvertToUnifiedOptions(oldOptions ValidationOptions) *core.ValidationOptions {
	options := core.NewValidationOptions()

	// Map severity to strict mode
	if oldOptions.Severity == "error" || oldOptions.Severity == "critical" {
		options.StrictMode = true
	}

	// Convert specific checks to skip rules
	if !oldOptions.CheckSecurity {
		options.SkipRules = append(options.SkipRules, "security")
	}
	if !oldOptions.CheckBestPractices {
		options.SkipRules = append(options.SkipRules, "best_practices")
	}
	if !oldOptions.CheckOptimization {
		options.SkipRules = append(options.SkipRules, "optimization")
	}

	// Add ignore rules
	options.SkipRules = append(options.SkipRules, oldOptions.IgnoreRules...)

	// Add context data
	if len(oldOptions.TrustedRegistries) > 0 {
		options.Context["trusted_registries"] = oldOptions.TrustedRegistries
	}

	return options
}

// ValidateDockerfile validates a Dockerfile using the unified framework
func (a *ValidationAdapter) ValidateDockerfile(content string, options ValidationOptions) (*ValidationResult, error) {
	ctx := context.Background()
	unifiedOptions := ConvertToUnifiedOptions(options)

	// Run validation
	unifiedResult := a.dockerValidator.Validate(ctx, content, unifiedOptions)

	// Convert back to old format
	return ConvertToUnifiedResult(unifiedResult), nil
}

// ValidateSecurity performs security validation using the unified framework
func (a *ValidationAdapter) ValidateSecurity(content string, trustedRegistries []string) (*ValidationResult, error) {
	ctx := context.Background()
	options := core.NewValidationOptions()
	options.Context["security_type"] = "dockerfile"
	options.Context["trusted_registries"] = trustedRegistries

	// Run security validation
	securityResult := a.securityValidator.Validate(ctx, content, options)

	// Convert to old format
	return ConvertToUnifiedResult(securityResult), nil
}

// ValidateContext validates build context using the unified framework
func (a *ValidationAdapter) ValidateContext(contextPath string, instructions []ContextInstruction) (*ValidationResult, error) {
	ctx := context.Background()
	options := core.NewValidationOptions()
	options.Context["validation_type"] = "dockerfile_context"

	// Prepare context data
	contextData := &validators.ContextData{
		ContextPath:  contextPath,
		Instructions: make([]validators.ContextInstruction, 0, len(instructions)),
	}

	for _, inst := range instructions {
		contextData.Instructions = append(contextData.Instructions, validators.ContextInstruction{
			Type:        inst.Type,
			Source:      inst.Source,
			Destination: inst.Destination,
			Line:        inst.Line,
			Options:     inst.Options,
		})
	}

	// Run validation
	unifiedResult := a.contextValidator.Validate(ctx, contextData, options)

	// Convert back to old format
	return ConvertToUnifiedResult(unifiedResult), nil
}

// ValidateImage validates image references using the unified framework
func (a *ValidationAdapter) ValidateImage(imageRef string, trustedRegistries []string) (*ValidationResult, error) {
	ctx := context.Background()
	options := core.NewValidationOptions()
	options.Context["validation_type"] = "image_reference"
	options.Context["trusted_registries"] = trustedRegistries

	// Run validation
	unifiedResult := a.imageValidator.Validate(ctx, imageRef, options)

	// Convert back to old format
	return ConvertToUnifiedResult(unifiedResult), nil
}

// UnifiedEnhancedSecurityValidator wraps the new security validator with policy support
type UnifiedEnhancedSecurityValidator struct {
	*SecurityValidator
	adapter *ValidationAdapter
}

// NewUnifiedEnhancedSecurityValidator creates an enhanced security validator using the unified framework
func NewUnifiedEnhancedSecurityValidator(logger zerolog.Logger, trustedRegistries []string) *UnifiedEnhancedSecurityValidator {
	baseValidator := NewSecurityValidator(logger, trustedRegistries)
	return &UnifiedEnhancedSecurityValidator{
		SecurityValidator: baseValidator,
		adapter:           NewValidationAdapter(logger),
	}
}

// Validate performs validation using the unified framework
func (v *UnifiedEnhancedSecurityValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	// Use the adapter to leverage unified validation
	return v.adapter.ValidateSecurity(content, v.trustedRegistries)
}

// ValidateWithEnhancedResults provides enhanced results using unified framework
func (v *UnifiedEnhancedSecurityValidator) ValidateWithEnhancedResults(content string, options ValidationOptions) (*DetailedSecurityResult, error) {
	// Get basic results
	baseResult, err := v.Validate(content, options)
	if err != nil {
		return nil, err
	}

	// Enhance with additional information
	enhancedResult := &DetailedSecurityResult{
		ValidationResult:  baseResult,
		ComplianceResults: []ComplianceResult{},
		SecurityScore:     v.calculateSecurityScore(baseResult),
		RiskLevel:         v.determineRiskLevel(baseResult),
		PolicyViolations:  []PolicyViolation{},
		Recommendations:   v.generateRecommendations(baseResult),
	}

	return enhancedResult, nil
}

// MigrateSyntaxValidator provides a migration path for syntax validation
func MigrateSyntaxValidator(content string, logger zerolog.Logger) (*ValidationResult, error) {
	adapter := NewValidationAdapter(logger)
	options := ValidationOptions{
		CheckSecurity:      false,
		CheckBestPractices: false,
		CheckOptimization:  false,
	}
	return adapter.ValidateDockerfile(content, options)
}

// MigrateContextValidator provides a migration path for context validation
func MigrateContextValidator(contextPath string, dockerfileContent string, logger zerolog.Logger) (*ValidationResult, error) {
	adapter := NewValidationAdapter(logger)

	// Parse Dockerfile to extract COPY/ADD instructions
	instructions := extractContextInstructions(dockerfileContent)

	return adapter.ValidateContext(contextPath, instructions)
}

// MigrateImageValidator provides a migration path for image validation
func MigrateImageValidator(dockerfileContent string, trustedRegistries []string, logger zerolog.Logger) (*ValidationResult, error) {
	adapter := NewValidationAdapter(logger)

	// Extract FROM instructions
	images := extractBaseImages(dockerfileContent)

	// Validate all images
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Info:     make([]string, 0),
	}

	for _, image := range images {
		imgResult, err := adapter.ValidateImage(image, trustedRegistries)
		if err != nil {
			return nil, err
		}

		// Merge results
		result.Valid = result.Valid && imgResult.Valid
		result.Errors = append(result.Errors, imgResult.Errors...)
		result.Warnings = append(result.Warnings, imgResult.Warnings...)
		result.Info = append(result.Info, imgResult.Info...)
	}

	return result, nil
}

// Helper functions for migration

func extractContextInstructions(dockerfileContent string) []ContextInstruction {
	instructions := []ContextInstruction{}
	lines := strings.Split(dockerfileContent, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "COPY") || strings.HasPrefix(upper, "ADD") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				instruction := ContextInstruction{
					Type:        parts[0],
					Source:      parts[1],
					Destination: parts[len(parts)-1],
					Line:        i + 1,
					Options:     make(map[string]string),
				}

				// Check for --from option
				if strings.Contains(trimmed, "--from=") {
					fromIdx := strings.Index(trimmed, "--from=")
					endIdx := strings.Index(trimmed[fromIdx:], " ")
					if endIdx > 0 {
						fromValue := trimmed[fromIdx+7 : fromIdx+endIdx]
						instruction.Options["from"] = fromValue
					}
				}

				instructions = append(instructions, instruction)
			}
		}
	}

	return instructions
}

func extractBaseImages(dockerfileContent string) []string {
	images := []string{}
	lines := strings.Split(dockerfileContent, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				image := parts[1]
				// Remove AS clause if present
				if asIdx := strings.Index(strings.ToUpper(trimmed), " AS "); asIdx > 0 {
					image = strings.Fields(trimmed[:asIdx])[1]
				}
				images = append(images, image)
			}
		}
	}

	return images
}

// ContextInstruction represents a COPY/ADD instruction for validation
type ContextInstruction struct {
	Type        string
	Source      string
	Destination string
	Line        int
	Options     map[string]string
}
