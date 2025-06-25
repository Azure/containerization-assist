package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/utils"
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
func NewWorkspaceManager(config WorkspaceConfig) (*WorkspaceManager, error) {
	if err := os.MkdirAll(config.BaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
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
	if err := wm.refreshDiskUsage(); err != nil {
		wm.logger.Warn().Err(err).Msg("Failed to initialize disk usage tracking")
	}

	return wm, nil
}

// InitializeWorkspace creates a new workspace for a session
func (wm *WorkspaceManager) InitializeWorkspace(sessionID string) (string, error) {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	// Check if workspace already exists
	if _, err := os.Stat(workspaceDir); err == nil {
		wm.logger.Info().Str("session_id", sessionID).Str("workspace", workspaceDir).Msg("Workspace already exists")
		return workspaceDir, nil
	}

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
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
			return "", fmt.Errorf("failed to create subdirectory %s: %w", subdir, err)
		}
	}

	wm.logger.Info().Str("session_id", sessionID).Str("workspace", workspaceDir).Msg("Initialized workspace")
	return workspaceDir, nil
}

// CloneRepository clones a Git repository to the session workspace
func (wm *WorkspaceManager) CloneRepository(sessionID, repoURL string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)
	repoDir := filepath.Join(workspaceDir, "repo")

	// Clean existing repo directory
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to clean repo directory: %w", err)
	}

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Check quota before cloning
	if err := wm.CheckQuota(sessionID, 100*1024*1024); err != nil { // Reserve 100MB for clone
		return err
	}

	// Clone repository with depth limit for security
	cmd := exec.Command("git", "clone", "--depth", "1", "--single-branch", repoURL, repoDir)

	// Set timeout for clone operation
	timeout := 5 * time.Minute
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // Disable interactive prompts

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil {
			// Log kill error but still return timeout error
			wm.logger.Warn().Err(err).Msg("Failed to kill git process during timeout")
		}
		return fmt.Errorf("repository clone timed out after %v", timeout)
	}

	// Update disk usage
	if err := wm.UpdateDiskUsage(sessionID); err != nil {
		wm.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update disk usage after clone")
	}

	wm.logger.Info().Str("session_id", sessionID).Str("repo_url", repoURL).Msg("Cloned repository")
	return nil
}

// ValidateLocalPath validates and sanitizes a local path
func (wm *WorkspaceManager) ValidateLocalPath(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Check for path traversal attempts
	if strings.Contains(absPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", absPath)
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
func (wm *WorkspaceManager) CleanupWorkspace(sessionID string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	if err := os.RemoveAll(workspaceDir); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}

	// Remove from disk usage tracking
	wm.mutex.Lock()
	delete(wm.diskUsage, sessionID)
	wm.mutex.Unlock()

	wm.logger.Info().Str("session_id", sessionID).Msg("Cleaned up workspace")
	return nil
}

// GenerateFileTree creates a string representation of the file tree
func (wm *WorkspaceManager) GenerateFileTree(path string) (string, error) {
	return utils.GenerateSimpleFileTree(path)
}

// CheckQuota verifies if additional disk space can be allocated
func (wm *WorkspaceManager) CheckQuota(sessionID string, additionalBytes int64) error {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	currentUsage := wm.diskUsage[sessionID]

	// Check per-session quota
	if currentUsage+additionalBytes > wm.maxSizePerSession {
		return fmt.Errorf("session disk quota would be exceeded: %d + %d > %d",
			currentUsage, additionalBytes, wm.maxSizePerSession)
	}

	// Check global quota
	totalUsage := wm.getTotalDiskUsage()
	if totalUsage+additionalBytes > wm.totalMaxSize {
		return fmt.Errorf("global disk quota would be exceeded: %d + %d > %d",
			totalUsage, additionalBytes, wm.totalMaxSize)
	}

	return nil
}

// UpdateDiskUsage calculates and updates disk usage for a session
func (wm *WorkspaceManager) UpdateDiskUsage(sessionID string) error {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	var totalSize int64
	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to calculate disk usage: %w", err)
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
		return fmt.Errorf("global disk quota exceeded: %d > %d", totalUsage, wm.totalMaxSize)
	}

	return nil
}

// Sandboxing methods

// SandboxedAnalysis runs repository analysis in a sandboxed environment
func (wm *WorkspaceManager) SandboxedAnalysis(sessionID, repoPath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, fmt.Errorf("sandboxing not enabled")
	}

	// Sandboxed execution not implemented
	// Would require Docker-in-Docker or similar technology
	return nil, fmt.Errorf("sandboxed analysis not implemented")
}

// SandboxedBuild runs Docker build in a sandboxed environment
func (wm *WorkspaceManager) SandboxedBuild(sessionID, dockerfilePath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, fmt.Errorf("sandboxing not enabled")
	}

	// Sandboxed execution not implemented
	// Would require Docker-in-Docker or similar technology
	return nil, fmt.Errorf("sandboxed build not implemented")
}

// Helper methods

func (wm *WorkspaceManager) refreshDiskUsage() error {
	sessions, err := os.ReadDir(wm.baseDir)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.IsDir() {
			sessionID := session.Name()
			if err := wm.UpdateDiskUsage(sessionID); err != nil {
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
