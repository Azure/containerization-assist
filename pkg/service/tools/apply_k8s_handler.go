package tools

import (
	"context"
	"fmt"
	"strings"

	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
)

// createApplyManifestsHandler creates the K8s manifests apply handler with atomic writes
func createApplyManifestsHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Validate required parameters
		if sessionID == "" || repoPath == "" || content == "" || path == "" {
			err := fmt.Errorf("missing required parameters: session_id, repo_path, path, and content are required")
			result := createErrorResult(err)
			return &result, nil
		}

		logger.Info("applying K8s manifests",
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

			// Extract manifest metadata
			metadata := extractManifestMetadata(content)

			data := map[string]interface{}{
				"dry_run":     true,
				"would_write": fullPath,
				"path":        path,
				"size":        len(content),
				"action":      action,
				"old_hash":    oldHash,
				"new_hash":    newHash,
				"metadata":    metadata,
			}

			result := createToolResult(true, data, &ChainHint{
				NextTool: "prepare_cluster",
				Reason:   "Ready to prepare cluster after applying manifests",
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
		diffSummary := CreateDiffSummary(path, changed, oldHash, newHash, []byte(content))

		logger.Info("file write complete",
			"path", fullPath,
			"changed", changed,
			"action", diffSummary.Action,
			"old_hash", oldHash,
			"new_hash", newHash)

		// Update session state if we have a session manager
		if deps.SessionManager != nil && sessionID != "" {
			if err := updateManifestsInSession(ctx, deps.SessionManager, sessionID, content, path); err != nil {
				logger.Warn("failed to update session state", "error", err)
				// Don't fail the operation, just log the warning
			}
		}

		// Extract manifest metadata
		metadata := extractManifestMetadata(content)

		// Prepare response data
		data := map[string]interface{}{
			"session_id":   sessionID,
			"written":      fullPath,
			"path":         path,
			"changed":      changed,
			"diff_summary": diffSummary,
			"metadata":     metadata,
			"message":      getManifestApplyMessage(diffSummary.Action),
		}

		// Set chain hint based on success
		chainHint := &ChainHint{
			NextTool: "prepare_cluster",
			Reason:   "K8s manifests applied successfully. Ready to prepare cluster.",
		}

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

// updateManifestsInSession updates the session state with the applied manifests
func updateManifestsInSession(ctx context.Context, sessionManager interface{}, sessionID, content, path string) error {
	// For now, just log that we would update the session
	// The actual session update implementation depends on the session manager interface
	logger := slog.Default()
	logger.Info("would update session with K8s manifests",
		"session_id", sessionID,
		"path", path,
		"metadata", extractManifestMetadata(content))

	// TODO: Implement proper session update when session manager interface is available
	return nil
}

// extractManifestMetadata extracts metadata from K8s manifest content
func extractManifestMetadata(content string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Extract basic info (simplified - in production would use proper YAML parser)
	if strings.Contains(content, "kind: Deployment") {
		metadata["has_deployment"] = true
	}
	if strings.Contains(content, "kind: Service") {
		metadata["has_service"] = true
	}
	if strings.Contains(content, "kind: Ingress") {
		metadata["has_ingress"] = true
	}
	if strings.Contains(content, "kind: ConfigMap") {
		metadata["has_configmap"] = true
	}
	if strings.Contains(content, "kind: Secret") {
		metadata["has_secret"] = true
	}

	// Count resources (simplified)
	metadata["resource_count"] = strings.Count(content, "---") + 1

	return metadata
}

// getManifestApplyMessage returns a user-friendly message based on the action
func getManifestApplyMessage(action string) string {
	switch action {
	case "created":
		return "K8s manifests successfully created"
	case "modified":
		return "K8s manifests successfully updated"
	case "unchanged":
		return "K8s manifests unchanged (identical content)"
	default:
		return "K8s manifests operation completed"
	}
}
