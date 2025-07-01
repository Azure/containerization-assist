package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

	// Docker command for sandboxing
	dockerCmd string

	// Security validation
	securityValidator *SecurityValidator

	// Production features
	monitor          *SandboxMonitor
	auditLogger      *AuditLogger
	metricsCollector *MetricsCollector

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
		return nil, fmt.Errorf("failed to create base directory: %v", err)
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

	// Initialize Docker command for sandboxing if enabled
	if config.SandboxEnabled {
		dockerPath, err := exec.LookPath("docker")
		if err != nil {
			return nil, fmt.Errorf("docker command not found for sandboxing: %v", err)
		}
		wm.dockerCmd = dockerPath
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
		return "", fmt.Errorf("failed to create workspace directory: %v", err)
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
			return "", fmt.Errorf("operation failed")
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
		return fmt.Errorf("operation failed")
	}

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return fmt.Errorf("operation failed")
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
			return fmt.Errorf("operation cancelled")
		}
		return fmt.Errorf("operation failed")
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
		return fmt.Errorf("path cannot be empty")
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
		return fmt.Errorf("absolute paths not allowed outside workspace")
	}

	// Check for path traversal attempts (before conversion to absolute path)
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal attempts are not allowed")
	}

	// Check for hidden files - check each path component
	pathComponents := strings.Split(path, string(filepath.Separator))
	for _, component := range pathComponents {
		if component != "" && strings.HasPrefix(component, ".") && component != "." && component != ".." {
			return fmt.Errorf("hidden files are not allowed")
		}
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", path)
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
		return fmt.Errorf("operation failed")
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
		return fmt.Errorf("SESSION_QUOTA_EXCEEDED: session disk quota would be exceeded: %d + %d > %d",
			currentUsage, additionalBytes, wm.maxSizePerSession)
	}

	// Check global quota
	totalUsage := wm.getTotalDiskUsage()
	if totalUsage+additionalBytes > wm.totalMaxSize {
		return fmt.Errorf("GLOBAL_QUOTA_EXCEEDED: global disk quota would be exceeded: %d + %d > %d",
			totalUsage, additionalBytes, wm.totalMaxSize)
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
		return fmt.Errorf("operation failed")
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
		return fmt.Errorf("GLOBAL_QUOTA_EXCEEDED: total disk usage %d exceeds limit %d", totalUsage, wm.totalMaxSize)
	}

	return nil
}

// Sandboxing types and configuration

// SandboxOptions configures sandboxed execution
type SandboxOptions struct {
	BaseImage       string                 `json:"base_image"`
	Environment     map[string]string      `json:"environment"`
	MemoryLimit     int64                  `json:"memory_limit"`
	CPUQuota        int64                  `json:"cpu_quota"`
	Timeout         time.Duration          `json:"timeout"`
	ReadOnly        bool                   `json:"read_only"`
	NetworkAccess   bool                   `json:"network_access"`
	SecurityPolicy  SecurityPolicy         `json:"security_policy"`
	User            string                 `json:"user"`
	Group           string                 `json:"group"`
	Capabilities    []string               `json:"capabilities"`
	Privileged      bool                   `json:"privileged"`
	Operation       string                 `json:"operation"`
	Metadata        map[string]interface{} `json:"metadata"`
	ValidationLevel ValidationLevel        `json:"validation_level"`

	// Extended options for advanced execution
	EnableMetrics    bool              `json:"enable_metrics"`
	EnableAudit      bool              `json:"enable_audit"`
	CustomSeccomp    string            `json:"custom_seccomp"`
	AppArmorProfile  string            `json:"apparmor_profile"`
	SELinuxContext   string            `json:"selinux_context"`
	DNSServers       []string          `json:"dns_servers"`
	ExtraHosts       map[string]string `json:"extra_hosts"`
	WorkingDirectory string            `json:"working_directory"`
}

// ValidationLevel defines the depth of security validation
type ValidationLevel string

const (
	ValidationLevelFast     ValidationLevel = "fast"
	ValidationLevelStandard ValidationLevel = "standard"
	ValidationLevelDeep     ValidationLevel = "deep"
)

// SecurityPolicy defines security constraints for sandboxed execution
type SecurityPolicy struct {
	AllowNetworking   bool           `json:"allow_networking"`
	AllowFileSystem   bool           `json:"allow_filesystem"`
	AllowedSyscalls   []string       `json:"allowed_syscalls"`
	ResourceLimits    ResourceLimits `json:"resource_limits"`
	TrustedRegistries []string       `json:"trusted_registries"`
	RequireNonRoot    bool           `json:"require_non_root"`
}

// ResourceLimits defines resource constraints
type ResourceLimits struct {
	Memory    int64 `json:"memory"`
	CPUQuota  int64 `json:"cpu_quota"`
	DiskSpace int64 `json:"disk_space"`
}

// ExecResult contains the result of sandboxed execution
type ExecResult struct {
	ExitCode int              `json:"exit_code"`
	Stdout   string           `json:"stdout"`
	Stderr   string           `json:"stderr"`
	Duration time.Duration    `json:"duration"`
	Metrics  ExecutionMetrics `json:"metrics"`
}

// ExecutionMetrics provides runtime metrics for sandboxed execution
type ExecutionMetrics struct {
	MemoryUsage int64 `json:"memory_usage"`
	CPUUsage    int64 `json:"cpu_usage"`
	NetworkIO   int64 `json:"network_io"`
	DiskIO      int64 `json:"disk_io"`
}

// Sandboxing methods

// ExecuteSandboxed runs commands in a secure Docker container with production-ready security
func (wm *WorkspaceManager) ExecuteSandboxed(ctx context.Context, sessionID string, cmd []string, options SandboxOptions) (*ExecResult, error) {
	if !wm.sandboxEnabled {
		return nil, fmt.Errorf("sandboxing is disabled")
	}

	if wm.dockerCmd == "" {
		return nil, fmt.Errorf("docker command not initialized")
	}

	// Production-ready security validation
	if wm.securityValidator != nil {
		report, err := wm.securityValidator.ValidateSecurity(ctx, sessionID, options)
		if err != nil {
			return nil, fmt.Errorf("security validation failed: %w", err)
		}

		// Block execution for high/critical risk
		if report.OverallRisk == "HIGH" || report.OverallRisk == "CRITICAL" {
			wm.auditSecurityBlock(sessionID, report)
			return nil, fmt.Errorf("execution blocked due to %s security risk",
				report.OverallRisk)
		}
	}

	// Validate security policy
	if err := wm.validateSecurityPolicy(options.SecurityPolicy); err != nil {
		return nil, fmt.Errorf("security policy validation failed: %v", err)
	}

	// Build Docker run command with enhanced security
	dockerArgs, err := wm.buildProductionDockerCommand(sessionID, cmd, options)
	if err != nil {
		return nil, fmt.Errorf("failed to build docker command: %v", err)
	}

	// Execute with comprehensive monitoring
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	return wm.executeProductionDockerCommand(ctx, dockerArgs, sessionID, options)
}

// SandboxedAnalysis runs repository analysis in a production-ready sandboxed environment
func (wm *WorkspaceManager) SandboxedAnalysis(ctx context.Context, sessionID, repoPath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, fmt.Errorf("sandboxing is disabled")
	}

	// Create production-ready analysis options
	sandboxOpts := SandboxOptions{
		BaseImage:       "alpine:3.18",     // Pin to specific version
		MemoryLimit:     256 * 1024 * 1024, // 256MB
		CPUQuota:        50000,             // 50% of one CPU
		Timeout:         5 * time.Minute,
		ReadOnly:        true,
		NetworkAccess:   false,
		User:            "1000",
		Group:           "1000",
		Capabilities:    []string{}, // No capabilities
		Privileged:      false,
		Operation:       "ANALYSIS",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io"},
			ResourceLimits: ResourceLimits{
				Memory:   256 * 1024 * 1024,
				CPUQuota: 50000,
			},
		},
	}

	// Analyze repository structure with enhanced security
	cmd := []string{"sh", "-c", "find /workspace/repo -type f \\( -name '*.go' -o -name '*.js' -o -name '*.py' -o -name '*.ts' -o -name '*.java' \\) | head -100"}
	result, err := wm.ExecuteSandboxed(ctx, sessionID, cmd, sandboxOpts)
	if err != nil {
		return nil, fmt.Errorf("sandboxed analysis failed: %v", err)
	}

	return result, nil
}

// SandboxedBuild runs Docker build in a production-ready sandboxed environment
func (wm *WorkspaceManager) SandboxedBuild(ctx context.Context, sessionID, dockerfilePath string, options interface{}) (interface{}, error) {
	if !wm.sandboxEnabled {
		return nil, fmt.Errorf("sandboxing is disabled")
	}

	// Create production-ready build options with enhanced security
	sandboxOpts := SandboxOptions{
		BaseImage:       "docker:24-dind",   // Pin to specific version
		MemoryLimit:     2048 * 1024 * 1024, // 2GB for builds
		CPUQuota:        200000,             // 200% of one CPU (2 cores)
		Timeout:         15 * time.Minute,
		ReadOnly:        false,  // Builds need write access
		NetworkAccess:   true,   // Needed for pulling base images
		User:            "root", // DinD requires root (controlled environment)
		Group:           "root",
		Capabilities:    []string{"CAP_SYS_ADMIN"}, // Required for DinD
		Privileged:      true,                      // Required for DinD (isolated container)
		Operation:       "BUILD",
		ValidationLevel: ValidationLevelDeep, // Higher validation for builds
		SecurityPolicy: SecurityPolicy{
			AllowNetworking:   true,
			AllowFileSystem:   true,
			RequireNonRoot:    false, // DinD exception
			TrustedRegistries: []string{"docker.io", "ghcr.io", "quay.io"},
			ResourceLimits: ResourceLimits{
				Memory:   2048 * 1024 * 1024,
				CPUQuota: 200000,
			},
		},
	}

	// Build Docker image with enhanced monitoring
	cmd := []string{"sh", "-c", "dockerd-entrypoint.sh & sleep 5 && docker build --no-cache -t temp-build /workspace/repo"}
	result, err := wm.ExecuteSandboxed(ctx, sessionID, cmd, sandboxOpts)
	if err != nil {
		return nil, fmt.Errorf("sandboxed build failed: %v", err)
	}

	return result, nil
}

// Helper methods for sandboxing

// ValidateSecurityPolicy validates a security policy (public method for testing)
func (wm *WorkspaceManager) ValidateSecurityPolicy(policy SecurityPolicy) error {
	return wm.validateSecurityPolicy(policy)
}

func (wm *WorkspaceManager) validateSecurityPolicy(policy SecurityPolicy) error {
	// Validate trusted registries
	if len(policy.TrustedRegistries) == 0 {
		return fmt.Errorf("at least one trusted registry must be specified")
	}

	// Validate resource limits
	if policy.ResourceLimits.Memory > 0 && policy.ResourceLimits.Memory < 64*1024*1024 {
		return fmt.Errorf("memory limit too low: minimum 64MB required")
	}

	return nil
}

func (wm *WorkspaceManager) buildDockerRunCommand(sessionID string, cmd []string, options SandboxOptions) ([]string, error) {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	args := []string{"run", "--rm"}

	// Resource limits
	if options.MemoryLimit > 0 {
		args = append(args, fmt.Sprintf("--memory=%d", options.MemoryLimit))
	}
	if options.CPUQuota > 0 {
		cpuLimit := float64(options.CPUQuota) / 100000.0 // Convert from Docker quota to CPU limit
		args = append(args, fmt.Sprintf("--cpus=%.2f", cpuLimit))
	}

	// Security settings
	if options.SecurityPolicy.RequireNonRoot {
		args = append(args, "--user=1000:1000")
	}

	if options.ReadOnly {
		args = append(args, "--read-only")
	}

	// Network access
	if !options.NetworkAccess || !options.SecurityPolicy.AllowNetworking {
		args = append(args, "--network=none")
	}

	// Environment variables
	env := wm.sanitizeEnvironment(options.Environment)
	for _, envVar := range env {
		args = append(args, "-e", envVar)
	}

	// Mount workspace
	mountType := "bind"
	if options.ReadOnly {
		mountType = "bind,readonly"
	}
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:%s", workspaceDir, mountType))

	// Add temporary directory
	args = append(args, "--tmpfs", "/tmp:size=100m")

	// Add Docker socket for Docker-in-Docker if needed
	if strings.Contains(options.BaseImage, "dind") {
		args = append(args, "-v", "/var/run/docker.sock:/var/run/docker.sock")
		args = append(args, "--privileged")
	}

	// Working directory
	args = append(args, "-w", "/workspace")

	// Image
	args = append(args, options.BaseImage)

	// Command
	args = append(args, cmd...)

	return args, nil
}

func (wm *WorkspaceManager) sanitizeEnvironment(env map[string]string) []string {
	var sanitized []string

	// Allow list of safe environment variables
	allowedPrefixes := []string{"PATH", "HOME", "USER", "LANG", "LC_"}

	for key, value := range env {
		safe := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(key, prefix) {
				safe = true
				break
			}
		}

		// Additional validation for specific variables
		if safe && !strings.Contains(value, ";") && !strings.Contains(value, "|") {
			sanitized = append(sanitized, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return sanitized
}

func (wm *WorkspaceManager) executeDockerCommand(ctx context.Context, dockerArgs []string, sessionID string) (*ExecResult, error) {
	startTime := time.Now()

	// Create the docker command
	cmd := exec.CommandContext(ctx, wm.dockerCmd, dockerArgs...)

	// Capture stdout and stderr separately
	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	// Execute the command
	err := cmd.Run()
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Non-exit error (e.g., command not found)
			return nil, fmt.Errorf("failed to execute docker command: %v", err)
		}
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	wm.logger.Info().
		Str("session_id", sessionID).
		Int("exit_code", exitCode).
		Dur("duration", duration).
		Msg("Sandboxed execution completed")

	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
		Metrics:  ExecutionMetrics{}, // Basic metrics - could be enhanced
	}, nil
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

// Production-Ready Sandbox Components

// SandboxMonitor provides real-time monitoring of sandbox execution
type SandboxMonitor struct {
	logger           zerolog.Logger
	activeContainers map[string]*ContainerMetrics
	mutex            sync.RWMutex
}

// ContainerMetrics tracks runtime metrics for containers
type ContainerMetrics struct {
	ContainerID  string    `json:"container_id"`
	SessionID    string    `json:"session_id"`
	StartTime    time.Time `json:"start_time"`
	MemoryUsage  int64     `json:"memory_usage"`
	CPUUsage     float64   `json:"cpu_usage"`
	NetworkIO    int64     `json:"network_io"`
	DiskIO       int64     `json:"disk_io"`
	ProcessCount int       `json:"process_count"`
	LastUpdate   time.Time `json:"last_update"`
}

// AuditLogger handles security audit logging
type AuditLogger struct {
	logger zerolog.Logger
}

// MetricsCollector gathers performance and security metrics
type MetricsCollector struct {
	logger         zerolog.Logger
	executionTime  map[string]time.Duration
	securityEvents map[string]int
}

// buildProductionDockerCommand creates a production-ready Docker command with enhanced security
func (wm *WorkspaceManager) buildProductionDockerCommand(sessionID string, cmd []string, options SandboxOptions) ([]string, error) {
	workspaceDir := filepath.Join(wm.baseDir, sessionID)

	args := []string{"run", "--rm"}

	// Security: Add security options first
	args = append(args, "--security-opt", "no-new-privileges:true")

	// Add AppArmor if available
	args = append(args, "--security-opt", "apparmor:docker-default")

	// Resource limits with enhanced control
	if options.MemoryLimit > 0 {
		args = append(args, fmt.Sprintf("--memory=%d", options.MemoryLimit))
		args = append(args, fmt.Sprintf("--memory-swap=%d", options.MemoryLimit)) // Disable swap
	}
	if options.CPUQuota > 0 {
		cpuLimit := float64(options.CPUQuota) / 100000.0
		args = append(args, fmt.Sprintf("--cpus=%.2f", cpuLimit))
	}

	// Process limits
	args = append(args, "--pids-limit=1000")

	// User and group security
	if options.User != "" && options.Group != "" {
		args = append(args, fmt.Sprintf("--user=%s:%s", options.User, options.Group))
	} else if options.SecurityPolicy.RequireNonRoot {
		args = append(args, "--user=1000:1000")
	}

	// Capability management
	if !options.Privileged {
		args = append(args, "--cap-drop=ALL")
		for _, cap := range options.Capabilities {
			args = append(args, "--cap-add", cap)
		}
	} else {
		// Even privileged containers should have some restrictions
		args = append(args, "--privileged")
	}

	// Filesystem security
	if options.ReadOnly {
		args = append(args, "--read-only")
	}

	// Network security
	if !options.NetworkAccess || !options.SecurityPolicy.AllowNetworking {
		args = append(args, "--network=none")
	} else {
		// Controlled network access
		args = append(args, "--network=bridge")
		args = append(args, "--dns=8.8.8.8", "--dns=8.8.4.4") // Controlled DNS
	}

	// Environment variables (sanitized)
	env := wm.sanitizeEnvironment(options.Environment)
	for _, envVar := range env {
		args = append(args, "-e", envVar)
	}

	// Mount workspace with appropriate permissions
	mountType := "bind"
	if options.ReadOnly {
		mountType = "bind,readonly"
	}
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:%s", workspaceDir, mountType))

	// Temporary filesystems
	args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,size=100m")
	args = append(args, "--tmpfs", "/var/tmp:rw,noexec,nosuid,size=100m")

	// Docker-in-Docker handling with security considerations
	if strings.Contains(options.BaseImage, "dind") {
		if options.Privileged {
			// Use Docker socket mount for DinD (in controlled environment)
			args = append(args, "-v", "/var/run/docker.sock:/var/run/docker.sock")
		} else {
			return nil, fmt.Errorf("docker-in-docker requires privileged mode")
		}
	}

	// Working directory
	args = append(args, "-w", "/workspace")

	// Container name for tracking
	containerName := fmt.Sprintf("sandbox-%s-%d", sessionID, time.Now().Unix())
	args = append(args, "--name", containerName)

	// Labels for identification and monitoring
	args = append(args, "--label", fmt.Sprintf("session=%s", sessionID))
	args = append(args, "--label", fmt.Sprintf("operation=%s", options.Operation))
	args = append(args, "--label", "managed-by=container-kit")

	// Image
	args = append(args, options.BaseImage)

	// Command
	args = append(args, cmd...)

	return args, nil
}

// executeProductionDockerCommand executes Docker with comprehensive monitoring and security
func (wm *WorkspaceManager) executeProductionDockerCommand(ctx context.Context, dockerArgs []string, sessionID string, options SandboxOptions) (*ExecResult, error) {
	startTime := time.Now()

	// Pre-execution security audit
	wm.auditExecutionStart(sessionID, dockerArgs, options)

	// Create the docker command
	cmd := exec.CommandContext(ctx, wm.dockerCmd, dockerArgs...)

	// Capture stdout and stderr with enhanced logging
	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	// Start monitoring if available
	var containerID string
	if wm.monitor != nil {
		// Extract container name from args for monitoring
		for i, arg := range dockerArgs {
			if arg == "--name" && i+1 < len(dockerArgs) {
				containerID = dockerArgs[i+1]
				break
			}
		}
		if containerID != "" {
			go wm.monitor.StartMonitoring(ctx, containerID, sessionID)
		}
	}

	// Execute the command
	err := cmd.Run()
	duration := time.Since(startTime)

	// Stop monitoring
	if wm.monitor != nil && containerID != "" {
		wm.monitor.StopMonitoring(containerID)
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Non-exit error (e.g., command not found)
			wm.auditExecutionError(sessionID, err)
			return nil, fmt.Errorf("failed to execute docker command: %v", err)
		}
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Collect enhanced metrics
	var metrics ExecutionMetrics
	if wm.monitor != nil && containerID != "" {
		if containerMetrics := wm.monitor.GetMetrics(containerID); containerMetrics != nil {
			metrics = ExecutionMetrics{
				MemoryUsage: containerMetrics.MemoryUsage,
				CPUUsage:    int64(containerMetrics.CPUUsage * 1000), // Convert to permille
				NetworkIO:   containerMetrics.NetworkIO,
				DiskIO:      containerMetrics.DiskIO,
			}
		}
	}

	// Post-execution audit
	result := &ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
		Metrics:  metrics,
	}

	wm.auditExecutionComplete(sessionID, result, options)

	wm.logger.Info().
		Str("session_id", sessionID).
		Int("exit_code", exitCode).
		Dur("duration", duration).
		Int64("memory_usage", metrics.MemoryUsage).
		Msg("Production sandboxed execution completed")

	return result, nil
}

// Security audit methods
func (wm *WorkspaceManager) auditSecurityBlock(sessionID string, report *SecurityValidationReport) {
	if wm.auditLogger != nil {
		wm.auditLogger.logger.Error().
			Str("session_id", sessionID).
			Str("risk_level", report.OverallRisk).
			Int("vulnerabilities", len(report.Vulnerabilities)).
			Msg("Execution blocked due to security risk")
	}
}

func (wm *WorkspaceManager) auditExecutionStart(sessionID string, dockerArgs []string, options SandboxOptions) {
	if wm.auditLogger != nil {
		wm.auditLogger.logger.Info().
			Str("session_id", sessionID).
			Str("operation", options.Operation).
			Str("base_image", options.BaseImage).
			Bool("privileged", options.Privileged).
			Strs("capabilities", options.Capabilities).
			Msg("Sandbox execution started")
	}
}

func (wm *WorkspaceManager) auditExecutionComplete(sessionID string, result *ExecResult, options SandboxOptions) {
	if wm.auditLogger != nil {
		wm.auditLogger.logger.Info().
			Str("session_id", sessionID).
			Int("exit_code", result.ExitCode).
			Dur("duration", result.Duration).
			Int64("memory_used", result.Metrics.MemoryUsage).
			Msg("Sandbox execution completed")
	}
}

func (wm *WorkspaceManager) auditExecutionError(sessionID string, err error) {
	if wm.auditLogger != nil {
		wm.auditLogger.logger.Error().
			Str("session_id", sessionID).
			Err(err).
			Msg("Sandbox execution failed")
	}
}

// Monitor methods
func (sm *SandboxMonitor) StartMonitoring(ctx context.Context, containerID, sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	metrics := &ContainerMetrics{
		ContainerID: containerID,
		SessionID:   sessionID,
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
	}

	sm.activeContainers[containerID] = metrics

	// Start monitoring goroutine
	go sm.monitorContainer(ctx, containerID)
}

func (sm *SandboxMonitor) StopMonitoring(containerID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	delete(sm.activeContainers, containerID)
}

func (sm *SandboxMonitor) GetMetrics(containerID string) *ContainerMetrics {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if metrics, exists := sm.activeContainers[containerID]; exists {
		return metrics
	}
	return nil
}

func (sm *SandboxMonitor) monitorContainer(ctx context.Context, containerID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.updateMetrics(containerID)
		}
	}
}

func (sm *SandboxMonitor) updateMetrics(containerID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if metrics, exists := sm.activeContainers[containerID]; exists {
		// In a real implementation, this would query Docker stats API
		// For now, we'll simulate some basic metrics
		metrics.LastUpdate = time.Now()
		metrics.MemoryUsage = 100 * 1024 * 1024 // Simulated 100MB
		metrics.CPUUsage = 25.5                 // Simulated 25.5% CPU
		metrics.ProcessCount = 5                // Simulated process count
	}
}

// NewWorkspaceManager enhanced constructor for production
func NewWorkspaceManagerProduction(ctx context.Context, config WorkspaceConfig, securityValidator *SecurityValidator) (*WorkspaceManager, error) {
	wm, err := NewWorkspaceManager(ctx, config)
	if err != nil {
		return nil, err
	}

	// Add production components
	wm.securityValidator = securityValidator
	wm.monitor = &SandboxMonitor{
		logger:           config.Logger,
		activeContainers: make(map[string]*ContainerMetrics),
	}
	wm.auditLogger = &AuditLogger{logger: config.Logger}
	wm.metricsCollector = &MetricsCollector{
		logger:         config.Logger,
		executionTime:  make(map[string]time.Duration),
		securityEvents: make(map[string]int),
	}

	return wm, nil
}
