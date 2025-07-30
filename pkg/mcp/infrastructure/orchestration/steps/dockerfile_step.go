// Package steps contains individual workflow step implementations.
package steps

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
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
func (s *DockerfileStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	if state.AnalyzeResult == nil {
		return fmt.Errorf("analyze result is required for Dockerfile generation")
	}

	// Check if this is a fixing mode call
	if state.FixingMode {
		state.Logger.Info("Regenerating Dockerfile with AI fixing",
			"previous_error", state.PreviousError,
			"failed_tool", state.FailedTool)

		// Check if AI-generated Dockerfile content is provided
		if aiContent, exists := state.RequestParams["ai_generated_dockerfile"]; exists {
			if dockerfileContent, ok := aiContent.(string); ok && dockerfileContent != "" {
				state.Logger.Info("Using AI-generated Dockerfile content for fixing")

				// Use AI-generated content directly
				dockerfileResult := &DockerfileResult{
					Content:     dockerfileContent,
					Path:        "Dockerfile",
					BaseImage:   extractBaseImageFromDockerfile(dockerfileContent),
					ExposedPort: extractPortFromDockerfile(dockerfileContent),
				}

				if err := WriteDockerfile(state.AnalyzeResult.RepoPath, dockerfileContent, state.Logger); err != nil {
					return fmt.Errorf("failed to write AI-generated Dockerfile to path '%s': %v", state.AnalyzeResult.RepoPath, err)
				}

				state.Logger.Info("AI-generated Dockerfile written successfully", "path", dockerfileResult.Path)

				// Convert to workflow type
				state.DockerfileResult = &workflow.DockerfileResult{
					Content:     dockerfileResult.Content,
					Path:        dockerfileResult.Path,
					BaseImage:   dockerfileResult.BaseImage,
					Metadata:    map[string]interface{}{"ai_generated": true, "fixing_mode": true},
					ExposedPort: dockerfileResult.ExposedPort,
				}

				return nil
			}
		}

		state.Logger.Info("No AI-generated content provided, falling back to standard generation with error context")
	} else {
		state.Logger.Info("Step 2: Generating Dockerfile")
	}

	// Convert workflow analyze result to infrastructure type
	infraAnalyzeResult := &AnalyzeResult{
		Language:  state.AnalyzeResult.Language,
		Framework: state.AnalyzeResult.Framework,
		Port:      state.AnalyzeResult.Port,
		Analysis:  state.AnalyzeResult.Metadata,
		RepoPath:  state.AnalyzeResult.RepoPath,
	}

	dockerfileResult, err := GenerateDockerfile(infraAnalyzeResult, state.Logger)
	if err != nil {
		return fmt.Errorf("dockerfile generation failed: %v", err)
	}

	if err = WriteDockerfile(infraAnalyzeResult.RepoPath, dockerfileResult.Content, state.Logger); err != nil {
		return fmt.Errorf("failed to write Dockerfile to path '%s': %v", infraAnalyzeResult.RepoPath, err)
	}

	state.Logger.Info("Dockerfile generation completed", "path", dockerfileResult.Path)

	// Convert to workflow type
	metadata := map[string]interface{}{"build_args": dockerfileResult.BuildArgs}
	if state.FixingMode {
		metadata["fixing_mode"] = true
		metadata["fixed_from_error"] = state.PreviousError
	}

	state.DockerfileResult = &workflow.DockerfileResult{
		Content:     dockerfileResult.Content,
		Path:        dockerfileResult.Path,
		BaseImage:   dockerfileResult.BaseImage,
		Metadata:    metadata,
		ExposedPort: dockerfileResult.ExposedPort,
	}

	return nil
}

// extractBaseImageFromDockerfile extracts the base image from Dockerfile content
func extractBaseImageFromDockerfile(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				baseImage := parts[1]
				// Remove AS alias if present
				if len(parts) >= 4 && strings.ToUpper(parts[2]) == "AS" {
					return baseImage
				}
				return baseImage
			}
		}
	}
	return "unknown"
}

// extractPortFromDockerfile extracts the exposed port from Dockerfile content
func extractPortFromDockerfile(content string) int {
	lines := strings.Split(content, "\n")
	re := regexp.MustCompile(`EXPOSE\s+(\d+)`)

	for _, line := range lines {
		line = strings.TrimSpace(strings.ToUpper(line))
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			if port, err := strconv.Atoi(matches[1]); err == nil {
				return port
			}
		}
	}
	return 0
}
