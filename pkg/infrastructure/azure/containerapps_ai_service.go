// Package azure provides AI-powered Azure Container Apps operations
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/Azure/containerization-assist/pkg/domain/sampling"
	"github.com/Azure/containerization-assist/pkg/infrastructure/ai_ml/prompts"
)

// AIManifestService provides AI-powered Azure Container Apps manifest operations
type AIManifestService struct {
	logger          *slog.Logger
	sampler         sampling.UnifiedSampler
	promptManager   *prompts.Manager
	fallbackService AzureContainerAppsManifestService // Fallback to template-based generation
}

// NewAIManifestService creates a new AI-powered Azure Container Apps manifest service
func NewAIManifestService(logger *slog.Logger, sampler sampling.UnifiedSampler) (*AIManifestService, error) {
	// Initialize prompt manager
	promptManager, err := prompts.NewManager(logger, prompts.ManagerConfig{
		EnableHotReload: false,
		AllowOverride:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize prompt manager: %w", err)
	}

	return &AIManifestService{
		logger:          logger,
		sampler:         sampler,
		promptManager:   promptManager,
		fallbackService: NewAzureContainerAppsManifestService(logger),
	}, nil
}

// GenerateManifestsWithAI generates Azure Container Apps manifests using AI
func (s *AIManifestService) GenerateManifestsWithAI(
	ctx context.Context,
	options AzureContainerAppsManifestOptions,
	analysisContext *AnalysisContext,
) (*AzureContainerAppsManifestResult, error) {
	startTime := time.Now()

	result := &AzureContainerAppsManifestResult{
		Template:  options.Template,
		OutputDir: options.OutputDir,
		Context:   make(map[string]interface{}),
		Manifests: make([]GeneratedAzureManifest, 0),
	}

	// Validate inputs
	if err := s.validateInputs(options); err != nil {
		result.Error = &AzureManifestError{
			Type:    "validation_error",
			Message: err.Error(),
		}
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Set defaults
	s.setDefaults(&options)

	// Create output directory
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		result.Error = &AzureManifestError{
			Type:    "directory_error",
			Message: fmt.Sprintf("Failed to create output directory: %v", err),
			Path:    options.OutputDir,
		}
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Generate manifests using AI
	manifests, err := s.generateWithAI(ctx, options, analysisContext)
	if err != nil {
		s.logger.Warn("AI generation failed, falling back to template-based generation",
			slog.String("error", err.Error()))

		// Fallback to template-based generation
		return s.fallbackService.GenerateManifests(ctx, options)
	}

	result.Manifests = manifests
	result.Success = true
	result.Duration = time.Since(startTime)

	// Set manifest path
	if len(manifests) > 0 {
		result.ManifestPath = manifests[0].Path
	} else {
		result.ManifestPath = options.OutputDir
	}

	return result, nil
}

// generateWithAI uses AI to generate manifests
func (s *AIManifestService) generateWithAI(
	ctx context.Context,
	options AzureContainerAppsManifestOptions,
	analysisContext *AnalysisContext,
) ([]GeneratedAzureManifest, error) {
	// Prepare template data
	templateData := prompts.TemplateData{
		"AppName":         options.AppName,
		"ImageRef":        options.ImageRef,
		"Port":            options.Port,
		"ResourceGroup":   options.ResourceGroup,
		"Location":        options.Location,
		"EnvironmentName": options.EnvironmentName,
		"OutputFormat":    strings.ToLower(options.Template),
		"MinReplicas":     options.MinReplicas,
		"MaxReplicas":     options.MaxReplicas,
		"CPU":             options.Resources.CPU,
		"Memory":          options.Resources.Memory,
	}

	// Add analysis context if available
	if analysisContext != nil {
		templateData["Language"] = analysisContext.Language
		templateData["Framework"] = analysisContext.Framework
		templateData["AnalysisContext"] = s.buildAnalysisContextString(analysisContext)
	} else {
		// Provide defaults if no analysis context
		templateData["Language"] = "unknown"
		templateData["Framework"] = "unknown"
		templateData["AnalysisContext"] = "No specific analysis context available"
	}

	// Render the prompt
	renderedPrompt, err := s.promptManager.RenderTemplate("azure-container-apps-generation", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt template: %w", err)
	}

	// Create sampling request
	samplingReq := sampling.Request{
		Prompt:       renderedPrompt.Content,
		SystemPrompt: renderedPrompt.SystemPrompt,
		MaxTokens:    renderedPrompt.MaxTokens,
		Temperature:  renderedPrompt.Temperature,
		Metadata: map[string]interface{}{
			"operation": "azure-container-apps-generation",
			"app_name":  options.AppName,
			"format":    options.Template,
		},
	}

	// Call AI sampler
	response, err := s.sampler.Sample(ctx, samplingReq)
	if err != nil {
		return nil, fmt.Errorf("AI sampling failed: %w", err)
	}

	// Parse and save the generated manifest
	return s.parseAndSaveManifest(response.Content, options)
}

// parseAndSaveManifest parses AI-generated content and saves it to files
func (s *AIManifestService) parseAndSaveManifest(content string, options AzureContainerAppsManifestOptions) ([]GeneratedAzureManifest, error) {
	manifests := make([]GeneratedAzureManifest, 0)

	// Determine file extension based on format
	ext := ".json"
	if strings.ToLower(options.Template) == "bicep" {
		ext = ".bicep"
	}

	// Main manifest file
	mainFileName := fmt.Sprintf("main%s", ext)
	mainPath := filepath.Join(options.OutputDir, mainFileName)

	// Write the main manifest
	if err := os.WriteFile(mainPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	manifests = append(manifests, GeneratedAzureManifest{
		Name:    mainFileName,
		Type:    strings.ToLower(options.Template),
		Path:    mainPath,
		Content: content,
		Size:    len(content),
		Valid:   true,
	})

	// For Bicep, also generate a parameters file
	if strings.ToLower(options.Template) == "bicep" {
		paramsContent := s.generateParametersFile(options)
		paramsPath := filepath.Join(options.OutputDir, "main.parameters.json")

		if err := os.WriteFile(paramsPath, []byte(paramsContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write parameters file: %w", err)
		}

		manifests = append(manifests, GeneratedAzureManifest{
			Name:    "main.parameters.json",
			Type:    "json",
			Path:    paramsPath,
			Content: paramsContent,
			Size:    len(paramsContent),
			Valid:   true,
		})
	}

	// Generate deployment script
	scriptContent := s.generateDeploymentScript(options)
	scriptPath := filepath.Join(options.OutputDir, "deploy.sh")

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		s.logger.Warn("Failed to write deployment script", slog.String("error", err.Error()))
	} else {
		manifests = append(manifests, GeneratedAzureManifest{
			Name:    "deploy.sh",
			Type:    "script",
			Path:    scriptPath,
			Content: scriptContent,
			Size:    len(scriptContent),
			Valid:   true,
		})
	}

	return manifests, nil
}

// ValidateManifestsWithAI validates Azure Container Apps manifests using AI
func (s *AIManifestService) ValidateManifestsWithAI(
	ctx context.Context,
	manifestPath string,
	manifestType string,
	analysisContext *AnalysisContext,
) (*api.ManifestValidationResult, error) {
	// Read manifest content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Prepare template data for validation
	templateData := prompts.TemplateData{
		"ManifestContent": string(content),
		"ManifestType":    manifestType,
	}

	if analysisContext != nil {
		templateData["Language"] = analysisContext.Language
		templateData["Framework"] = analysisContext.Framework
		templateData["AppDescription"] = analysisContext.Description
	}

	// Render the validation prompt
	renderedPrompt, err := s.promptManager.RenderTemplate("azure-manifest-analysis", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render validation prompt: %w", err)
	}

	// Create sampling request
	samplingReq := sampling.Request{
		Prompt:       renderedPrompt.Content,
		SystemPrompt: renderedPrompt.SystemPrompt,
		MaxTokens:    renderedPrompt.MaxTokens,
		Temperature:  renderedPrompt.Temperature,
		Metadata: map[string]interface{}{
			"operation":     "azure-manifest-validation",
			"manifest_type": manifestType,
		},
	}

	// Call AI sampler
	response, err := s.sampler.Sample(ctx, samplingReq)
	if err != nil {
		return nil, fmt.Errorf("AI validation failed: %w", err)
	}

	// Parse AI response
	return s.parseValidationResponse(response.Content)
}

// Helper methods

func (s *AIManifestService) validateInputs(options AzureContainerAppsManifestOptions) error {
	if options.AppName == "" {
		return fmt.Errorf("app name is required")
	}
	if options.ImageRef == "" {
		return fmt.Errorf("image reference is required")
	}
	if options.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if options.Template != "bicep" && options.Template != "arm" {
		return fmt.Errorf("invalid template type: %s (must be 'bicep' or 'arm')", options.Template)
	}
	return nil
}

func (s *AIManifestService) setDefaults(options *AzureContainerAppsManifestOptions) {
	if options.MinReplicas == 0 {
		options.MinReplicas = 1
	}
	if options.MaxReplicas == 0 {
		options.MaxReplicas = 10
	}
	if options.Resources == nil {
		options.Resources = &ContainerResources{
			CPU:    0.5,
			Memory: "1.0Gi",
		}
	}
	if options.Port == 0 {
		options.Port = 8080
	}
	if options.ResourceGroup == "" {
		options.ResourceGroup = "containerized-apps-rg"
	}
	if options.Location == "" {
		options.Location = "eastus"
	}
	if options.EnvironmentName == "" {
		options.EnvironmentName = "containerized-apps-env"
	}
}

func (s *AIManifestService) buildAnalysisContextString(ctx *AnalysisContext) string {
	var parts []string

	if ctx.Language != "" {
		parts = append(parts, fmt.Sprintf("%s application", ctx.Language))
	}
	if ctx.Framework != "" {
		parts = append(parts, fmt.Sprintf("using %s framework", ctx.Framework))
	}
	if ctx.Dependencies != nil && len(ctx.Dependencies) > 0 {
		deps := strings.Join(ctx.Dependencies[:min(3, len(ctx.Dependencies))], ", ")
		parts = append(parts, fmt.Sprintf("with dependencies: %s", deps))
	}
	if ctx.HasDatabase {
		parts = append(parts, "requires database connectivity")
	}
	if ctx.HasCache {
		parts = append(parts, "uses caching")
	}
	if ctx.IsAPI {
		parts = append(parts, "exposes REST API")
	}
	if ctx.IsWebApp {
		parts = append(parts, "serves web interface")
	}

	if len(parts) == 0 {
		return "Standard containerized application"
	}

	return strings.Join(parts, ", ")
}

func (s *AIManifestService) generateParametersFile(options AzureContainerAppsManifestOptions) string {
	params := map[string]interface{}{
		"$schema":        "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#",
		"contentVersion": "1.0.0.0",
		"parameters": map[string]interface{}{
			"appName": map[string]interface{}{
				"value": options.AppName,
			},
			"imageName": map[string]interface{}{
				"value": options.ImageRef,
			},
			"containerPort": map[string]interface{}{
				"value": options.Port,
			},
			"environmentName": map[string]interface{}{
				"value": options.EnvironmentName,
			},
		},
	}

	jsonBytes, _ := json.MarshalIndent(params, "", "  ")
	return string(jsonBytes)
}

func (s *AIManifestService) generateDeploymentScript(options AzureContainerAppsManifestOptions) string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n\n")
	script.WriteString("# Azure Container Apps Deployment Script\n")
	script.WriteString("# Generated by Container Kit\n\n")

	script.WriteString("# Variables\n")
	script.WriteString(fmt.Sprintf("RESOURCE_GROUP=\"%s\"\n", options.ResourceGroup))
	script.WriteString(fmt.Sprintf("LOCATION=\"%s\"\n", options.Location))
	script.WriteString(fmt.Sprintf("APP_NAME=\"%s\"\n", options.AppName))
	script.WriteString("\n")

	script.WriteString("# Create resource group if it doesn't exist\n")
	script.WriteString("echo \"Creating resource group...\"\n")
	script.WriteString("az group create --name $RESOURCE_GROUP --location $LOCATION\n\n")

	if strings.ToLower(options.Template) == "bicep" {
		script.WriteString("# Deploy using Bicep\n")
		script.WriteString("echo \"Deploying Azure Container Apps using Bicep...\"\n")
		script.WriteString("az deployment group create \\\n")
		script.WriteString("  --resource-group $RESOURCE_GROUP \\\n")
		script.WriteString("  --template-file main.bicep \\\n")
		script.WriteString("  --parameters @main.parameters.json\n")
	} else {
		script.WriteString("# Deploy using ARM template\n")
		script.WriteString("echo \"Deploying Azure Container Apps using ARM template...\"\n")
		script.WriteString("az deployment group create \\\n")
		script.WriteString("  --resource-group $RESOURCE_GROUP \\\n")
		script.WriteString("  --template-file main.json \\\n")
		script.WriteString("  --parameters @main.parameters.json\n")
	}

	script.WriteString("\n# Get application URL\n")
	script.WriteString("echo \"Getting application URL...\"\n")
	script.WriteString("az containerapp show --name $APP_NAME --resource-group $RESOURCE_GROUP --query properties.configuration.ingress.fqdn -o tsv\n")

	return script.String()
}

func (s *AIManifestService) parseValidationResponse(content string) (*api.ManifestValidationResult, error) {
	// Try to parse the AI response as JSON
	var aiResponse struct {
		Issues       []interface{} `json:"issues"`
		Improvements []interface{} `json:"improvements"`
		Validation   struct {
			SyntaxValid       bool `json:"syntax_valid"`
			DeploymentReady   bool `json:"deployment_ready"`
			SecurityCompliant bool `json:"security_compliant"`
			ProductionReady   bool `json:"production_ready"`
		} `json:"validation"`
		Score struct {
			Security      int `json:"security"`
			Performance   int `json:"performance"`
			Reliability   int `json:"reliability"`
			BestPractices int `json:"best_practices"`
			Overall       int `json:"overall"`
		} `json:"score"`
	}

	if err := json.Unmarshal([]byte(content), &aiResponse); err != nil {
		// If JSON parsing fails, return a basic validation result
		return &api.ManifestValidationResult{
			ValidationResult: api.ValidationResult{
				Valid:    false,
				Errors:   []api.ValidationError{{Message: "Failed to parse AI validation response"}},
				Warnings: []api.ValidationWarning{},
				Metadata: map[string]interface{}{
					"ai_response": content,
				},
			},
		}, nil
	}

	// Convert AI response to validation result
	result := &api.ManifestValidationResult{
		ValidationResult: api.ValidationResult{
			Valid:    aiResponse.Validation.DeploymentReady && len(aiResponse.Issues) == 0,
			Errors:   make([]api.ValidationError, 0),
			Warnings: make([]api.ValidationWarning, 0),
			Metadata: map[string]interface{}{
				"scores":             aiResponse.Score,
				"validation_details": aiResponse.Validation,
			},
		},
	}

	// Convert issues to errors/warnings
	for _, issue := range aiResponse.Issues {
		if issueMap, ok := issue.(map[string]interface{}); ok {
			severity, _ := issueMap["severity"].(string)
			description, _ := issueMap["description"].(string)
			recommendation, _ := issueMap["recommendation"].(string)

			if severity == "critical" || severity == "high" {
				result.Errors = append(result.Errors, api.ValidationError{
					Code:    severity,
					Message: description,
					Value:   recommendation,
				})
			} else {
				result.Warnings = append(result.Warnings, api.ValidationWarning{
					Code:    severity,
					Message: description,
					Value:   recommendation,
				})
			}
		}
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AnalysisContext provides context from repository analysis
type AnalysisContext struct {
	Language     string
	Framework    string
	Dependencies []string
	Description  string
	HasDatabase  bool
	HasCache     bool
	IsAPI        bool
	IsWebApp     bool
	Port         int
}
