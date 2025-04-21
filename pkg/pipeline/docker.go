package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/utils"
)

func (s *PipelineState) InitializeDockerFileState(dockerFilePath string) error {
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
	s.Dockerfile.Content = string(content)
	s.Dockerfile.Path = dockerFilePath
	s.Dockerfile.BuildErrors = ""

	fmt.Printf("Successfully initialized Dockerfile state from: %s\n", dockerFilePath)
	return nil
}

func AnalyzeDockerfile(client *ai.AzOpenAIClient, state *PipelineState) (*FileAnalysisResult, error) {
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

	// Add valid docker images to the context
	promptText += fmt.Sprintf(`
APPROVED DOCKER IMAGES: The following Docker images are approved for use:
%s

Please prioritize using these approved images in the Dockerfile, especially for Java-based applications
where the approved Java images should be used whenever possible.
`, docker.ApprovedDockerImages)

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

**IMPORTANT: Output the fixed Dockerfile between <<<DOCKERFILE>>> tags. :IMPORTANT**

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

// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func IterateDockerfileBuild(maxIterations int, state *PipelineState, targetDir string, generateSnapshot bool, c *clients.Clients) error {
	fmt.Printf("Starting Dockerfile build iteration process for: %s\n", state.Dockerfile.Path)

	// Check if Docker is installed before starting the iteration process
	if err := docker.CheckDockerInstalled(); err != nil { // Need to move this to the start of the pipeline
		return err
	}

	for i := 0; i < maxIterations; i++ {
		fmt.Printf("\n=== Dockerfile Iteration %d of %d ===\n", i+1, maxIterations)
		state.IterationCount += 1

		// Get AI to fix the Dockerfile - call analyzeDockerfile directly
		result, err := AnalyzeDockerfile(c.AzOpenAIClient, state)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		state.Dockerfile.Content = result.FixedContent
		fmt.Println("AI suggested fixes:")
		fmt.Println(result.Analysis)

		fmt.Printf("Updated Dockerfile written. Attempting build again...\n")

		// Try to build
		buildErrors, err := c.BuildDockerfileContent(state.Dockerfile.Content, targetDir, state.RegistryURL, state.ImageName)
		if err == nil {
			fmt.Println("🎉 Docker build succeeded!")
			fmt.Println("Successful Dockerfile: \n", state.Dockerfile.Content)
			if generateSnapshot {
				if err := WriteIterationSnapshot(state, targetDir); err != nil {
					return fmt.Errorf("writing iteration snapshot: %w", err)
				}
			}
			return nil
		}

		fmt.Printf("Docker build failed with error: %v\n", buildErrors)

		fmt.Println("Docker build failed. Using AI to fix issues...")

		state.Dockerfile.BuildErrors = buildErrors

		if generateSnapshot {
			if err := WriteIterationSnapshot(state, targetDir); err != nil {
				return fmt.Errorf("writing iteration snapshot: %w", err)
			}
		}

		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
}
