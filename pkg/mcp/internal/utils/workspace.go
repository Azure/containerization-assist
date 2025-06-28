package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/utils"
	"github.com/rs/zerolog"
)

// WorkspaceManager manages file system workspaces with quotas and sandboxing
type WorkspaceManager struct {
	baseDir           string
	maxSizePerSession int64 // Per-session disk quota
	totalMaxSize      int64 // Total disk quota across all sessions
	cleanup           bool  // Auto-cleanup after session ends
	sandboxEnabled    bool  // Enable sandboxed execution

	// Quota tracking
	diskUsage map[string]int64 // sessionID -> bytes used
	mutex     sync.RWMutex

	// Logger
	logger zerolog.Logger
}

// WorkspaceConfig holds configuration for the workspace manager
type WorkspaceConfig struct {
	BaseDir           string
	MaxSizePerSession int64
	TotalMaxSize      int64
	Cleanup           bool
	SandboxEnabled    bool
	Logger            zerolog.Logger
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager(ctx context.Context, config WorkspaceConfig) (*WorkspaceManager, error) {
	if err := os.MkdirAll(config.BaseDir, 0o755); err != nil {
		return nil, mcptypes.NewRichError("DIRECTORY_CREATION_FAILED", fmt.Sprintf("failed to create base directory: %v", err), "filesystem_error")
	}

	wm := &WorkspaceManager{
		baseDir:           config.BaseDir,
		maxSizePerSession: config.MaxSizePerSession,
		totalMaxSize:      config.TotalMaxSize,
		cleanup:           config.Cleanup,
		sandboxEnabled:    config.SandboxEnabled,
		diskUsage:         make(map[string]int64),
		logger:            config.Logger,
	}

	// Initialize disk usage tracking
	if err := wm.refreshDiskUsage(ctx); err != nil {
		wm.logger.Warn().Err(err).Msg("Failed to initialize disk usage tracking")
	}

	return wm, nil
}

// InitializeWorkspace creates a new workspace for a session
func (wm *WorkspaceManager) InitializeWorkspace(ctx context.Context, sessionID string) (string, error) {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	// Check if workspace already exists
	if _, err := os.Stat(workspaceDir); err == nil {
		wm.logger.Info().Str("session_id", sessionID).Str("workspace", workspaceDir).Msg("Workspace already exists")
		return workspaceDir, nil
	}

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return "", mcptypes.NewRichError("WORKSPACE_CREATION_FAILED", fmt.Sprintf("failed to create workspace: %v", err), "filesystem_error")
	}

	// Create subdirectories
	subdirs := []string{
		"repo",      // For cloned repositories
		"build",     // For build artifacts
		"manifests", // For generated manifests
		"logs",      // For execution logs
		"cache",     // For cached data
	}

	for _, subdir := range subdirs {
		subdirPath := filepath.Join(workspaceDir, subdir)
		if err := os.MkdirAll(subdirPath, 0o755); err != nil {
			return "", mcptypes.NewRichError("SUBDIRECTORY_CREATION_FAILED", fmt.Sprintf("failed to create subdirectory %s: %v", subdir, err), "filesystem_error")
		}
	}

	wm.logger.Info().Str("session_id", sessionID).Str("workspace", workspaceDir).Msg("Initialized workspace")
	return workspaceDir, nil
}

// CloneRepository clones a Git repository to the session workspace
func (wm *WorkspaceManager) CloneRepository(ctx context.Context, sessionID, repoURL string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)
	repoDir := filepath.Join(workspaceDir, "repo")

	// Clean existing repo directory
	if err := os.RemoveAll(repoDir); err != nil {
		return mcptypes.NewRichError("REPO_CLEANUP_FAILED", fmt.Sprintf("failed to clean repo directory: %v", err), "filesystem_error")
	}

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return mcptypes.NewRichError("REPO_DIRECTORY_CREATION_FAILED", fmt.Sprintf("failed to create repo directory: %v", err), "filesystem_error")
	}

	// Check quota before cloning
	if err := wm.CheckQuota(sessionID, 100*1024*1024); err != nil { // Reserve 100MB for clone
		return err
	}

	// Clone repository with depth limit for security
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", repoURL, repoDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // Disable interactive prompts

	// Run command with context cancellation
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return mcptypes.NewRichError("REPOSITORY_CLONE_CANCELLED", "repository clone was cancelled", "cancellation_error")
		}
		return mcptypes.NewRichError("REPOSITORY_CLONE_FAILED", fmt.Sprintf("failed to clone repository: %v", err), "git_error")
	}

	// Update disk usage
	if err := wm.UpdateDiskUsage(ctx, sessionID); err != nil {
		wm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update disk usage after clone")
	}

	wm.logger.Info().Str("session_id", sessionID).Str("repo_url", repoURL).Msg("Cloned repository")
	return nil
}

// ValidateLocalPath validates and sanitizes a local path
func (wm *WorkspaceManager) ValidateLocalPath(ctx context.Context, path string) error {
	// Check for empty path first
	if path == "" {
		return mcptypes.NewRichError("EMPTY_PATH", "path cannot be empty", "validation_error")
	}

	// Convert to absolute path - relative paths are relative to workspace base directory
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(wm.baseDir, path)
	}

	// Check for absolute paths outside workspace
	if filepath.IsAbs(path) && !strings.HasPrefix(absPath, wm.baseDir) {
		return mcptypes.NewRichError("ABSOLUTE_PATH_BLOCKED", "absolute paths not allowed outside workspace", "security_error")
	}

	// Check for path traversal attempts (before conversion to absolute path)
	if strings.Contains(path, "..") {
		return mcptypes.NewRichError("PATH_TRAVERSAL_BLOCKED", "path traversal not allowed", "security_error")
	}

	// Check for hidden files - check each path component
	pathComponents := strings.Split(path, string(filepath.Separator))
	for _, component := range pathComponents {
		if component != "" && strings.HasPrefix(component, ".") && component != "." && component != ".." {
			return mcptypes.NewRichError("HIDDEN_FILES_BLOCKED", "hidden files not allowed", "security_error")
		}
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return mcptypes.NewRichError("PATH_NOT_FOUND", fmt.Sprintf("path does not exist: %s", absPath), "filesystem_error")
	}

	// Additional security checks can be added here
	// e.g., check against allowed base paths

	return nil
}

// GetFilePath returns a safe file path within the session workspace
func (wm *WorkspaceManager) GetFilePath(sessionID, relativePath string) string {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)
	return filepath.Join(workspaceDir, relativePath)
}

// CleanupWorkspace removes a session's workspace
func (wm *WorkspaceManager) CleanupWorkspace(ctx context.Context, sessionID string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	if err := os.RemoveAll(workspaceDir); err != nil {
		return mcptypes.NewRichError("WORKSPACE_CLEANUP_FAILED", fmt.Sprintf("failed to cleanup workspace: %v", err), "filesystem_error")
	}

	// Remove from disk usage tracking
	wm.mutex.Lock()
	delete(wm.diskUsage, sessionID)
	wm.mutex.Unlock()

	wm.logger.Info().Str("session_id", sessionID).Msg("Cleaned up workspace")
	return nil
}

// GenerateFileTree creates a string representation of the file tree
func (wm *WorkspaceManager) GenerateFileTree(ctx context.Context, path string) (string, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return utils.GenerateSimpleFileTree(path)
}

// CheckQuota verifies if additional disk space can be allocated
func (wm *WorkspaceManager) CheckQuota(sessionID string, additionalBytes int64) error {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	currentUsage := wm.diskUsage[sessionID]

	// Check per-session quota
	if currentUsage+additionalBytes > wm.maxSizePerSession {
		return mcptypes.NewRichError("SESSION_QUOTA_EXCEEDED", fmt.Sprintf("session disk quota would be exceeded: %d + %d > %d", currentUsage, additionalBytes, wm.maxSizePerSession), "quota_error")
	}

	// Check global quota
	totalUsage := wm.getTotalDiskUsage()
	if totalUsage+additionalBytes > wm.totalMaxSize {
		return mcptypes.NewRichError("GLOBAL_QUOTA_EXCEEDED", fmt.Sprintf("global disk quota would be exceeded: %d + %d > %d", totalUsage, additionalBytes, wm.totalMaxSize), "quota_error")
	}

	return nil
}

// UpdateDiskUsage calculates and updates disk usage for a session
func (wm *WorkspaceManager) UpdateDiskUsage(ctx context.Context, sessionID string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	// Check if directory exists
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		// Directory doesn't exist, set usage to 0
		wm.mutex.Lock()
		wm.diskUsage[sessionID] = 0
		wm.mutex.Unlock()
		return nil
	}

	var totalSize int64
	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return mcptypes.NewRichError("DISK_USAGE_CALCULATION_FAILED", fmt.Sprintf("failed to calculate disk usage: %v", err), "filesystem_error")
	}

	wm.mutex.Lock()
	wm.diskUsage[sessionID] = totalSize
	wm.mutex.Unlock()

	return nil
}

// GetDiskUsage returns the current disk usage for a session
func (wm *WorkspaceManager) GetDiskUsage(sessionID string) int64 {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
	return wm.diskUsage[sessionID]
}

// GetBaseDir returns the base directory for workspaces
func (wm *WorkspaceManager) GetBaseDir() string {
	return wm.baseDir
}

// EnforceGlobalQuota checks and enforces global disk quotas
func (wm *WorkspaceManager) EnforceGlobalQuota() error {
	totalUsage := wm.getTotalDiskUsage()

	if totalUsage > wm.totalMaxSize {
		// Find sessions that can be cleaned up (oldest first)
		// This is a simplified implementation - could be more sophisticated
		return mcptypes.NewRichError("GLOBAL_QUOTA_EXCEEDED", fmt.Sprintf("global disk quota exceeded: %d > %d", totalUsage, wm.totalMaxSize), "quota_error")
	}

	return nil
}

// Sandboxing methods

// SandboxedAnalysis runs repository analysis in a sandboxed environment
func (wm *WorkspaceManager) SandboxedAnalysis(ctx context.Context, sessionID, repoPath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, mcptypes.NewRichError("SANDBOXING_DISABLED", "sandboxing not enabled", "configuration_error")
	}

	// Sandboxed execution not implemented
	// Would require Docker-in-Docker or similar technology
	return nil, mcptypes.NewRichError("SANDBOXED_ANALYSIS_NOT_IMPLEMENTED", "sandboxed analysis not implemented", "feature_error")
}

// SandboxedBuild runs Docker build in a sandboxed environment
func (wm *WorkspaceManager) SandboxedBuild(ctx context.Context, sessionID, dockerfilePath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, mcptypes.NewRichError("SANDBOXING_DISABLED", "sandboxing not enabled", "configuration_error")
	}

	// Sandboxed execution not implemented
	// Would require Docker-in-Docker or similar technology
	return nil, mcptypes.NewRichError("SANDBOXED_BUILD_NOT_IMPLEMENTED", "sandboxed build not implemented", "feature_error")
}

// Helper methods

func (wm *WorkspaceManager) refreshDiskUsage(ctx context.Context) error {
	sessions, err := os.ReadDir(wm.baseDir)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if session.IsDir() {
			sessionID := session.Name()
			if err := wm.UpdateDiskUsage(ctx, sessionID); err != nil {
				wm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update disk usage")
			}
		}
	}

	return nil
}

func (wm *WorkspaceManager) getTotalDiskUsage() int64 {
	var total int64
	for _, usage := range wm.diskUsage {
		total += usage
	}
	return total
}

// GetStats returns workspace statistics
func (wm *WorkspaceManager) GetStats() *WorkspaceStats {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	return &WorkspaceStats{
		TotalSessions:   len(wm.diskUsage),
		TotalDiskUsage:  wm.getTotalDiskUsage(),
		TotalDiskLimit:  wm.totalMaxSize,
		PerSessionLimit: wm.maxSizePerSession,
		SandboxEnabled:  wm.sandboxEnabled,
	}
}

// WorkspaceStats provides statistics about workspace usage
type WorkspaceStats struct {
	TotalSessions   int   `json:"total_sessions"`
	TotalDiskUsage  int64 `json:"total_disk_usage_bytes"`
	TotalDiskLimit  int64 `json:"total_disk_limit_bytes"`
	PerSessionLimit int64 `json:"per_session_limit_bytes"`
	SandboxEnabled  bool  `json:"sandbox_enabled"`
}
