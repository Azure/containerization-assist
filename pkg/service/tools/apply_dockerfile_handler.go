package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

// createApplyDockerfileHandler creates the Dockerfile apply handler with atomic writes
func createApplyDockerfileHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := deps.Logger
		if logger == nil {
			logger = slog.Default()
		}

		args := req.GetArguments()

		// Extract parameters
		sessionID, _ := args["session_id"].(string)
		repoPath, _ := args["repo_path"].(string)
		content, _ := args["content"].(string)
		path, _ := args["path"].(string)
		dryRun, _ := args["dry_run"].(bool)

		// Default path if not specified
		if path == "" {
			path = "Dockerfile"
		}

		// Validate required parameters
		if sessionID == "" || repoPath == "" || content == "" {
			err := fmt.Errorf("missing required parameters: session_id, repo_path, and content are required")
			result := createErrorResult(err)
			return &result, nil
		}

		logger.Info("applying Dockerfile",
			"session_id", sessionID,
			"repo_path", repoPath,
			"path", path,
			"dry_run", dryRun,
			"content_length", len(content))

		// Validate path (prevent path traversal)
		if err := ValidatePath(path); err != nil {
			logger.Error("invalid path", "path", path, "error", err)
			result := createErrorResult(fmt.Errorf("invalid path: %w", err))
			return &result, nil
		}

		// Ensure path is within workspace
		fullPath, err := EnsureWithinWorkspace(repoPath, path)
		if err != nil {
			logger.Error("path escape attempt", "repo", repoPath, "path", path, "error", err)
			result := createErrorResult(fmt.Errorf("path security violation: %w", err))
			return &result, nil
		}

		// Create diff summary for dry run or actual write
		var diffSummary DiffSummary

		// Dry run mode - just validate and return what would happen
		if dryRun {
			logger.Info("dry run mode - simulating write", "path", fullPath)

			// Check if file exists
			oldHash := ""
			if hash, err := ComputeFileHash(fullPath); err == nil {
				oldHash = hash
			}

			// Compute new hash
			newHash := ComputeContentHash([]byte(content))

			// Determine action
			action := "create"
			if oldHash != "" {
				if oldHash == newHash {
					action = "unchanged"
				} else {
					action = "modify"
				}
			}

			data := map[string]interface{}{
				"dry_run":      true,
				"would_write":  fullPath,
				"path":         path,
				"size":         len(content),
				"action":       action,
				"old_hash":     oldHash,
				"new_hash":     newHash,
				"base_image":   ExtractBaseImage(content),
				"exposed_port": ExtractExposedPort(content),
			}

			result := createToolResult(true, data, &ChainHint{
				NextTool: "build_image",
				Reason:   "Ready to build container image after applying Dockerfile",
			})
			return &result, nil
		}

		// Perform atomic write with idempotency check
		changed, oldHash, newHash, err := WriteFileAtomic(fullPath, []byte(content), 0o644)
		if err != nil {
			logger.Error("write failed", "path", fullPath, "error", err)
			result := createErrorResult(fmt.Errorf("failed to write file: %w", err))
			return &result, nil
		}

		// Create diff summary
		diffSummary = CreateDiffSummary(path, changed, oldHash, newHash, []byte(content))

		logger.Info("file write complete",
			"path", fullPath,
			"changed", changed,
			"action", diffSummary.Action,
			"old_hash", oldHash,
			"new_hash", newHash)

		// Update session state if we have a session manager
		if deps.SessionManager != nil && sessionID != "" {
			if err := updateDockerfileInSession(ctx, deps.SessionManager, sessionID, content, path); err != nil {
				logger.Warn("failed to update session state", "error", err)
				// Don't fail the operation, just log the warning
			}
		}

		// Prepare response data
		data := map[string]interface{}{
			"session_id":   sessionID,
			"written":      fullPath,
			"path":         path,
			"changed":      changed,
			"diff_summary": diffSummary,
			"base_image":   ExtractBaseImage(content),
			"exposed_port": ExtractExposedPort(content),
			"message":      getApplyMessage(diffSummary.Action),
		}

		// Set chain hint based on success
		chainHint := &ChainHint{
			NextTool: "build_image",
			Reason:   "Dockerfile applied successfully. Ready to build container image.",
		}

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

// updateDockerfileInSession updates the session state with the applied Dockerfile
func updateDockerfileInSession(ctx context.Context, sessionManager interface{}, sessionID, content, path string) error {
	// For now, just log that we would update the session
	// The actual session update implementation depends on the session manager interface
	logger := slog.Default()
	logger.Info("would update session with Dockerfile",
		"session_id", sessionID,
		"path", path,
		"base_image", ExtractBaseImage(content),
		"exposed_port", ExtractExposedPort(content))

	// TODO: Implement proper session update when session manager interface is available
	return nil
}

// getApplyMessage returns a user-friendly message based on the action
func getApplyMessage(action string) string {
	switch action {
	case "created":
		return "Dockerfile successfully created"
	case "modified":
		return "Dockerfile successfully updated"
	case "unchanged":
		return "Dockerfile unchanged (identical content)"
	default:
		return "Dockerfile operation completed"
	}
}

// ComputeContentHash computes SHA256 hash of content
func ComputeContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
