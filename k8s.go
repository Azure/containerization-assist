package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"io/ioutil"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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

func analyzeKubernetesManifest(client *azopenai.Client, deploymentID string, input FileAnalysisInput) (*FileAnalysisResult, error) {
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

	// Add repository file information if provided
	if input.RepoFileTree != "" {
		promptText += fmt.Sprintf(`
Repository files structure:
%s
`, input.RepoFileTree)
	}

	promptText += `
Please:
1. Identify any issues in the Kubernetes manifest
2. Provide a fixed version of the manifest
3. Explain what changes were made and why

Output the fixed manifest between <<<MANIFEST>>> tags.`

	resp, err := client.GetChatCompletions(
		context.Background(),
		azopenai.ChatCompletionsOptions{
			DeploymentName: to.Ptr(deploymentID),
			Messages: []azopenai.ChatRequestMessageClassification{
				&azopenai.ChatRequestUserMessage{
					Content: azopenai.NewChatRequestUserMessageContent(promptText),
				},
			},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		content := *resp.Choices[0].Message.Content

		// Extract the fixed manifest from between the tags
		re := regexp.MustCompile(`<<<MANIFEST>>>([\s\S]*?)<<<MANIFEST>>>`)
		matches := re.FindStringSubmatch(content)

		fixedContent := ""
		if len(matches) > 1 {
			// Found the manifest between tags
			fixedContent = strings.TrimSpace(matches[1])
		} else {
			// If tags aren't found, try to extract the content intelligently
			apiVersionRe := regexp.MustCompile(`(?m)^apiVersion:[\s\S]*?$`)
			if apiVersionMatches := apiVersionRe.FindString(content); apiVersionMatches != "" {
				// Simple heuristic: Start from apiVersion
				fixedContent = apiVersionMatches
			} else {
				// Fallback: use the entire content
				fixedContent = content
			}
		}

		return &FileAnalysisResult{
			FixedContent: fixedContent,
			Analysis:     content,
		}, nil
	}

	return nil, fmt.Errorf("no response from AI model")
}

func checkKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl or ensure it's available in your PATH")
	}
	return nil
}

// deployKubernetesManifests attempts to deploy multiple Kubernetes manifests and track results for each
func deployKubernetesManifests(manifestPaths []string) (bool, []ManifestDeployResult) {
	results := make([]ManifestDeployResult, len(manifestPaths))
	overallSuccess := true

	// Deploy each manifest individually to track errors per manifest
	for i, path := range manifestPaths {
		fmt.Printf("Deploying manifest: %s\n", path)
		success, outputStr := deployAndVerifySingleManifest(path)

		// Record the result
		results[i] = ManifestDeployResult{
			Path:    path,
			Success: success,
			Output:  outputStr,
		}

		if !success {
			overallSuccess = false
			fmt.Printf("Deployment failed for %s\n", path)
		} else {
			fmt.Printf("Successfully deployed %s\n", path)
		}
	}

	return overallSuccess, results
}

// deployAndVerifySingleManifest applies a single manifest and verifies pod health
func deployAndVerifySingleManifest(manifestPath string) (bool, string) {
	// Apply the manifest
	cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		fmt.Printf("Kubernetes deployment failed for %s with error: %v\n", manifestPath, err)
		return false, outputStr
	}

	fmt.Printf("Successfully applied %s\n", manifestPath)

	// Only check pod status for deployment.yaml files
	baseFilename := filepath.Base(manifestPath)
	if !IsK8sDeployment(manifestPath) {
		fmt.Printf("Skipping pod health check for non-deployment manifest: %s\n", baseFilename)
		return true, outputStr
	}
	// Check if the manifest is a deployment
	if IsK8sDeployment(manifestPath) {
		fmt.Printf("Checking pod health for deployment...\n")

		// Extract namespace and app labels from the manifest
		// This is simplified - would need to actually take this from the manifest
		namespace := "default"        // Default namespace
		labelSelector := "app=my-app" // Default label selector

		// Wait for pods to become healthy
		podSuccess, podOutput := checkPodStatus(namespace, labelSelector, time.Minute)
		if !podSuccess {
			fmt.Printf("Pods are not healthy: %s\n", podOutput)
			return false, outputStr + "\n" + podOutput
		}
		fmt.Println("Pod health check passed")
	} else {
		fmt.Printf("Skipping pod health check for non-deployment manifest: %s\n", baseFilename)
	}

	return true, outputStr
}

// IsK8sDeployment checks if a given YAML file is a Kubernetes Deployment Version 1
func IsK8sDeployment(filePath string) bool {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return false
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal(data, &obj); err != nil {
		log.Printf("Error parsing YAML: %v", err)
		return false
	}

	u := &unstructured.Unstructured{Object: obj}
	return u.GetKind() == "Deployment"
}

// IsK8sDeployment checks if a given YAML file is a Kubernetes Deployment Version 2
func IsK8sDeploymentUsingInstance(filePath string) bool {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return false
	}

	var documents []map[string]interface{}
	if err := yaml.Unmarshal(data, &documents); err != nil {
		log.Printf("Error parsing YAML: %v", err)
		return false
	}

	for _, doc := range documents {
		if doc != nil {
			u := &unstructured.Unstructured{Object: doc}
			if u.GetKind() == "Deployment" && u.GetAPIVersion() != "" && doc["spec"] != nil {
				return true
			}
		}
	}

	return false
}


// iterateMultipleManifestsDeploy attempts to iteratively fix and deploy multiple Kubernetes manifests
// Once a manifest is succesfully deployed, it is removed from the list of pending manifests
func iterateMultipleManifestsDeploy(client *azopenai.Client, deploymentID string, manifestDir string, fileStructurePath string, maxIterations int) error {
	fmt.Printf("Starting Kubernetes manifest deployment iteration process for: %s\n", manifestDir)

	// Check if kubectl is installed before starting the iteration process
	if err := checkKubectlInstalled(); err != nil {
		return err
	}

	// Find all Kubernetes manifest files
	manifestPaths, err := findKubernetesManifests(manifestDir)
	if err != nil {
		return err
	}

	if len(manifestPaths) == 0 {
		return fmt.Errorf("no manifest files found at %s", manifestDir)
	}

	fmt.Printf("Found %d manifest file(s) to deploy\n", len(manifestPaths))
	for i, path := range manifestPaths {
		fmt.Printf("%d. %s\n", i+1, path)
	}

	// Get repository structure
	repoStructure, err := os.ReadFile(fileStructurePath)
	if err != nil {
		return fmt.Errorf("error reading repository structure: %v", err)
	}

	// Load all manifest contents
	manifests := make(map[string]string)
	for _, path := range manifestPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading manifest %s: %v", path, err)
		}
		manifests[path] = string(content)
	}

	// Track which manifests still need to be deployed
	pendingManifests := make(map[string]bool)
	for _, path := range manifestPaths {
		pendingManifests[path] = true
	}

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\n=== Iteration %d of %d ===\n", i+1, maxIterations)

		// Get the list of manifests that are still pending
		var currentManifests []string
		for path := range pendingManifests {
			currentManifests = append(currentManifests, path)
		}

		// If no manifests are pending, we're done
		if len(currentManifests) == 0 {
			fmt.Println("🎉 All Kubernetes manifests deployed successfully!")
			return nil
		}

		// Try to deploy only pending manifests
		fmt.Printf("Attempting to deploy %d manifest(s)...\n", len(currentManifests))
		_, deployResults := deployKubernetesManifests(currentManifests)

		// Create a map to quickly look up results by path
		resultsByPath := make(map[string]ManifestDeployResult)
		for _, result := range deployResults {
			resultsByPath[result.Path] = result
			// Remove successfully deployed manifests from pending list
			if result.Success {
				fmt.Printf("✅ Successfully deployed: %s\n", result.Path)
				delete(pendingManifests, result.Path)
			}
		}

		// If no pending manifests remain, we're done
		if len(pendingManifests) == 0 {
			fmt.Println("🎉 All Kubernetes manifests deployed successfully!")
			return nil
		}

		fmt.Printf("🔄 %d manifests still need fixing. Using AI to fix issues...\n", len(pendingManifests))

		// Fix each manifest file that still has issues
		for path := range pendingManifests {
			content := manifests[path]
			fmt.Printf("\nAnalyzing and fixing: %s\n", path)

			// Use the specific error output for this manifest
			specificErrors := resultsByPath[path].Output

			// Include information about other manifest files that may be related
			var contextInfo strings.Builder
			contextInfo.WriteString("Other manifests in the same deployment:\n")
			for otherPath := range manifests {
				if otherPath != path {
					contextInfo.WriteString(fmt.Sprintf("- %s\n", filepath.Base(otherPath)))
				}
			}

			// Prepare input for AI analysis with specific error information
			input := FileAnalysisInput{
				Content:       content,
				ErrorMessages: specificErrors,
				RepoFileTree:  string(repoStructure),
				FilePath:      path,
			}

			// Get AI to fix the manifest
			fixResult, err := analyzeKubernetesManifest(client, deploymentID, input)
			if err != nil {
				return fmt.Errorf("error in AI analysis for %s: %v", path, err)
			}

			// Update the manifest content in our map
			manifests[path] = fixResult.FixedContent
			fmt.Println("AI suggested fixes for", path)
			fmt.Println(fixResult.Analysis)

			// Write the fixed manifest
			if err := os.WriteFile(path, []byte(fixResult.FixedContent), 0644); err != nil {
				return fmt.Errorf("error writing fixed Kubernetes manifest %s: %v", path, err)
			}
		}

		fmt.Printf("Updated Kubernetes manifests with errors. Attempting deployment again...\n")
		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Kubernetes manifests after %d iterations; %d manifests still have issues", maxIterations, len(pendingManifests))
}
