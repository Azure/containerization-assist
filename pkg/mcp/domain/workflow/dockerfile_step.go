// Package workflow contains individual workflow step implementations.
package workflow

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps"
)

// DockerfileStep implements Dockerfile generation
type DockerfileStep struct{}

// NewDockerfileStep creates a new dockerfile step
func NewDockerfileStep() Step {
	return &DockerfileStep{}
}

// Name returns the step name
func (s *DockerfileStep) Name() string {
	return "generate_dockerfile"
}

// MaxRetries returns the maximum number of retries for this step
func (s *DockerfileStep) MaxRetries() int {
	return 2
}

// Execute generates a Dockerfile
func (s *DockerfileStep) Execute(ctx context.Context, state *WorkflowState) error {
	if state.AnalyzeResult == nil {
		return fmt.Errorf("analyze result is required for Dockerfile generation")
	}

	state.Logger.Info("Step 2: Generating Dockerfile")

	// Convert workflow analyze result to infrastructure type
	infraAnalyzeResult := &steps.AnalyzeResult{
		Language:  state.AnalyzeResult.Language,
		Framework: state.AnalyzeResult.Framework,
		Port:      state.AnalyzeResult.Port,
		Analysis:  state.AnalyzeResult.Metadata,
		RepoPath:  state.AnalyzeResult.RepoPath,
	}

	dockerfileResult, err := steps.GenerateDockerfile(infraAnalyzeResult, state.Logger)
	if err != nil {
		return fmt.Errorf("dockerfile generation failed: %v", err)
	}

	state.Logger.Info("Dockerfile generation completed", "path", dockerfileResult.Path)

	// Convert to workflow type
	state.DockerfileResult = &DockerfileResult{
		Content:     dockerfileResult.Content,
		Path:        dockerfileResult.Path,
		BaseImage:   dockerfileResult.BaseImage,
		Metadata:    map[string]interface{}{"build_args": dockerfileResult.BuildArgs},
		ExposedPort: dockerfileResult.ExposedPort,
	}

	return nil
}
