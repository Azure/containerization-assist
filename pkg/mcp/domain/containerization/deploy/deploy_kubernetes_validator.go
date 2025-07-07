package deploy

import (
	"context"
	"fmt"
	"time"

	validationCore "github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

// convertValidationResult converts validationCore.NonGenericResult to validation.Result
func convertValidationResult(result *validationCore.NonGenericResult) *validation.Result {
	if result == nil {
		return &validation.Result{
			Valid: true,
			Score: 100,
		}
	}

	// Convert errors
	var errors []validation.Error
	for _, err := range result.Errors {
		errors = append(errors, validation.Error{
			Field:   err.Field,
			Message: err.Message,
			Code:    err.Code,
		})
	}

	// Convert warnings
	var warnings []validation.Warning
	for _, warn := range result.Warnings {
		warnings = append(warnings, validation.Warning{
			Field:   warn.Field,
			Message: warn.Message,
			Code:    warn.Code,
		})
	}

	return &validation.Result{
		Valid:    result.Valid,
		Errors:   errors,
		Warnings: warnings,
		Score:    int(result.Score), // Convert float64 to int
		Duration: result.Duration,
	}
}

// performHealthCheckUnified performs health check with unified validation
func (t *AtomicDeployKubernetesTool) performHealthCheckUnified(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) (*validation.Result, error) {
	// Create unified health validator
	healthValidator := validators.NewHealthValidator()

	// Perform original health check
	err := t.performHealthCheck(ctx, session, args, result, nil)

	// If we have health result, validate it with unified validator
	var validationResult *validationCore.NonGenericResult
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

	// Convert NonGenericResult to ValidationResult
	convertedResult := convertValidationResult(validationResult)
	return convertedResult, err
}

// handleHealthCheckErrorUnified handles health check errors with unified validation
func (t *AtomicDeployKubernetesTool) handleHealthCheckErrorUnified(ctx context.Context, err error, healthResult *kubernetes.HealthCheckResult, deployResult *AtomicDeployKubernetesResult) (*validationCore.NonGenericResult, error) {
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
	validationResult.AddError(&validationCore.Error{
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
func (t *AtomicDeployKubernetesTool) ValidateUnified(ctx context.Context, args interface{}) (*validationCore.NonGenericResult, error) {
	// Create deployment validator
	deploymentValidator := validators.NewDeploymentValidator()

	// Convert args to deployment args
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		result := &validationCore.NonGenericResult{
			Valid: false,
			Errors: []*validationCore.Error{{
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

		return result, errors.NewError().Messagef("invalid argument type for atomic_deploy_kubernetes: expected AtomicDeployKubernetesArgs, received %T", args).WithLocation(

		// Convert to validator format
		).Build()
	}

	validatorArgs := validators.DeploymentArgs{
		ImageRef:        deployArgs.ImageRef,
		SessionID:       deployArgs.SessionID,
		Namespace:       deployArgs.Namespace,
		AppName:         deployArgs.AppName,
		WaitTimeout:     deployArgs.WaitTimeout,
		SkipHealthCheck: deployArgs.SkipHealthCheck,
		ManifestPath:    deployArgs.ManifestPath,
		DryRun:          deployArgs.DryRun,
		Force:           deployArgs.Force,
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
			criticalError = errors.NewError().Messagef("validation error in field %s: %s (severity: %s)", err.Field, err.Message, err.Severity).Build(

			// Also perform original validation for backward compatibility
			)
			break
		}
	}

	originalErr := t.Validate(ctx, args)
	if originalErr != nil && criticalError == nil {
		criticalError = originalErr
	}

	return validationResult, criticalError
}

// updateSessionStateUnified updates session state with validation insights
func (t *AtomicDeployKubernetesTool) updateSessionStateUnified(session *core.SessionState, result *AtomicDeployKubernetesResult, validationResult *validationCore.NonGenericResult) error {
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
func ValidateDeploymentConfigUnified(config map[string]interface{}) *validationCore.NonGenericResult {
	return validators.ValidateDeploymentConfig(config)
}

// ValidateDeploymentArgsUnified validates deployment arguments using unified validation
func ValidateDeploymentArgsUnified(args AtomicDeployKubernetesArgs) *validationCore.NonGenericResult {
	deploymentArgs := validators.DeploymentArgs{
		ImageRef:        args.ImageRef,
		SessionID:       args.SessionID,
		Namespace:       args.Namespace,
		AppName:         args.AppName,
		WaitTimeout:     args.WaitTimeout,
		SkipHealthCheck: args.SkipHealthCheck,
		ManifestPath:    args.ManifestPath,
		DryRun:          args.DryRun,
		Force:           args.Force,
	}

	if args.WaitTimeout > 0 {
		deploymentArgs.Timeout = time.Duration(args.WaitTimeout) * time.Second
	}

	return validators.ValidateDeploymentArgs(deploymentArgs)
}

// ValidateDeploymentResultUnified validates deployment result using unified validation
func ValidateDeploymentResultUnified(result AtomicDeployKubernetesResult) *validationCore.NonGenericResult {
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

	// Populate error field based on failure analysis or success status
	if !result.Success && result.ConsolidatedFailureAnalysis != nil {
		errorMsg := fmt.Sprintf("Deployment failed at %s: %s",
			result.ConsolidatedFailureAnalysis.FailureStage,
			result.ConsolidatedFailureAnalysis.FailureType)
		if len(result.ConsolidatedFailureAnalysis.RootCauses) > 0 {
			errorMsg += fmt.Sprintf(" - Root causes: %v", result.ConsolidatedFailureAnalysis.RootCauses)
		}
		deploymentResult.Error = errors.NewError().Messagef("%s", errorMsg).WithLocation().Build()
	}

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
func ValidateDeploymentPipeline(config map[string]interface{}) *validationCore.NonGenericResult {
	ctx := context.Background()

	// Create multiple validators for different aspects
	deploymentValidator := validators.NewDeploymentValidator()
	manifestValidator := validators.NewKubernetesValidator()
	healthValidator := validators.NewHealthValidator()

	// Combined validation result
	overallResult := &validationCore.NonGenericResult{
		Valid:    true,
		Errors:   make([]*validationCore.Error, 0),
		Warnings: make([]*validationCore.Warning, 0),
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
		overallResult.AddError(&validationCore.Error{
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
