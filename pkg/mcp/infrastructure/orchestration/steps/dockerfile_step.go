// Package steps contains individual workflow step implementations.
package steps

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

func init() {
	Register(NewDockerfileStep())
}

// DockerfileStep implements Dockerfile generation
type DockerfileStep struct{}

// NewDockerfileStep creates a new dockerfile step
func NewDockerfileStep() workflow.Step {
	return &DockerfileStep{}
}

// Name returns the step name
func (s *DockerfileStep) Name() string {
	return "generate_dockerfile"
}

// Execute generates a Dockerfile
func (s *DockerfileStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	// Check if generated Dockerfile content is provided
	var dockerfileContent string
	var hasContent bool

	if content, exists := state.RequestParams["dockerfile_content"]; exists {
		if contentStr, ok := content.(string); ok && contentStr != "" {
			dockerfileContent = contentStr
			hasContent = true
			state.Logger.Info("Using provided Dockerfile content")
		}
	}

	if hasContent {
		// Use generated content directly
		dockerfileResult := &DockerfileResult{
			Content:     dockerfileContent,
			Path:        "Dockerfile",
			BaseImage:   extractBaseImageFromDockerfile(dockerfileContent),
			ExposedPort: extractPortFromDockerfile(dockerfileContent),
		}

		if err := WriteDockerfile(state.AnalyzeResult.RepoPath, dockerfileContent, state.Logger); err != nil {
			return nil, fmt.Errorf("failed to write AI-generated Dockerfile to path '%s': %v", state.AnalyzeResult.RepoPath, err)
		}

		state.Logger.Info("Dockerfile written successfully", "path", dockerfileResult.Path)

		// Convert to workflow type
		state.DockerfileResult = &workflow.DockerfileResult{
			Content:     dockerfileResult.Content,
			Path:        dockerfileResult.Path,
			BaseImage:   dockerfileResult.BaseImage,
			Metadata:    map[string]interface{}{"ai_generated": true},
			ExposedPort: dockerfileResult.ExposedPort,
		}

		// Return StepResult with dockerfile data
		return &workflow.StepResult{
			Success: true,
			Data: map[string]interface{}{
				"content":      dockerfileResult.Content,
				"path":         dockerfileResult.Path,
				"base_image":   dockerfileResult.BaseImage,
				"exposed_port": dockerfileResult.ExposedPort,
			},
			Metadata: map[string]interface{}{
				"ai_generated": true,
			},
		}, nil
	}

	// If no content provided, generate Dockerfile normally
	state.Logger.Info("No content provided, generating Dockerfile from analysis")

	if state.AnalyzeResult == nil {
		return nil, fmt.Errorf("analyze result is required for Dockerfile generation")
	}

	state.Logger.Info("Step 2: Generating Dockerfile")

	infraAnalyzeResult := &AnalyzeResult{
		Language:  state.AnalyzeResult.Language,
		Framework: state.AnalyzeResult.Framework,
		Port:      state.AnalyzeResult.Port,
		Analysis:  state.AnalyzeResult.Metadata,
		RepoPath:  state.AnalyzeResult.RepoPath,
	}

	dockerfileResult, err := GenerateDockerfile(infraAnalyzeResult, state.Logger)
	if err != nil {
		return nil, fmt.Errorf("dockerfile generation failed: %v", err)
	}

	state.Logger.Info("Dockerfile generated; returning content in MCP response instead of writing to disk")

	// Add instructions for user to create/update Dockerfile
	instructions := "To use this Dockerfile, create or update a file named 'Dockerfile' in your project root with the following content."
	if dockerfileResult.Path != "Dockerfile" {
		instructions += "\nFile name: " + dockerfileResult.Path
	}

	state.DockerfileResult = &workflow.DockerfileResult{
		Content:     dockerfileResult.Content,
		Path:        dockerfileResult.Path,
		BaseImage:   dockerfileResult.BaseImage,
		Metadata:    map[string]interface{}{"build_args": dockerfileResult.BuildArgs, "instructions": instructions},
		ExposedPort: dockerfileResult.ExposedPort,
	}

	// Return StepResult with dockerfile data
	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"content":      dockerfileResult.Content,
			"path":         dockerfileResult.Path,
			"base_image":   dockerfileResult.BaseImage,
			"exposed_port": dockerfileResult.ExposedPort,
		},
		Metadata: map[string]interface{}{
			"build_args":   dockerfileResult.BuildArgs,
			"instructions": instructions,
		},
	}, nil
}

// extractBaseImageFromDockerfile extracts the base image from Dockerfile content
func extractBaseImageFromDockerfile(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		baseImage := parts[1]
		// Remove AS alias if present
		if len(parts) >= 4 && strings.ToUpper(parts[2]) == "AS" {
			return baseImage
		}
		return baseImage
	}
	return "unknown"
}

// extractPortFromDockerfile extracts the exposed port from Dockerfile content
func extractPortFromDockerfile(content string) int {
	lines := strings.Split(content, "\n")
	re := regexp.MustCompile(`EXPOSE\s+(\d+)`)

	for _, line := range lines {
		line = strings.TrimSpace(strings.ToUpper(line))
		matches := re.FindStringSubmatch(line)
		if len(matches) < 1 {
			continue
		}
		if port, err := strconv.Atoi(matches[1]); err == nil {
			return port
		}
	}
	return 0
}
