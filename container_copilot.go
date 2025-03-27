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

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// ManifestDeployResult stores the result of a single manifest deployment
type ManifestDeployResult struct {
	Path    string
	Success bool
	Output  string
}

// FileAnalysisInput represents the common input structure for file analysis
type FileAnalysisInput struct {
	Content       string `json:"content"` // Plain text content of the file
	ErrorMessages string `json:"error_messages,omitempty"`
	RepoFileTree  string `json:"repo_files,omitempty"` // String representation of the file tree
	FilePath      string `json:"file_path,omitempty"`  // Path to the original file
}

// FileAnalysisResult represents the common analysis result
type FileAnalysisResult struct {
	FixedContent string `json:"fixed_content"`
	Analysis     string `json:"analysis"`
}

func analyzeDockerfile(client *azopenai.Client, deploymentID string, input FileAnalysisInput) (*FileAnalysisResult, error) {
	// Create prompt for analyzing the Dockerfile
	promptText := fmt.Sprintf(`Analyze the following Dockerfile for errors and suggest fixes:
Dockerfile:
%s
`, input.Content)

	// Add error information if provided and not empty
	if input.ErrorMessages != "" {
		promptText += fmt.Sprintf(`
Errors encountered when running this Dockerfile:
%s
`, input.ErrorMessages)
	} else {
		promptText += `
No error messages were provided. Please check for potential issues in the Dockerfile.
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
1. Identify any issues in the Dockerfile
2. Provide a fixed version of the Dockerfile
3. Explain what changes were made and why

Output the fixed Dockerfile between <<<DOCKERFILE>>> tags.`

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

		// Extract the fixed Dockerfile from between the tags
		re := regexp.MustCompile(`<<<DOCKERFILE>>>([\s\S]*?)<<<DOCKERFILE>>>`)
		matches := re.FindStringSubmatch(content)

		fixedContent := ""
		if len(matches) > 1 {
			// Found the dockerfile between tags
			fixedContent = strings.TrimSpace(matches[1])
		} else {
			// If tags aren't found, try to extract the content intelligently
			// Look for multi-line dockerfile content after FROM
			fromRe := regexp.MustCompile(`(?m)^FROM[\s\S]*?$`)
			if fromMatches := fromRe.FindString(content); fromMatches != "" {
				// Simple heuristic: Consider everything from the first FROM as the dockerfile
				fixedContent = fromMatches
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

func checkDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
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
	if baseFilename == "deployment.yaml" {
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

// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func iterateDockerfileBuild(client *azopenai.Client, deploymentID string, dockerfilePath string, repoStructure string, maxIterations int) error {
	fmt.Printf("Starting Dockerfile build iteration process for: %s\n", dockerfilePath)

	// Check if Docker is installed before starting the iteration process
	if err := checkDockerInstalled(); err != nil {
		return err
	}

	// Read the original Dockerfile
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("error reading Dockerfile: %v", err)
	}

	currentDockerfile := string(dockerfileContent)

	for i := range maxIterations {
		fmt.Printf("\n=== Iteration %d of %d ===\n", i+1, maxIterations)

		// Try to build
		success, buildOutput := buildDockerfile(dockerfilePath)
		if success {
			fmt.Println("🎉 Docker build succeeded!")
			fmt.Println("Successful Dockerfile: \n", currentDockerfile)

			//Temp code for pushing to kind registry
			registryName := os.Getenv("REGISTRY")
			cmd := exec.Command("docker", "push", registryName+"/tomcat-hello-world-workflow:latest")
			output, err := cmd.CombinedOutput()
			outputStr := string(output)
			fmt.Println("Output: ", outputStr)

			if err != nil {
				fmt.Println("Registry push failed with error:", err)
				return fmt.Errorf("error pushing to registry: %v", err)
			}

			return nil
		}

		fmt.Println("Docker build failed. Using AI to fix issues...")

		// Prepare input for AI analysis
		input := FileAnalysisInput{
			Content:       currentDockerfile,
			ErrorMessages: buildOutput,
			RepoFileTree:  string(repoStructure),
			FilePath:      dockerfilePath,
		}

		// Get AI to fix the Dockerfile - call analyzeDockerfile directly
		result, err := analyzeDockerfile(client, deploymentID, input)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		currentDockerfile = result.FixedContent
		fmt.Println("AI suggested fixes:")
		fmt.Println(result.Analysis)

		// Write the fixed Dockerfile
		if err := os.WriteFile(dockerfilePath, []byte(currentDockerfile), 0644); err != nil {
			return fmt.Errorf("error writing fixed Dockerfile: %v", err)
		}

		fmt.Printf("Updated Dockerfile written. Attempting build again...\n")
		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
}

// findKubernetesManifests finds all kubernetes manifest files (YAML/YML) at the given path
// Path can be either a directory or a single file
func findKubernetesManifests(path string) ([]string, error) {
	var manifestPaths []string

	// Check if the input is a directory or a file
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error accessing path %s: %v", path, err)
	}

	if fileInfo.IsDir() {
		// It's a directory - find all YAML files
		fmt.Printf("Looking for Kubernetes manifest files in directory: %s\n", path)

		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) {
				manifestPaths = append(manifestPaths, filePath)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking manifest directory: %v", err)
		}
	} else {
		// It's a single file
		fmt.Printf("Using single manifest file: %s\n", path)

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			manifestPaths = append(manifestPaths, path)
		} else {
			return nil, fmt.Errorf("file %s is not a YAML/YML file", path)
		}
	}

	return manifestPaths, nil
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

func main() {
	// Get environment variables
	apiKey := os.Getenv("AZURE_OPENAI_KEY")
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	deploymentID := "o3-mini-2"

	if apiKey == "" || endpoint == "" {
		fmt.Println("Error: AZURE_OPENAI_KEY and AZURE_OPENAI_ENDPOINT environment variables must be set")
		os.Exit(1)
	}

	// Create a client with KeyCredential
	keyCredential := azcore.NewKeyCredential(apiKey)
	client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
	if err != nil {
		fmt.Printf("Error creating Azure OpenAI client: %v\n", err)
		os.Exit(1)
	}

	// Check command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "iterate-dockerfile-build":
			maxIterations := 5
			dockerfilePath := "../../../Dockerfile"
			fileStructurePath := "repo_structure_json.txt"

			// Allow custom dockerfile path
			if len(os.Args) > 2 {
				dockerfilePath = os.Args[2]
			}

			// Allow file structure JSON path
			if len(os.Args) > 3 {
				fileStructurePath = os.Args[3]
			}

			// Allow custom max iterations
			if len(os.Args) > 4 {
				fmt.Sscanf(os.Args[4], "%d", &maxIterations)
			}

			repoStructure, err := getFileTree()
			if err != nil {
				fmt.Printf("failed to get file tree: %s",err.Error())
				os.Exit(1)
			}

			if err := iterateDockerfileBuild(client, deploymentID, dockerfilePath, repoStructure, maxIterations); err != nil {
				fmt.Printf("Error in dockerfile iteration process: %v\n", err)
				os.Exit(1)
			}

		case "iterate-kubernetes-deploy":
			maxIterations := 5
			manifestPath := "../../../manifests" // Default directory containing manifests
			fileStructurePath := "repo_structure_json.txt"

			// Allow custom manifest path (can be a directory or file)
			if len(os.Args) > 2 {
				manifestPath = os.Args[2]
			}

			// Allow file structure path
			if len(os.Args) > 3 {
				fileStructurePath = os.Args[3]
			}

			// Allow custom max iterations
			if len(os.Args) > 4 {
				fmt.Sscanf(os.Args[4], "%d", &maxIterations)
			}

			if err := iterateMultipleManifestsDeploy(client, deploymentID, manifestPath, fileStructurePath, maxIterations); err != nil {
				fmt.Printf("Error in Kubernetes deployment process: %v", err)
				os.Exit(1)
			}

		default:
			// Default behavior - test Azure OpenAI
			resp, err := client.GetChatCompletions(
				context.Background(),
				azopenai.ChatCompletionsOptions{
					DeploymentName: to.Ptr(deploymentID),
					Messages: []azopenai.ChatRequestMessageClassification{
						&azopenai.ChatRequestUserMessage{
							Content: azopenai.NewChatRequestUserMessageContent("Hello Azure OpenAI! Tell me this is working in one short sentence."),
						},
					},
				},
				nil,
			)
			if err != nil {
				fmt.Printf("Error getting chat completions: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Azure OpenAI Test:")
			if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
				fmt.Printf("Response: %s\n", *resp.Choices[0].Message.Content)
			}
		}
		return
	}

	// If no arguments provided, print usage
	fmt.Println("Usage:")
	fmt.Println("  go run container_copilot.go                          - Test Azure OpenAI connection")
	fmt.Println("  go run container_copilot.go iterate-dockerfile-build [dockerfile-path] [file-structure-path] [max-iterations] - Iteratively build and fix a Dockerfile")
	fmt.Println("  go run container_copilot.go iterate-kubernetes-deploy [manifest-path-or-dir] [file-structure-path] [max-iterations] - Iteratively deploy and fix Kubernetes manifest(s)")
}
