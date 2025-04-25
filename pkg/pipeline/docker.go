package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
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
	// Load the Dockerfile template
	templatePath := filepath.Join("pkg", "ai", "templates", "dockerfile_template.xml") // TODO: make this configurable

	// Verify the template file exists
	if _, err := os.Stat(templatePath); err != nil {
		return nil, fmt.Errorf("error checking Dockerfile template at path %s: %v", templatePath, err)
	}

	prompt, err := ai.LoadDockerfilePromptTemplate(templatePath)
	if err != nil {
		fmt.Printf("Error loading Dockerfile prompt template: %v\n", err)
	}

	prompt.FillDockerfilePrompt(
		state.Dockerfile.Content,
		FormatManifestErrors(state),
		docker.ApprovedDockerImages,
		state.Dockerfile.BuildErrors,
		state.RepoFileTree,
	)

	// Save the prompt to file for debugging
	ai.EncodeXMLToFile(prompt, "dockerfile_prompt_test.xml") //TODO: ADD this to as a debug option, possibly add to SNAPSHOTS

	// Get the prompt text
	promptText, err := ai.EncodeXMLToString(prompt)
	if err != nil {
		return nil, fmt.Errorf("error generating prompt: %w", err)
	}

	//ai.PrintXMLContent(promptText)

	// Get AI completion
	responseText, err := client.GetChatCompletion(promptText)
	if err != nil {
		return nil, fmt.Errorf("error getting AI completion: %w", err)
	}

	// Print human-readable response for debugging
	//fmt.Println("AI response received:")
	//ai.PrintXMLContent(responseText)

	// Extract and validate the response sections
	// Explaination response is not used in the current implementation // Used to make model think more deeply about changes
	fixedContent, analysis, _, err := ai.ExtractResponseSections(responseText)
	if err != nil {
		return nil, fmt.Errorf("error extracting response sections: %w", err)
	}

	return &FileAnalysisResult{
		FixedContent: fixedContent,
		Analysis:     analysis,
	}, nil
}

// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func IterateDockerfileBuild(maxIterations int, state *PipelineState, targetDir string, c *clients.Clients) error {
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

			return nil
		}

		fmt.Printf("Docker build failed with error: %v\n", buildErrors)

		fmt.Println("Docker build failed. Using AI to fix issues...")

		state.Dockerfile.BuildErrors = buildErrors
		if err := WriteIterationSnapshot(state, targetDir); err != nil {
			return fmt.Errorf("writing iteration snapshot: %w", err)
		}
		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
}
