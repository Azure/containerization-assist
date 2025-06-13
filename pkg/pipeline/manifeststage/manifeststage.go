package manifeststage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// ManifestStage implements the pipeline.PipelineStage interface for Kubernetes manifests
var _ pipeline.PipelineStage = &ManifestStage{}

// GetPendingManifests returns a map of manifest names that still need to be deployed
func GetPendingManifests(state *pipeline.PipelineState) map[string]bool {
	pendingManifests := make(map[string]bool)

	for name, manifest := range state.K8sObjects {
		if !manifest.IsSuccessfullyDeployed {
			pendingManifests[name] = true
		}
	}

	return pendingManifests
}

// analyzeKubernetesManifest uses AI to analyze and fix Kubernetes manifest content
func analyzeKubernetesManifest(ctx context.Context, client *ai.AzOpenAIClient, input pipeline.FileAnalysisInput, state *pipeline.PipelineState) (*pipeline.FileAnalysisResult, error) {
	// Create prompt for analyzing the Kubernetes manifest
	promptText := fmt.Sprintf(`Analyze the following Kubernetes manifest file for errors and suggest fixes:
Manifest:
%s
`, input.Content)

	// Add error information if provided and not empty
	if input.ErrorMessages != "" {
		promptText += fmt.Sprintf(`
Errors encountered when applying this manifest:
%s
`, input.ErrorMessages)
	} else {
		promptText += `
No error messages were provided. Please check for potential issues in the Kubernetes manifest.
`
	}

	// Add Dockerfile content for reference if available
	if state != nil && state.Dockerfile.Content != "" {
		promptText += fmt.Sprintf(`
Reference Dockerfile for this application:
%s

Consider the Dockerfile when analyzing the Kubernetes manifest, especially for image compatibility, ports, and environment variables.
`, state.Dockerfile.Content)
	}

	// Add repository analysis results if available
	if repoAnalysis, ok := state.Metadata[pipeline.RepoAnalysisResultKey].(string); ok && repoAnalysis != "" {
		promptText += fmt.Sprintf(`
IMPORTANT CONTEXT: The repository has been analyzed and the following information was gathered:
%s

Please use this repository analysis information to ensure Databases are accounted for in the manifest.
`, repoAnalysis)
	}

	promptText += `
Please:
1. Identify any issues in the Kubernetes manifest
2. Provide a fixed version of the manifest
3. Explain what changes were made and why

- Do NOT create brand new manifests - Only fix the provided manifest.
- Verify that the health check paths exist before using httpGet probe; if they dont't, use a tcpSocket probe instead. 
- Prefer using secrets for sensitive information like database passwords and configmap for non-sensitive data. Do NOT use hardcoded values in the manifest.
- For a Spring Boot application, make sure the Actuator dependency is included in the pom.xml before using /actuator/health as the HTTP GET path in the startup probe.
- The default configmap name is 'app-config' and the default secret name is 'secret-ref'. Do NOT change these names while referring to them in the manifests.
IMPORTANT: Do NOT change the name of the app or the name of the container image.`

	promptText += fmt.Sprintf(`
ADDITIONAL CONTEXT (You might not need to use this, so only use it if it is relevant for generating working Kubernetes manifests):
%s`, state.ExtraContext)

	promptText += `
Output the fixed manifest content between <MANIFEST> and </MANIFEST> tags. These tags must not appear anywhere else in your response except for wrapping the corrected manifest content.`

	content, tokenUsage, err := client.GetChatCompletion(ctx, promptText)
	if err != nil {
		return nil, err
	}

	// Accumulate token usage in pipeline state
	state.TokenUsage.PromptTokens += tokenUsage.PromptTokens
	state.TokenUsage.CompletionTokens += tokenUsage.CompletionTokens
	state.TokenUsage.TotalTokens += tokenUsage.TotalTokens

	parser := &pipeline.DefaultParser{}
	fixedContent, err := parser.ExtractContent(content, "MANIFEST")
	if err != nil {
		return nil, fmt.Errorf("failed to extract fixed manifest: %v", err)
	}

	return &pipeline.FileAnalysisResult{
		FixedContent: fixedContent,
		Analysis:     content,
	}, nil
}

// DeployStateManifests deploys manifests from pipeline state
func DeployStateManifests(ctx context.Context, state *pipeline.PipelineState, c *clients.Clients) error {
	pendingManifests := GetPendingManifests(state)
	if len(pendingManifests) == 0 {
		logger.Info("No pending manifests to deploy")
		return nil
	}

	logger.Infof("Attempting to deploy %d manifests", len(pendingManifests))

	var failedManifests []string

	// Deploy each pending manifest using existing verification
	for name := range pendingManifests {
		manifest := state.K8sObjects[name]

		// Overwrite the original manifest file in place
		manifestPath := manifest.ManifestPath
		if err := os.WriteFile(manifestPath, manifest.Content, 0644); err != nil {
			return fmt.Errorf("failed to write manifest %s: %v", name, err)
		}
		logger.Infof("  %s", name)
		success, output, err := c.DeployAndVerifySingleManifest(ctx, manifestPath, manifest.IsDeployment())
		if err != nil {
			return fmt.Errorf("error deploying manifest %s: %v", name, err)
		}

		if !success {
			manifest.ErrorLog = output
			manifest.IsSuccessfullyDeployed = false
			logger.Errorf("Failed to deploy manifest %s", name)
			failedManifests = append(failedManifests, name)
			continue
		}

		logger.Infof("Successfully deployed manifest: %s", name)
		manifest.IsSuccessfullyDeployed = true
		manifest.ErrorLog = ""
	}

	// Return error if any manifests failed to deploy
	if len(failedManifests) > 0 {
		return fmt.Errorf("failed to deploy manifests: %v", failedManifests)
	}

	return nil
}

// ManifestStage implements the pipeline.Pipeline interface for Kubernetes manifests
type ManifestStage struct {
	AIClient *ai.AzOpenAIClient
	Parser   pipeline.Parser
}

// Generate creates Kubernetes manifests if needed
func (p *ManifestStage) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	manifestPath := filepath.Join(targetDir)
	if state.RegistryURL == "" || state.ImageName == "" {
		return fmt.Errorf("registry URL or image name not provided in state")
	}

	// Check if manifests already exist
	k8sObjects, err := k8s.FindK8sObjects(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	// If no manifests exist, generate them using Draft
	if len(k8sObjects) == 0 {
		logger.Info("No existing Kubernetes manifests found, generating manifests...")

		// Generate the manifests using Draft templates
		registryAndImage := fmt.Sprintf("%s/%s", state.RegistryURL, state.ImageName)
		logger.Debugf("Generating manifests with image name %s", registryAndImage)
		if err := k8s.WriteManifestsFromTemplate(k8s.ManifestsBasic, targetDir); err != nil {
			return fmt.Errorf("writing manifests from template: %w", err)
		}

		// Re-scan for the newly generated manifests
		k8sObjects, err = k8s.FindK8sObjects(targetDir)
		if err != nil {
			return fmt.Errorf("failed to find generated manifests: %w", err)
		}

		if len(k8sObjects) == 0 {
			return fmt.Errorf("no Kubernetes manifests were generated")
		}

		logger.Infof("Successfully generated %d Kubernetes manifests", len(k8sObjects))
	} else {
		logger.Infof("Found %d existing Kubernetes manifests in %s", len(k8sObjects), targetDir)
	}

	// Initialize manifests in the state
	return InitializeManifests(state, targetDir)
}

// GetErrors returns a formatted string of all manifest errors
func (p *ManifestStage) GetErrors(state *pipeline.PipelineState) string {
	return FormatManifestErrors(state)
}

// WriteSuccessfulFiles writes successful manifests to disk
func (p *ManifestStage) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	anyWritten := false

	// Write any successfully deployed manifests regardless of global state.Success
	for name, object := range state.K8sObjects {
		if object.IsSuccessfullyDeployed && object.ManifestPath != "" && len(object.Content) > 0 {
			logger.Infof("Writing updated manifest: %s", name)
			if err := os.WriteFile(object.ManifestPath, object.Content, 0644); err != nil {
				logger.Errorf("Error writing manifest %s: %v", name, err)
				continue
			}
			anyWritten = true
		}
	}

	if anyWritten {
		return nil
	}
	return fmt.Errorf("no successful manifests to write")
}

// Run executes the manifest deployment pipeline
func (p *ManifestStage) Run(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}, options pipeline.RunnerOptions) error {
	// Type assertion for clients
	c, ok := clientsObj.(*clients.Clients)
	if !ok {
		return fmt.Errorf("invalid clients type")
	}

	if err := k8s.CheckKubectlInstalled(); err != nil {
		return err
	}

	if len(state.K8sObjects) == 0 {
		return fmt.Errorf("no manifest files found in state")
	}

	// Fix each manifest that still has issues
	pendingObjects := GetPendingManifests(state)
	for name := range pendingObjects {
		thisObject := state.K8sObjects[name]
		logger.Infof("Analyzing and fixing: %s", name)

		input := pipeline.FileAnalysisInput{
			Content:       string(thisObject.Content),
			ErrorMessages: thisObject.ErrorLog,
			FilePath:      thisObject.ManifestPath,
			//Repo tree is currently not provided to the prompt
		}

		failedImagePull := strings.Contains(thisObject.ErrorLog, "ImagePullBackOff")
		if failedImagePull {
			return fmt.Errorf("imagePullBackOff error detected in manifest %s. Skipping AI analysis", name)
		}

		// Pass the entire state instead of just the Dockerfile
		result, err := analyzeKubernetesManifest(ctx, p.AIClient, input, state)
		if err != nil {
			return fmt.Errorf("error in AI analysis for %s: %v", name, err)
		}

		thisObject.Content = []byte(result.FixedContent)
		logger.Debugf("AI suggested fixes for %s", name)
		logger.Debug(result.Analysis)
	}
	logger.Info("Updated manifests with fixes. Attempting deployment...")

	// Try to deploy pending manifests
	err := DeployStateManifests(ctx, state, c)
	if err == nil {
		// All manifests deployed successfully, but don't set global success state
		// as that's handled by the central pipeline orchestrator
		logger.Info("🎉 All Kubernetes manifests deployed successfully!\n")
		return nil
	}

	logger.Info("🔄 Some manifests failed to deploy. Using AI to fix issues...\n")
	// Log status of each manifest
	for name, thisObject := range state.K8sObjects {
		if thisObject.IsSuccessfullyDeployed {
			logger.Infof("  ✅ %s kind:%s source:%s\n", name, thisObject.Kind, thisObject.ManifestPath)
		} else {
			logger.Errorf("  ❌ %s kind:%s source:%s\n", name, thisObject.Kind, thisObject.ManifestPath)
		}
	}

	return fmt.Errorf("failed to fix Kubernetes manifests")
}

// InitializeManifests populates the K8sObjects field in PipelineState with manifests found in the specified path
// If path is empty, the default manifest path will be used
func InitializeManifests(state *pipeline.PipelineState, path string) error {
	k8sObjects, err := k8s.FindK8sObjects(path)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	state.K8sObjects = make(map[string]*k8s.K8sObject)

	if len(k8sObjects) == 0 {
		// No manifests found, but that's okay - just return with empty map
		return nil
	}

	logger.Infof("Found %d Kubernetes objects from %s", len(k8sObjects), path)
	for _, obj := range k8sObjects {
		logger.Debugf("  '%s' kind: %s source: %s", obj.Metadata.Name, obj.Kind, obj.ManifestPath)
	}

	for i := range k8sObjects {
		obj := k8sObjects[i]
		objKey := fmt.Sprintf("%s-%s", obj.Kind, obj.Metadata.Name)
		state.K8sObjects[objKey] = &obj
	}

	return nil
}

// FormatManifestErrors returns a string containing all manifest errors with their names
func FormatManifestErrors(state *pipeline.PipelineState) string {
	var errorBuilder strings.Builder

	for name, manifest := range state.K8sObjects {
		if manifest.ErrorLog != "" {
			errorBuilder.WriteString(fmt.Sprintf("\nManifest %q:\n%s\n", name, manifest.ErrorLog))
		}
	}

	return errorBuilder.String()
}

// Initialize prepares the pipeline state with initial manifest-related values
func (p *ManifestStage) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// For manifest pipeline, path should be the directory containing manifests
	return InitializeManifests(state, path)
}

// Deploy handles deploying Kubernetes manifests
func (p *ManifestStage) Deploy(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}) error {
	// Type assertion for clients
	c, ok := clientsObj.(*clients.Clients)
	if !ok {
		return fmt.Errorf("invalid clients type")
	}

	logger.Info("Deploying Kubernetes manifests...\n")
	return DeployStateManifests(ctx, state, c)
}
