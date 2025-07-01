package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SandboxExecutor provides advanced sandboxed execution capabilities
type SandboxExecutor struct {
	workspace        *WorkspaceManager
	logger           zerolog.Logger
	metricsCollector *SandboxMetricsCollector
	securityPolicy   *SecurityPolicyEngine
	resourceMonitor  *ResourceMonitor
}

// SandboxMetricsCollector tracks execution metrics
type SandboxMetricsCollector struct {
	mutex   sync.RWMutex
	metrics map[string]*ExecutionMetrics
	history []ExecutionRecord
}

// SecurityPolicyEngine enforces security policies
type SecurityPolicyEngine struct {
	policies      map[string]SecurityPolicy
	defaultPolicy SecurityPolicy
	auditLog      []SecurityAuditEntry
	mutex         sync.RWMutex
}

// ResourceMonitor tracks resource usage
type ResourceMonitor struct {
	limits         ResourceLimits
	usage          map[string]*ResourceUsage
	alertThreshold float64
	mutex          sync.RWMutex
}

// ExecutionRecord represents a single execution history entry
type ExecutionRecord struct {
	ID            string        `json:"id"`
	SessionID     string        `json:"session_id"`
	Command       []string      `json:"command"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	ExitCode      int           `json:"exit_code"`
	ResourceUsage ResourceUsage `json:"resource_usage"`
	SecurityFlags []string      `json:"security_flags"`
}

// ResourceUsage tracks resource consumption
type ResourceUsage struct {
	CPUTime        time.Duration `json:"cpu_time"`
	MemoryPeak     int64         `json:"memory_peak"`
	NetworkIO      int64         `json:"network_io"`
	DiskIO         int64         `json:"disk_io"`
	ContainerCount int           `json:"container_count"`
}

// SecurityAuditEntry represents a security event
type SecurityAuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	EventType   string    `json:"event_type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Action      string    `json:"action"`
}

// ExecutorOptions extends basic sandbox options for the executor
type ExecutorOptions struct {
	EnableMetrics    bool              `json:"enable_metrics"`
	EnableAudit      bool              `json:"enable_audit"`
	CustomSeccomp    string            `json:"custom_seccomp"`
	AppArmorProfile  string            `json:"apparmor_profile"`
	SELinuxContext   string            `json:"selinux_context"`
	DNSServers       []string          `json:"dns_servers"`
	ExtraHosts       map[string]string `json:"extra_hosts"`
	WorkingDirectory string            `json:"working_directory"`
}

// NewSandboxExecutor creates a new sandbox executor
func NewSandboxExecutor(workspace *WorkspaceManager, logger zerolog.Logger) *SandboxExecutor {
	return &SandboxExecutor{
		workspace:        workspace,
		logger:           logger.With().Str("component", "sandbox_executor").Logger(),
		metricsCollector: NewSandboxMetricsCollector(),
		securityPolicy:   NewSecurityPolicyEngine(),
		resourceMonitor:  NewResourceMonitor(),
	}
}

// NewSandboxMetricsCollector creates a new metrics collector
func NewSandboxMetricsCollector() *SandboxMetricsCollector {
	return &SandboxMetricsCollector{
		metrics: make(map[string]*ExecutionMetrics),
		history: make([]ExecutionRecord, 0),
	}
}

// NewSecurityPolicyEngine creates a new security policy engine
func NewSecurityPolicyEngine() *SecurityPolicyEngine {
	return &SecurityPolicyEngine{
		policies: make(map[string]SecurityPolicy),
		defaultPolicy: SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io", "gcr.io", "quay.io"},
			ResourceLimits: ResourceLimits{
				Memory:    512 * 1024 * 1024,  // 512MB
				CPUQuota:  50000,              // 50% CPU
				DiskSpace: 1024 * 1024 * 1024, // 1GB
			},
		},
		auditLog: make([]SecurityAuditEntry, 0),
	}
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		limits: ResourceLimits{
			Memory:    1024 * 1024 * 1024,     // 1GB default
			CPUQuota:  100000,                 // 100% CPU default
			DiskSpace: 5 * 1024 * 1024 * 1024, // 5GB default
		},
		usage:          make(map[string]*ResourceUsage),
		alertThreshold: 0.8, // Alert at 80% usage
	}
}

// ExecuteAdvanced performs sandboxed execution with advanced features
func (se *SandboxExecutor) ExecuteAdvanced(ctx context.Context, sessionID string, cmd []string, options SandboxOptions) (*ExecResult, error) {
	// Validate security policy
	if err := se.validateAdvancedSecurity(sessionID, options); err != nil {
		se.auditSecurityEvent(sessionID, "EXECUTION_BLOCKED", "HIGH", err.Error(), "DENY")
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Check resource availability
	if err := se.checkResourceAvailability(sessionID, options); err != nil {
		return nil, fmt.Errorf("insufficient resources: %w", err)
	}

	// Record execution start
	record := ExecutionRecord{
		ID:        fmt.Sprintf("%s-%d", sessionID, time.Now().UnixNano()),
		SessionID: sessionID,
		Command:   cmd,
		StartTime: time.Now(),
	}

	// Execute with monitoring
	result, err := se.executeWithMonitoring(ctx, sessionID, cmd, options, &record)

	// Record execution end
	record.EndTime = time.Now()
	if result != nil {
		record.ExitCode = result.ExitCode
	}

	// Store execution record
	se.metricsCollector.addRecord(record)

	// Audit successful execution
	if err == nil {
		se.auditSecurityEvent(sessionID, "EXECUTION_COMPLETED", "INFO",
			fmt.Sprintf("Command executed: %v", cmd), "ALLOW")
	}

	return result, err
}

// executeWithMonitoring executes commands with resource monitoring
func (se *SandboxExecutor) executeWithMonitoring(ctx context.Context, sessionID string, cmd []string, options SandboxOptions, record *ExecutionRecord) (*ExecResult, error) {
	// Build secure Docker command
	dockerArgs, err := se.buildSecureDockerCommand(sessionID, cmd, options)
	if err != nil {
		return nil, err
	}

	// Start resource monitoring
	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	defer cancelMonitor()

	resourceChan := make(chan ResourceUsage, 1)
	go se.monitorResources(monitorCtx, sessionID, resourceChan)

	// Execute command
	result, err := se.workspace.executeDockerCommand(ctx, dockerArgs, sessionID)

	// Collect final resource usage
	select {
	case usage := <-resourceChan:
		record.ResourceUsage = usage
		se.resourceMonitor.updateUsage(sessionID, usage)
	case <-time.After(1 * time.Second):
		// Timeout collecting metrics
		se.logger.Warn().Str("session_id", sessionID).Msg("Timeout collecting resource metrics")
	}

	// Update metrics
	if result != nil && options.EnableMetrics {
		se.metricsCollector.updateMetrics(sessionID, result)
	}

	return result, err
}

// buildSecureDockerCommand builds a secure Docker command with advanced options
func (se *SandboxExecutor) buildSecureDockerCommand(sessionID string, cmd []string, options SandboxOptions) ([]string, error) {
	args := []string{"run", "--rm"}

	// Basic security settings
	args = append(args, "--security-opt", "no-new-privileges:true")

	// Custom seccomp profile
	if options.CustomSeccomp != "" {
		args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", options.CustomSeccomp))
	}

	// AppArmor profile
	if options.AppArmorProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("apparmor=%s", options.AppArmorProfile))
	}

	// SELinux context
	if options.SELinuxContext != "" {
		args = append(args, "--security-opt", fmt.Sprintf("label=%s", options.SELinuxContext))
	}

	// Capabilities
	if len(options.Capabilities) > 0 {
		for _, cap := range options.Capabilities {
			args = append(args, "--cap-add", cap)
		}
	} else {
		// Drop all capabilities by default
		args = append(args, "--cap-drop", "ALL")
	}

	// Resource limits
	if options.MemoryLimit > 0 {
		args = append(args, fmt.Sprintf("--memory=%d", options.MemoryLimit))
		args = append(args, fmt.Sprintf("--memory-swap=%d", options.MemoryLimit)) // Prevent swap
	}

	if options.CPUQuota > 0 {
		cpuLimit := float64(options.CPUQuota) / 100000.0
		args = append(args, fmt.Sprintf("--cpus=%.2f", cpuLimit))
	}

	// User and group
	if options.User != "" && options.Group != "" {
		args = append(args, "--user", fmt.Sprintf("%s:%s", options.User, options.Group))
	} else if options.SecurityPolicy.RequireNonRoot {
		args = append(args, "--user", "1000:1000")
	}

	// Network settings
	if !options.NetworkAccess || !options.SecurityPolicy.AllowNetworking {
		args = append(args, "--network=none")
	} else {
		// Custom DNS if network is enabled
		for _, dns := range options.DNSServers {
			args = append(args, "--dns", dns)
		}
	}

	// Extra hosts
	for hostname, ip := range options.ExtraHosts {
		args = append(args, "--add-host", fmt.Sprintf("%s:%s", hostname, ip))
	}

	// Read-only root filesystem
	if options.ReadOnly {
		args = append(args, "--read-only")
	}

	// Working directory
	if options.WorkingDirectory != "" {
		args = append(args, "-w", options.WorkingDirectory)
	} else {
		args = append(args, "-w", "/workspace")
	}

	// Environment variables
	env := se.workspace.sanitizeEnvironment(options.Environment)
	for _, envVar := range env {
		args = append(args, "-e", envVar)
	}

	// Mounts
	workspaceDir := se.workspace.GetFilePath(sessionID, "")
	args = append(args, "-v", fmt.Sprintf("%s:/workspace:ro", workspaceDir))

	// Temporary directory
	args = append(args, "--tmpfs", "/tmp:size=100m,noexec,nosuid,nodev")

	// Image and command
	args = append(args, options.BaseImage)
	args = append(args, cmd...)

	return args, nil
}

// validateAdvancedSecurity performs advanced security validation
func (se *SandboxExecutor) validateAdvancedSecurity(sessionID string, options SandboxOptions) error {
	// Get applicable security policy
	policy := se.securityPolicy.getPolicy(sessionID)

	// Validate base security policy
	if err := se.workspace.ValidateSecurityPolicy(policy); err != nil {
		return err
	}

	// Validate image is from trusted registry
	if !se.isImageTrusted(options.BaseImage, policy.TrustedRegistries) {
		return fmt.Errorf("image %s not from trusted registry", options.BaseImage)
	}

	// Validate capabilities
	if len(options.Capabilities) > 0 && policy.RequireNonRoot {
		dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_PTRACE"}
		for _, cap := range options.Capabilities {
			for _, dangerous := range dangerousCaps {
				if strings.EqualFold(cap, dangerous) {
					return fmt.Errorf("dangerous capability requested: %s", cap)
				}
			}
		}
	}

	// Validate resource limits
	if options.MemoryLimit > policy.ResourceLimits.Memory {
		return fmt.Errorf("requested memory %d exceeds policy limit %d",
			options.MemoryLimit, policy.ResourceLimits.Memory)
	}

	return nil
}

// checkResourceAvailability checks if resources are available
func (se *SandboxExecutor) checkResourceAvailability(sessionID string, options SandboxOptions) error {
	se.resourceMonitor.mutex.RLock()
	defer se.resourceMonitor.mutex.RUnlock()

	// Calculate total current usage
	var totalMemory int64
	var totalCPU int64
	for _, usage := range se.resourceMonitor.usage {
		totalMemory += usage.MemoryPeak
		totalCPU += int64(usage.CPUTime)
	}

	// Check if adding this would exceed limits
	if totalMemory+options.MemoryLimit > se.resourceMonitor.limits.Memory {
		return fmt.Errorf("insufficient memory: %d available, %d requested",
			se.resourceMonitor.limits.Memory-totalMemory, options.MemoryLimit)
	}

	return nil
}

// monitorResources monitors resource usage during execution
func (se *SandboxExecutor) monitorResources(ctx context.Context, sessionID string, resourceChan chan<- ResourceUsage) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	usage := ResourceUsage{
		ContainerCount: 1,
	}

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			usage.CPUTime = time.Since(startTime)
			resourceChan <- usage
			return
		case <-ticker.C:
			// In production, would query actual container stats
			// For now, simulate resource tracking
			usage.CPUTime = time.Since(startTime)
			usage.MemoryPeak = int64(time.Since(startTime).Seconds() * 1024 * 1024) // Simulate memory usage

			// Check for resource alerts
			se.checkResourceAlerts(sessionID, usage)
		}
	}
}

// checkResourceAlerts checks if resource usage exceeds thresholds
func (se *SandboxExecutor) checkResourceAlerts(sessionID string, usage ResourceUsage) {
	se.resourceMonitor.mutex.RLock()
	limits := se.resourceMonitor.limits
	threshold := se.resourceMonitor.alertThreshold
	se.resourceMonitor.mutex.RUnlock()

	// Check memory threshold
	if float64(usage.MemoryPeak) > float64(limits.Memory)*threshold {
		se.logger.Warn().
			Str("session_id", sessionID).
			Int64("memory_usage", usage.MemoryPeak).
			Int64("memory_limit", limits.Memory).
			Msg("Memory usage exceeds alert threshold")
	}
}

// auditSecurityEvent records a security event
func (se *SandboxExecutor) auditSecurityEvent(sessionID, eventType, severity, description, action string) {
	entry := SecurityAuditEntry{
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		EventType:   eventType,
		Severity:    severity,
		Description: description,
		Action:      action,
	}

	se.securityPolicy.mutex.Lock()
	se.securityPolicy.auditLog = append(se.securityPolicy.auditLog, entry)
	se.securityPolicy.mutex.Unlock()

	se.logger.Info().
		Str("session_id", sessionID).
		Str("event_type", eventType).
		Str("severity", severity).
		Str("action", action).
		Msg("Security event audited")
}

// isImageTrusted checks if an image is from a trusted registry
func (se *SandboxExecutor) isImageTrusted(image string, trustedRegistries []string) bool {
	for _, registry := range trustedRegistries {
		if strings.HasPrefix(image, registry+"/") || image == registry {
			return true
		}
		// Check for library images (e.g., "alpine" -> "docker.io/library/alpine")
		if !strings.Contains(image, "/") && registry == "docker.io" {
			return true
		}
	}
	return false
}

// GetExecutionHistory returns execution history
func (se *SandboxExecutor) GetExecutionHistory(sessionID string) []ExecutionRecord {
	se.metricsCollector.mutex.RLock()
	defer se.metricsCollector.mutex.RUnlock()

	var history []ExecutionRecord
	for _, record := range se.metricsCollector.history {
		if record.SessionID == sessionID {
			history = append(history, record)
		}
	}
	return history
}

// GetSecurityAuditLog returns security audit entries
func (se *SandboxExecutor) GetSecurityAuditLog(sessionID string) []SecurityAuditEntry {
	se.securityPolicy.mutex.RLock()
	defer se.securityPolicy.mutex.RUnlock()

	var entries []SecurityAuditEntry
	for _, entry := range se.securityPolicy.auditLog {
		if entry.SessionID == sessionID || sessionID == "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetResourceUsage returns current resource usage
func (se *SandboxExecutor) GetResourceUsage() map[string]*ResourceUsage {
	se.resourceMonitor.mutex.RLock()
	defer se.resourceMonitor.mutex.RUnlock()

	usage := make(map[string]*ResourceUsage)
	for k, v := range se.resourceMonitor.usage {
		usage[k] = v
	}
	return usage
}

// Helper methods for internal components

func (smc *SandboxMetricsCollector) updateMetrics(sessionID string, result *ExecResult) {
	smc.mutex.Lock()
	defer smc.mutex.Unlock()

	if smc.metrics[sessionID] == nil {
		smc.metrics[sessionID] = &ExecutionMetrics{}
	}

	// Update metrics based on result
	// TODO: Track success/failure metrics based on result.ExitCode
}

func (smc *SandboxMetricsCollector) addRecord(record ExecutionRecord) {
	smc.mutex.Lock()
	defer smc.mutex.Unlock()

	smc.history = append(smc.history, record)

	// Keep only last 1000 records
	if len(smc.history) > 1000 {
		smc.history = smc.history[len(smc.history)-1000:]
	}
}

func (spe *SecurityPolicyEngine) getPolicy(sessionID string) SecurityPolicy {
	spe.mutex.RLock()
	defer spe.mutex.RUnlock()

	if policy, exists := spe.policies[sessionID]; exists {
		return policy
	}
	return spe.defaultPolicy
}

func (rm *ResourceMonitor) updateUsage(sessionID string, usage ResourceUsage) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if rm.usage[sessionID] == nil {
		rm.usage[sessionID] = &ResourceUsage{}
	}

	// Update with latest usage
	rm.usage[sessionID] = &usage
}

// ExportMetrics exports execution metrics in JSON format
func (se *SandboxExecutor) ExportMetrics(ctx context.Context) ([]byte, error) {
	se.metricsCollector.mutex.RLock()
	defer se.metricsCollector.mutex.RUnlock()

	data := struct {
		Timestamp time.Time                    `json:"timestamp"`
		Metrics   map[string]*ExecutionMetrics `json:"metrics"`
		History   []ExecutionRecord            `json:"history"`
		Resources map[string]*ResourceUsage    `json:"resources"`
	}{
		Timestamp: time.Now(),
		Metrics:   se.metricsCollector.metrics,
		History:   se.metricsCollector.history,
		Resources: se.GetResourceUsage(),
	}

	return json.MarshalIndent(data, "", "  ")
}
