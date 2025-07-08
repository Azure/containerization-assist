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

// AtomicPushImageTool implements atomic Docker image push using core operations
type AtomicPushImageTool struct {
	pipelineOps    core.TypedPipelineOperations
	sessionManager session.UnifiedSessionManager
	logger         *slog.Logger
}

// NewAtomicPushImageTool creates a new atomic push image tool
func NewAtomicPushImageTool(pipelineOps core.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicPushImageTool {
	return &AtomicPushImageTool{
		pipelineOps:    pipelineOps,
		sessionManager: sessionManager,
		logger:         logger.With("tool", "atomic_push_image"),
	}
}

// ExecuteWithContext executes the atomic push operation
func (t *AtomicPushImageTool) ExecuteWithContext(ctx context.Context, args *AtomicPushImageArgs) (*AtomicPushImageResult, error) {
	startTime := time.Now()

	// Create result object
	result := &AtomicPushImageResult{
		SessionID:     args.SessionID,
		ImageName:     args.ImageName,
		ImageTag:      args.ImageTag,
		FullImageRef:  fmt.Sprintf("%s:%s", args.ImageName, args.ImageTag),
		RegistryURL:   args.RegistryURL,
		Success:       false,
		TotalDuration: 0,
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

	t.logger.Info("Starting atomic Docker push",
		"session_id", session.SessionID,
		"image_name", args.ImageName,
		"image_tag", args.ImageTag,
		"registry", args.RegistryURL)

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.TotalDuration = time.Since(startTime)
		t.logger.Info("Dry-run push completed", "image_ref", result.FullImageRef)
		return result, nil
	}

	// Perform the push
	pushStartTime := time.Now()

	// Convert to core push parameters
	pushParams := core.PushImageParams{
		ImageName: result.FullImageRef,
		Registry:  args.RegistryURL,
	}

	_, err = t.pipelineOps.PushImageTyped(ctx, session.SessionID, pushParams)
	result.PushDuration = time.Since(pushStartTime)
	result.TotalDuration = time.Since(startTime)

	if err != nil {
		result.Success = false
		t.logger.Error("Failed to push image",
			"error", err,
			"image_ref", result.FullImageRef,
			"registry", args.RegistryURL)
		return result, errors.NewError().Message("failed to push image").Cause(err).Build()
	}

	// Success
	result.Success = true
	t.logger.Info("Push operation completed successfully",
		"image_ref", result.FullImageRef,
		"registry", args.RegistryURL,
		"push_duration", result.PushDuration)

	return result, nil
}
