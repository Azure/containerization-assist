package build

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"log/slog"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// BuildValidatorImpl handles build business logic validation and troubleshooting.
//
// This validator focuses on:
// 1. Build prerequisites validation (file existence, Docker daemon availability)
// 2. Troubleshooting and error analysis
// 3. Complex business rules that cannot be replaced by simple tag-based validation
//
// Simple argument validation (required fields, format validation) is handled
// by the tag-based validation system defined in the argument structs.
type BuildValidatorImpl struct {
	logger         *slog.Logger
	buildValidator core.Validator
}

// NewBuildValidator creates a new build validator with unified validation framework
func NewBuildValidator(logger *slog.Logger) *BuildValidatorImpl {
	return &BuildValidatorImpl{
		logger:         logger,
		buildValidator: validators.NewDockerfileValidator(),
	}
}

// UnifiedBuildValidator provides a unified validation interface
type UnifiedBuildValidator struct {
	impl *BuildValidatorImpl
}

// NewUnifiedBuildValidator creates a new unified build validator
func NewUnifiedBuildValidator(logger *slog.Logger) *UnifiedBuildValidator {
	return &UnifiedBuildValidator{
		impl: NewBuildValidator(logger),
	}
}

// ValidateUnified performs unified validation for build prerequisites
func (bv *BuildValidatorImpl) ValidateUnified(ctx context.Context, args interface{}) (*core.BuildResult, error) {
	bv.logger.Info("Starting unified build validation")

	// Create validation data based on input type
	var validationData map[string]interface{}

	switch v := args.(type) {
	case *AtomicBuildImageArgs:
		validationData = map[string]interface{}{
			"dockerfile_path": v.DockerfilePath,
			"image_name":      v.ImageName,
			"image_tag":       v.ImageTag,
			"build_context":   v.BuildContext,
			"registry_url":    v.RegistryURL,
			"platform":        v.Platform,
		}
	case map[string]interface{}:
		validationData = v
	default:
		result := core.NewBuildResult("build_validator", "1.0.0")
		result.AddError(core.NewError("INVALID_ARGUMENT_TYPE", "Invalid argument type for build validation", core.ErrTypeBuild, core.SeverityCritical))
		return result, errors.NewError().Messagef("invalid argument type: expected AtomicBuildImageArgs or map[string]interface{}, got %T", args).Build()
	}

	// Perform unified validation
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := bv.buildValidator.Validate(ctx, validationData, options)

	// Convert NonGenericResult to BuildResult
	result := core.NewBuildResult("build_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	bv.logger.Info("Unified build validation completed", "valid", result.Valid, "errors", len(result.Errors), "warnings", len(result.Warnings))

	return result, nil
}

// ValidateBuildPrerequisites validates that all prerequisites for building are met
func (bv *BuildValidatorImpl) ValidateBuildPrerequisites(dockerfilePath string, buildContext string) error {
	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return errors.Validationf("build_validator", "Dockerfile not found at %s", dockerfilePath)
	}
	// Check if build context exists
	if _, err := os.Stat(buildContext); os.IsNotExist(err) {
		return errors.Validationf("build_validator", "Build context directory not found at %s", buildContext)
	}
	// Check if Docker is available
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return errors.NewError().Messagef("Docker is not available. Please ensure Docker is installed and running").Build(

		// Helper method to check if Trivy is installed
		)
	}
	return nil
}

// AddPushTroubleshootingTips adds troubleshooting tips for push failures
func (bv *BuildValidatorImpl) AddPushTroubleshootingTips(err error, registryURL string) []string {
	tips := []string{}
	errorMsg := err.Error()
	if strings.Contains(errorMsg, "authentication required") ||
		strings.Contains(errorMsg, "unauthorized") {
		tips = append(tips,
			"Authentication failed. Run: docker login "+registryURL,
			"Check if your credentials are correct",
			"For private registries, ensure you have push permissions")
	}
	if strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "no such host") {
		tips = append(tips,
			"Cannot connect to registry. Check if the registry URL is correct",
			"Verify network connectivity to "+registryURL,
			"If using a private registry, ensure it's accessible from your network")
	}
	if strings.Contains(errorMsg, "denied") {
		tips = append(tips,
			"Access denied. Verify you have push permissions to this repository",
			"Check if the repository exists and you have write access",
			"For organization repositories, ensure your account is properly configured")
	}
	return tips
}

// AddTroubleshootingTips adds general troubleshooting tips based on the error
func (bv *BuildValidatorImpl) AddTroubleshootingTips(err error) []string {
	tips := []string{}
	if err == nil {
		return tips
	}
	errorMsg := err.Error()
	// Docker daemon issues
	if strings.Contains(errorMsg, "Cannot connect to the Docker daemon") {
		tips = append(tips,
			"Ensure Docker Desktop is running",
			"Try: sudo systemctl start docker (Linux)",
			"Check Docker daemon logs for errors")
	}
	// Dockerfile syntax errors
	if strings.Contains(errorMsg, "failed to parse Dockerfile") ||
		strings.Contains(errorMsg, "unknown instruction") {
		tips = append(tips,
			"Check Dockerfile syntax",
			"Ensure all instructions are valid",
			"Verify proper line endings (LF, not CRLF)")
	}
	// Build context issues
	if strings.Contains(errorMsg, "no such file or directory") {
		tips = append(tips,
			"Verify all files referenced in Dockerfile exist",
			"Check if build context includes all necessary files",
			"Ensure relative paths are correct from build context")
	}
	// Network issues
	if strings.Contains(errorMsg, "temporary failure resolving") ||
		strings.Contains(errorMsg, "network is unreachable") {
		tips = append(tips,
			"Check internet connectivity",
			"Verify DNS settings",
			"Try using a different DNS server (e.g., 8.8.8.8)")
	}
	// Space issues
	if strings.Contains(errorMsg, "no space left on device") {
		tips = append(tips,
			"Free up disk space",
			"Run: docker system prune -a",
			"Check available space with: df -h")
	}
	return tips
}

// ValidateArgs validates the atomic build image arguments using tag-based validation
// NOTE: This method now relies on the struct validation tags defined in AtomicBuildImageArgs.
// Manual validation has been replaced by the tag-based validation system.
// Business rules validation (like conditional requirements) should be handled separately.
func (bv *BuildValidatorImpl) ValidateArgs(args *AtomicBuildImageArgs) error {
	ctx := context.Background()
	result, err := bv.ValidateUnified(ctx, args)
	if err != nil {
		return err
	}

	if !result.Valid {
		// Convert validation errors to legacy error format for backward compatibility
		if len(result.Errors) > 0 {
			return errors.NewError().Messagef("validation failed: %s", result.Errors[0].Message).Build()
		}
	}

	// Business logic validation (conditional requirements)
	// This cannot be replaced by simple tags and must remain as business logic
	if args.PushAfterBuild && args.RegistryURL == "" {
		return errors.NewError().Messagef("registry URL is required when push_after_build is true").Build()
	}

	return nil
}

// ValidateArgsUnified validates the atomic build image arguments using unified validation framework
func (bv *BuildValidatorImpl) ValidateArgsUnified(ctx context.Context, args *AtomicBuildImageArgs) (*core.BuildResult, error) {
	return bv.ValidateUnified(ctx, args)
}

// Unified validation interface methods for UnifiedBuildValidator

// Validate implements the GenericValidator interface
func (ubv *UnifiedBuildValidator) Validate(ctx context.Context, data core.BuildValidationData, options *core.ValidationOptions) *core.BuildResult {
	result, err := ubv.impl.ValidateUnified(ctx, data)
	if err != nil {
		if result == nil {
			result = core.NewBuildResult("unified_build_validator", "1.0.0")
		}
		result.AddError(core.NewError("VALIDATION_ERROR", err.Error(), core.ErrTypeBuild, core.SeverityHigh))
	}
	return result
}

// GetName returns the validator name
func (ubv *UnifiedBuildValidator) GetName() string {
	return "unified_build_validator"
}

// GetVersion returns the validator version
func (ubv *UnifiedBuildValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (ubv *UnifiedBuildValidator) GetSupportedTypes() []string {
	return []string{"BuildValidationData", "AtomicBuildImageArgs", "map[string]interface{}"}
}

// ValidateWithTroubleshooting performs validation and provides troubleshooting tips
func (ubv *UnifiedBuildValidator) ValidateWithTroubleshooting(ctx context.Context, args *AtomicBuildImageArgs) (*core.BuildResult, []string) {
	result, err := ubv.impl.ValidateUnified(ctx, args)

	var tips []string
	if err != nil {
		tips = ubv.impl.AddTroubleshootingTips(err)
		if result == nil {
			result = core.NewBuildResult("unified_build_validator", "1.0.0")
		}
		result.AddError(core.NewError("VALIDATION_ERROR", err.Error(), core.ErrTypeBuild, core.SeverityHigh))
	}

	// Add suggestions from validation result to tips
	tips = append(tips, result.Suggestions...)

	return result, tips
}

// Migration helpers for backward compatibility

// MigrateBuildValidatorToUnified provides a drop-in replacement for legacy BuildValidator
func MigrateBuildValidatorToUnified(logger *slog.Logger) *UnifiedBuildValidator {
	return NewUnifiedBuildValidator(logger)
}
