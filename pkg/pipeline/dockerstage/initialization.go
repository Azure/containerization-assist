package dockerstage

import (
	"fmt"
	"os"

	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// InitializeDockerFileState populates the Dockerfile field in PipelineState with initial values
// This function assumes the Dockerfile already exists at the given path
func InitializeDockerFileState(state *pipeline.PipelineState, dockerFilePath string) error {
	// Read the Dockerfile content
	content, err := os.ReadFile(dockerFilePath)
	if err != nil {
		return fmt.Errorf("error reading Dockerfile at path %s: %v", dockerFilePath, err)
	}

	// Update pipeline state with Dockerfile information
	state.Dockerfile.Content = string(content)
	state.Dockerfile.Path = dockerFilePath
	state.Dockerfile.BuildErrors = ""

	logger.Infof("Successfully initialized Dockerfile state from: %s\n", dockerFilePath)
	return nil
}
