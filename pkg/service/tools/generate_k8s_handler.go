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

// K8sManifestsGenerationResult represents the JSON structure returned by AI
type K8sManifestsGenerationResult struct {
	Manifests        []K8sManifest `json:"manifests"`
	Namespace        string        `json:"namespace"`
	ServiceName      string        `json:"service_name"`
	ServicePort      int           `json:"service_port"`
	DeploymentName   string        `json:"deployment_name"`
	ConfigMaps       []string      `json:"config_maps,omitempty"`
	Secrets          []string      `json:"secrets,omitempty"`
	ResourceLimits   ResourceSpec  `json:"resource_limits"`
	SecurityFeatures []string      `json:"security_features"`
}

type K8sManifest struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Priority int    `json:"priority"`
}

type ResourceSpec struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// createPromptFirstK8sHandler creates a handler that uses AI generation with validation
func createPromptFirstK8sHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Load workflow state to get analysis and build results
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to load workflow state: %w", err))
			return &result, nil
		}

		if state.Artifacts == nil || state.Artifacts.AnalyzeResult == nil {
			result := createErrorResult(fmt.Errorf("repository analysis not found - run analyze_repository first"))
			return &result, nil
		}

		if state.Artifacts.BuildResult == nil {
			result := createErrorResult(fmt.Errorf("build result not found - run build_image first"))
			return &result, nil
		}

		// Extract data for prompt template
		analyzeResult := state.Artifacts.AnalyzeResult
		buildResult := state.Artifacts.BuildResult

		// Prepare template parameters
		templateParams := map[string]interface{}{
			"AppName":      getAppNameFromArtifact(analyzeResult),
			"ImageRef":     buildResult.ImageRef,
			"Port":         getPortFromArtifact(analyzeResult),
			"Language":     getLanguageFromArtifact(analyzeResult),
			"Framework":    getFrameworkFromArtifact(analyzeResult),
			"Dependencies": getDependenciesFromArtifact(analyzeResult),
		}

		// Generate with critique/retry loop
		k8sResult, validationResult, err := generateK8sManifestsWithCritique(ctx, templateParams, deps, 3)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to generate valid k8s manifests: %w", err))
			return &result, nil
		}

		// Store the result in workflow state
		manifestStrings := make([]string, len(k8sResult.Manifests))
		for i, manifest := range k8sResult.Manifests {
			manifestStrings[i] = manifest.Content
		}

		state.Artifacts.K8sResult = &K8sArtifact{
			Manifests: manifestStrings,
			Namespace: k8sResult.Namespace,
			Endpoint:  fmt.Sprintf("%s:%d", k8sResult.ServiceName, k8sResult.ServicePort),
			Services:  []string{k8sResult.ServiceName},
			Metadata: map[string]interface{}{
				"service_name":      k8sResult.ServiceName,
				"service_port":      k8sResult.ServicePort,
				"deployment_name":   k8sResult.DeploymentName,
				"config_maps":       k8sResult.ConfigMaps,
				"secrets":           k8sResult.Secrets,
				"resource_limits":   k8sResult.ResourceLimits,
				"security_features": k8sResult.SecurityFeatures,
			},
		}

		// Save updated state
		if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
			logger.Error("Failed to save workflow state", "error", err)
		}

		// Create success response
		data := map[string]interface{}{
			"manifests":         k8sResult.Manifests,
			"namespace":         k8sResult.Namespace,
			"service_name":      k8sResult.ServiceName,
			"service_port":      k8sResult.ServicePort,
			"deployment_name":   k8sResult.DeploymentName,
			"resource_limits":   k8sResult.ResourceLimits,
			"security_features": k8sResult.SecurityFeatures,
			"validation_score":  validationResult.QualityScore,
		}

		chainHint := createChainHint("prepare_cluster", "Kubernetes manifests generated and validated successfully. Ready to prepare cluster")

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

// generateK8sManifestsWithAI uses the k8s-generation-json template to create manifests
func generateK8sManifestsWithAI(ctx context.Context, params map[string]interface{}, deps ToolDependencies) (*K8sManifestsGenerationResult, error) {
	// Load the k8s generation template
	template, err := deps.PromptManager.GetTemplate("k8s-generation-json")
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
	var result K8sManifestsGenerationResult
	_, err = deps.SamplingClient.SampleJSONWithSchema(ctx, samplingReq, &result, getK8sGenerationSchema())
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	return &result, nil
}

// validateK8sManifestsContent validates the generated Kubernetes manifests
func validateK8sManifestsContent(k8sResult *K8sManifestsGenerationResult) (*validation.Result, error) {
	result := validation.NewResult()

	// Basic validation
	if len(k8sResult.Manifests) == 0 {
		result.AddError("K8S001", "manifests", "No manifests generated")
		return result, nil
	}

	// Check for required manifest types
	hasDeployment := false
	hasService := false

	for _, manifest := range k8sResult.Manifests {
		if manifest.Kind == "Deployment" {
			hasDeployment = true
		}
		if manifest.Kind == "Service" {
			hasService = true
		}

		// Validate manifest content is not empty
		if manifest.Content == "" {
			result.AddError("K8S002", fmt.Sprintf("%s/%s", manifest.Kind, manifest.Name), "Manifest content is empty")
		}
	}

	if !hasDeployment {
		result.AddError("K8S003", "manifests", "Missing Deployment manifest")
	}
	if !hasService {
		result.Findings = append(result.Findings, validation.Finding{
			Code:     "K8S004",
			Severity: validation.SeverityWarn,
			Path:     "manifests",
			Message:  "Consider adding Service manifest for external access",
		})
	}

	// Add stats
	result.Stats["manifest_count"] = len(k8sResult.Manifests)
	result.Stats["service_port"] = k8sResult.ServicePort

	// Calculate quality score
	if result.IsValid {
		score := 100
		if !hasService {
			score -= 10
		}
		result.QualityScore = score
	}

	return result, nil
}

// getK8sGenerationSchema returns the JSON schema for k8s generation
func getK8sGenerationSchema() string {
	return `{
		"type": "object",
		"required": ["manifests", "namespace", "service_name"],
		"properties": {
			"manifests": {
				"type": "array",
				"minItems": 1,
				"items": {
					"type": "object",
					"required": ["kind", "name", "content"],
					"properties": {
						"kind": {"type": "string"},
						"name": {"type": "string"},
						"content": {"type": "string", "minLength": 10},
						"priority": {"type": "integer"}
					}
				}
			},
			"namespace": {"type": "string", "minLength": 1},
			"service_name": {"type": "string", "minLength": 1},
			"service_port": {"type": "integer", "minimum": 1, "maximum": 65535},
			"deployment_name": {"type": "string"},
			"config_maps": {"type": "array", "items": {"type": "string"}},
			"secrets": {"type": "array", "items": {"type": "string"}},
			"resource_limits": {
				"type": "object",
				"properties": {
					"cpu": {"type": "string"},
					"memory": {"type": "string"}
				}
			},
			"security_features": {
				"type": "array",
				"items": {"type": "string"}
			}
		}
	}`
}

// Helper functions

func getAppNameFromArtifact(analyzeResult *AnalyzeArtifact) string {
	// Extract app name from repo path or use default
	if analyzeResult.RepoPath != "" {
		parts := strings.Split(strings.TrimSuffix(analyzeResult.RepoPath, "/"), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return "my-app"
}

func combineManifests(manifests []K8sManifest) string {
	var combined strings.Builder
	for i, manifest := range manifests {
		if i > 0 {
			combined.WriteString("\n---\n")
		}
		combined.WriteString(manifest.Content)
	}
	return combined.String()
}

// generateK8sManifestsWithCritique generates K8s manifests with validation and critique loop
func generateK8sManifestsWithCritique(ctx context.Context, templateParams map[string]interface{}, deps ToolDependencies, maxRetries int) (*K8sManifestsGenerationResult, *validation.Result, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Generate K8s manifests
		k8sResult, err := generateK8sManifestsWithAI(ctx, templateParams, deps)
		if err != nil {
			return nil, nil, err
		}

		// Validate generated content using comprehensive validation from validation_handlers.go
		validationResult := validation.NewResult()
		validationResult.Stats["manifest_count"] = len(k8sResult.Manifests)
		validationResult.Stats["service_port"] = k8sResult.ServicePort

		// Perform comprehensive validation using existing functions
		combinedContent := combineManifests(k8sResult.Manifests)
		validateK8sManifestSyntax(combinedContent, validationResult)
		validateK8sManifestSecurity(combinedContent, validationResult)
		validateK8sManifestBestPractices(combinedContent, validationResult)

		// Calculate quality score
		validationResult.CalculateQualityScore()

		// If valid, return result
		if validationResult.IsValid {
			return k8sResult, validationResult, nil
		}

		// If invalid and we have retries left, use critique to fix
		if attempt < maxRetries {
			fixedResult, err := critiqueAndFixK8sManifests(ctx, k8sResult, validationResult, deps)
			if err == nil && fixedResult != nil {
				// Re-validate the fixed result
				fixedValidation := validation.NewResult()
				fixedCombined := combineManifests(fixedResult.Manifests)
				validateK8sManifestSyntax(fixedCombined, fixedValidation)
				validateK8sManifestSecurity(fixedCombined, fixedValidation)
				validateK8sManifestBestPractices(fixedCombined, fixedValidation)
				fixedValidation.CalculateQualityScore()

				if fixedValidation.IsValid {
					return fixedResult, fixedValidation, nil
				}
			}
			// If critique failed or didn't fix issues, continue to next attempt
		}
	}

	// Final attempt failed
	return nil, nil, fmt.Errorf("failed to generate valid k8s manifests after %d attempts", maxRetries)
}

// critiqueAndFixK8sManifests uses the k8s-critique template to fix validation issues
func critiqueAndFixK8sManifests(ctx context.Context, original *K8sManifestsGenerationResult, validationResult *validation.Result, deps ToolDependencies) (*K8sManifestsGenerationResult, error) {
	// Load critique template
	template, err := deps.PromptManager.GetTemplate("k8s-critique")
	if err != nil {
		return nil, err
	}

	// Prepare critique parameters
	critiqueParams := map[string]interface{}{
		"OriginalContent":    combineManifests(original.Manifests),
		"ValidationFindings": validationResult.Findings,
		"QualityScore":       validationResult.QualityScore,
		"ErrorCount":         validationResult.ErrorCount(),
		"WarningCount":       validationResult.WarningCount(),
		"Manifests":          original.Manifests,
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

	var fixedResult K8sManifestsGenerationResult
	_, err = deps.SamplingClient.SampleJSONWithSchema(ctx, samplingReq, &fixedResult, getK8sGenerationSchema())
	return &fixedResult, err
}
