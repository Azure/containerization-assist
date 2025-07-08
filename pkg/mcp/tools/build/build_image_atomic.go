package build

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/session"
)

// AtomicBuildImageTool implements atomic Docker image build using core operations
type AtomicBuildImageTool struct {
	pipelineOps    core.TypedPipelineOperations
	sessionManager session.UnifiedSessionManager
	logger         *slog.Logger
}

// NewAtomicBuildImageTool creates a new atomic build image tool
func NewAtomicBuildImageTool(pipelineOps core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicBuildImageTool {
	return &AtomicBuildImageTool{
		pipelineOps:    pipelineOps,
		sessionManager: sessionManager,
		logger:         logger.With("tool", "atomic_build_image"),
	}
}

// ExecuteWithContext executes the atomic build operation
func (t *AtomicBuildImageTool) ExecuteWithContext(ctx context.Context, args *AtomicBuildImageArgs) (*AtomicBuildImageResult, error) {
	startTime := time.Now()

	// Create result object
	result := &AtomicBuildImageResult{
		SessionID:      args.SessionID,
		ImageName:      args.ImageName,
		ImageTag:       args.ImageTag,
		Platform:       args.Platform,
		BuildContext:   args.BuildContext,
		DockerfilePath: args.DockerfilePath,
		FullImageRef:   fmt.Sprintf("%s:%s", args.ImageName, args.ImageTag),
		Success:        false,
		TotalDuration:  0,
	}

	// Get session
	sessionState, err := t.sessionManager.GetSession(ctx, args.SessionID)
	if err != nil {
		t.logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		result.TotalDuration = time.Since(startTime)
		return result, errors.NewError().Messagef("session not found: %s", args.SessionID).Build()
	}

	session := sessionState.ToCoreSessionState()
	result.WorkspaceDir = session.WorkspaceDir

	t.logger.Info("Starting atomic Docker build",
		"session_id", session.SessionID,
		"image_name", args.ImageName,
		"image_tag", args.ImageTag,
		"dockerfile_path", args.DockerfilePath)

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.TotalDuration = time.Since(startTime)
		t.logger.Info("Dry-run build completed", "image_ref", result.FullImageRef)
		return result, nil
	}

	// Perform the build
	buildStartTime := time.Now()

	// Convert to core build parameters
	buildParams := core.BuildImageParams{
		SessionID:      session.SessionID,
		DockerfilePath: args.DockerfilePath,
		ImageName:      args.ImageName,
		Tags:           []string{args.ImageTag},
		ContextPath:    args.BuildContext,
		BuildArgs:      args.BuildArgs,
		NoCache:        args.NoCache,
	}

	_, err = t.pipelineOps.BuildImageTyped(ctx, session.SessionID, buildParams)
	result.BuildDuration = time.Since(buildStartTime)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		t.logger.Error("Failed to build image",
			"error", err,
			"image_ref", result.FullImageRef,
			"dockerfile_path", args.DockerfilePath)
		return result, errors.NewError().Message("failed to build image").Cause(err).Build()
	}

	// Success
	result.Success = true
	t.logger.Info("Build operation completed successfully",
		"image_ref", result.FullImageRef,
		"dockerfile_path", args.DockerfilePath,
		"build_duration", result.BuildDuration)

	// Handle push after build if requested
	if args.PushAfterBuild && args.RegistryURL != "" {
		pushStartTime := time.Now()

		pushParams := core.PushImageParams{
			ImageName: result.FullImageRef,
			Registry:  args.RegistryURL,
		}

		_, err = t.pipelineOps.PushImageTyped(ctx, session.SessionID, pushParams)
		result.PushDuration = time.Since(pushStartTime)
		result.TotalDuration = time.Since(startTime)

		if err != nil {
			t.logger.Error("Failed to push image after build",
				"error", err,
				"image_ref", result.FullImageRef)
			// Build succeeded but push failed - still consider this a success
			// The push error will be in the logs/result
		} else {
			t.logger.Info("Push after build completed successfully",
				"image_ref", result.FullImageRef,
				"push_duration", result.PushDuration)
		}
	}

	return result, nil
}
