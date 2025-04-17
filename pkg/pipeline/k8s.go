package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/utils"
)

// FormatManifestErrors returns a string containing all manifest errors with their names
func FormatManifestErrors(state *PipelineState) string {
	var errorBuilder strings.Builder

	for name, manifest := range state.K8sObjects {
		if manifest.ErrorLog != "" {
			errorBuilder.WriteString(fmt.Sprintf("\nManifest %q:\n%s\n", name, manifest.ErrorLog))
		}
	}

	return errorBuilder.String()
}

// GetPendingManifests returns a map of manifest names that still need to be deployed
func GetPendingManifests(state *PipelineState) map[string]bool {
	pendingManifests := make(map[string]bool)

	for name, manifest := range state.K8sObjects {
		if !manifest.IsSuccessfullyDeployed {
			pendingManifests[name] = true
		}
	}

	return pendingManifests
}

func analyzeKubernetesManifest(client *ai.AzOpenAIClient, input FileAnalysisInput, state *PipelineState) (*FileAnalysisResult, error) {
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

Do not create brand new manifests. Only fix the provided manifest.

Output the fixed manifest between <<<MANIFEST>>> tags.`

	content, err := client.GetChatCompletion(promptText)
	if err != nil {
		return nil, err
	}

	fixedContent, err := utils.GrabContentBetweenTags(content, "MANIFEST")
	if err != nil {
		return nil, fmt.Errorf("failed to extract fixed manifest: %v", err)
	}

	return &FileAnalysisResult{
		FixedContent: fixedContent,
		Analysis:     content,
	}, nil
}

// deployStateManifests deploys manifests from pipeline state
func (s *PipelineState) DeployStateManifests(c *clients.Clients) error {
	// Create a temporary directory for manifest files
	tmpDir, err := os.MkdirTemp("", "container-copilot-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pendingManifests := GetPendingManifests(s)
	if len(pendingManifests) == 0 {
		fmt.Println("No pending manifests to deploy")
		return nil
	}

	fmt.Printf("Attempting to deploy %d manifests\n", len(pendingManifests))

	var failedManifests []string

	// Deploy each pending manifest using existing verification
	for name := range pendingManifests {
		manifest := s.K8sObjects[name]

		// Write manifest to temporary file
		tmpFile := filepath.Join(tmpDir, name)
		if err := os.WriteFile(tmpFile, []byte(manifest.Content), 0644); err != nil {
			return fmt.Errorf("failed to write manifest %s: %v", name, err)
		}

		// Use existing deployment verification
		success, output, err := c.DeployAndVerifySingleManifest(tmpFile, manifest.IsDeployment())
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

// Update iterateMultipleManifestsDeploy to use the new deployment function
func IterateMultipleManifestsDeploy(maxIterations int, state *PipelineState, targetDir string, c *clients.Clients) error {
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

			input := FileAnalysisInput{
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
			result, err := analyzeKubernetesManifest(c.AzOpenAIClient, input, state)
			if err != nil {
				return fmt.Errorf("error in AI analysis for %s: %v", name, err)
			}

			thisObject.Content = []byte(result.FixedContent)
			fmt.Printf("AI suggested fixes for %s\n", name)
			fmt.Println(result.Analysis)
		}
		fmt.Println("Updated manifests with fixes. Attempting deployment...")

		// Try to deploy pending manifests
		err := state.DeployStateManifests(c)
		if err == nil {
			state.Success = true
			fmt.Printf("🎉 All Kubernetes manifests deployed successfully!")
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
		if err := WriteIterationSnapshot(state, targetDir); err != nil {
			return fmt.Errorf("writing iteration snapshot: %w", err)
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed to fix Kubernetes manifests after %d iterations", maxIterations)
}
