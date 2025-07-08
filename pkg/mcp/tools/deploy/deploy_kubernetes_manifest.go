package deploy

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
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
	// Convert to typed parameters for GenerateManifestsTyped
	manifestParams := core.GenerateManifestsParams{
		ImageRef:    args.ImageRef,
		AppName:     args.AppName,
		Namespace:   args.Namespace,
		Port:        args.Port,
		Replicas:    1, // Default value
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}
	manifestResult, err := t.pipelineAdapter.GenerateManifestsTyped(ctx, session.SessionID, manifestParams)
	result.GenerationDuration = time.Since(generationStart)

	// Convert from typed result to kubernetes.ManifestGenerationResult
	if manifestResult != nil {
		// manifestResult is already typed as *core.GenerateManifestsResult
		result.ManifestResult = &kubernetes.ManifestGenerationResult{
			Success:   true, // Success since no error was returned
			OutputDir: result.WorkspaceDir,
		}
	}

	if err != nil {
		_ = t.handleGenerationError(ctx, err, result.ManifestResult, result)
		return errors.NewError().Messagef("error").WithLocation(

		// Manifest generation success is already handled above
		).Build()
	}

	t.logger.Info("Kubernetes manifests generated successfully",
		"session_id", session.SessionID,
		"app_name", args.AppName,
		"namespace", args.Namespace)

	// Progress reporting removed

	return nil
}

// handleGenerationError creates an error for manifest generation failures
func (t *AtomicDeployKubernetesTool) handleGenerationError(_ context.Context, err error, _ *kubernetes.ManifestGenerationResult, _ *AtomicDeployKubernetesResult) error {
	return errors.NewError().Messagef("error").WithLocation().Build()
}
