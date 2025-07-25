// Package sampling provides specialized sampling operations
package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	domain "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
)

// SpecializedClient provides specialized operations that build on the unified sampler
type SpecializedClient struct {
	sampler domain.UnifiedSampler
	logger  *slog.Logger
}

// NewSpecializedClient creates a new specialized client with the unified sampler
func NewSpecializedClient(logger *slog.Logger) *SpecializedClient {
	sampler := CreateDomainClient(logger)
	return &SpecializedClient{
		sampler: sampler,
		logger:  logger,
	}
}

// AnalyzeKubernetesManifest analyzes and fixes Kubernetes manifests
func (c *SpecializedClient) AnalyzeKubernetesManifest(ctx context.Context, manifestContent string, deploymentError error, dockerfileContent string, repoAnalysis string) (*ManifestFix, error) {
	// Prepare error string
	var errorStr string
	if deploymentError != nil {
		errorStr = deploymentError.Error()
	} else {
		errorStr = "No deployment error provided"
	}

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"ManifestContent":   manifestContent,
		"DeploymentError":   errorStr,
		"DockerfileContent": dockerfileContent,
		"RepoAnalysis":      repoAnalysis,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("kubernetes-manifest-fix", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubernetes manifest fix template: %w", err)
	}

	// Create sampling request
	req := domain.Request{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
		Metadata: map[string]interface{}{
			"template_id": "kubernetes-manifest-fix",
			"error":       errorStr,
		},
	}

	// Sample
	response, err := c.sampler.Sample(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI fix for manifest: %w", err)
	}

	// Parse the response
	parser := NewDefaultParser()
	result, err := parser.ParseManifestFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest fix response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = response.TokensUsed
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.GeneratedAt = time.Now()

	// Add original error info
	if deploymentError != nil {
		result.OriginalIssues = []string{deploymentError.Error()}
	} else {
		result.OriginalIssues = []string{errorStr}
	}

	// Validate the result
	if err := result.Validate(); err != nil {
		c.logger.Warn("Generated manifest fix failed validation", "error", err)
		result.ValidationStatus.IsValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, err.Error())
	}

	// Perform content validation
	validator := NewDefaultValidator()
	contentValidation := validator.ValidateManifestContent(result.FixedManifest)

	// Merge validation results
	result.ValidationStatus.SyntaxValid = contentValidation.SyntaxValid
	result.ValidationStatus.BestPractices = contentValidation.BestPractices
	result.ValidationStatus.Errors = append(result.ValidationStatus.Errors, contentValidation.Errors...)
	result.ValidationStatus.Warnings = append(result.ValidationStatus.Warnings, contentValidation.Warnings...)

	if !contentValidation.IsValid {
		result.ValidationStatus.IsValid = false
	}

	return result, nil
}

// AnalyzePodCrashLoop diagnoses pod crash loops
func (c *SpecializedClient) AnalyzePodCrashLoop(ctx context.Context, podLogs string, manifestContent string, dockerfileContent string, errorDetails string) (*ErrorAnalysis, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"PodLogs":           podLogs,
		"ManifestContent":   manifestContent,
		"DockerfileContent": dockerfileContent,
		"ErrorDetails":      errorDetails,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("pod-crash-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render pod crash analysis template: %w", err)
	}

	// Create sampling request
	req := domain.Request{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
		Metadata: map[string]interface{}{
			"template_id": "pod-crash-analysis",
		},
	}

	// Sample
	response, err := c.sampler.Sample(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze pod crash: %w", err)
	}

	// Parse the response
	parser := NewDefaultParser()
	result, err := parser.ParseErrorAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error analysis response: %w", err)
	}

	return result, nil
}

// AnalyzeSecurityScan analyzes security scan results
func (c *SpecializedClient) AnalyzeSecurityScan(ctx context.Context, scanResults string, dockerfileContent string, criticalOnly bool) (*SecurityAnalysis, error) {
	// Use the unified sampler's analyze method
	domainResult, err := c.sampler.AnalyzeSecurityScan(ctx, scanResults)
	if err != nil {
		return nil, err
	}

	// Convert domain result to infrastructure result
	result := &SecurityAnalysis{
		RiskLevel:       RiskLevel(domainResult.RiskLevel),
		Recommendations: domainResult.Recommendations,
		CriticalIssues:  make([]SecurityIssue, 0),
		Remediations:    make([]Remediation, 0),
		Metadata: ResponseMetadata{
			TemplateID:  "security-scan-analysis",
			TokensUsed:  estimateTokens(scanResults),
			Temperature: 0.3,
			GeneratedAt: time.Now(),
		},
	}

	// Convert vulnerabilities to issues
	for _, vuln := range domainResult.Vulnerabilities {
		if criticalOnly && !strings.EqualFold(vuln.Severity, "critical") {
			continue
		}
		issue := SecurityIssue{
			CVE:         vuln.ID,
			Severity:    Severity(vuln.Severity),
			Description: vuln.Description,
			Component:   vuln.Package,
			FixVersion:  vuln.FixVersion,
		}
		result.CriticalIssues = append(result.CriticalIssues, issue)
	}

	// Convert remediations
	for _, rem := range domainResult.Remediations {
		result.Remediations = append(result.Remediations, Remediation{
			Action:   rem,
			Priority: PriorityHigh,
		})
	}

	return result, nil
}

// ImproveRepositoryAnalysis enhances repository analysis with additional context
func (c *SpecializedClient) ImproveRepositoryAnalysis(ctx context.Context, initialAnalysis string, fileTree string, readmeContent string) (*RepositoryAnalysis, error) {
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
	rendered, err := templateManager.RenderTemplate("improve-repository-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render improve analysis template: %w", err)
	}

	// Create sampling request
	req := domain.Request{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
		Metadata: map[string]interface{}{
			"template_id": "improve-repository-analysis",
		},
	}

	// Sample
	response, err := c.sampler.Sample(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to improve repository analysis: %w", err)
	}

	// Parse the response
	parser := NewDefaultParser()
	result, err := parser.ParseRepositoryAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository analysis response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = response.TokensUsed
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.GeneratedAt = time.Now()

	return result, nil
}

// GenerateDockerfile generates a new Dockerfile based on language and framework
func (c *SpecializedClient) GenerateDockerfile(ctx context.Context, language string, framework string, port int) (*DockerfileFix, error) {
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
	rendered, err := templateManager.RenderTemplate("generate-dockerfile", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render generate dockerfile template: %w", err)
	}

	// Create sampling request
	req := domain.Request{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
		Metadata: map[string]interface{}{
			"template_id": "generate-dockerfile",
			"language":    language,
			"framework":   framework,
			"port":        port,
		},
	}

	// Sample
	response, err := c.sampler.Sample(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate dockerfile: %w", err)
	}

	// Parse the response
	parser := NewDefaultParser()
	result, err := parser.ParseDockerfileFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dockerfile generation response: %w", err)
	}

	// Enrich metadata
	result.Metadata.TemplateID = rendered.ID
	result.Metadata.TokensUsed = response.TokensUsed
	result.Metadata.Temperature = rendered.Temperature
	result.Metadata.GeneratedAt = time.Now()

	// Validate the generated Dockerfile
	validator := NewDefaultValidator()
	contentValidation := validator.ValidateDockerfileContent(result.FixedDockerfile)

	result.ValidationStatus = contentValidation

	return result, nil
}

// FixDockerfile fixes issues in an existing Dockerfile
func (c *SpecializedClient) FixDockerfile(ctx context.Context, language string, framework string, port int, dockerfileContent string, buildError string) (*DockerfileFix, error) {
	// Extract issues from build error
	issues := []string{buildError}

	// Use the unified sampler's fix method
	domainResult, err := c.sampler.FixDockerfile(ctx, dockerfileContent, issues)
	if err != nil {
		return nil, err
	}

	// Convert domain result to infrastructure result
	result := &DockerfileFix{
		FixedDockerfile:  domainResult.FixedContent,
		ChangesApplied:   make([]Change, 0),
		OptimizationTips: strings.Split(domainResult.Explanation, "\n"),
		OriginalError:    buildError,
		Metadata: ResponseMetadata{
			TemplateID:     domainResult.Metadata.TemplateID,
			TokensUsed:     domainResult.Metadata.TokensUsed,
			Temperature:    domainResult.Metadata.Temperature,
			ProcessingTime: domainResult.Metadata.ProcessingTime,
			GeneratedAt:    domainResult.Metadata.Timestamp,
		},
	}

	// Convert changes
	for _, change := range domainResult.Changes {
		result.ChangesApplied = append(result.ChangesApplied, Change{
			Description: change,
			Type:        ChangeTypeOptimization,
		})
	}

	// Validate the fixed Dockerfile
	validator := NewDefaultValidator()
	result.ValidationStatus = validator.ValidateDockerfileContent(result.FixedDockerfile)

	return result, nil
}

// AnalyzeDockerfileIssue analyzes and fixes Dockerfile build issues
func (c *SpecializedClient) AnalyzeDockerfileIssue(ctx context.Context, dockerfileContent string, buildError error, repoAnalysis string) (string, error) {
	// Prepare error string
	var errorStr string
	if buildError != nil {
		errorStr = buildError.Error()
	} else {
		errorStr = "No build error provided"
	}

	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return "", fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"DockerfileContent": dockerfileContent,
		"BuildError":        errorStr,
		"RepoAnalysis":      repoAnalysis,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("dockerfile-fix", templateData)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	req := domain.Request{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
		Metadata: map[string]interface{}{
			"template_id":       "generate-dockerfile",
			"dockerfileContent": dockerfileContent,
			"buildError":        buildError,
			"repoAnalysis":      repoAnalysis,
		},
	}

	start := time.Now()
	// Sample
	response, err := c.sampler.Sample(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to fix dockerfile: %w", err)
	}

	c.logger.Info("Dockerfile fix completed", "duration", time.Since(start))
	return response.Content, nil
}
