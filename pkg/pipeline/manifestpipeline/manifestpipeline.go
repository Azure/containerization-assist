package manifestpipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

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

	promptText += `
Please:
1. Identify any issues in the Kubernetes manifest
2. Provide a fixed version of the manifest
3. Explain what changes were made and why

Do NOT create brand new manifests - Only fix the provided manifest.
IMPORTANT: Do NOT change the name of the app or the name of the container image.

Output the fixed manifest content between <MANIFEST> and </MANIFEST> tags. These tags must not appear anywhere else in your response except for wrapping the corrected manifest content.`

	content, err := client.GetChatCompletion(ctx, promptText)
	if err != nil {
		return nil, err
	}

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
func DeployStateManifests(state *pipeline.PipelineState, c *clients.Clients) error {
	pendingManifests := GetPendingManifests(state)
	if len(pendingManifests) == 0 {
		fmt.Println("No pending manifests to deploy")
		return nil
	}

	fmt.Printf("Attempting to deploy %d manifests\n", len(pendingManifests))

	var failedManifests []string

	// Deploy each pending manifest using existing verification
	for name := range pendingManifests {
		manifest := state.K8sObjects[name]

		// Overwrite the original manifest file in place
		manifestPath := manifest.ManifestPath
		if err := os.WriteFile(manifestPath, manifest.Content, 0644); err != nil {
			return fmt.Errorf("failed to write manifest %s: %v", name, err)
		}
		success, output, err := c.DeployAndVerifySingleManifest(manifestPath, manifest.IsDeployment())
		if err != nil {
			return fmt.Errorf("error deploying manifest %s: %v", name, err)
		}

		if !success {
			manifest.ErrorLog = output
			manifest.IsSuccessfullyDeployed = false
			fmt.Printf("Failed to deploy manifest %s\n", name)
			failedManifests = append(failedManifests, name)
			continue
		}

		fmt.Printf("Successfully deployed manifest: %s\n", name)
		manifest.IsSuccessfullyDeployed = true
		manifest.ErrorLog = ""
	}

	// Return error if any manifests failed to deploy
	if len(failedManifests) > 0 {
		return fmt.Errorf("failed to deploy manifests: %v", failedManifests)
	}

	return nil
}

// ManifestPipeline implements the pipeline.Pipeline interface for Kubernetes manifests
type ManifestPipeline struct {
	AIClient *ai.AzOpenAIClient
	Parser   pipeline.Parser
}

// Generate creates Kubernetes manifests if needed
func (p *ManifestPipeline) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	if state.RegistryURL == "" || state.ImageName == "" {
		return fmt.Errorf("registry URL or image name not provided in state")
	}

	// Check if manifests already exist
	k8sObjects, err := k8s.FindK8sObjects(targetDir)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	// If no manifests exist, generate them using Draft
	if len(k8sObjects) == 0 {
		fmt.Printf("No existing Kubernetes manifests found, generating manifests...\n")

		// Generate the manifests using Draft
		registryAndImage := fmt.Sprintf("%s/%s", state.RegistryURL, state.ImageName)
		if err := docker.GenerateDeploymentFilesWithDraft(targetDir, registryAndImage); err != nil {
			return fmt.Errorf("generating deployment files with Draft: %w", err)
		}

		// Re-scan for the newly generated manifests
		k8sObjects, err = k8s.FindK8sObjects(targetDir)
		if err != nil {
			return fmt.Errorf("failed to find generated manifests: %w", err)
		}

		if len(k8sObjects) == 0 {
			return fmt.Errorf("no Kubernetes manifests were generated")
		}

		fmt.Printf("Successfully generated %d Kubernetes manifests\n", len(k8sObjects))
	} else {
		fmt.Printf("Found %d existing Kubernetes manifests in %s\n", len(k8sObjects), targetDir)
	}

	// Initialize manifests in the state
	return InitializeManifests(state, targetDir)
}

// GetErrors returns a formatted string of all manifest errors
func (p *ManifestPipeline) GetErrors(state *pipeline.PipelineState) string {
	return FormatManifestErrors(state)
}

// WriteSuccessfulFiles writes successful manifests to disk
func (p *ManifestPipeline) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	anyWritten := false

	// Write any successfully deployed manifests regardless of global state.Success
	for name, object := range state.K8sObjects {
		if object.IsSuccessfullyDeployed && object.ManifestPath != "" && len(object.Content) > 0 {
			fmt.Printf("Writing updated manifest: %s\n", name)
			if err := os.WriteFile(object.ManifestPath, object.Content, 0644); err != nil {
				fmt.Printf("Error writing manifest %s: %v\n", name, err)
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
func (p *ManifestPipeline) Run(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}, options pipeline.RunnerOptions) error {
	// Type assertion for clients
	c, ok := clientsObj.(*clients.Clients)
	if !ok {
		return fmt.Errorf("invalid clients type")
	}

	maxIterations := options.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 5 // Default
	}

	targetDir := options.TargetDirectory
	generateSnapshot := options.GenerateSnapshot

	fmt.Printf("Starting Kubernetes manifest deployment iteration process\n")

	if err := k8s.CheckKubectlInstalled(); err != nil {
		return err
	}

	if len(state.K8sObjects) == 0 {
		return fmt.Errorf("no manifest files found in state")
	}

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\n=== Manifests Iteration %d of %d ===\n", i+1, maxIterations)
		state.IterationCount += 1

		// Fix each manifest that still has issues
		pendingObjects := GetPendingManifests(state)
		for name := range pendingObjects {
			thisObject := state.K8sObjects[name]
			fmt.Printf("\nAnalyzing and fixing: %s\n", name)

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
			fmt.Printf("AI suggested fixes for %s\n", name)
			fmt.Println(result.Analysis)
		}
		fmt.Println("Updated manifests with fixes. Attempting deployment...")

		// Try to deploy pending manifests
		err := DeployStateManifests(state, c)
		if err == nil {
			// All manifests deployed successfully, but don't set global success state
			// as that's handled by the central pipeline orchestrator
			fmt.Printf("🎉 All Kubernetes manifests deployed successfully!\n")

			if generateSnapshot {
				if err := pipeline.WriteIterationSnapshot(state, targetDir, p); err != nil {
					return fmt.Errorf("writing iteration snapshot: %w", err)
				}
			}
			return nil
		}

		if i < maxIterations-1 {
			fmt.Printf("🔄 Some manifests failed to deploy. Using AI to fix issues...\n")
			// Log status of each manifest
			for name, thisObject := range state.K8sObjects {
				if thisObject.IsSuccessfullyDeployed {
					fmt.Printf("  ✅ %s kind:%s source:%s\n", name, thisObject.Kind, thisObject.ManifestPath)
				} else {
					fmt.Printf("  ❌ %s kind:%s source:%s\n", name, thisObject.Kind, thisObject.ManifestPath)
				}
			}
		}

		if generateSnapshot {
			if err := pipeline.WriteIterationSnapshot(state, targetDir, p); err != nil {
				return fmt.Errorf("writing iteration snapshot: %w", err)
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed to fix Kubernetes manifests after %d iterations", maxIterations)
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

	fmt.Printf("Found %d Kubernetes objects from %s\n", len(k8sObjects), path)
	for _, obj := range k8sObjects {
		fmt.Printf("  '%s' kind: %s source: %s\n", obj.Metadata.Name, obj.Kind, obj.ManifestPath)
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
func (p *ManifestPipeline) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// For manifest pipeline, path should be the directory containing manifests
	return InitializeManifests(state, path)
}

// Deploy handles deploying Kubernetes manifests
func (p *ManifestPipeline) Deploy(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}) error {
	// Type assertion for clients
	c, ok := clientsObj.(*clients.Clients)
	if !ok {
		return fmt.Errorf("invalid clients type")
	}

	fmt.Printf("Deploying Kubernetes manifests...\n")
	return DeployStateManifests(state, c)
}
