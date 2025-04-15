package main

import (
	"container-copilot/utils"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// buildDockerfileContent builds a Docker image from a string containing Dockerfile contents
func (c *Clients) buildDockerfileContent(dockerfileContent string, targetDir string, registry string, imageName string) (string, error) {
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

	registryPrefix := ""
	if registry != "" {
		registryPrefix = registry + "/"
	}

	// Build the image using the temporary Dockerfile
	fmt.Printf("building docker image with tag '%s%s:latest'\n", registryPrefix, imageName)
	buildErrors, err := c.Docker.Build(dockerfilePath, registryPrefix+imageName+":latest", targetDir)

	if err != nil {
		return buildErrors, fmt.Errorf("docker build failed: %v", err)
	}

	fmt.Printf("built docker image")
	return buildErrors, nil
}

func analyzeDockerfile(client *AzOpenAIClient, state *PipelineState) (*FileAnalysisResult, error) {
	dockerfile := state.Dockerfile

	// Create prompt for analyzing the Dockerfile
	promptText := fmt.Sprintf(`
You are an expert in Dockerfile analysis and debugging.
Your task is to analyze the provided Dockerfile for potential issues and suggest fixes.

Analyze the following Dockerfile for errors and suggest fixes:
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

	fmt.Println("Dockerfile build errors: ", dockerfile.BuildErrors)

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

Favor using the latest base images and best practices for Dockerfile writing
If applicable, use multi-stage builds to reduce image size
Make sure to account for the file structure of the repository

**IMPORTANT Output the fixed Dockerfile between <<<DOCKERFILE>>> tags. IMPORTANT** 

I will tip you if you provide a correct and working Dockerfile.
`

	content, err := client.GetChatCompletion(promptText)
	if err != nil {
		return nil, err
	}

	fixedContent, err := utils.GrabContentBetweenTags(content, "DOCKERFILE")
	if err != nil {
		return nil, fmt.Errorf("failed to extract fixed Dockerfile: %v", err)
	}

	return &FileAnalysisResult{
		FixedContent: fixedContent,
		Analysis:     content,
	}, nil
}

// checkDockerRunning verifies if the Docker daemon is running.
func (c *Clients) checkDockerRunning() error {
	if output, err := c.Docker.Info(); err != nil {
		return fmt.Errorf("Docker daemon is not running. Please start Docker and try again. Error details: %s", string(output))
	}
	return nil
}

// validateRegistryReachable checks if the local Docker registry is reachable.
func validateRegistryReachable(registryURL string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", registryURL))
	if err != nil {
		return fmt.Errorf("failed to reach local registry at %s: %w", registryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("unexpected response from registry: %d", resp.StatusCode)
	}
	return nil
}

func checkDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
	}
	return nil
}

// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func (c *Clients) iterateDockerfileBuild(maxIterations int, state *PipelineState, targetDir string) error {
	fmt.Printf("Starting Dockerfile build iteration process for: %s\n", state.Dockerfile.Path)

	// Check if Docker is installed before starting the iteration process
	if err := checkDockerInstalled(); err != nil { // Need to move this to the start of the pipeline
		return err
	}

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\n=== Dockerfile Iteration %d of %d ===\n", i+1, maxIterations)

		// Get AI to fix the Dockerfile - call analyzeDockerfile directly
		result, err := analyzeDockerfile(c.AzOpenAIClient, state)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		state.Dockerfile.Content = result.FixedContent
		fmt.Println("AI suggested fixes:")
		fmt.Println(result.Analysis)

		fmt.Printf("Updated Dockerfile written. Attempting build again...\n")

		// Try to build
		buildErrors, err := c.buildDockerfileContent(state.Dockerfile.Content, targetDir, state.RegistryURL, state.ImageName)
		if err == nil {
			fmt.Println("🎉 Docker build succeeded!")
			fmt.Println("Successful Dockerfile: \n", state.Dockerfile.Content)

			return nil
		}

		fmt.Printf("Docker build failed with error: %v\n", err)

		fmt.Println("Docker build failed. Using AI to fix issues...")

		state.Dockerfile.BuildErrors = buildErrors
		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
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

func (c *Clients) pushDockerImage(image string) error {

	output, err := c.Docker.Push(image)
	fmt.Println("Output: ", output)

	if err != nil {
		fmt.Println("Registry push failed with error:", err)
		return fmt.Errorf("error pushing to registry: %v", err)
	}

	return nil
}
