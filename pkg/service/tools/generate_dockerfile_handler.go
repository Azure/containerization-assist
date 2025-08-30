package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Azure/containerization-assist/pkg/domain/validation"
	"github.com/Azure/containerization-assist/pkg/infrastructure/ai_ml/sampling"
)

// DockerfileGenerationResult represents the JSON structure returned by AI
type DockerfileGenerationResult struct {
	Dockerfile             string   `json:"dockerfile"`
	BaseImage              string   `json:"base_image"`
	ExposedPort            int      `json:"exposed_port"`
	BuildStageCount        int      `json:"build_stage_count"`
	FinalImageSizeEstimate string   `json:"final_image_size_estimate"`
	SecurityFeatures       []string `json:"security_features"`
	OptimizationFeatures   []string `json:"optimization_features"`
}

// createPromptFirstDockerfileHandler creates a handler that uses AI generation with validation
func createPromptFirstDockerfileHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := deps.Logger
		if logger == nil {
			logger = slog.Default()
		}

		// Extract session_id from arguments
		args := req.GetArguments()
		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			result := createErrorResult(fmt.Errorf("session_id is required"))
			return &result, nil
		}

		// Load workflow state to get analysis results
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to load workflow state: %w", err))
			return &result, nil
		}

		if state.Artifacts == nil || state.Artifacts.AnalyzeResult == nil {
			result := createErrorResult(fmt.Errorf("repository analysis not found - run analyze_repository first"))
			return &result, nil
		}

		// Extract analysis data for prompt template
		analyzeResult := state.Artifacts.AnalyzeResult

		// Prepare template parameters
		templateParams := map[string]interface{}{
			"Language":     getLanguageFromArtifact(analyzeResult),
			"Framework":    getFrameworkFromArtifact(analyzeResult),
			"Port":         getPortFromArtifact(analyzeResult),
			"Dependencies": getDependenciesFromArtifact(analyzeResult),
			"BuildSystem":  getBuildSystemFromArtifact(analyzeResult),
		}

		// Generate with critique/retry loop
		dockerfileResult, validationResult, err := generateDockerfileWithCritique(ctx, templateParams, deps, 3)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to generate valid dockerfile: %w", err))
			return &result, nil
		}

		// Store the result in workflow state
		state.Artifacts.DockerfileResult = &DockerfileArtifact{
			Content:   dockerfileResult.Dockerfile,
			BaseImage: dockerfileResult.BaseImage,
			Metadata: map[string]interface{}{
				"exposed_port":              dockerfileResult.ExposedPort,
				"build_stage_count":         dockerfileResult.BuildStageCount,
				"final_image_size_estimate": dockerfileResult.FinalImageSizeEstimate,
				"security_features":         dockerfileResult.SecurityFeatures,
				"optimization_features":     dockerfileResult.OptimizationFeatures,
			},
		}

		// Save updated state
		if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
			logger.Error("Failed to save workflow state", "error", err)
		}

		// Create success response
		data := map[string]interface{}{
			"dockerfile":                dockerfileResult.Dockerfile,
			"base_image":                dockerfileResult.BaseImage,
			"exposed_port":              dockerfileResult.ExposedPort,
			"build_stage_count":         dockerfileResult.BuildStageCount,
			"final_image_size_estimate": dockerfileResult.FinalImageSizeEstimate,
			"security_features":         dockerfileResult.SecurityFeatures,
			"optimization_features":     dockerfileResult.OptimizationFeatures,
			"validation_score":          validationResult.QualityScore,
		}

		chainHint := createChainHint("build_image", "Dockerfile generated and validated successfully. Ready to build the image")

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

// generateDockerfileWithAI uses the dockerfile-generation-json template to create a Dockerfile
func generateDockerfileWithAI(ctx context.Context, params map[string]interface{}, deps ToolDependencies) (*DockerfileGenerationResult, error) {
	// Load the dockerfile generation template
	template, err := deps.PromptManager.GetTemplate("dockerfile-generation-json")
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	// Render the template with parameters
	rendered, err := template.Render(params)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Create sampling request
	samplingReq := sampling.SamplingRequest{
		Prompt:       rendered.Content,
		SystemPrompt: rendered.SystemPrompt,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
	}

	// Generate with JSON schema validation using the injected sampling client
	var result DockerfileGenerationResult
	_, err = deps.SamplingClient.SampleJSONWithSchema(ctx, samplingReq, &result, getDockerfileGenerationSchema())
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	return &result, nil
}

// getDockerfileGenerationSchema returns the JSON schema for dockerfile generation
func getDockerfileGenerationSchema() string {
	return `{
		"type": "object",
		"required": ["dockerfile", "base_image", "exposed_port"],
		"properties": {
			"dockerfile": {
				"type": "string",
				"minLength": 10
			},
			"base_image": {
				"type": "string",
				"minLength": 1
			},
			"exposed_port": {
				"type": "integer",
				"minimum": 1,
				"maximum": 65535
			},
			"build_stage_count": {
				"type": "integer",
				"minimum": 1
			},
			"final_image_size_estimate": {
				"type": "string"
			},
			"security_features": {
				"type": "array",
				"items": {
					"type": "string"
				}
			},
			"optimization_features": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`
}

// Helper functions for parameter extraction
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	return defaultValue
}

// validateDockerfileContent validates the generated Dockerfile content
func validateDockerfileContent(dockerfile string) (*validation.Result, error) {
	result := validation.NewResult()

	// Basic validation - check if dockerfile has FROM instruction
	if dockerfile == "" {
		result.AddError("DF001", "line 1", "Dockerfile content cannot be empty")
		return result, nil
	}

	// Check for basic Dockerfile structure
	if !strings.Contains(dockerfile, "FROM ") {
		result.AddError("DF002", "dockerfile", "Missing FROM instruction")
	}

	// Add some stats
	result.Stats["lines"] = strings.Count(dockerfile, "\n") + 1
	result.Stats["size"] = len(dockerfile)

	// Calculate quality score based on basic checks
	if result.IsValid {
		score := 100
		if !strings.Contains(dockerfile, "USER ") {
			score -= 10 // Deduct for not using non-root user
			result.Findings = append(result.Findings, validation.Finding{
				Code:     "DF003",
				Severity: validation.SeverityWarn,
				Path:     "security",
				Message:  "Consider adding USER instruction for security",
			})
		}
		if !strings.Contains(dockerfile, "HEALTHCHECK") {
			score -= 5 // Deduct for missing health check
		}
		result.QualityScore = score
	}

	return result, nil
}

// Helper functions to extract data from analysis artifact

func getLanguageFromArtifact(analyzeResult *AnalyzeArtifact) string {
	if analyzeResult.Language != "" {
		return analyzeResult.Language
	}
	return "generic"
}

func getFrameworkFromArtifact(analyzeResult *AnalyzeArtifact) string {
	if analyzeResult.Framework != "" {
		return analyzeResult.Framework
	}
	return "unknown"
}

func getPortFromArtifact(analyzeResult *AnalyzeArtifact) int {
	// Extract port from analyze result, default to 8080
	if analyzeResult.Port > 0 {
		return analyzeResult.Port
	}
	return 8080
}

func getDependenciesFromArtifact(analyzeResult *AnalyzeArtifact) []string {
	// Extract dependencies from analyze result
	var deps []string
	for _, dep := range analyzeResult.Dependencies {
		deps = append(deps, dep.Name)
	}
	if len(deps) > 0 {
		return deps
	}
	return []string{}
}

func getBuildSystemFromArtifact(analyzeResult *AnalyzeArtifact) string {
	if analyzeResult.BuildCommand != "" {
		// Infer build system from build command
		if strings.Contains(analyzeResult.BuildCommand, "npm") {
			return "npm"
		}
		if strings.Contains(analyzeResult.BuildCommand, "yarn") {
			return "yarn"
		}
		if strings.Contains(analyzeResult.BuildCommand, "mvn") {
			return "maven"
		}
		if strings.Contains(analyzeResult.BuildCommand, "gradle") {
			return "gradle"
		}
	}
	return "auto-detect"
}

// generateDockerfileWithCritique generates a Dockerfile with validation and critique loop
func generateDockerfileWithCritique(ctx context.Context, templateParams map[string]interface{}, deps ToolDependencies, maxRetries int) (*DockerfileGenerationResult, *validation.Result, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Generate Dockerfile
		dockerfileResult, err := generateDockerfileWithAI(ctx, templateParams, deps)
		if err != nil {
			return nil, nil, err
		}

		// Validate generated content using the comprehensive validation from validation_handlers.go
		validationResult := validation.NewResult()
		validationResult.Stats["lines"] = strings.Count(dockerfileResult.Dockerfile, "\n") + 1
		validationResult.Stats["size"] = len(dockerfileResult.Dockerfile)

		// Perform comprehensive validation using existing functions
		validateDockerfileSyntax(dockerfileResult.Dockerfile, validationResult)
		validateDockerfileSecurity(dockerfileResult.Dockerfile, validationResult)
		validateDockerfileBestPractices(dockerfileResult.Dockerfile, validationResult)

		// Calculate quality score
		validationResult.CalculateQualityScore()

		// If valid, return result
		if validationResult.IsValid {
			return dockerfileResult, validationResult, nil
		}

		// If invalid and we have retries left, use critique to fix
		if attempt < maxRetries {
			fixedResult, err := critiqueAndFixDockerfile(ctx, dockerfileResult, validationResult, deps)
			if err == nil && fixedResult != nil {
				// Re-validate the fixed result
				fixedValidation := validation.NewResult()
				validateDockerfileSyntax(fixedResult.Dockerfile, fixedValidation)
				validateDockerfileSecurity(fixedResult.Dockerfile, fixedValidation)
				validateDockerfileBestPractices(fixedResult.Dockerfile, fixedValidation)
				fixedValidation.CalculateQualityScore()

				if fixedValidation.IsValid {
					return fixedResult, fixedValidation, nil
				}
			}
			// If critique failed or didn't fix issues, continue to next attempt
		}
	}

	// Final attempt failed
	return nil, nil, fmt.Errorf("failed to generate valid dockerfile after %d attempts", maxRetries)
}

// critiqueAndFixDockerfile uses the dockerfile-critique template to fix validation issues
func critiqueAndFixDockerfile(ctx context.Context, original *DockerfileGenerationResult, validationResult *validation.Result, deps ToolDependencies) (*DockerfileGenerationResult, error) {
	// Load critique template
	template, err := deps.PromptManager.GetTemplate("dockerfile-critique")
	if err != nil {
		return nil, err
	}

	// Prepare critique parameters
	critiqueParams := map[string]interface{}{
		"OriginalContent":    original.Dockerfile,
		"ValidationFindings": validationResult.Findings,
		"QualityScore":       validationResult.QualityScore,
		"ErrorCount":         validationResult.ErrorCount(),
		"WarningCount":       validationResult.WarningCount(),
	}

	// Render critique prompt
	rendered, err := template.Render(critiqueParams)
	if err != nil {
		return nil, err
	}

	// Generate fixed version
	samplingReq := sampling.SamplingRequest{
		Prompt:       rendered.Content,
		SystemPrompt: rendered.SystemPrompt,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
	}

	var fixedResult DockerfileGenerationResult
	_, err = deps.SamplingClient.SampleJSONWithSchema(ctx, samplingReq, &fixedResult, getDockerfileGenerationSchema())
	return &fixedResult, err
}
