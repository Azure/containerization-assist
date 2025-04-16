package runner

import (
	"container-copilot/utils"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// checkPodStatus verifies if pods from the deployment are running correctly
func (c *Clients) checkPodStatus(namespace string, labelSelector string, timeout time.Duration) (bool, string) {
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		outputStr, err := c.Kube.GetPods(namespace)
		//fmt.Println("Kubectl get pods output:", string(output))
		if err != nil {
			return false, fmt.Sprintf("Error checking pod status: %v\nOutput: %s", err, outputStr)
		}

		// Check for problematic pod states in the output
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

func checkKubectlInstalled() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl executable not found in PATH. Please install kubectl or ensure it's available in your PATH")
	}
	return nil
}

// deployAndVerifySingleManifest applies a single manifest and verifies pod health
func (c *Clients) deployAndVerifySingleManifest(manifestPath string, isDeployment bool) (bool, string, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, "", fmt.Errorf("reading manifest file: %w", err)
	}
	_, err = readK8sObjects(content)
	if err != nil {
		return false, "", fmt.Errorf("reading k8s objects from manifest file %s: %w", manifestPath, err)
	}

	// Apply the manifest
	outputStr, err := c.Kube.Apply(manifestPath)

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
	podSuccess, podOutput := c.checkPodStatus(namespace, labelSelector, time.Minute)
	if !podSuccess {
		fmt.Printf("Pods are not healthy for deployment with manifest %s\n", manifestPath)
		return false, outputStr + "\n" + podOutput, nil
	}
	fmt.Println("Pod health check passed")

	return true, outputStr, nil
}

// deployStateManifests deploys manifests from pipeline state
func (c *Clients) deployStateManifests(state *PipelineState) error {
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
		manifest := state.K8sObjects[name]

		// Write manifest to temporary file
		tmpFile := filepath.Join(tmpDir, name)
		if err := os.WriteFile(tmpFile, []byte(manifest.Content), 0644); err != nil {
			return fmt.Errorf("failed to write manifest %s: %v", name, err)
		}

		// Use existing deployment verification
		success, output, err := c.deployAndVerifySingleManifest(tmpFile, manifest.IsDeploymentType)
		if err != nil {
			return fmt.Errorf("error deploying manifest %s: %v", name, err)
		}

		if !success {
			manifest.errorLog = output
			manifest.IsSuccessfullyDeployed = false
			fmt.Printf("Failed to deploy manifest %s\n", name)
			failedManifests = append(failedManifests, name)
			continue
		}

		fmt.Printf("Successfully deployed manifest: %s\n", name)
		manifest.IsSuccessfullyDeployed = true
		manifest.errorLog = ""
	}

	// Return error if any manifests failed to deploy
	if len(failedManifests) > 0 {
		return fmt.Errorf("failed to deploy manifests: %v", failedManifests)
	}

	return nil
}

// Update iterateMultipleManifestsDeploy to use the new deployment function
func (c *Clients) iterateMultipleManifestsDeploy(maxIterations int, state *PipelineState, targetDir string) error {
	fmt.Printf("Starting Kubernetes manifest deployment iteration process\n")

	if err := checkKubectlInstalled(); err != nil {
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
				ErrorMessages: thisObject.errorLog,
				FilePath:      thisObject.ManifestPath,
				//Repo tree is currently not provided to the prompt
			}

			failedImagePull := strings.Contains(thisObject.errorLog, "ImagePullBackOff")
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
		err := c.deployStateManifests(state)
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
		if err := writeIterationSnapshot(state, targetDir); err != nil {
			return fmt.Errorf("writing iteration snapshot: %w", err)
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed to fix Kubernetes manifests after %d iterations", maxIterations)
}

func GetDeploymentLogs(deploymentName string, namespace string) error {
	// Loading kubeconfig from default location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Get the Deployment
	// Note only please: We may want to handle the case where the deployment does not exist
	// or is not found in the specified namespace
	// This is a simplified example and may need to be adjusted based on our needs
	deployClient := client.AppsV1().Deployments(namespace)
	deployment, err := deployClient.Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get the matching pods
	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: deployment.Spec.Selector.MatchLabels})
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s", deploymentName)
	}

	// Print logs for the first pod (or loop through all if desired)
	for _, pod := range pods.Items {
		fmt.Printf("Logs for Pod: %s\n", pod.Name)
		err := streamPodLogs(client, namespace, pod.Name)
		if err != nil {
			log.Printf("Error getting logs for pod %s: %v\n", pod.Name, err)
		}
	}

	return nil
}

// streamPodLogs prints logs to stdout
func streamPodLogs(clientset *kubernetes.Clientset, namespace, podName string) error {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	stream, err := req.Stream(context.Background())
	if err != nil {
		return fmt.Errorf("error opening log stream: %w", err)
	}
	defer stream.Close()

	_, err = io.Copy(os.Stdout, stream)
	return err
}
