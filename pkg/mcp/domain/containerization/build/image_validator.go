package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/rs/zerolog"
)

// UnifiedImageValidator provides image validation using the unified validation framework
type UnifiedImageValidator struct {
	logger            zerolog.Logger
	imageValidator    *validators.ImageValidator
	trustedRegistries []string
}

// NewUnifiedImageValidator creates an image validator that uses the unified framework
func NewUnifiedImageValidator(logger zerolog.Logger, trustedRegistries []string) *UnifiedImageValidator {
	imageValidator := validators.NewImageValidator()

	// Set trusted registries on the underlying validator
	if len(trustedRegistries) > 0 {
		imageValidator.SetTrustedRegistries(trustedRegistries)
	}

	validator := &UnifiedImageValidator{
		logger:            logger.With().Str("component", "unified_image_validator").Logger(),
		imageValidator:    imageValidator,
		trustedRegistries: trustedRegistries,
	}

	return validator
}

// Validate performs image validation using the unified framework
func (v *UnifiedImageValidator) Validate(content string, options ValidationOptions) (*BuildValidationResult, error) {
	v.logger.Info().
		Str("mode", "unified").
		Int("trusted_registries", len(v.trustedRegistries)).
		Msg("Starting image validation with unified framework")

	// Extract base images from Dockerfile
	images := extractBaseImages(content)

	// Initialize result
	result := core.NewBuildResult("unified-image-validator", "1.0.0")

	// Validate each image using unified framework
	ctx := context.Background()
	coreOptions := ConvertToUnifiedOptions(options)

	for _, imageRef := range images {
		// Prepare image data for validation
		imageData := map[string]interface{}{
			"image_ref":          imageRef,
			"trusted_registries": v.trustedRegistries,
		}

		imgResult := v.imageValidator.Validate(ctx, imageData, coreOptions)

		// Convert and merge results
		convertedResult := ConvertToUnifiedResult(imgResult)
		result.Valid = result.Valid && convertedResult.Valid
		result.Errors = append(result.Errors, convertedResult.Errors...)
		result.Warnings = append(result.Warnings, convertedResult.Warnings...)
		result.Suggestions = append(result.Suggestions, convertedResult.Suggestions...)
	}

	// Add multi-stage build info if applicable
	if len(images) > 1 {
		result.AddSuggestion(
			fmt.Sprintf("Multi-stage build detected with %d stages", len(images)))
		v.validateMultiStageConsistency(images, result)
	}

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("github.com/Azure/container-kit/pkg/mcp/domain/errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Int("images_validated", len(images)).
		Msg("Image validation completed")

	return result, nil
}

// validateMultiStageConsistency checks for consistency in multi-stage builds
func (v *UnifiedImageValidator) validateMultiStageConsistency(images []string, result *BuildValidationResult) {
	// Track base image usage
	baseImageMap := make(map[string]int)
	for _, img := range images {
		// Extract base image without tag
		base := img
		if idx := strings.Index(base, ":"); idx > 0 {
			base = base[:idx]
		}
		baseImageMap[base]++
	}

	// Check for too many different base images
	if len(baseImageMap) > 3 {
		warning := core.NewWarning(
			"MULTI_STAGE_CONSISTENCY",
			"Using many different base images may impact build cache efficiency",
		)
		warning.Error.WithRule("multi_stage_consistency")
		result.AddWarning(warning)
		result.AddSuggestion("Consider using fewer distinct base images for better layer caching")
	}

	// Check for consistent registry usage
	registryMap := make(map[string]int)
	for _, img := range images {
		registry := extractRegistry(img)
		registryMap[registry]++
	}

	if len(registryMap) > 1 {
		result.AddSuggestion("Multiple registries detected. Consider using a single registry for consistency")
	}
}

// extractRegistry extracts the registry from an image reference
func extractRegistry(imageRef string) string {
	// Remove tag if present
	if idx := strings.Index(imageRef, ":"); idx > 0 {
		imageRef = imageRef[:idx]
	}

	// Check for registry
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		return parts[0]
	}

	// Default to Docker Hub
	return "docker.io"
}

// SetTrustedRegistries updates the trusted registries list
func (v *UnifiedImageValidator) SetTrustedRegistries(registries []string) {
	v.trustedRegistries = registries
	v.imageValidator.SetTrustedRegistries(registries)
}

// MigrateImageValidatorToUnified provides a drop-in replacement for the old ImageValidator
// Legacy function - kept for compatibility during migration period
func MigrateImageValidatorToUnified(logger zerolog.Logger, trustedRegistries []string) *UnifiedImageValidator {
	return NewUnifiedImageValidator(logger, trustedRegistries)
}

// CreateImageValidatorWithUnified creates an image validator using the unified framework
// This function can be used as a drop-in replacement for NewImageValidator
func CreateImageValidatorWithUnified(logger zerolog.Logger, trustedRegistries []string) *UnifiedImageValidator {
	logger.Info().
		Int("trusted_registries", len(trustedRegistries)).
		Msg("Creating image validator with unified validation framework")
	return NewUnifiedImageValidator(logger, trustedRegistries)
}
