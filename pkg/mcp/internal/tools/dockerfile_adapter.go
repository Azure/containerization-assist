package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/dockerfile"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// DockerfileAdapter integrates the refactored Dockerfile validation modules
type DockerfileAdapter struct {
	syntaxValidator   *dockerfile.SyntaxValidator
	securityValidator *dockerfile.SecurityValidator
	imageValidator    *dockerfile.ImageValidator
	contextValidator  *dockerfile.ContextValidator
	logger            zerolog.Logger
}

// NewDockerfileAdapter creates a new Dockerfile adapter
func NewDockerfileAdapter(logger zerolog.Logger) *DockerfileAdapter {
	return &DockerfileAdapter{
		syntaxValidator:   dockerfile.NewSyntaxValidator(logger),
		securityValidator: dockerfile.NewSecurityValidator(logger, []string{}),
		imageValidator:    dockerfile.NewImageValidator(logger, []string{}),
		contextValidator:  dockerfile.NewContextValidator(logger),
		logger:            logger.With().Str("component", "dockerfile_adapter").Logger(),
	}
}

// ValidateWithModules performs validation using the refactored modules
func (a *DockerfileAdapter) ValidateWithModules(ctx context.Context, dockerfileContent string, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	startTime := time.Now()

	a.logger.Info().
		Bool("check_security", args.CheckSecurity).
		Bool("check_optimization", args.CheckOptimization).
		Bool("check_best_practices", args.CheckBestPractices).
		Msg("Starting modular Dockerfile validation")

	// Create validation options
	options := dockerfile.ValidationOptions{
		UseHadolint:        args.UseHadolint,
		Severity:           args.Severity,
		IgnoreRules:        args.IgnoreRules,
		TrustedRegistries:  args.TrustedRegistries,
		CheckSecurity:      args.CheckSecurity,
		CheckOptimization:  args.CheckOptimization,
		CheckBestPractices: args.CheckBestPractices,
	}

	// Create validation context
	validationCtx := dockerfile.ValidationContext{
		DockerfilePath:    args.DockerfilePath,
		DockerfileContent: dockerfileContent,
		SessionID:         args.SessionID,
		Options:           options,
	}

	// Initialize result
	result := &AtomicValidateDockerfileResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_validate_dockerfile", args.SessionID, args.DryRun),
		BaseAIContextResult: NewBaseAIContextResult("validate", false, 0),
		SessionID:           args.SessionID,
		DockerfilePath:      args.DockerfilePath,
		ValidationContext:   make(map[string]interface{}),
		Errors:              make([]DockerfileValidationError, 0),
		Warnings:            make([]DockerfileValidationWarning, 0),
		SecurityIssues:      make([]DockerfileSecurityIssue, 0),
		OptimizationTips:    make([]OptimizationTip, 0),
	}

	lines := strings.Split(dockerfileContent, "\n")

	// Step 1: Syntax validation
	syntaxResult, err := a.syntaxValidator.Validate(dockerfileContent, options)
	if err != nil {
		a.logger.Error().Err(err).Msg("Syntax validation failed")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Merge syntax results
	a.mergeSyntaxResults(result, syntaxResult)

	// Perform syntax analysis
	if syntaxAnalysis, ok := a.syntaxValidator.Analyze(lines, validationCtx).(dockerfile.SyntaxAnalysis); ok {
		result.ValidationContext["syntax_analysis"] = syntaxAnalysis
	}

	// Step 2: Security validation
	if args.CheckSecurity {
		// Update security validator with trusted registries
		a.securityValidator = dockerfile.NewSecurityValidator(a.logger, args.TrustedRegistries)

		securityResult, err := a.securityValidator.Validate(dockerfileContent, options)
		if err != nil {
			a.logger.Error().Err(err).Msg("Security validation failed")
		} else {
			a.mergeSecurityResults(result, securityResult)
		}

		// Perform security analysis
		if securityAnalysis, ok := a.securityValidator.Analyze(lines, validationCtx).(dockerfile.SecurityAnalysis); ok {
			result.SecurityAnalysis = convertSecurityAnalysis(securityAnalysis)
		}
	}

	// Step 3: Image validation
	a.imageValidator = dockerfile.NewImageValidator(a.logger, args.TrustedRegistries)
	imageResult, err := a.imageValidator.Validate(dockerfileContent, options)
	if err != nil {
		a.logger.Error().Err(err).Msg("Image validation failed")
	} else {
		a.mergeImageResults(result, imageResult)
	}

	// Perform image analysis
	if imageAnalysis, ok := a.imageValidator.Analyze(lines, validationCtx).(dockerfile.BaseImageAnalysis); ok {
		result.BaseImageAnalysis = convertBaseImageAnalysis(imageAnalysis)
	}

	// Step 4: Context validation
	contextResult, err := a.contextValidator.Validate(dockerfileContent, options)
	if err != nil {
		a.logger.Error().Err(err).Msg("Context validation failed")
	} else {
		a.mergeContextResults(result, contextResult)
	}

	// Perform context analysis
	if contextAnalysis, ok := a.contextValidator.Analyze(lines, validationCtx).(dockerfile.ContextAnalysis); ok {
		result.ValidationContext["context_analysis"] = contextAnalysis

		// Generate layer analysis from context analysis
		result.LayerAnalysis = a.generateLayerAnalysis(contextAnalysis, lines)
	}

	// Step 5: Generate optimization tips if requested
	if args.CheckOptimization {
		a.generateOptimizationTips(result, lines)
	}

	// Calculate final validation state
	result.IsValid = len(result.Errors) == 0 && len(result.SecurityIssues) == 0
	result.TotalIssues = len(result.Errors) + len(result.Warnings) + len(result.SecurityIssues)
	result.ValidationScore = a.calculateValidationScore(result)
	result.ValidatorUsed = "modular"

	// Generate suggestions
	if args.IncludeSuggestions {
		a.generateSuggestions(result)
	}

	// Generate fixes if requested
	if args.GenerateFixes && !result.IsValid {
		corrected, fixes := a.generateCorrectedDockerfile(dockerfileContent, result)
		result.CorrectedDockerfile = corrected
		result.FixesApplied = fixes
	}

	result.Duration = time.Since(startTime)
	result.BaseAIContextResult.IsSuccessful = result.IsValid
	result.BaseAIContextResult.Duration = result.Duration

	a.logger.Info().
		Bool("is_valid", result.IsValid).
		Int("total_issues", result.TotalIssues).
		Int("validation_score", result.ValidationScore).
		Dur("duration", result.Duration).
		Msg("Modular Dockerfile validation completed")

	return result, nil
}

// Merge methods for combining results from different validators

func (a *DockerfileAdapter) mergeSyntaxResults(result *AtomicValidateDockerfileResult, syntaxResult *dockerfile.ValidationResult) {
	// Convert errors
	for _, err := range syntaxResult.Errors {
		result.Errors = append(result.Errors, DockerfileValidationError{
			Type:        err.Type,
			Line:        err.Line,
			Column:      err.Column,
			Rule:        err.Rule,
			Message:     err.Message,
			Instruction: err.Instruction,
			Severity:    err.Severity,
			Fix:         err.Fix,
		})
	}

	// Convert warnings
	for _, warn := range syntaxResult.Warnings {
		result.Warnings = append(result.Warnings, DockerfileValidationWarning{
			Type:       warn.Type,
			Line:       warn.Line,
			Rule:       warn.Rule,
			Message:    warn.Message,
			Suggestion: warn.Suggestion,
			Impact:     warn.Impact,
		})
	}

	// Merge suggestions
	result.Suggestions = append(result.Suggestions, syntaxResult.Suggestions...)
}

func (a *DockerfileAdapter) mergeSecurityResults(result *AtomicValidateDockerfileResult, securityResult *dockerfile.ValidationResult) {
	// Convert security issues
	for _, issue := range securityResult.SecurityIssues {
		result.SecurityIssues = append(result.SecurityIssues, DockerfileSecurityIssue{
			Type:          issue.Type,
			Line:          issue.Line,
			Severity:      issue.Severity,
			Description:   issue.Description,
			Remediation:   issue.Remediation,
			CVEReferences: issue.CVEReferences,
		})
	}

	// Also add security-related errors and warnings
	for _, err := range securityResult.Errors {
		result.Errors = append(result.Errors, DockerfileValidationError{
			Type:     "security",
			Line:     err.Line,
			Message:  err.Message,
			Severity: err.Severity,
		})
	}

	for _, warn := range securityResult.Warnings {
		result.Warnings = append(result.Warnings, DockerfileValidationWarning{
			Type:    "security",
			Line:    warn.Line,
			Message: warn.Message,
			Impact:  "security",
		})
	}
}

func (a *DockerfileAdapter) mergeImageResults(result *AtomicValidateDockerfileResult, imageResult *dockerfile.ValidationResult) {
	// Merge errors and warnings
	for _, err := range imageResult.Errors {
		result.Errors = append(result.Errors, DockerfileValidationError{
			Type:     err.Type,
			Line:     err.Line,
			Message:  err.Message,
			Severity: err.Severity,
		})
	}

	for _, warn := range imageResult.Warnings {
		result.Warnings = append(result.Warnings, DockerfileValidationWarning{
			Type:       warn.Type,
			Line:       warn.Line,
			Message:    warn.Message,
			Suggestion: warn.Suggestion,
			Impact:     warn.Impact,
		})
	}
}

func (a *DockerfileAdapter) mergeContextResults(result *AtomicValidateDockerfileResult, contextResult *dockerfile.ValidationResult) {
	// Merge errors and warnings
	for _, err := range contextResult.Errors {
		result.Errors = append(result.Errors, DockerfileValidationError{
			Type:     err.Type,
			Line:     err.Line,
			Message:  err.Message,
			Severity: err.Severity,
		})
	}

	for _, warn := range contextResult.Warnings {
		result.Warnings = append(result.Warnings, DockerfileValidationWarning{
			Type:       warn.Type,
			Line:       warn.Line,
			Message:    warn.Message,
			Suggestion: warn.Suggestion,
			Impact:     warn.Impact,
		})
	}

	// Merge suggestions
	result.Suggestions = append(result.Suggestions, contextResult.Suggestions...)
}

// Conversion methods

func convertSecurityAnalysis(analysis dockerfile.SecurityAnalysis) SecurityAnalysis {
	return SecurityAnalysis{
		RunsAsRoot:      analysis.RunsAsRoot,
		ExposedPorts:    analysis.ExposedPorts,
		HasSecrets:      analysis.HasSecrets,
		UsesPackagePin:  analysis.UsesPackagePin,
		SecurityScore:   analysis.SecurityScore,
		Recommendations: analysis.Recommendations,
	}
}

func convertBaseImageAnalysis(analysis dockerfile.BaseImageAnalysis) BaseImageAnalysis {
	return BaseImageAnalysis{
		Image:           analysis.Image,
		Registry:        analysis.Registry,
		IsTrusted:       analysis.IsTrusted,
		IsOfficial:      analysis.IsOfficial,
		HasKnownVulns:   analysis.HasKnownVulns,
		Alternatives:    analysis.Alternatives,
		Recommendations: analysis.Recommendations,
	}
}

// generateLayerAnalysis generates layer analysis from context analysis
func (a *DockerfileAdapter) generateLayerAnalysis(contextAnalysis dockerfile.ContextAnalysis, lines []string) LayerAnalysis {
	analysis := LayerAnalysis{
		ProblematicSteps: make([]ProblematicStep, 0),
		Optimizations:    make([]LayerOptimization, 0),
	}

	// Count layers
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "RUN") ||
			strings.HasPrefix(trimmed, "COPY") ||
			strings.HasPrefix(trimmed, "ADD") {
			analysis.TotalLayers++
		}
	}

	// Add problematic steps from context analysis
	for _, warning := range contextAnalysis.LargeFileWarnings {
		analysis.ProblematicSteps = append(analysis.ProblematicSteps, ProblematicStep{
			Line:        0, // Would need to parse from warning
			Instruction: "COPY/ADD",
			Issue:       warning,
			Impact:      "build_time",
		})
	}

	// Generate optimizations
	if contextAnalysis.AddOperations > 0 && contextAnalysis.CopyOperations > 0 {
		analysis.Optimizations = append(analysis.Optimizations, LayerOptimization{
			Type:        "instruction_choice",
			Description: "Replace ADD with COPY where appropriate",
			Before:      "ADD file.txt /app/",
			After:       "COPY file.txt /app/",
			Benefit:     "Clearer intent and potentially faster",
		})
	}

	analysis.CacheableSteps = analysis.TotalLayers - len(analysis.ProblematicSteps)

	return analysis
}

// generateOptimizationTips generates optimization suggestions
func (a *DockerfileAdapter) generateOptimizationTips(result *AtomicValidateDockerfileResult, lines []string) {
	// Layer consolidation tip
	runCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "RUN") {
			runCount++
		}
	}

	if runCount > 3 {
		result.OptimizationTips = append(result.OptimizationTips, OptimizationTip{
			Type:             "layer_consolidation",
			Description:      fmt.Sprintf("Found %d RUN commands that could be consolidated", runCount),
			Impact:           "size_reduction",
			Suggestion:       "Combine multiple RUN commands with && to reduce layers",
			EstimatedSavings: "10-20% size reduction",
		})
	}

	// Add more optimization tips based on analysis results
	if result.BaseImageAnalysis.Image != "" && !strings.Contains(result.BaseImageAnalysis.Image, "alpine") {
		result.OptimizationTips = append(result.OptimizationTips, OptimizationTip{
			Type:        "base_image_size",
			Description: "Using potentially large base image",
			Impact:      "size_reduction",
			Suggestion:  "Consider using Alpine-based images for smaller size",
		})
	}
}

// generateSuggestions generates remediation suggestions
func (a *DockerfileAdapter) generateSuggestions(result *AtomicValidateDockerfileResult) {
	// Add suggestions based on issues found
	if result.SecurityAnalysis.RunsAsRoot {
		result.Suggestions = append(result.Suggestions,
			"Add a non-root user with: RUN adduser -D appuser && USER appuser")
	}

	if !result.SecurityAnalysis.UsesPackagePin {
		result.Suggestions = append(result.Suggestions,
			"Pin package versions for reproducible builds")
	}

	if len(result.SecurityIssues) > 0 {
		result.Suggestions = append(result.Suggestions,
			"Review and address all security issues before building")
	}
}

// calculateValidationScore calculates the overall validation score
func (a *DockerfileAdapter) calculateValidationScore(result *AtomicValidateDockerfileResult) int {
	score := 100

	// Deduct for errors
	score -= len(result.Errors) * 10
	score -= result.CriticalIssues * 15

	// Deduct for security issues
	for _, issue := range result.SecurityIssues {
		switch issue.Severity {
		case "critical":
			score -= 20
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// Deduct for warnings (less severe)
	score -= len(result.Warnings) * 3

	// Bonus for best practices
	if result.SecurityAnalysis.UsesPackagePin {
		score += 5
	}
	if !result.SecurityAnalysis.RunsAsRoot {
		score += 10
	}

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// generateCorrectedDockerfile generates a corrected version of the Dockerfile
func (a *DockerfileAdapter) generateCorrectedDockerfile(dockerfileContent string, result *AtomicValidateDockerfileResult) (string, []string) {
	fixes := make([]string, 0)
	lines := strings.Split(dockerfileContent, "\n")
	corrected := make([]string, len(lines))
	copy(corrected, lines)

	// Apply basic fixes
	// This is a simplified version - in practice, would need more sophisticated fixing

	// Add FROM if missing
	hasFrom := false
	for _, line := range corrected {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "FROM") {
			hasFrom = true
			break
		}
	}

	if !hasFrom {
		corrected = append([]string{"FROM alpine:latest"}, corrected...)
		fixes = append(fixes, "Added missing FROM instruction")
	}

	// Add USER instruction if running as root
	if result.SecurityAnalysis.RunsAsRoot {
		corrected = append(corrected, "", "# Create non-root user", "RUN adduser -D appuser", "USER appuser")
		fixes = append(fixes, "Added non-root user for security")
	}

	return strings.Join(corrected, "\n"), fixes
}
