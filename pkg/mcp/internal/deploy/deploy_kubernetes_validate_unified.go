package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	validationCore "github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
)

// performHealthCheckUnified performs health check with unified validation
func (t *AtomicDeployKubernetesTool) performHealthCheckUnified(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) (*validationCore.ValidationResult, error) {
	// Create unified health validator
	healthValidator := validators.NewHealthValidator()

	// Perform original health check
	err := t.performHealthCheck(ctx, session, args, result, nil)

	// If we have health result, validate it with unified validator
	var validationResult *validationCore.ValidationResult
	if result.HealthResult != nil {
		options := validationCore.NewValidationOptions()
		validationResult = healthValidator.Validate(ctx, result.HealthResult, options)

		// Add validation insights to result metadata
		if result.HealthResult != nil {
			if result.HealthResult.Context == nil {
				result.HealthResult.Context = make(map[string]interface{})
			}
			result.HealthResult.Context["validation_result"] = validationResult
			result.HealthResult.Context["validation_score"] = validationResult.Score
			result.HealthResult.Context["risk_level"] = validationResult.RiskLevel

			// Add validation errors/warnings as context
			if len(validationResult.Errors) > 0 {
				result.HealthResult.Context["validation_errors"] = len(validationResult.Errors)
			}
			if len(validationResult.Warnings) > 0 {
				result.HealthResult.Context["validation_warnings"] = len(validationResult.Warnings)
			}
		}
	}

	return validationResult, err
}

// handleHealthCheckErrorUnified handles health check errors with unified validation
func (t *AtomicDeployKubernetesTool) handleHealthCheckErrorUnified(ctx context.Context, err error, healthResult *kubernetes.HealthCheckResult, deployResult *AtomicDeployKubernetesResult) (*validationCore.ValidationResult, error) {
	// Create error validation data
	errorData := map[string]interface{}{
		"error":         err.Error(),
		"error_type":    fmt.Sprintf("%T", err),
		"health_result": healthResult,
		"timestamp":     time.Now(),
	}

	// Validate error context
	healthValidator := validators.NewHealthValidator()
	options := validationCore.NewValidationOptions()
	validationResult := healthValidator.Validate(ctx, errorData, options)

	// Add error-specific validation
	validationResult.AddError(&validationCore.ValidationError{
		Code:     "HEALTH_CHECK_FAILED",
		Message:  fmt.Sprintf("Health check failed: %v", err),
		Type:     validationCore.ErrTypeSystem,
		Severity: validationCore.SeverityHigh,
	})

	// Call original error handler
	originalErr := t.handleHealthCheckError(ctx, err, healthResult, deployResult)

	return validationResult, originalErr
}

// ValidateUnified validates tool arguments using unified validation
func (t *AtomicDeployKubernetesTool) ValidateUnified(ctx context.Context, args interface{}) (*validationCore.ValidationResult, error) {
	// Create deployment validator
	deploymentValidator := validators.NewDeploymentValidator()

	// Convert args to deployment args
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		result := &validationCore.ValidationResult{
			Valid: false,
			Errors: []*validationCore.ValidationError{{
				Code:     "INVALID_ARGUMENT_TYPE",
				Message:  fmt.Sprintf("Expected AtomicDeployKubernetesArgs, got %T", args),
				Type:     validationCore.ErrTypeValidation,
				Severity: validationCore.SeverityCritical,
			}},
			Metadata: validationCore.ValidationMetadata{
				ValidatedAt:      time.Now(),
				ValidatorName:    "deployment-args-validator",
				ValidatorVersion: "1.0.0",
			},
		}

		return result, utils.NewWithData("invalid_arguments", "Invalid argument type for atomic_deploy_kubernetes", map[string]interface{}{
			"expected": "AtomicDeployKubernetesArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Convert to validator format
	validatorArgs := validators.DeploymentArgs{
		ImageRef:        deployArgs.ImageRef,
		SessionID:       deployArgs.SessionID,
		Namespace:       deployArgs.Namespace,
		AppName:         deployArgs.AppName,
		WaitTimeout:     deployArgs.WaitTimeout,
		SkipHealthCheck: false, // TODO: Add field to AtomicDeployKubernetesArgs
		ManifestPath:    "",    // TODO: Add field to AtomicDeployKubernetesArgs
		DryRun:          deployArgs.DryRun,
		Force:           false, // TODO: Add field to AtomicDeployKubernetesArgs
	}

	// If WaitTimeout is specified, convert to Duration
	if deployArgs.WaitTimeout > 0 {
		validatorArgs.Timeout = time.Duration(deployArgs.WaitTimeout) * time.Second
	}

	// Perform unified validation
	options := validationCore.NewValidationOptions().WithStrictMode(false)
	validationResult := deploymentValidator.Validate(ctx, validatorArgs, options)

	// Check for critical errors that should prevent execution
	var criticalError error
	for _, err := range validationResult.Errors {
		if err.Severity == validationCore.SeverityCritical {
			criticalError = utils.NewWithData("VALIDATION_ERROR", err.Message, map[string]interface{}{
				"field":    err.Field,
				"severity": err.Severity,
			})
			break
		}
	}

	// Also perform original validation for backward compatibility
	originalErr := t.Validate(ctx, args)
	if originalErr != nil && criticalError == nil {
		criticalError = originalErr
	}

	return validationResult, criticalError
}

// updateSessionStateUnified updates session state with validation insights
func (t *AtomicDeployKubernetesTool) updateSessionStateUnified(session *core.SessionState, result *AtomicDeployKubernetesResult, validationResult *validationCore.ValidationResult) error {
	// Perform original session update
	err := t.updateSessionState(session, result)

	// Add unified validation insights to session metadata
	if validationResult != nil {
		if session.Metadata == nil {
			session.Metadata = make(map[string]interface{})
		}

		// Add validation metrics
		session.Metadata["validation_score"] = validationResult.Score
		session.Metadata["validation_risk_level"] = validationResult.RiskLevel
		session.Metadata["validation_error_count"] = len(validationResult.Errors)
		session.Metadata["validation_warning_count"] = len(validationResult.Warnings)
		session.Metadata["validation_duration"] = validationResult.Duration.String()

		// Add validation summary
		validationSummary := map[string]interface{}{
			"valid":             validationResult.Valid,
			"validator_name":    validationResult.Metadata.ValidatorName,
			"validator_version": validationResult.Metadata.ValidatorVersion,
			"validated_at":      validationResult.Metadata.ValidatedAt,
		}

		if len(validationResult.Suggestions) > 0 {
			validationSummary["suggestions"] = validationResult.Suggestions
		}

		session.Metadata["validation_summary"] = validationSummary

		// Add error details if present
		if len(validationResult.Errors) > 0 {
			errorSummary := make([]map[string]interface{}, 0, len(validationResult.Errors))
			for _, validationError := range validationResult.Errors {
				errorSummary = append(errorSummary, map[string]interface{}{
					"code":     validationError.Code,
					"message":  validationError.Message,
					"severity": validationError.Severity,
					"field":    validationError.Field,
				})
			}
			session.Metadata["validation_errors"] = errorSummary
		}

		// Update session timestamp
		session.UpdatedAt = time.Now()
	}

	return err
}

// ValidateDeploymentConfigUnified validates deployment configuration using unified validation
func ValidateDeploymentConfigUnified(config map[string]interface{}) *validationCore.ValidationResult {
	return validators.ValidateDeploymentConfig(config)
}

// ValidateDeploymentArgsUnified validates deployment arguments using unified validation
func ValidateDeploymentArgsUnified(args AtomicDeployKubernetesArgs) *validationCore.ValidationResult {
	deploymentArgs := validators.DeploymentArgs{
		ImageRef:        args.ImageRef,
		SessionID:       args.SessionID,
		Namespace:       args.Namespace,
		AppName:         args.AppName,
		WaitTimeout:     args.WaitTimeout,
		SkipHealthCheck: false, // TODO: Add field to AtomicDeployKubernetesArgs
		ManifestPath:    "",    // TODO: Add field to AtomicDeployKubernetesArgs
		DryRun:          args.DryRun,
		Force:           false, // TODO: Add field to AtomicDeployKubernetesArgs
	}

	if args.WaitTimeout > 0 {
		deploymentArgs.Timeout = time.Duration(args.WaitTimeout) * time.Second
	}

	return validators.ValidateDeploymentArgs(deploymentArgs)
}

// ValidateDeploymentResultUnified validates deployment result using unified validation
func ValidateDeploymentResultUnified(result AtomicDeployKubernetesResult) *validationCore.ValidationResult {
	deploymentResult := validators.DeploymentResult{
		Success:             result.Success,
		ImageRef:            result.ImageRef,
		Namespace:           result.Namespace,
		AppName:             result.AppName,
		TotalDuration:       result.TotalDuration,
		DeploymentDuration:  result.DeploymentDuration,
		HealthCheckDuration: result.HealthCheckDuration,
		GenerationDuration:  result.GenerationDuration,
		HealthResult:        result.HealthResult,
	}

	// TODO: Add Error field to AtomicDeployKubernetesResult or use existing error handling
	// if result.Error != "" {
	//	deploymentResult.Error = fmt.Errorf("%s", result.Error)
	// }

	return validators.ValidateDeploymentResult(deploymentResult)
}

// GetDeploymentValidationMetrics returns validation metrics for deployment
func GetDeploymentValidationMetrics(result AtomicDeployKubernetesResult) map[string]interface{} {
	validationResult := ValidateDeploymentResultUnified(result)

	metrics := map[string]interface{}{
		"validation_score":       validationResult.Score,
		"risk_level":             validationResult.RiskLevel,
		"error_count":            len(validationResult.Errors),
		"warning_count":          len(validationResult.Warnings),
		"suggestion_count":       len(validationResult.Suggestions),
		"validation_duration":    validationResult.Duration.String(),
		"deployment_success":     result.Success,
		"total_duration_seconds": result.TotalDuration.Seconds(),
	}

	// Add error breakdown by severity
	if len(validationResult.Errors) > 0 {
		severityCount := make(map[string]int)
		for _, err := range validationResult.Errors {
			severityCount[string(err.Severity)]++
		}
		metrics["errors_by_severity"] = severityCount
	}

	// Add health metrics if available
	if result.HealthResult != nil {
		metrics["health_check_duration_seconds"] = result.HealthCheckDuration.Seconds()

		metrics["health_check_success"] = result.HealthResult.Success
		metrics["pod_count"] = len(result.HealthResult.Pods)
		metrics["service_count"] = len(result.HealthResult.Services)

		// Calculate health ratio
		if len(result.HealthResult.Pods) > 0 {
			readyPods := 0
			for _, pod := range result.HealthResult.Pods {
				if pod.Ready {
					readyPods++
				}
			}
			metrics["health_ratio"] = float64(readyPods) / float64(len(result.HealthResult.Pods))
		}
	}

	return metrics
}

// ValidateDeploymentPipeline validates an entire deployment pipeline
func ValidateDeploymentPipeline(config map[string]interface{}) *validationCore.ValidationResult {
	ctx := context.Background()

	// Create multiple validators for different aspects
	deploymentValidator := validators.NewDeploymentValidator()
	manifestValidator := validators.NewKubernetesValidator()
	healthValidator := validators.NewHealthValidator()

	// Combined validation result
	overallResult := &validationCore.ValidationResult{
		Valid:    true,
		Errors:   make([]*validationCore.ValidationError, 0),
		Warnings: make([]*validationCore.ValidationWarning, 0),
		Metadata: validationCore.ValidationMetadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "deployment-pipeline-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	options := validationCore.NewValidationOptions()

	// Validate deployment configuration
	deploymentResult := deploymentValidator.Validate(ctx, config, options)
	overallResult.Merge(deploymentResult)

	// Validate manifests if provided
	if manifestData, exists := config["manifests"]; exists {
		manifestResult := manifestValidator.Validate(ctx, manifestData, options)
		overallResult.Merge(manifestResult)
	}

	// Validate health configuration if provided
	if healthConfig, exists := config["health_check"]; exists {
		healthResult := healthValidator.Validate(ctx, healthConfig, options)
		overallResult.Merge(healthResult)
	}

	// Add pipeline-specific validations
	if _, hasImageRef := config["image_ref"]; !hasImageRef {
		overallResult.AddError(&validationCore.ValidationError{
			Code:     "MISSING_IMAGE_REF",
			Message:  "Deployment pipeline requires image reference",
			Type:     validationCore.ErrTypeValidation,
			Severity: validationCore.SeverityCritical,
			Field:    "image_ref",
		})
	}

	// Calculate overall score based on component scores
	scores := []float64{deploymentResult.Score}
	if manifestData, exists := config["manifests"]; exists && manifestData != nil {
		manifestResult := manifestValidator.Validate(ctx, manifestData, options)
		scores = append(scores, manifestResult.Score)
	}
	if healthConfig, exists := config["health_check"]; exists && healthConfig != nil {
		healthResult := healthValidator.Validate(ctx, healthConfig, options)
		scores = append(scores, healthResult.Score)
	}

	// Average the scores
	var totalScore float64
	for _, score := range scores {
		totalScore += score
	}
	overallResult.Score = totalScore / float64(len(scores))

	// Set risk level based on errors and score
	if len(overallResult.Errors) >= 3 || overallResult.Score < 50 {
		overallResult.RiskLevel = "high"
	} else if len(overallResult.Errors) >= 1 || overallResult.Score < 80 {
		overallResult.RiskLevel = "medium"
	} else {
		overallResult.RiskLevel = "low"
	}

	return overallResult
}
