package sampling

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// AnalyzeKubernetesManifest uses MCP sampling to analyze and fix Kubernetes manifests
func (c *Client) AnalyzeKubernetesManifest(ctx context.Context, manifestContent string, deploymentError error, dockerfileContent string, repoAnalysis string) (result *ManifestFix, err error) {
	err = tracing.TraceSamplingRequest(ctx, "kubernetes-manifest-fix", func(tracedCtx context.Context) error {
		var internalErr error
		result, internalErr = c.analyzeKubernetesManifestInternal(tracedCtx, manifestContent, deploymentError, dockerfileContent, repoAnalysis)
		return internalErr
	})
	return result, err
}

// analyzeKubernetesManifestInternal contains the actual implementation with metrics
func (c *Client) analyzeKubernetesManifestInternal(ctx context.Context, manifestContent string, deploymentError error, dockerfileContent string, repoAnalysis string) (result *ManifestFix, err error) {
	// Add tracing attributes
	span := tracing.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(tracing.AttrSamplingTemplateID, "kubernetes-manifest-fix"),
			attribute.String(tracing.AttrSamplingContentType, "manifest"),
			attribute.Int(tracing.AttrSamplingContentSize, len(manifestContent)),
		)
	}

	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			validationResult = result.ValidationStatus
			responseSize = len(result.FixedManifest)
		}

		metrics.RecordSamplingRequest(
			ctx,
			"kubernetes-manifest-fix",
			success,
			duration,
			estimateTokens(manifestContent+dockerfileContent+repoAnalysis),
			estimateTokens(manifestContent+dockerfileContent+repoAnalysis),
			estimateTokens(strconv.Itoa(responseSize)),
			"manifest",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to fix Kubernetes manifest",
		"error_preview", deploymentError.Error()[:min(100, len(deploymentError.Error()))])

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"ManifestContent":   manifestContent,
		"DeploymentError":   deploymentError.Error(),
		"DockerfileContent": dockerfileContent,
		"RepoAnalysis":      repoAnalysis,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("kubernetes-manifest-fix", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubernetes manifest fix template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI fix for manifest: %w", err)
	}

	// Parse the response into structured format
	parser := NewDefaultParser()
	result, err = parser.ParseManifestFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest fix response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = estimateTokens(response.Content)
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.ProcessingTime = time.Since(time.Now()) // Will be overridden by caller if needed

	// Add original error info
	result.OriginalIssues = []string{deploymentError.Error()}

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated manifest fix failed validation", "error", err)
		result.ValidationStatus.IsValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, err.Error())
	}

	// Perform comprehensive content validation with tracing
	var contentValidation ValidationResult
	_, validationErr := tracing.TraceSamplingValidation(ctx, "manifest", func(validationCtx context.Context) (bool, error) {
		validator := NewDefaultValidator()
		contentValidation = validator.ValidateManifestContent(result.FixedManifest)
		return contentValidation.IsValid, nil
	})

	if validationErr != nil {
		c.logger.Warn("Content validation tracing failed", "error", validationErr)
	}

	// Merge validation results
	result.ValidationStatus.SyntaxValid = contentValidation.SyntaxValid
	result.ValidationStatus.BestPractices = contentValidation.BestPractices
	result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, contentValidation.Errors...)
	result.ValidationStatus.Warnings = append(result.ValidationStatus.Warnings, contentValidation.Warnings...)

	// Update overall validity
	if !contentValidation.IsValid {
		result.ValidationStatus.IsValid = false
	}

	// Add validation results to tracing span
	if span.IsRecording() {
		span.SetAttributes(
			attribute.Bool(tracing.AttrSamplingValidationValid, result.ValidationStatus.IsValid),
			attribute.Int(tracing.AttrSamplingSecurityIssues, countSecurityIssues(result.ValidationStatus.Errors)),
		)
	}

	// Set success flag for metrics
	success = result.ValidationStatus.IsValid
	return result, nil
}

// AnalyzePodCrashLoop uses MCP sampling to diagnose and suggest fixes for pod crash loops
func (c *Client) AnalyzePodCrashLoop(ctx context.Context, podLogs string, manifestContent string, dockerfileContent string, errorDetails string) (result *ErrorAnalysis, err error) {
	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			responseSize = len(result.RootCause + result.Fix)
		}

		metrics.RecordSamplingRequest(
			ctx,
			"pod-crash-analysis",
			success,
			duration,
			estimateTokens(podLogs+manifestContent+dockerfileContent+errorDetails),
			estimateTokens(podLogs+manifestContent+dockerfileContent+errorDetails),
			estimateTokens(strconv.Itoa(responseSize)),
			"error-analysis",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to diagnose pod crash loop")

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"PodLogs":           podLogs,
		"ErrorDetails":      errorDetails,
		"ManifestContent":   manifestContent,
		"DockerfileContent": dockerfileContent,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("pod-crash-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render pod crash analysis template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze pod crash: %w", err)
	}

	result = parseErrorAnalysis(response.Content)
	success = result != nil && result.RootCause != ""
	return result, nil
}

// AnalyzeSecurityScan uses MCP sampling to analyze security scan results and suggest remediations
func (c *Client) AnalyzeSecurityScan(ctx context.Context, scanResults string, dockerfileContent string, criticalOnly bool) (result *SecurityAnalysis, err error) {
	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			responseSize = result.Metadata.TokensUsed
		}

		metrics.RecordSamplingRequest(
			ctx,
			"security-scan-analysis",
			success,
			duration,
			estimateTokens(scanResults+dockerfileContent),
			estimateTokens(scanResults+dockerfileContent),
			estimateTokens(strconv.Itoa(responseSize)),
			"security-analysis",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to analyze security scan results")

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"ScanResults":       scanResults,
		"DockerfileContent": dockerfileContent,
		"CriticalOnly":      criticalOnly,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("security-scan-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render security scan analysis template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze security scan: %w", err)
	}

	// Parse the response into structured format
	parser := NewDefaultParser()
	result, err = parser.ParseSecurityAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse security analysis response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = estimateTokens(response.Content)
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.ProcessingTime = time.Since(time.Now())

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated security analysis failed validation", "error", err)
		success = false
	} else {
		success = true
	}

	return result, nil
}

// ImproveRepositoryAnalysis uses MCP sampling to enhance repository analysis
func (c *Client) ImproveRepositoryAnalysis(ctx context.Context, initialAnalysis string, fileTree string, readmeContent string) (result *RepositoryAnalysis, err error) {
	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			responseSize = result.Metadata.TokensUsed
		}

		metrics.RecordSamplingRequest(
			ctx,
			"repository-analysis",
			success,
			duration,
			estimateTokens(initialAnalysis+fileTree+readmeContent),
			estimateTokens(initialAnalysis+fileTree+readmeContent),
			estimateTokens(strconv.Itoa(responseSize)),
			"repository-analysis",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to improve repository analysis")

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"InitialAnalysis": initialAnalysis,
		"FileTree":        fileTree,
		"ReadmeContent":   readmeContent,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("repository-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render repository analysis template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to improve repository analysis: %w", err)
	}

	// Parse the response into structured format
	parser := NewDefaultParser()
	result, err = parser.ParseRepositoryAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository analysis response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = estimateTokens(response.Content)
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.ProcessingTime = time.Since(time.Now())

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated repository analysis failed validation", "error", err)
		success = false
	} else {
		success = true
	}

	return result, nil
}

// GenerateDockerfile uses MCP sampling to generate production-ready Dockerfiles
func (c *Client) GenerateDockerfile(ctx context.Context, language string, framework string, port int) (result *DockerfileFix, err error) {
	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			validationResult = result.ValidationStatus
			responseSize = len(result.FixedDockerfile)
		}

		metrics.RecordSamplingRequest(
			ctx,
			"dockerfile-generation",
			success,
			duration,
			estimateTokens(language+framework),
			estimateTokens(language+framework),
			estimateTokens(strconv.Itoa(responseSize)),
			"dockerfile",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to generate Dockerfile",
		"language", language,
		"framework", framework,
		"port", port)

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Language":  language,
		"Framework": framework,
		"Port":      port,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("dockerfile-generation", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render dockerfile generation template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Parse the response into structured format
	parser := NewDefaultParser()
	result, err = parser.ParseDockerfileFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dockerfile generation response: %w", err)
	}

	// This is generation, not fixing, so clear the original error
	result.OriginalError = ""

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = estimateTokens(response.Content)
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.ProcessingTime = time.Since(time.Now())

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated Dockerfile failed validation", "error", err)
		result.ValidationStatus.IsValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, err.Error())
	}

	// Perform comprehensive content validation
	validator := NewDefaultValidator()
	contentValidation := validator.ValidateDockerfileContent(result.FixedDockerfile)

	// Merge validation results
	result.ValidationStatus.SyntaxValid = contentValidation.SyntaxValid
	result.ValidationStatus.BestPractices = contentValidation.BestPractices
	result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, contentValidation.Errors...)
	result.ValidationStatus.Warnings = append(result.ValidationStatus.Warnings, contentValidation.Warnings...)

	// Update overall validity
	if !contentValidation.IsValid {
		result.ValidationStatus.IsValid = false
	}

	// Set success flag for metrics
	success = result.ValidationStatus.IsValid
	return result, nil
}

// FixDockerfile uses MCP sampling to fix Dockerfile build errors
func (c *Client) FixDockerfile(ctx context.Context, language string, framework string, port int, dockerfileContent string, buildError string) (result *DockerfileFix, err error) {
	startTime := time.Now()
	var success bool
	defer func() {
		// Record metrics for this request
		duration := time.Since(startTime)
		metrics := GetGlobalMetrics()

		validationResult := ValidationResult{
			IsValid:       success,
			SyntaxValid:   success,
			BestPractices: success,
		}

		responseSize := 0
		if result != nil {
			validationResult = result.ValidationStatus
			responseSize = len(result.FixedDockerfile)
		}

		metrics.RecordSamplingRequest(
			ctx,
			"dockerfile-fix",
			success,
			duration,
			estimateTokens(dockerfileContent+buildError+language+framework),
			estimateTokens(dockerfileContent+buildError+language+framework),
			estimateTokens(strconv.Itoa(responseSize)),
			"dockerfile",
			responseSize,
			validationResult,
		)
	}()

	c.logger.Info("Requesting AI assistance to fix Dockerfile",
		"language", language,
		"framework", framework,
		"error_preview", buildError[:min(100, len(buildError))])

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Language":          language,
		"Framework":         framework,
		"Port":              port,
		"DockerfileContent": dockerfileContent,
		"BuildError":        buildError,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("dockerfile-fix", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render dockerfile fix template: %w", err)
	}

	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to fix Dockerfile: %w", err)
	}

	// Parse the response into structured format
	parser := NewDefaultParser()
	result, err = parser.ParseDockerfileFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dockerfile fix response: %w", err)
	}

	// Set the original error
	result.OriginalError = buildError

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = estimateTokens(response.Content)
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.ProcessingTime = time.Since(time.Now())

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated Dockerfile fix failed validation", "error", err)
		result.ValidationStatus.IsValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, err.Error())
	}

	// Perform comprehensive content validation
	validator := NewDefaultValidator()
	contentValidation := validator.ValidateDockerfileContent(result.FixedDockerfile)

	// Merge validation results
	result.ValidationStatus.SyntaxValid = contentValidation.SyntaxValid
	result.ValidationStatus.BestPractices = contentValidation.BestPractices
	result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, contentValidation.Errors...)
	result.ValidationStatus.Warnings = append(result.ValidationStatus.Warnings, contentValidation.Warnings...)

	// Update overall validity
	if !contentValidation.IsValid {
		result.ValidationStatus.IsValid = false
	}

	// Set success flag for metrics
	success = result.ValidationStatus.IsValid
	return result, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// countSecurityIssues counts the number of security-related errors
func countSecurityIssues(errors []string) int {
	count := 0
	for _, err := range errors {
		if strings.Contains(err, "SECURITY:") {
			count++
		}
	}
	return count
}
