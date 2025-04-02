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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// buildDockerfile attempts to build the Docker image and returns any error output
func buildDockerfile(dockerfilePath string) (bool, string) {
	// Get the directory containing the Dockerfile to use as build context
	dockerfileDir := filepath.Dir(dockerfilePath)

	registryName := os.Getenv("REGISTRY")

	// Run Docker build with explicit context path
	// Use the absolute path for the dockerfile and specify the context directory
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", registryName+"/tomcat-hello-world-workflow:latest", dockerfileDir)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		fmt.Println("Docker build failed with error:", err)
		return false, outputStr
	}

	return true, outputStr
}

// buildDockerfileContent builds a Docker image from a string containing Dockerfile contents
func buildDockerfileContent(dockerfileContent string) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "docker-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up

	// Create temporary Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %v", err)
	}

	// Get registry name from environment
	registryName := os.Getenv("REGISTRY")
	if registryName == "" {
		return "", fmt.Errorf("REGISTRY environment variable not set")
	}

	// Build the image using the temporary Dockerfile
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", registryName+"/container-copilot:latest", tmpDir)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return outputStr, fmt.Errorf("docker build failed: %v", err)
	}

	return outputStr, nil
}

func analyzeDockerfile(client *azopenai.Client, deploymentID string, state *PipelineState) (*FileAnalysisResult, error) {
	dockerfile := state.Dockerfile

	// Create prompt for analyzing the Dockerfile
	promptText := fmt.Sprintf(`Analyze the following Dockerfile for errors and suggest fixes:
Dockerfile:
%s
`, dockerfile.Content)

	// Check for manifest deployment errors and add them to the context
	manifestErrors := FormatManifestErrors(state)
	if manifestErrors != "" {
		promptText += fmt.Sprintf(`
IMPORTANT CONTEXT: Kubernetes manifest deployments failed with the following errors.
These deployment failures may indicate issues with the Docker image produced by this Dockerfile:
%s

Please consider these deployment errors when fixing the Dockerfile.
`, manifestErrors)
	}

	// Add error information if provided and not empty
	if dockerfile.BuildErrors != "" {
		promptText += fmt.Sprintf(`
Errors encountered when running this Dockerfile:
%s
`, dockerfile.BuildErrors)
	} else {
		promptText += `
No error messages were provided. Please check for potential issues in the Dockerfile.
`
	}

	// Add repository file information if provided
	if state.RepoFileTree != "" {
		promptText += fmt.Sprintf(`
Repository files structure:
%s
`, state.RepoFileTree)
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
			fmt.Println("Warning: No Dockerfile content found in the response. Attempting to extract it...")
			// If tags aren't found, try to extract the content intelligently
			// Look for multi-line dockerfile content after FROM
			fromRe := regexp.MustCompile(`(?m)^FROM[\s\S]*?$`)
			if fromMatches := fromRe.FindString(content); fromMatches != "" {
				// Simple heuristic: Consider everything from the first FROM as the dockerfile
				fixedContent = fromMatches
			} else {
				fmt.Println("Warning: No Dockerfile content found in the response.")
			}
		}

		return &FileAnalysisResult{
			FixedContent: fixedContent,
			Analysis:     content,
		}, nil
	}

	return nil, fmt.Errorf("no response from AI model")
}

func checkDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
	}
	return nil
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

// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func iterateDockerfileBuildWithPrevErrors(client *azopenai.Client, deploymentID string, state *PipelineState) error { //Will name better, didn't want to change the original function name yet
	fmt.Printf("Starting Dockerfile build iteration process for: %s\n", state.Dockerfile.Path)

	// Check if Docker is installed before starting the iteration process
	if err := checkDockerInstalled(); err != nil { // Need to move this to the start of the pipeline
		return err
	}

	maxIterations := 5
	for i := range maxIterations {
		fmt.Printf("\n=== Iteration %d of %d ===\n", i+1, maxIterations)

		// Try to build
		buildOutput, err := buildDockerfileContent(state.Dockerfile.Content)
		if err == nil {
			fmt.Println("🎉 Docker build succeeded!")
			fmt.Println("Successful Dockerfile: \n", state.Dockerfile.Content)

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

		state.Dockerfile.BuildErrors = buildOutput

		// Get AI to fix the Dockerfile - call analyzeDockerfile directly
		result, err := analyzeDockerfile(client, deploymentID, state)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		state.Dockerfile.Content = result.FixedContent
		fmt.Println("AI suggested fixes:")
		fmt.Println(result.Analysis)

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

func initializeDockerFileState(pipelineState *PipelineState, dockerFilePath string) error {
	// Check if Dockerfile exists
	if _, err := os.Stat(dockerFilePath); err != nil {
		return fmt.Errorf("error checking Dockerfile at path %s: %v", dockerFilePath, err)
	}

	// Read the Dockerfile content
	content, err := os.ReadFile(dockerFilePath)
	if err != nil {
		return fmt.Errorf("error reading Dockerfile at path %s: %v", dockerFilePath, err)
	}

	// Update pipeline state with Dockerfile information
	pipelineState.Dockerfile.Content = string(content)
	pipelineState.Dockerfile.Path = dockerFilePath
	pipelineState.Dockerfile.BuildErrors = ""

	fmt.Printf("Successfully initialized Dockerfile state from: %s\n", dockerFilePath)
	return nil
}
