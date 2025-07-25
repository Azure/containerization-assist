package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
)

// FixDockerfileWithAI uses MCP sampling to fix a Dockerfile that failed to build
func FixDockerfileWithAI(ctx context.Context, dockerfilePath string, buildError error, analyzeResult *AnalyzeResult, logger *slog.Logger) error {
	logger.Info("Requesting AI assistance to fix Dockerfile",
		"dockerfile_path", dockerfilePath,
		"error", buildError)

	// Read current Dockerfile
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Create specialized sampling client
	samplingClient := sampling.NewSpecializedClient(logger)

	// Prepare repository analysis summary
	repoAnalysis := fmt.Sprintf("Language: %s, Framework: %s, Port: %d",
		analyzeResult.Language, analyzeResult.Framework, analyzeResult.Port)

	// Get AI fix for the Dockerfile
	fixedDockerfile, err := samplingClient.AnalyzeDockerfileIssue(
		ctx,
		string(dockerfileContent),
		buildError,
		repoAnalysis,
	)
	if err != nil {
		return fmt.Errorf("failed to get AI fix for Dockerfile: %w", err)
	}

	// Write the fixed Dockerfile back
	if err := os.WriteFile(dockerfilePath, []byte(fixedDockerfile), 0644); err != nil {
		return fmt.Errorf("failed to write fixed Dockerfile: %w", err)
	}

	logger.Info("Successfully fixed Dockerfile with AI assistance",
		"dockerfile_path", dockerfilePath)

	return nil
}
