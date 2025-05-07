package dockerpipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/Azure/container-copilot/pkg/pipeline/manifestpipeline"
)

// DockerPipeline implements the pipeline.Pipeline interface for Dockerfiles
type DockerPipeline struct {
	AIClient         *ai.AzOpenAIClient
	UseDraftTemplate bool
	Parser           pipeline.Parser
}

// Generate creates a Dockerfile based on inputs
func (p *DockerPipeline) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	dockerfilePath := filepath.Join(targetDir, "Dockerfile")

	// Check if Dockerfile already exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Info("No Dockerfile found, generating one...\n")

		if p.UseDraftTemplate {
			// Use the existing function from the docker package
			resp, err := docker.GetDockerfileTemplateName(ctx, p.AIClient, targetDir)
			if err != nil {
				return fmt.Errorf("getting Dockerfile template name: %w", err)
			}

			// Accumulate token usage from template selection
			state.TokenUsage.PromptTokens += resp.TokenUsage.PromptTokens
			state.TokenUsage.CompletionTokens += resp.TokenUsage.CompletionTokens
			state.TokenUsage.TotalTokens += resp.TokenUsage.TotalTokens

			templateName := resp.Content

			logger.Infof("Using Dockerfile template: %s\n", templateName)

			// Generate the Dockerfile from template
			if err := docker.WriteDockerfileFromTemplate(templateName, targetDir); err != nil {
				return fmt.Errorf("writing Dockerfile from template: %w", err)
			}
		} else {
			logger.Info("Creating empty Dockerfile\n")
			// Create an empty file
			if err := os.WriteFile(dockerfilePath, []byte{}, 0644); err != nil {
				return fmt.Errorf("writing empty Dockerfile: %w", err)
			}
		}
	} else {
		logger.Infof("Found existing Dockerfile at %s\n", dockerfilePath)
	}

	// Read the content and update state
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("reading Dockerfile: %w", err)
	}

	state.Dockerfile.Content = string(content)
	state.Dockerfile.Path = dockerfilePath

	return nil
}

// GetErrors returns Docker-related errors from the state
func (p *DockerPipeline) GetErrors(state *pipeline.PipelineState) string {
	return state.Dockerfile.BuildErrors
}

// WriteSuccessfulFiles writes the successful Dockerfile to disk
func (p *DockerPipeline) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	// Only write if there's content and no build errors, regardless of global state.Success
	if state.Dockerfile.Path != "" && state.Dockerfile.Content != "" && state.Dockerfile.BuildErrors == "" {
		logger.Infof("Writing final Dockerfile to %s\n", state.Dockerfile.Path)
		if err := os.WriteFile(state.Dockerfile.Path, []byte(state.Dockerfile.Content), 0644); err != nil {
			return fmt.Errorf("writing Dockerfile: %w", err)
		}
		return nil
	}
	return fmt.Errorf("no successful Dockerfile to write")
}

// Run executes the Dockerfile generation and build pipeline
func (p *DockerPipeline) Run(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}, options pipeline.RunnerOptions) error {
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

	logger.Infof("Starting Dockerfile build iteration process for: %s\n", state.Dockerfile.Path)

	// Check if Docker is installed before starting the iteration process
	if err := docker.CheckDockerInstalled(); err != nil {
		return err
	}

	for i := 0; i < maxIterations; i++ {
		logger.Infof("\n=== Dockerfile Iteration %d of %d ===\n", i+1, maxIterations)
		state.IterationCount += 1

		// Get AI to fix the Dockerfile
		result, err := AnalyzeDockerfile(ctx, p.AIClient, state)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		state.Dockerfile.Content = result.FixedContent
		logger.Info("AI suggested fixes:")
		logger.Debug(result.Analysis)

		logger.Info("Updated Dockerfile written. Attempting build again...\n")

		// Try to build
		buildErrors, err := c.BuildDockerfileContent(state.Dockerfile.Content, targetDir, state.RegistryURL, state.ImageName)
		if err == nil {
			logger.Info("🎉 Docker build succeeded!")
			logger.Infof("Successful Dockerfile: \n", state.Dockerfile.Content)

			// Clear any previous build errors to indicate success
			state.Dockerfile.BuildErrors = ""

			if generateSnapshot {
				if err := pipeline.WriteIterationSnapshot(state, targetDir, p); err != nil {
					return fmt.Errorf("writing iteration snapshot: %w", err)
				}
			}
			return nil
		}

		logger.Errorf("Docker build failed with error: %v\n", buildErrors)

		logger.Error("Docker build failed. Using AI to fix issues...")

		state.Dockerfile.BuildErrors = buildErrors

		// Update the previous attempts summary
		runningSummary, err := c.AzOpenAIClient.GetChatCompletionWithFormat(ctx, docker.DockerfileRunningErrors, state.Dockerfile.PreviousAttemptsSummary, result.Analysis+"\n Current Build Errors"+buildErrors)
		if err != nil {
			logger.Errorf("Warning: Failed to generate dockerfile error summary: %v\n", err)
		} else {
			state.Dockerfile.PreviousAttemptsSummary = runningSummary.Content
			logger.Infof("\n Updated Summary of Previous Dockerfile Attempts: \n%s", state.Dockerfile.PreviousAttemptsSummary)
		}

		if generateSnapshot {
			if err := pipeline.WriteIterationSnapshot(state, targetDir, p); err != nil {
				return fmt.Errorf("writing iteration snapshot: %w", err)
			}
		}

		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
}

// AnalyzeDockerfile uses AI to analyze and fix Dockerfile content
func AnalyzeDockerfile(ctx context.Context, client *ai.AzOpenAIClient, state *pipeline.PipelineState) (*pipeline.FileAnalysisResult, error) {
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
	manifestErrors := manifestpipeline.FormatManifestErrors(state)
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

	// Running LLM Summary of previous attempts
	if state.Dockerfile.PreviousAttemptsSummary != "" {
		promptText += fmt.Sprintf(`
	Summary of your previous attempts to fix the Dockerfile that were NOT successful:
	%s
	`, state.Dockerfile.PreviousAttemptsSummary)
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

Favor using the latest stable base images and best practices for Dockerfile writing when appropriate.
If applicable, use multi-stage builds to reduce image size.
Ensure that all COPY and RUN instructions are consistent with the actual file structure of the repository — do not assume specific folders or filenames without confirming they exist.
Avoid relying on runtime wildcard patterns (e.g., find or *.jar in CMD) unless the build stage guarantees those files exist at the expected paths.
If using shell logic in CMD or RUN, it should fail clearly if expected files are missing — avoid silent errors or infinite loops.

**IMPORTANT: Output the fixed Dockerfile content between <DOCKERFILE> and </DOCKERFILE> tags. These tags must not appear anywhere else in your response except for wrapping the corrected dokerfile content. :IMPORTANT**

I will tip you if you provide a correct and working Dockerfile.
`

	resp, err := client.GetChatCompletion(ctx, promptText)
	if err != nil {
		return nil, err
	}

	// Accumulate token usage in pipeline state
	state.TokenUsage.PromptTokens += resp.TokenUsage.PromptTokens
	state.TokenUsage.CompletionTokens += resp.TokenUsage.CompletionTokens
	state.TokenUsage.TotalTokens += resp.TokenUsage.TotalTokens

	content := resp.Content

	parser := &pipeline.DefaultParser{}
	fixedContent, err := parser.ExtractContent(content, "DOCKERFILE")
	if err != nil {
		return nil, fmt.Errorf("failed to extract fixed Dockerfile: %v", err)
	}

	return &pipeline.FileAnalysisResult{
		FixedContent: fixedContent,
		Analysis:     content,
	}, nil
}

// Initialize prepares the pipeline state with initial Docker-related values
func (p *DockerPipeline) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// Initialize a blank Dockerfile state if the file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// If file doesn't exist, just initialize with empty content
		state.Dockerfile.Content = ""
		state.Dockerfile.Path = path
		state.Dockerfile.BuildErrors = ""
		state.Dockerfile.PreviousAttemptsSummary = ""
		logger.Info("Initialized empty Dockerfile state (file will be created later)\n")
		return nil
	}

	// Otherwise, read the existing file content
	return InitializeDockerFileState(state, path)
}

// Deploy handles pushing the Docker image to the registry
func (p *DockerPipeline) Deploy(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}) error {
	// Type assertion for clients
	c, ok := clientsObj.(*clients.Clients)
	if !ok {
		return fmt.Errorf("invalid clients type")
	}

	// Only deploy if the build was successful
	if state.Dockerfile.BuildErrors != "" {
		return fmt.Errorf("cannot deploy Docker image with build errors")
	}

	// Build the image name with registry
	registryAndImage := fmt.Sprintf("%s/%s", state.RegistryURL, state.ImageName)
	logger.Infof("Pushing Docker image %s to registry\n", registryAndImage)

	// Push the Docker image
	if err := c.PushDockerImage(registryAndImage); err != nil {
		return fmt.Errorf("pushing image %s: %w", registryAndImage, err)
	}

	logger.Infof("Successfully pushed Docker image %s to registry\n", registryAndImage)
	return nil
}
