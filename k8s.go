package main

import (
	"container-copilot/utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// checkPodStatus verifies if pods from the deployment are running correctly
func checkPodStatus(namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "json")
		output, err := cmd.CombinedOutput()

		//fmt.Println("Kubectl get pods output:", string(output))
		if err != nil {
			return false, fmt.Sprintf("Error checking pod status: %v\nOutput: %s", err, string(output))
		}

		// Check for problematic pod states in the output
		outputStr := string(output)
		if strings.Contains(outputStr, "CrashLoopBackOff") ||
			strings.Contains(outputStr, "Error") ||
			strings.Contains(outputStr, "ImagePullBackOff") {
			return false, fmt.Sprintf("Pods are in a failed state:\n%s", outputStr)
		}

		// Check if all pods are running and ready
		if strings.Contains(outputStr, "\"phase\": \"Running\"") && !strings.Contains(outputStr, "\"ready\": false") {
			return true, "All pods are running and ready"
		}

		// Wait before checking again
		time.Sleep(5 * time.Second)
	}

	return false, "Timeout waiting for pods to become ready"
}

func analyzeKubernetesManifest(client *AzOpenAIClient, input FileAnalysisInput, state *PipelineState) (*FileAnalysisResult, error) {
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

func checkKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl or ensure it's available in your PATH")
	}
	return nil
}

// deployAndVerifySingleManifest applies a single manifest and verifies pod health
func deployAndVerifySingleManifest(manifestPath string, isDeployment bool) (bool, string, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, "", fmt.Errorf("reading manifest file: %w", err)
	}
	_, err = readK8sObjects(content)
	if err != nil {
		return false, "", fmt.Errorf("reading k8s objects from manifest file %s: %w", manifestPath, err)
	}

	// Apply the manifest
	cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		fmt.Printf("Kubernetes deployment failed for %s with error: %v\n", manifestPath, err)
		return false, outputStr, nil
	}

	fmt.Printf("Successfully applied %s\n", manifestPath)

	// Only check pod status for deployment.yaml files
	baseFilename := filepath.Base(manifestPath)
	if !isDeployment {
		fmt.Printf("Skipping pod health check for non-deployment manifest: %s\n", baseFilename)
		return true, outputStr, nil
	}

	fmt.Printf("Checking pod health for deployment...\n")

	// Extract namespace and app labels from the manifest
	// This is simplified - would need to actually take this from the manifest
	namespace := "default"        // Default namespace
	labelSelector := "app=my-app" // Default label selector

	// Wait for pods to become healthy
	podSuccess, podOutput := checkPodStatus(namespace, labelSelector, time.Minute)
	if !podSuccess {
		fmt.Printf("Pods are not healthy for deployment with manifest %s\n", manifestPath)
		return false, outputStr + "\n" + podOutput, nil
	}
	fmt.Println("Pod health check passed")

	return true, outputStr, nil
}

// deployStateManifests deploys manifests from pipeline state
func deployStateManifests(state *PipelineState) error {
	// Create a temporary directory for manifest files
	tmpDir, err := os.MkdirTemp("", "container-copilot-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pendingManifests := GetPendingManifests(state)
	if len(pendingManifests) == 0 {
		fmt.Println("No pending manifests to deploy")
		return nil
	}

	fmt.Printf("Attempting to deploy %d manifests\n", len(pendingManifests))

	var failedManifests []string

	// Deploy each pending manifest using existing verification
	for name := range pendingManifests {
		manifest := state.K8sManifests[name]

		// Write manifest to temporary file
		tmpFile := filepath.Join(tmpDir, name)
		if err := os.WriteFile(tmpFile, []byte(manifest.Content), 0644); err != nil {
			return fmt.Errorf("failed to write manifest %s: %v", name, err)
		}

		// Use existing deployment verification
		success, output, err := deployAndVerifySingleManifest(tmpFile, manifest.isDeploymentType)
		if err != nil {
			return fmt.Errorf("error deploying manifest %s: %v", name, err)
		}

		if !success {
			manifest.errorLog = output
			manifest.isDeployed = false
			fmt.Printf("Failed to deploy manifest %s\n", name)
			failedManifests = append(failedManifests, name)
			continue
		}

		fmt.Printf("Successfully deployed manifest: %s\n", name)
		manifest.isDeployed = true
		manifest.errorLog = ""
	}

	// Return error if any manifests failed to deploy
	if len(failedManifests) > 0 {
		return fmt.Errorf("failed to deploy manifests: %v", failedManifests)
	}

	return nil
}

// Update iterateMultipleManifestsDeploy to use the new deployment function
func iterateMultipleManifestsDeploy(client *AzOpenAIClient, maxIterations int, state *PipelineState) error {
	fmt.Printf("Starting Kubernetes manifest deployment iteration process\n")

	if err := checkKubectlInstalled(); err != nil {
		return err
	}

	if len(state.K8sManifests) == 0 {
		return fmt.Errorf("no manifest files found in state")
	}

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\n=== Manifests Iteration %d of %d ===\n", i+1, maxIterations)

		// Fix each manifest that still has issues
		pendingManifests := GetPendingManifests(state)
		for name := range pendingManifests {
			manifest := state.K8sManifests[name]
			fmt.Printf("\nAnalyzing and fixing: %s\n", name)

			input := FileAnalysisInput{
				Content:       manifest.Content,
				ErrorMessages: manifest.errorLog,
				FilePath:      manifest.Path,
				//Repo tree is currently not provided to the prompt
			}

			failedImagePull := strings.Contains(manifest.errorLog, "ImagePullBackOff")
			if failedImagePull {
				return fmt.Errorf("ImagePullBackOff error detected in manifest %s. Skipping AI analysis.\n", name)
			}

			// Pass the entire state instead of just the Dockerfile
			result, err := analyzeKubernetesManifest(client, input, state)
			if err != nil {
				return fmt.Errorf("error in AI analysis for %s: %v", name, err)
			}

			manifest.Content = result.FixedContent
			fmt.Printf("AI suggested fixes for %s\n", name)
			fmt.Println(result.Analysis)
		}
		fmt.Println("Updated manifests with fixes. Attempting deployment...")

		// Try to deploy pending manifests
		err := deployStateManifests(state)
		if err == nil {
			state.Success = true
			fmt.Printf("🎉 All Kubernetes manifests deployed successfully!")
			return nil
		}

		if i < maxIterations-1 {
			fmt.Printf("🔄 Some manifests failed to deploy. Using AI to fix issues...\n")
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed to fix Kubernetes manifests after %d iterations", maxIterations)
}
