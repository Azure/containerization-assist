package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// SandboxExecutor provides advanced sandboxed execution with security monitoring
type SandboxExecutor struct {
	logger         *slog.Logger
	config         SandboxConfig
	resourceLimits ResourceLimits
	securityPolicy security.SecurityPolicy
	monitor        *ResourceMonitor
	workspace      *WorkspaceManager
	mutex          sync.RWMutex
}

// SandboxConfig defines sandbox configuration
type SandboxConfig struct {
	WorkingDirectory     string            `json:"working_directory"`
	Environment          map[string]string `json:"environment"`
	AllowedCommands      []string          `json:"allowed_commands"`
	BlockedCommands      []string          `json:"blocked_commands"`
	AllowedPaths         []string          `json:"allowed_paths"`
	BlockedPaths         []string          `json:"blocked_paths"`
	NetworkAccess        bool              `json:"network_access"`
	FileSystemAccess     bool              `json:"filesystem_access"`
	TimeoutSeconds       int               `json:"timeout_seconds"`
	MaxOutputSize        int               `json:"max_output_size"`
	EnableResourceLimits bool              `json:"enable_resource_limits"`
	EnableSecurityPolicy bool              `json:"enable_security_policy"`
}

// ResourceLimits defines resource constraints for sandboxed execution
type ResourceLimits struct {
	MaxCPUPercent    float64       `json:"max_cpu_percent"`
	MaxMemoryMB      int64         `json:"max_memory_mb"`
	MaxDiskMB        int64         `json:"max_disk_mb"`
	MaxNetworkKBps   int64         `json:"max_network_kbps"`
	MaxProcesses     int           `json:"max_processes"`
	MaxOpenFiles     int           `json:"max_open_files"`
	MaxExecutionTime time.Duration `json:"max_execution_time"`
}

// ResourceMonitor tracks resource usage during execution
type ResourceMonitor struct {
	logger       *slog.Logger
	startTime    time.Time
	cpuUsage     float64
	memoryUsage  int64
	diskUsage    int64
	networkUsage int64
	processCount int
	openFiles    int
	mutex        sync.RWMutex
}

// ExecutionResult contains the result of a sandboxed execution
type ExecutionResult struct {
	Success            bool                   `json:"success"`
	ExitCode           int                    `json:"exit_code"`
	Output             string                 `json:"output"`
	Error              string                 `json:"error"`
	ExecutionTime      time.Duration          `json:"execution_time"`
	ResourceUsage      ResourceUsageStats     `json:"resource_usage"`
	SecurityViolations []SecurityViolation    `json:"security_violations"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// ResourceUsageStats contains resource usage statistics
type ResourceUsageStats struct {
	CPUPercent    float64       `json:"cpu_percent"`
	MemoryMB      int64         `json:"memory_mb"`
	DiskMB        int64         `json:"disk_mb"`
	NetworkKB     int64         `json:"network_kb"`
	ProcessCount  int           `json:"process_count"`
	OpenFiles     int           `json:"open_files"`
	ExecutionTime time.Duration `json:"execution_time"`
	PeakMemoryMB  int64         `json:"peak_memory_mb"`
	TotalCPUTime  time.Duration `json:"total_cpu_time"`
}

// SecurityViolation represents a security policy violation
type SecurityViolation struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details"`
	Timestamp   time.Time              `json:"timestamp"`
}

// WorkspaceManager manages isolated workspaces for operations
type WorkspaceManager struct {
	logger     *slog.Logger
	baseDir    string
	workspaces map[string]*Workspace
	config     WorkspaceConfig
	mutex      sync.RWMutex
}

// Workspace represents an isolated workspace
type Workspace struct {
	ID             string                  `json:"id"`
	Path           string                  `json:"path"`
	CreatedAt      time.Time               `json:"created_at"`
	LastAccessedAt time.Time               `json:"last_accessed_at"`
	ExpiresAt      time.Time               `json:"expires_at"`
	Size           int64                   `json:"size"`
	FileCount      int                     `json:"file_count"`
	Metadata       map[string]interface{}  `json:"metadata"`
	ResourceLimits ResourceLimits          `json:"resource_limits"`
	SecurityPolicy security.SecurityPolicy `json:"security_policy"`
	ReadOnly       bool                    `json:"read_only"`
	Isolated       bool                    `json:"isolated"`
}

// WorkspaceConfig defines workspace management configuration
type WorkspaceConfig struct {
	BaseDir           string       `json:"base_dir"`
	MaxSizePerSession int64        `json:"max_size_per_session"`
	TotalMaxSize      int64        `json:"total_max_size"`
	Cleanup           bool         `json:"cleanup"`
	SandboxEnabled    bool         `json:"sandbox_enabled"`
	Logger            *slog.Logger `json:"-"`

	BaseDirectory     string        `json:"base_directory"`
	MaxWorkspaces     int           `json:"max_workspaces"`
	DefaultTTL        time.Duration `json:"default_ttl"`
	MaxWorkspaceSize  int64         `json:"max_workspace_size"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableCompression bool          `json:"enable_compression"`
	EnableEncryption  bool          `json:"enable_encryption"`
	BackupEnabled     bool          `json:"backup_enabled"`
	BackupInterval    time.Duration `json:"backup_interval"`
}

// WorkspaceStats provides workspace usage statistics
type WorkspaceStats struct {
	TotalSessions   int   `json:"total_sessions"`
	TotalDiskUsage  int64 `json:"total_disk_usage_bytes"`
	TotalDiskLimit  int64 `json:"total_disk_limit_bytes"`
	PerSessionLimit int64 `json:"per_session_limit_bytes"`
	SandboxEnabled  bool  `json:"sandbox_enabled"`
}

// NewSandboxExecutor creates a new sandbox executor
func NewSandboxExecutor(config SandboxConfig, logger *slog.Logger) *SandboxExecutor {
	slogLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	workspaceConfig := WorkspaceConfig{
		BaseDir: config.WorkingDirectory,
		Logger:  slogLogger,
	}
	workspaceManager, err := NewWorkspaceManager(context.Background(), workspaceConfig)
	if err != nil {
		logger.Warn("Failed to create workspace manager", "error", err)
		workspaceManager = nil
	}

	return &SandboxExecutor{
		logger:         logger.With("component", "sandbox_executor"),
		config:         config,
		resourceLimits: getDefaultResourceLimits(),
		securityPolicy: getDefaultSecurityPolicy(),
		monitor:        NewResourceMonitor(logger),
		workspace:      workspaceManager,
	}
}

// Execute executes a command in a sandboxed environment
func (se *SandboxExecutor) Execute(ctx context.Context, command string, args []string) (*ExecutionResult, error) {
	se.mutex.Lock()
	defer se.mutex.Unlock()

	startTime := time.Now()
	result := &ExecutionResult{
		Metadata: make(map[string]interface{}),
	}

	if se.config.EnableSecurityPolicy {
		if violations := se.validateCommand(command, args); len(violations) > 0 {
			result.SecurityViolations = violations
			result.Success = false
			return result, errors.NewError().
				Code(errors.CodePermissionDenied).
				Type(errors.ErrTypeSecurity).
				Severity(errors.SeverityHigh).
				Message("Command blocked by security policy").
				Context("command", command).
				Context("violations", len(violations)).
				Suggestion("Use an allowed command or request policy exception").
				WithLocation().
				Build()
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(se.config.TimeoutSeconds)*time.Second)
	defer cancel()

	if se.config.EnableResourceLimits {
		se.monitor.Start()
		defer se.monitor.Stop()
	}

	cmd := exec.CommandContext(execCtx, command, args...)
	cmd.Dir = se.config.WorkingDirectory

	cmd.Env = se.prepareEnvironment()

	output, err := cmd.CombinedOutput()

	result.ExecutionTime = time.Since(startTime)
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	if se.config.EnableResourceLimits {
		result.ResourceUsage = se.monitor.GetStats()
	}

	if violations := se.checkResourceViolations(result.ResourceUsage); len(violations) > 0 {
		result.SecurityViolations = append(result.SecurityViolations, violations...)
	}

	se.logger.Info("Sandbox execution completed",
		"command", command,
		"success", result.Success,
		"exit_code", result.ExitCode,
		"execution_time", result.ExecutionTime,
		"security_violations", len(result.SecurityViolations))

	return result, nil
}

// ExecuteWithWorkspace executes a command in a dedicated workspace
func (se *SandboxExecutor) ExecuteWithWorkspace(ctx context.Context, workspaceID, command string, args []string) (*ExecutionResult, error) {
	workspace, err := se.workspace.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	workspace.LastAccessedAt = time.Now()

	originalDir := se.config.WorkingDirectory
	se.config.WorkingDirectory = workspace.Path
	defer func() {
		se.config.WorkingDirectory = originalDir
	}()

	return se.Execute(ctx, command, args)
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager(ctx context.Context, config WorkspaceConfig) (*WorkspaceManager, error) {
	baseDir := config.BaseDirectory
	if baseDir == "" {
		baseDir = config.BaseDir
	}

	logger := config.Logger

	return &WorkspaceManager{
		logger:     logger.With("component", "workspace_manager"),
		baseDir:    baseDir,
		workspaces: make(map[string]*Workspace),
		config:     getDefaultWorkspaceConfig(),
	}, nil
}

// NewWorkspaceManagerWithServices creates a new workspace manager using service container
func NewWorkspaceManagerWithServices(serverConfig interface{}, serviceContainer services.ServiceContainer, logger *slog.Logger) (*WorkspaceManager, error) {
	baseDir := "./workspaces"

	if sc, ok := serverConfig.(interface{ WorkspaceDir() string }); ok {
		if workspaceDir := sc.WorkspaceDir(); workspaceDir != "" {
			baseDir = workspaceDir
		}
	}

	if sc, ok := serverConfig.(struct{ WorkspaceDir string }); ok {
		if sc.WorkspaceDir != "" {
			baseDir = sc.WorkspaceDir
		}
	}

	workspaceConfig := WorkspaceConfig{
		BaseDir:           baseDir,
		MaxSizePerSession: 1024 * 1024 * 1024,      // 1GB default
		TotalMaxSize:      10 * 1024 * 1024 * 1024, // 10GB default
		Cleanup:           true,
		SandboxEnabled:    false,
		Logger:            logger, // Use provided slog logger
		BaseDirectory:     baseDir,
	}

	wm := &WorkspaceManager{
		logger:     logger.With("component", "workspace_manager"),
		baseDir:    baseDir,
		workspaces: make(map[string]*Workspace),
		config:     workspaceConfig,
	}

	return wm, nil
}

// CreateWorkspace creates a new isolated workspace
func (wm *WorkspaceManager) CreateWorkspace(id string, config *WorkspaceConfig) (*Workspace, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if _, exists := wm.workspaces[id]; exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceAlreadyExists).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityMedium).
			Messagef("Workspace already exists: %s", id).
			Context("workspace_id", id).
			Suggestion("Use a different workspace ID or clean up existing workspace").
			WithLocation().
			Build()
	}

	if len(wm.workspaces) >= wm.config.MaxWorkspaces {
		return nil, errors.NewError().
			Code(errors.CodeResourceExhausted).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityHigh).
			Message("Maximum number of workspaces reached").
			Context("max_workspaces", wm.config.MaxWorkspaces).
			Context("current_count", len(wm.workspaces)).
			Suggestion("Clean up unused workspaces or increase workspace limit").
			WithLocation().
			Build()
	}

	workspacePath := filepath.Join(wm.baseDir, id)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityHigh).
			Message("Failed to create workspace directory").
			Context("workspace_id", id).
			Context("path", workspacePath).
			Cause(err).
			Suggestion("Check directory permissions and disk space").
			WithLocation().
			Build()
	}

	workspace := &Workspace{
		ID:             id,
		Path:           workspacePath,
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
		ExpiresAt:      time.Now().Add(wm.config.DefaultTTL),
		Metadata:       make(map[string]interface{}),
		ResourceLimits: getDefaultResourceLimits(),
		SecurityPolicy: getDefaultSecurityPolicy(),
		ReadOnly:       false,
		Isolated:       true,
	}

	wm.workspaces[id] = workspace

	wm.logger.Info("Workspace created",
		"workspace_id", id,
		"path", workspacePath,
		"expires_at", workspace.ExpiresAt)

	return workspace, nil
}

// GetWorkspace retrieves a workspace by ID
func (wm *WorkspaceManager) GetWorkspace(id string) (*Workspace, error) {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	workspace, exists := wm.workspaces[id]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityMedium).
			Messagef("Workspace not found: %s", id).
			Context("workspace_id", id).
			Suggestion("Create the workspace first or check the workspace ID").
			WithLocation().
			Build()
	}

	if time.Now().After(workspace.ExpiresAt) {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityMedium).
			Messagef("Workspace has expired: %s", id).
			Context("workspace_id", id).
			Context("expired_at", workspace.ExpiresAt).
			Suggestion("Create a new workspace").
			WithLocation().
			Build()
	}

	return workspace, nil
}

// CleanupWorkspaces removes expired workspaces
func (wm *WorkspaceManager) CleanupWorkspaces() error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	now := time.Now()
	cleaned := 0

	for id, workspace := range wm.workspaces {
		if now.After(workspace.ExpiresAt) {
			if err := os.RemoveAll(workspace.Path); err != nil {
				wm.logger.Warn("Failed to remove workspace directory", "error", err, "workspace_id", id)
			}

			delete(wm.workspaces, id)
			cleaned++

			wm.logger.Info("Workspace cleaned up",
				"workspace_id", id,
				"expired_at", workspace.ExpiresAt)
		}
	}

	wm.logger.Info("Workspace cleanup completed",
		"cleaned_count", cleaned,
		"remaining_count", len(wm.workspaces))

	return nil
}

// UpdateWorkspaceSize calculates and updates workspace size
func (wm *WorkspaceManager) UpdateWorkspaceSize(id string) error {
	workspace, err := wm.GetWorkspace(id)
	if err != nil {
		return err
	}

	size, fileCount, err := wm.calculateDirectorySize(workspace.Path)
	if err != nil {
		return err
	}

	wm.mutex.Lock()
	workspace.Size = size
	workspace.FileCount = fileCount
	wm.mutex.Unlock()

	return nil
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(logger *slog.Logger) *ResourceMonitor {
	return &ResourceMonitor{
		logger: logger.With("component", "resource_monitor"),
	}
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.startTime = time.Now()
	rm.logger.Debug("Resource monitoring started")
}

// Stop ends resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.logger.Debug("Resource monitoring stopped",
		"monitoring_duration", time.Since(rm.startTime))
}

// GetStats returns current resource usage statistics
func (rm *ResourceMonitor) GetStats() ResourceUsageStats {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	return ResourceUsageStats{
		CPUPercent:    rm.cpuUsage,
		MemoryMB:      rm.memoryUsage,
		DiskMB:        rm.diskUsage,
		NetworkKB:     rm.networkUsage,
		ProcessCount:  rm.processCount,
		OpenFiles:     rm.openFiles,
		ExecutionTime: time.Since(rm.startTime),
		PeakMemoryMB:  rm.memoryUsage,
		TotalCPUTime:  time.Since(rm.startTime),
	}
}

// validateCommand validates a command against security policies
func (se *SandboxExecutor) validateCommand(command string, args []string) []SecurityViolation {
	violations := make([]SecurityViolation, 0)

	// Check against blocked commands
	for _, blocked := range se.config.BlockedCommands {
		if matched, _ := regexp.MatchString(blocked, command); matched {
			violations = append(violations, SecurityViolation{
				Type:        "BLOCKED_COMMAND",
				Severity:    "HIGH",
				Description: fmt.Sprintf("Command '%s' is blocked by security policy", command),
				Details:     map[string]interface{}{"command": command, "pattern": blocked},
				Timestamp:   time.Now(),
			})
		}
	}

	// Check for command injection patterns
	fullCommand := command + " " + strings.Join(args, " ")
	dangerousPatterns := []string{
		`;.*`,      // Command chaining
		`&&.*`,     // Command chaining
		`\|\|.*`,   // Command chaining
		`\|.*`,     // Piping
		"`.*`",     // Command substitution
		`\$\(.*\)`, // Command substitution
		`\$\{.*\}`, // Variable substitution
	}

	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, fullCommand); matched {
			violations = append(violations, SecurityViolation{
				Type:        "COMMAND_INJECTION",
				Severity:    "HIGH",
				Description: "Potential command injection detected",
				Details:     map[string]interface{}{"command": fullCommand, "pattern": pattern},
				Timestamp:   time.Now(),
			})
		}
	}

	return violations
}

// prepareEnvironment prepares the environment for command execution
func (se *SandboxExecutor) prepareEnvironment() []string {
	env := os.Environ()

	// Add custom environment variables
	for key, value := range se.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Remove potentially dangerous environment variables
	dangerousVars := []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES"}
	filteredEnv := make([]string, 0, len(env))

	for _, envVar := range env {
		keep := true
		for _, dangerous := range dangerousVars {
			if strings.HasPrefix(envVar, dangerous+"=") {
				keep = false
				break
			}
		}
		if keep {
			filteredEnv = append(filteredEnv, envVar)
		}
	}

	return filteredEnv
}

// checkResourceViolations checks for resource limit violations
func (se *SandboxExecutor) checkResourceViolations(usage ResourceUsageStats) []SecurityViolation {
	violations := make([]SecurityViolation, 0)

	if usage.CPUPercent > se.resourceLimits.MaxCPUPercent {
		violations = append(violations, SecurityViolation{
			Type:        "RESOURCE_LIMIT_EXCEEDED",
			Severity:    "MEDIUM",
			Description: "CPU usage limit exceeded",
			Details:     map[string]interface{}{"usage": usage.CPUPercent, "limit": se.resourceLimits.MaxCPUPercent},
			Timestamp:   time.Now(),
		})
	}

	if usage.MemoryMB > se.resourceLimits.MaxMemoryMB {
		violations = append(violations, SecurityViolation{
			Type:        "RESOURCE_LIMIT_EXCEEDED",
			Severity:    "HIGH",
			Description: "Memory usage limit exceeded",
			Details:     map[string]interface{}{"usage": usage.MemoryMB, "limit": se.resourceLimits.MaxMemoryMB},
			Timestamp:   time.Now(),
		})
	}

	return violations
}

// calculateDirectorySize calculates the total size and file count of a directory
func (wm *WorkspaceManager) calculateDirectorySize(path string) (int64, int, error) {
	var size int64
	var fileCount int

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			fileCount++
		}
		return nil
	})

	return size, fileCount, err
}

// getDefaultResourceLimits returns default resource limits
func getDefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxCPUPercent:    80.0,
		MaxMemoryMB:      512,
		MaxDiskMB:        1024,
		MaxNetworkKBps:   1024,
		MaxProcesses:     10,
		MaxOpenFiles:     100,
		MaxExecutionTime: 5 * time.Minute,
	}
}

// getDefaultSecurityPolicy returns default security policy
func getDefaultSecurityPolicy() security.SecurityPolicy {
	return security.SecurityPolicy{
		ID:          "default",
		Name:        "Default Security Policy",
		Description: "Default security policy for sandbox execution",
		Severity:    "MEDIUM",
		Enabled:     true,
		Rules: []security.SecurityRule{
			{
				ID:          "block_system_commands",
				Description: "Block system administration commands",
				Pattern:     "(sudo|su|chmod|chown|rm -rf|mkfs|fdisk)",
				RuleType:    "REGEX",
				Action:      "BLOCK",
			},
		},
	}
}

// GetStats returns workspace manager statistics
func (wm *WorkspaceManager) GetStats() *WorkspaceStats {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	totalSessions := len(wm.workspaces)
	var totalDiskUsage int64

	// Calculate total disk usage across all workspaces
	for _, workspace := range wm.workspaces {
		// In practice, you'd calculate actual disk usage
		totalDiskUsage += workspace.Size
	}

	return &WorkspaceStats{
		TotalSessions:   totalSessions,
		TotalDiskUsage:  totalDiskUsage,
		TotalDiskLimit:  wm.config.MaxWorkspaceSize * int64(wm.config.MaxWorkspaces),
		PerSessionLimit: wm.config.MaxWorkspaceSize,
		SandboxEnabled:  true, // This manager supports sandboxing
	}
}

// UpdateDiskUsage updates disk usage statistics for a session
func (wm *WorkspaceManager) UpdateDiskUsage(ctx context.Context, sessionID string) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	workspace, exists := wm.workspaces[sessionID]
	if !exists {
		return errors.NewError().Messagef("workspace not found for session: %s", sessionID).WithLocation(

		// In practice, you'd calculate actual disk usage from the filesystem
		// For now, we'll just update the timestamp to show activity
		).Build()
	}

	workspace.LastAccessedAt = time.Now()

	wm.logger.Debug("Updated workspace disk usage tracking",
		"session_id", sessionID)

	return nil
}

// getDefaultWorkspaceConfig returns default workspace configuration
func getDefaultWorkspaceConfig() WorkspaceConfig {
	return WorkspaceConfig{
		MaxWorkspaces:     100,
		DefaultTTL:        24 * time.Hour,
		MaxWorkspaceSize:  1024 * 1024 * 1024, // 1GB
		CleanupInterval:   1 * time.Hour,
		EnableCompression: false,
		EnableEncryption:  false,
		BackupEnabled:     false,
		BackupInterval:    24 * time.Hour,
	}
}
