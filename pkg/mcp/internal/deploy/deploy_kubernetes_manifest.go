package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"
)

// performManifestGeneration generates Kubernetes manifests
func (t *AtomicDeployKubernetesTool) performManifestGeneration(ctx context.Context, session *core.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, _ interface{}) error {
	// Progress reporting removed

	generationStart := time.Now()

	// Generate Kubernetes manifests using pipeline adapter
	port := args.Port
	if port == 0 {
		port = 80 // Default port
	}
	manifestResult, err := t.pipelineAdapter.GenerateKubernetesManifests(
		session.SessionID,
		args.ImageRef,
		args.AppName,
		port,
		"", // cpuRequest - not specified for deploy tool
		"", // memoryRequest - not specified for deploy tool
		"", // cpuLimit - not specified for deploy tool
		"", // memoryLimit - not specified for deploy tool
	)
	result.GenerationDuration = time.Since(generationStart)

	// Convert from mcptypes.KubernetesManifestResult to kubernetes.ManifestGenerationResult
	if manifestResult != nil {
		result.ManifestResult = &kubernetes.ManifestGenerationResult{
			Success:   manifestResult.Success,
			OutputDir: result.WorkspaceDir,
		}
		if manifestResult.Error != nil {
			result.ManifestResult.Error = &kubernetes.ManifestError{
				Type:    manifestResult.Error.Type,
				Message: manifestResult.Error.Message,
			}
		}
		// Convert manifests
		for _, manifest := range manifestResult.Manifests {
			result.ManifestResult.Manifests = append(result.ManifestResult.Manifests, kubernetes.GeneratedManifest{
				Kind:    manifest.Kind,
				Name:    manifest.Name,
				Path:    manifest.Path,
				Content: manifest.Content,
			})
		}
	}

	if err != nil {
		_ = t.handleGenerationError(ctx, err, result.ManifestResult, result)
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", err), "generation_error")
	}

	if manifestResult != nil && !manifestResult.Success {
		generationErr := mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %s", manifestResult.Error.Message), "generation_error")
		_ = t.handleGenerationError(ctx, generationErr, result.ManifestResult, result)
		return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", generationErr), "generation_error")
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("app_name", args.AppName).
		Str("namespace", args.Namespace).
		Msg("Kubernetes manifests generated successfully")

	// Progress reporting removed

	return nil
}

// handleGenerationError creates an error for manifest generation failures
func (t *AtomicDeployKubernetesTool) handleGenerationError(_ context.Context, err error, _ *kubernetes.ManifestGenerationResult, _ *AtomicDeployKubernetesResult) error {
	return mcp.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", err), "generation_error")
}
