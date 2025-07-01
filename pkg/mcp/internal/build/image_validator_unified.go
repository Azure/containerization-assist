package build

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// UnifiedImageValidator provides image validation using the unified validation framework
type UnifiedImageValidator struct {
	logger            zerolog.Logger
	adapter           *ValidationAdapter
	trustedRegistries []string
}

// NewUnifiedImageValidator creates an image validator that uses the unified framework
func NewUnifiedImageValidator(logger zerolog.Logger, trustedRegistries []string) *UnifiedImageValidator {
	validator := &UnifiedImageValidator{
		logger:            logger.With().Str("component", "unified_image_validator").Logger(),
		adapter:           NewValidationAdapter(logger),
		trustedRegistries: trustedRegistries,
	}

	// Set trusted registries on the underlying validator
	if len(trustedRegistries) > 0 {
		validator.adapter.imageValidator.SetTrustedRegistries(trustedRegistries)
	}

	return validator
}

// Validate performs image validation using the unified framework
func (v *UnifiedImageValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().
		Str("mode", "unified").
		Int("trusted_registries", len(v.trustedRegistries)).
		Msg("Starting image validation with unified framework")

	// Extract base images from Dockerfile
	images := extractBaseImages(content)

	// Initialize result
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Info:     make([]string, 0),
	}

	// Validate each image
	for _, imageRef := range images {
		imgResult, err := v.adapter.ValidateImage(imageRef, v.trustedRegistries)
		if err != nil {
			v.logger.Error().Err(err).Str("image", imageRef).Msg("Failed to validate image")
			continue
		}

		// Merge results
		result.Valid = result.Valid && imgResult.Valid
		result.Errors = append(result.Errors, imgResult.Errors...)
		result.Warnings = append(result.Warnings, imgResult.Warnings...)
		result.Info = append(result.Info, imgResult.Info...)
	}

	// Add multi-stage build info if applicable
	if len(images) > 1 {
		result.Info = append(result.Info,
			fmt.Sprintf("Multi-stage build detected with %d stages", len(images)))
		v.validateMultiStageConsistency(images, result)
	}

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Int("images_validated", len(images)).
		Msg("Image validation completed")

	return result, nil
}

// validateMultiStageConsistency checks for consistency in multi-stage builds
func (v *UnifiedImageValidator) validateMultiStageConsistency(images []string, result *ValidationResult) {
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
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "Using many different base images may impact build cache efficiency",
			Rule:    "multi_stage_consistency",
		})
		result.Info = append(result.Info,
			"Consider using fewer distinct base images for better layer caching")
	}

	// Check for consistent registry usage
	registryMap := make(map[string]int)
	for _, img := range images {
		registry := extractRegistry(img)
		registryMap[registry]++
	}

	if len(registryMap) > 1 {
		result.Info = append(result.Info,
			"Multiple registries detected. Consider using a single registry for consistency")
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
	v.adapter.imageValidator.SetTrustedRegistries(registries)
}

// MigrateImageValidatorToUnified provides a drop-in replacement for the old ImageValidator
func MigrateImageValidatorToUnified(v *ImageValidator) *UnifiedImageValidator {
	return NewUnifiedImageValidator(v.logger, v.trustedRegistries)
}

// CreateImageValidatorWithUnified creates an image validator using the unified framework
// This function can be used as a drop-in replacement for NewImageValidator
func CreateImageValidatorWithUnified(logger zerolog.Logger, trustedRegistries []string) *UnifiedImageValidator {
	logger.Info().
		Int("trusted_registries", len(trustedRegistries)).
		Msg("Creating image validator with unified validation framework")
	return NewUnifiedImageValidator(logger, trustedRegistries)
}
