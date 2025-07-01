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
	// Use the correct interface method with map args
	manifestArgs := map[string]interface{}{
		"image_ref": args.ImageRef,
		"app_name":  args.AppName,
		"port":      port,
		"namespace": args.Namespace,
	}
	manifestResult, err := t.pipelineAdapter.GenerateManifests(ctx, session.SessionID, manifestArgs)
	result.GenerationDuration = time.Since(generationStart)

	// Convert from interface{} to kubernetes.ManifestGenerationResult
	if manifestResult != nil {
		// Convert interface{} to expected structure
		if manifestMap, ok := manifestResult.(map[string]interface{}); ok {
			result.ManifestResult = &kubernetes.ManifestGenerationResult{
				Success:   getBoolFromMap(manifestMap, "success", false),
				OutputDir: result.WorkspaceDir,
			}

			// Handle error if present
			if errorData, exists := manifestMap["error"]; exists && errorData != nil {
				if errorMap, ok := errorData.(map[string]interface{}); ok {
					result.ManifestResult.Error = &kubernetes.ManifestError{
						Type:    getStringFromMap(errorMap, "type", "unknown"),
						Message: getStringFromMap(errorMap, "message", "unknown error"),
					}
				}
			}

			// Convert manifests if present
			if manifestsData, exists := manifestMap["manifests"]; exists {
				if manifests, ok := manifestsData.([]interface{}); ok {
					for _, manifest := range manifests {
						if manifestMap, ok := manifest.(map[string]interface{}); ok {
							result.ManifestResult.Manifests = append(result.ManifestResult.Manifests, kubernetes.GeneratedManifest{
								Kind:    getStringFromMap(manifestMap, "kind", "unknown"),
								Name:    getStringFromMap(manifestMap, "name", "unknown"),
								Path:    getStringFromMap(manifestMap, "path", ""),
								Content: getStringFromMap(manifestMap, "content", ""),
							})
						}
					}
				}
			}
		} else {
			// Fallback for unexpected result type
			result.ManifestResult = &kubernetes.ManifestGenerationResult{
				Success:   false,
				OutputDir: result.WorkspaceDir,
			}
		}
	}

	if err != nil {
		_ = t.handleGenerationError(ctx, err, result.ManifestResult, result)
		return fmt.Errorf("error")
	}

	// Check manifest generation success through type assertion
	if manifestResult != nil {
		if manifestMap, ok := manifestResult.(map[string]interface{}); ok {
			if !getBoolFromMap(manifestMap, "success", false) {
				generationErr := fmt.Errorf("manifest generation failed")
				_ = t.handleGenerationError(ctx, generationErr, result.ManifestResult, result)
				return generationErr
			}
		}
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
	return fmt.Errorf("error")
}
