package session

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ResourceMonitor provides comprehensive session resource monitoring and cleanup
type ResourceMonitor struct {
	logger         zerolog.Logger
	sessionManager *SessionManager
	mutex          sync.RWMutex

	// Configuration
	monitoringInterval time.Duration
	cleanupInterval    time.Duration
	resourceLimits     ResourceLimits
	enableAutoCleanup  bool

	// Monitoring state
	isMonitoring      bool
	monitoringContext context.Context
	cancelMonitoring  context.CancelFunc

	// Resource tracking
	resourceSnapshots []ResourceSnapshot
	maxSnapshots      int
	currentResources  *CurrentResourceUsage

	// Cleanup automation
	cleanupRules      []CleanupRule
	cleanupHistory    []CleanupEvent
	maxCleanupHistory int

	// Alerts and thresholds
	alertThresholds map[string]AlertThreshold
	activeAlerts    map[string]*ActiveAlert
	alertCallback   AlertCallback
}

// ResourceLimits defines limits for session resources
type ResourceLimits struct {
	MaxMemoryUsage  int64         `json:"max_memory_usage"` // Bytes
	MaxDiskUsage    int64         `json:"max_disk_usage"`   // Bytes
	MaxSessionCount int           `json:"max_session_count"`
	MaxSessionAge   time.Duration `json:"max_session_age"`
	MaxIdleTime     time.Duration `json:"max_idle_time"`
	MaxFileHandles  int           `json:"max_file_handles"`
	MaxGoroutines   int           `json:"max_goroutines"`
	CPUThresholdPct float64       `json:"cpu_threshold_pct"`
}

// ResourceSnapshot captures resource usage at a point in time
type ResourceSnapshot struct {
	Timestamp      time.Time         `json:"timestamp"`
	SessionCount   int               `json:"session_count"`
	ActiveSessions int               `json:"active_sessions"`
	IdleSessions   int               `json:"idle_sessions"`
	MemoryUsage    MemoryUsage       `json:"memory_usage"`
	DiskUsage      DiskUsage         `json:"disk_usage"`
	SystemLoad     SystemLoad        `json:"system_load"`
	FileHandles    int               `json:"file_handles"`
	GoroutineCount int               `json:"goroutine_count"`
	SessionDetails []SessionResource `json:"session_details"`
}

// CurrentResourceUsage tracks current resource consumption
type CurrentResourceUsage struct {
	mutex            sync.RWMutex
	TotalMemory      int64     `json:"total_memory"`
	TotalDisk        int64     `json:"total_disk"`
	ActiveSessions   int       `json:"active_sessions"`
	OldestSession    time.Time `json:"oldest_session"`
	ResourcePressure float64   `json:"resource_pressure"` // 0-100%
	HealthStatus     string    `json:"health_status"`     // "HEALTHY", "WARNING", "CRITICAL"
	LastUpdated      time.Time `json:"last_updated"`
}

// MemoryUsage details memory consumption
type MemoryUsage struct {
	HeapAlloc      uint64  `json:"heap_alloc"`
	HeapSys        uint64  `json:"heap_sys"`
	HeapIdle       uint64  `json:"heap_idle"`
	HeapInuse      uint64  `json:"heap_inuse"`
	StackInuse     uint64  `json:"stack_inuse"`
	SystemTotal    uint64  `json:"system_total"`
	GCPauseTotal   uint64  `json:"gc_pause_total"`
	NumGC          uint32  `json:"num_gc"`
	MemoryPressure float64 `json:"memory_pressure"` // 0-100%
}

// DiskUsage details disk consumption
type DiskUsage struct {
	WorkspaceSize   int64   `json:"workspace_size"`
	TempSize        int64   `json:"temp_size"`
	LogSize         int64   `json:"log_size"`
	CacheSize       int64   `json:"cache_size"`
	FreeSpace       int64   `json:"free_space"`
	TotalSpace      int64   `json:"total_space"`
	UsagePercentage float64 `json:"usage_percentage"`
}

// SystemLoad provides system load information
type SystemLoad struct {
	CPUPercent       float64 `json:"cpu_percent"`
	LoadAverage1Min  float64 `json:"load_average_1min"`
	LoadAverage5Min  float64 `json:"load_average_5min"`
	LoadAverage15Min float64 `json:"load_average_15min"`
	ProcessCount     int     `json:"process_count"`
	ThreadCount      int     `json:"thread_count"`
}

// SessionResource tracks per-session resource usage
type SessionResource struct {
	SessionID     string        `json:"session_id"`
	MemoryUsage   int64         `json:"memory_usage"`
	DiskUsage     int64         `json:"disk_usage"`
	FileHandles   int           `json:"file_handles"`
	Age           time.Duration `json:"age"`
	IdleTime      time.Duration `json:"idle_time"`
	LastActivity  time.Time     `json:"last_activity"`
	ResourceScore float64       `json:"resource_score"` // Resource efficiency score
}

// CleanupRule defines automatic cleanup criteria
type CleanupRule struct {
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Enabled        bool               `json:"enabled"`
	Priority       int                `json:"priority"` // Higher number = higher priority
	Conditions     []CleanupCondition `json:"conditions"`
	Actions        []CleanupAction    `json:"actions"`
	Cooldown       time.Duration      `json:"cooldown"`                 // Minimum time between executions
	MaxExecutions  int                `json:"max_executions,omitempty"` // 0 = unlimited
	ExecutionCount int                `json:"execution_count"`
	LastExecuted   time.Time          `json:"last_executed"`
}

// CleanupCondition defines when cleanup should trigger
type CleanupCondition struct {
	Type        string      `json:"type"`      // "memory", "disk", "age", "idle", "count"
	Operator    string      `json:"operator"`  // "gt", "lt", "eq", "gte", "lte"
	Threshold   interface{} `json:"threshold"` // Value to compare against
	Scope       string      `json:"scope"`     // "session", "system", "total"
	Description string      `json:"description"`
}

// CleanupAction defines what action to take during cleanup
type CleanupAction struct {
	Type        string                 `json:"type"`       // "terminate", "archive", "compress", "alert"
	Target      string                 `json:"target"`     // What to target for cleanup
	Parameters  map[string]interface{} `json:"parameters"` // Action-specific parameters
	Description string                 `json:"description"`
}

// CleanupEvent records cleanup execution history
type CleanupEvent struct {
	ID               string        `json:"id"`
	Timestamp        time.Time     `json:"timestamp"`
	RuleName         string        `json:"rule_name"`
	Trigger          string        `json:"trigger"` // What triggered the cleanup
	ActionsExecuted  []string      `json:"actions_executed"`
	SessionsAffected []string      `json:"sessions_affected"`
	ResourcesFreed   ResourceFreed `json:"resources_freed"`
	Duration         time.Duration `json:"duration"`
	Success          bool          `json:"success"`
	ErrorMessage     string        `json:"error_message,omitempty"`
}

// ResourceFreed tracks resources released during cleanup
type ResourceFreed struct {
	Memory      int64   `json:"memory_bytes"`
	Disk        int64   `json:"disk_bytes"`
	FileHandles int     `json:"file_handles"`
	Sessions    int     `json:"sessions"`
	ImpactScore float64 `json:"impact_score"` // Overall impact of cleanup
}

// AlertThreshold defines when to trigger alerts
type AlertThreshold struct {
	Name              string        `json:"name"`
	ResourceType      string        `json:"resource_type"` // "memory", "disk", "sessions", "cpu"
	WarningThreshold  float64       `json:"warning_threshold"`
	CriticalThreshold float64       `json:"critical_threshold"`
	Duration          time.Duration `json:"duration"` // How long threshold must be exceeded
	Enabled           bool          `json:"enabled"`
}

// ActiveAlert represents an active alert condition
type ActiveAlert struct {
	ID             string    `json:"id"`
	ThresholdName  string    `json:"threshold_name"`
	Level          string    `json:"level"` // "WARNING", "CRITICAL"
	StartTime      time.Time `json:"start_time"`
	LastUpdate     time.Time `json:"last_update"`
	CurrentValue   float64   `json:"current_value"`
	ThresholdValue float64   `json:"threshold_value"`
	Message        string    `json:"message"`
	Acknowledged   bool      `json:"acknowledged"`
}

// AlertCallback is called when alerts are triggered
type AlertCallback func(alert *ActiveAlert)

// ResourceMonitorConfig configures the resource monitor
type ResourceMonitorConfig struct {
	MonitoringInterval time.Duration  `json:"monitoring_interval"`
	CleanupInterval    time.Duration  `json:"cleanup_interval"`
	ResourceLimits     ResourceLimits `json:"resource_limits"`
	EnableAutoCleanup  bool           `json:"enable_auto_cleanup"`
	MaxSnapshots       int            `json:"max_snapshots"`
	MaxCleanupHistory  int            `json:"max_cleanup_history"`
	AlertCallback      AlertCallback  `json:"-"`
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(sessionManager *SessionManager, config ResourceMonitorConfig, logger zerolog.Logger) *ResourceMonitor {
	monitor := &ResourceMonitor{
		logger:             logger.With().Str("component", "resource_monitor").Logger(),
		sessionManager:     sessionManager,
		monitoringInterval: config.MonitoringInterval,
		cleanupInterval:    config.CleanupInterval,
		resourceLimits:     config.ResourceLimits,
		enableAutoCleanup:  config.EnableAutoCleanup,
		maxSnapshots:       config.MaxSnapshots,
		maxCleanupHistory:  config.MaxCleanupHistory,
		alertCallback:      config.AlertCallback,

		resourceSnapshots: make([]ResourceSnapshot, 0, config.MaxSnapshots),
		currentResources:  &CurrentResourceUsage{},
		cleanupRules:      make([]CleanupRule, 0),
		cleanupHistory:    make([]CleanupEvent, 0, config.MaxCleanupHistory),
		alertThresholds:   make(map[string]AlertThreshold),
		activeAlerts:      make(map[string]*ActiveAlert),
	}

	// Set default values
	if monitor.monitoringInterval == 0 {
		monitor.monitoringInterval = 30 * time.Second
	}
	if monitor.cleanupInterval == 0 {
		monitor.cleanupInterval = 5 * time.Minute
	}
	if monitor.maxSnapshots == 0 {
		monitor.maxSnapshots = 1000
	}
	if monitor.maxCleanupHistory == 0 {
		monitor.maxCleanupHistory = 100
	}

	// Initialize default cleanup rules
	monitor.initializeDefaultCleanupRules()

	// Initialize default alert thresholds
	monitor.initializeDefaultAlertThresholds()

	return monitor
}

// StartMonitoring begins resource monitoring
func (rm *ResourceMonitor) StartMonitoring(ctx context.Context) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if rm.isMonitoring {
		return fmt.Errorf("resource monitoring is already running")
	}

	rm.monitoringContext, rm.cancelMonitoring = context.WithCancel(ctx)
	rm.isMonitoring = true

	// Start monitoring goroutine
	go rm.monitoringLoop()

	// Start cleanup goroutine if enabled
	if rm.enableAutoCleanup {
		go rm.cleanupLoop()
	}

	rm.logger.Info().
		Dur("monitoring_interval", rm.monitoringInterval).
		Dur("cleanup_interval", rm.cleanupInterval).
		Bool("auto_cleanup", rm.enableAutoCleanup).
		Msg("Resource monitoring started")

	return nil
}

// StopMonitoring stops resource monitoring
func (rm *ResourceMonitor) StopMonitoring() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if !rm.isMonitoring {
		return
	}

	if rm.cancelMonitoring != nil {
		rm.cancelMonitoring()
	}

	rm.isMonitoring = false
	rm.logger.Info().Msg("Resource monitoring stopped")
}

// GetCurrentResources returns current resource usage
func (rm *ResourceMonitor) GetCurrentResources() *CurrentResourceUsage {
	rm.currentResources.mutex.RLock()
	defer rm.currentResources.mutex.RUnlock()

	// Return a copy
	current := *rm.currentResources
	return &current
}

// GetResourceHistory returns recent resource snapshots
func (rm *ResourceMonitor) GetResourceHistory(limit int) []ResourceSnapshot {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if limit <= 0 || limit > len(rm.resourceSnapshots) {
		limit = len(rm.resourceSnapshots)
	}

	// Return most recent snapshots
	start := len(rm.resourceSnapshots) - limit
	result := make([]ResourceSnapshot, limit)
	copy(result, rm.resourceSnapshots[start:])

	return result
}

// AddCleanupRule adds a new cleanup rule
func (rm *ResourceMonitor) AddCleanupRule(rule CleanupRule) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.cleanupRules = append(rm.cleanupRules, rule)

	rm.logger.Info().
		Str("rule_name", rule.Name).
		Bool("enabled", rule.Enabled).
		Int("priority", rule.Priority).
		Msg("Added cleanup rule")
}

// SetAlertThreshold sets an alert threshold
func (rm *ResourceMonitor) SetAlertThreshold(name string, threshold AlertThreshold) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.alertThresholds[name] = threshold

	rm.logger.Info().
		Str("threshold_name", name).
		Str("resource_type", threshold.ResourceType).
		Float64("warning", threshold.WarningThreshold).
		Float64("critical", threshold.CriticalThreshold).
		Msg("Set alert threshold")
}

// GetActiveAlerts returns currently active alerts
func (rm *ResourceMonitor) GetActiveAlerts() []*ActiveAlert {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	alerts := make([]*ActiveAlert, 0, len(rm.activeAlerts))
	for _, alert := range rm.activeAlerts {
		alertCopy := *alert
		alerts = append(alerts, &alertCopy)
	}

	return alerts
}

// AcknowledgeAlert acknowledges an active alert
func (rm *ResourceMonitor) AcknowledgeAlert(alertID string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	alert, exists := rm.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Acknowledged = true
	alert.LastUpdate = time.Now()

	rm.logger.Info().
		Str("alert_id", alertID).
		Str("threshold", alert.ThresholdName).
		Msg("Alert acknowledged")

	return nil
}

// GetCleanupHistory returns recent cleanup events
func (rm *ResourceMonitor) GetCleanupHistory(limit int) []CleanupEvent {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if limit <= 0 || limit > len(rm.cleanupHistory) {
		limit = len(rm.cleanupHistory)
	}

	// Return most recent events
	start := len(rm.cleanupHistory) - limit
	result := make([]CleanupEvent, limit)
	copy(result, rm.cleanupHistory[start:])

	return result
}

// TriggerCleanup manually triggers cleanup evaluation
func (rm *ResourceMonitor) TriggerCleanup() error {
	if !rm.isMonitoring {
		return fmt.Errorf("resource monitoring is not running")
	}

	go rm.evaluateCleanup()
	return nil
}

// Private methods

func (rm *ResourceMonitor) monitoringLoop() {
	ticker := time.NewTicker(rm.monitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.monitoringContext.Done():
			return
		case <-ticker.C:
			rm.collectResourceSnapshot()
			rm.updateCurrentResources()
			rm.evaluateAlerts()
		}
	}
}

func (rm *ResourceMonitor) cleanupLoop() {
	ticker := time.NewTicker(rm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.monitoringContext.Done():
			return
		case <-ticker.C:
			rm.evaluateCleanup()
		}
	}
}

func (rm *ResourceMonitor) collectResourceSnapshot() {
	snapshot := ResourceSnapshot{
		Timestamp: time.Now(),
	}

	// Collect session statistics
	sessions := rm.getSessionDataList()
	snapshot.SessionCount = len(sessions)

	activeSessions := 0
	idleSessions := 0
	sessionDetails := make([]SessionResource, 0, len(sessions))

	for _, session := range sessions {
		sessionResource := rm.collectSessionResource(session)
		sessionDetails = append(sessionDetails, sessionResource)

		if time.Since(sessionResource.LastActivity) > 10*time.Minute {
			idleSessions++
		} else {
			activeSessions++
		}
	}

	snapshot.ActiveSessions = activeSessions
	snapshot.IdleSessions = idleSessions
	snapshot.SessionDetails = sessionDetails

	// Collect memory statistics
	snapshot.MemoryUsage = rm.collectMemoryUsage()

	// Collect disk usage
	snapshot.DiskUsage = rm.collectDiskUsage()

	// Collect system load
	snapshot.SystemLoad = rm.collectSystemLoad()

	// Collect file handles and goroutines
	snapshot.FileHandles = rm.countFileHandles()
	snapshot.GoroutineCount = runtime.NumGoroutine()

	// Store snapshot
	rm.mutex.Lock()
	rm.resourceSnapshots = append(rm.resourceSnapshots, snapshot)

	// Limit snapshot history
	if len(rm.resourceSnapshots) > rm.maxSnapshots {
		rm.resourceSnapshots = rm.resourceSnapshots[1:]
	}
	rm.mutex.Unlock()

	rm.logger.Debug().
		Int("session_count", snapshot.SessionCount).
		Int("active_sessions", snapshot.ActiveSessions).
		Int("idle_sessions", snapshot.IdleSessions).
		Uint64("heap_alloc", snapshot.MemoryUsage.HeapAlloc).
		Int64("disk_usage", snapshot.DiskUsage.WorkspaceSize).
		Msg("Collected resource snapshot")
}

func (rm *ResourceMonitor) collectSessionResource(session SessionData) SessionResource {
	sessionResource := SessionResource{
		SessionID:    session.ID,
		DiskUsage:    session.DiskUsage,
		Age:          time.Since(session.CreatedAt),
		LastActivity: session.UpdatedAt,
		IdleTime:     time.Since(session.UpdatedAt),
	}

	// Calculate resource score (efficiency metric)
	if sessionResource.Age > 0 {
		ageHours := sessionResource.Age.Hours()
		diskMB := float64(sessionResource.DiskUsage) / (1024 * 1024)

		// Lower score for sessions that use more resources relative to their age/activity
		sessionResource.ResourceScore = 100.0 / (1.0 + diskMB/(ageHours+1))
	}

	return sessionResource
}

func (rm *ResourceMonitor) collectMemoryUsage() MemoryUsage {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memUsage := MemoryUsage{
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		StackInuse:   m.StackInuse,
		SystemTotal:  m.Sys,
		GCPauseTotal: m.PauseTotalNs,
		NumGC:        m.NumGC,
	}

	// Calculate memory pressure
	if rm.resourceLimits.MaxMemoryUsage > 0 {
		memUsage.MemoryPressure = float64(memUsage.HeapAlloc) / float64(rm.resourceLimits.MaxMemoryUsage) * 100
	}

	return memUsage
}

func (rm *ResourceMonitor) collectDiskUsage() DiskUsage {
	// This is a simplified implementation
	// In a real system, you'd use system calls to get actual disk usage

	diskUsage := DiskUsage{
		WorkspaceSize: rm.calculateWorkspaceSize(),
		TempSize:      rm.calculateTempSize(),
		LogSize:       rm.calculateLogSize(),
		CacheSize:     rm.calculateCacheSize(),
	}

	// Calculate total usage
	diskUsage.TotalSpace = diskUsage.WorkspaceSize + diskUsage.TempSize + diskUsage.LogSize + diskUsage.CacheSize

	if rm.resourceLimits.MaxDiskUsage > 0 {
		diskUsage.UsagePercentage = float64(diskUsage.TotalSpace) / float64(rm.resourceLimits.MaxDiskUsage) * 100
	}

	return diskUsage
}

func (rm *ResourceMonitor) collectSystemLoad() SystemLoad {
	// This is a simplified implementation
	// In a real system, you'd read from /proc/loadavg and /proc/stat

	return SystemLoad{
		CPUPercent:      rm.calculateCPUUsage(),
		LoadAverage1Min: 0.0,                    // Would read from system
		ProcessCount:    runtime.NumGoroutine(), // Simplified
		ThreadCount:     runtime.NumGoroutine(),
	}
}

func (rm *ResourceMonitor) calculateWorkspaceSize() int64 {
	// Simplified calculation - sum session disk usage
	sessions := rm.getSessionDataList()
	total := int64(0)
	for _, session := range sessions {
		total += session.DiskUsage
	}
	return total
}

func (rm *ResourceMonitor) calculateTempSize() int64 {
	// Simplified - would scan temp directories
	return 0
}

func (rm *ResourceMonitor) calculateLogSize() int64 {
	// Simplified - would scan log directories
	return 0
}

func (rm *ResourceMonitor) calculateCacheSize() int64 {
	// Simplified - would scan cache directories
	return 0
}

func (rm *ResourceMonitor) calculateCPUUsage() float64 {
	// Simplified - in reality would read from /proc/stat
	return 0.0
}

func (rm *ResourceMonitor) countFileHandles() int {
	// Simplified - would read from /proc/self/fd or similar
	return 0
}

func (rm *ResourceMonitor) updateCurrentResources() {
	rm.currentResources.mutex.Lock()
	defer rm.currentResources.mutex.Unlock()

	// Get latest snapshot
	rm.mutex.RLock()
	if len(rm.resourceSnapshots) == 0 {
		rm.mutex.RUnlock()
		return
	}
	latest := rm.resourceSnapshots[len(rm.resourceSnapshots)-1]
	rm.mutex.RUnlock()

	// Update current resources
	rm.currentResources.TotalMemory = int64(latest.MemoryUsage.HeapAlloc)
	rm.currentResources.TotalDisk = latest.DiskUsage.TotalSpace
	rm.currentResources.ActiveSessions = latest.ActiveSessions
	rm.currentResources.LastUpdated = time.Now()

	// Find oldest session
	oldestTime := time.Now()
	for _, session := range latest.SessionDetails {
		if session.LastActivity.Before(oldestTime) {
			oldestTime = session.LastActivity
		}
	}
	rm.currentResources.OldestSession = oldestTime

	// Calculate resource pressure and health
	rm.calculateResourcePressure()
	rm.updateHealthStatus()
}

func (rm *ResourceMonitor) calculateResourcePressure() {
	memoryPressure := float64(0)
	diskPressure := float64(0)
	sessionPressure := float64(0)

	if rm.resourceLimits.MaxMemoryUsage > 0 {
		memoryPressure = float64(rm.currentResources.TotalMemory) / float64(rm.resourceLimits.MaxMemoryUsage) * 100
	}

	if rm.resourceLimits.MaxDiskUsage > 0 {
		diskPressure = float64(rm.currentResources.TotalDisk) / float64(rm.resourceLimits.MaxDiskUsage) * 100
	}

	if rm.resourceLimits.MaxSessionCount > 0 {
		sessionPressure = float64(rm.currentResources.ActiveSessions) / float64(rm.resourceLimits.MaxSessionCount) * 100
	}

	// Take the maximum pressure as overall pressure
	rm.currentResources.ResourcePressure = memoryPressure
	if diskPressure > rm.currentResources.ResourcePressure {
		rm.currentResources.ResourcePressure = diskPressure
	}
	if sessionPressure > rm.currentResources.ResourcePressure {
		rm.currentResources.ResourcePressure = sessionPressure
	}
}

func (rm *ResourceMonitor) updateHealthStatus() {
	pressure := rm.currentResources.ResourcePressure

	switch {
	case pressure >= 90:
		rm.currentResources.HealthStatus = "CRITICAL"
	case pressure >= 70:
		rm.currentResources.HealthStatus = "WARNING"
	default:
		rm.currentResources.HealthStatus = "HEALTHY"
	}
}

func (rm *ResourceMonitor) evaluateAlerts() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	current := rm.currentResources
	current.mutex.RLock()
	defer current.mutex.RUnlock()

	for name, threshold := range rm.alertThresholds {
		if !threshold.Enabled {
			continue
		}

		currentValue := rm.getCurrentValueForThreshold(threshold.ResourceType)

		// Check warning threshold
		if currentValue >= threshold.WarningThreshold {
			rm.handleAlert(name, "WARNING", currentValue, threshold.WarningThreshold, threshold)
		}

		// Check critical threshold
		if currentValue >= threshold.CriticalThreshold {
			rm.handleAlert(name, "CRITICAL", currentValue, threshold.CriticalThreshold, threshold)
		}

		// Clear alert if value is below warning threshold
		if currentValue < threshold.WarningThreshold {
			rm.clearAlert(name)
		}
	}
}

func (rm *ResourceMonitor) getCurrentValueForThreshold(resourceType string) float64 {
	switch resourceType {
	case "memory":
		return rm.currentResources.ResourcePressure
	case "disk":
		if rm.resourceLimits.MaxDiskUsage > 0 {
			return float64(rm.currentResources.TotalDisk) / float64(rm.resourceLimits.MaxDiskUsage) * 100
		}
	case "sessions":
		if rm.resourceLimits.MaxSessionCount > 0 {
			return float64(rm.currentResources.ActiveSessions) / float64(rm.resourceLimits.MaxSessionCount) * 100
		}
	case "cpu":
		// Would implement CPU monitoring
		return 0.0
	}
	return 0.0
}

func (rm *ResourceMonitor) handleAlert(name, level string, currentValue, thresholdValue float64, threshold AlertThreshold) {
	alertID := fmt.Sprintf("%s_%s", name, level)

	// Check if alert already exists
	if existingAlert, exists := rm.activeAlerts[alertID]; exists {
		existingAlert.LastUpdate = time.Now()
		existingAlert.CurrentValue = currentValue
		return
	}

	// Create new alert
	alert := &ActiveAlert{
		ID:             alertID,
		ThresholdName:  name,
		Level:          level,
		StartTime:      time.Now(),
		LastUpdate:     time.Now(),
		CurrentValue:   currentValue,
		ThresholdValue: thresholdValue,
		Message:        fmt.Sprintf("%s %s threshold exceeded: %.2f%% >= %.2f%%", threshold.ResourceType, level, currentValue, thresholdValue),
		Acknowledged:   false,
	}

	rm.activeAlerts[alertID] = alert

	// Call alert callback if configured
	if rm.alertCallback != nil {
		go rm.alertCallback(alert)
	}

	rm.logger.Warn().
		Str("alert_id", alertID).
		Str("level", level).
		Float64("current_value", currentValue).
		Float64("threshold_value", thresholdValue).
		Msg("Alert triggered")
}

func (rm *ResourceMonitor) clearAlert(name string) {
	for alertID := range rm.activeAlerts {
		if fmt.Sprintf("%s_WARNING", name) == alertID || fmt.Sprintf("%s_CRITICAL", name) == alertID {
			delete(rm.activeAlerts, alertID)
			rm.logger.Info().
				Str("alert_id", alertID).
				Msg("Alert cleared")
		}
	}
}

func (rm *ResourceMonitor) evaluateCleanup() {
	rm.mutex.RLock()
	rules := make([]CleanupRule, len(rm.cleanupRules))
	copy(rules, rm.cleanupRules)
	rm.mutex.RUnlock()

	// Sort rules by priority (higher first)
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[j].Priority > rules[i].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Check cooldown
		if time.Since(rule.LastExecuted) < rule.Cooldown {
			continue
		}

		// Check execution limit
		if rule.MaxExecutions > 0 && rule.ExecutionCount >= rule.MaxExecutions {
			continue
		}

		// Evaluate conditions
		if rm.evaluateCleanupConditions(rule.Conditions) {
			rm.executeCleanupRule(rule)
		}
	}
}

func (rm *ResourceMonitor) evaluateCleanupConditions(conditions []CleanupCondition) bool {
	for _, condition := range conditions {
		if !rm.evaluateCleanupCondition(condition) {
			return false // All conditions must be true
		}
	}
	return len(conditions) > 0 // At least one condition must exist
}

func (rm *ResourceMonitor) evaluateCleanupCondition(condition CleanupCondition) bool {
	currentValue := rm.getCurrentValueForCondition(condition)
	threshold, ok := condition.Threshold.(float64)
	if !ok {
		return false
	}

	switch condition.Operator {
	case "gt":
		return currentValue > threshold
	case "gte":
		return currentValue >= threshold
	case "lt":
		return currentValue < threshold
	case "lte":
		return currentValue <= threshold
	case "eq":
		return currentValue == threshold
	default:
		return false
	}
}

func (rm *ResourceMonitor) getCurrentValueForCondition(condition CleanupCondition) float64 {
	switch condition.Type {
	case "memory":
		return rm.currentResources.ResourcePressure
	case "disk":
		if rm.resourceLimits.MaxDiskUsage > 0 {
			return float64(rm.currentResources.TotalDisk) / float64(rm.resourceLimits.MaxDiskUsage) * 100
		}
	case "count":
		return float64(rm.currentResources.ActiveSessions)
	case "age":
		return time.Since(rm.currentResources.OldestSession).Hours()
	case "idle":
		// Calculate average idle time
		return 0.0 // Would implement
	}
	return 0.0
}

func (rm *ResourceMonitor) executeCleanupRule(rule CleanupRule) {
	event := CleanupEvent{
		ID:               fmt.Sprintf("cleanup_%d", time.Now().Unix()),
		Timestamp:        time.Now(),
		RuleName:         rule.Name,
		Trigger:          "automatic",
		ActionsExecuted:  make([]string, 0),
		SessionsAffected: make([]string, 0),
		Success:          true,
	}

	startTime := time.Now()

	rm.logger.Info().
		Str("rule_name", rule.Name).
		Str("cleanup_id", event.ID).
		Msg("Executing cleanup rule")

	// Execute actions
	for _, action := range rule.Actions {
		if err := rm.executeCleanupAction(action, &event); err != nil {
			event.Success = false
			event.ErrorMessage = err.Error()
			rm.logger.Error().
				Err(err).
				Str("action_type", action.Type).
				Str("cleanup_id", event.ID).
				Msg("Cleanup action failed")
			break
		}
		event.ActionsExecuted = append(event.ActionsExecuted, action.Type)
	}

	event.Duration = time.Since(startTime)

	// Update rule execution tracking
	rm.mutex.Lock()
	for i := range rm.cleanupRules {
		if rm.cleanupRules[i].Name == rule.Name {
			rm.cleanupRules[i].ExecutionCount++
			rm.cleanupRules[i].LastExecuted = time.Now()
			break
		}
	}

	// Store cleanup event
	rm.cleanupHistory = append(rm.cleanupHistory, event)
	if len(rm.cleanupHistory) > rm.maxCleanupHistory {
		rm.cleanupHistory = rm.cleanupHistory[1:]
	}
	rm.mutex.Unlock()

	rm.logger.Info().
		Str("cleanup_id", event.ID).
		Bool("success", event.Success).
		Dur("duration", event.Duration).
		Int("sessions_affected", len(event.SessionsAffected)).
		Msg("Cleanup rule execution completed")
}

func (rm *ResourceMonitor) executeCleanupAction(action CleanupAction, event *CleanupEvent) error {
	switch action.Type {
	case "terminate":
		return rm.executeTerminateAction(action, event)
	case "archive":
		return rm.executeArchiveAction(action, event)
	case "alert":
		return rm.executeAlertAction(action, event)
	default:
		return fmt.Errorf("unknown cleanup action type: %s", action.Type)
	}
}

func (rm *ResourceMonitor) executeTerminateAction(action CleanupAction, event *CleanupEvent) error {
	// Find sessions to terminate based on criteria
	sessions := rm.getSessionDataList()
	candidateSessions := rm.findCleanupCandidates(sessions, action.Parameters)

	terminatedCount := 0
	memoryFreed := int64(0)
	diskFreed := int64(0)

	for _, session := range candidateSessions {
		if err := rm.terminateSession(session.ID); err != nil {
			rm.logger.Error().
				Err(err).
				Str("session_id", session.ID).
				Msg("Failed to terminate session during cleanup")
			continue
		}

		event.SessionsAffected = append(event.SessionsAffected, session.ID)
		terminatedCount++
		diskFreed += session.DiskUsage

		rm.logger.Debug().
			Str("session_id", session.ID).
			Int64("disk_freed", session.DiskUsage).
			Msg("Terminated session during cleanup")
	}

	event.ResourcesFreed.Sessions = terminatedCount
	event.ResourcesFreed.Memory = memoryFreed
	event.ResourcesFreed.Disk = diskFreed
	event.ResourcesFreed.ImpactScore = float64(terminatedCount * 10) // Simplified scoring

	return nil
}

func (rm *ResourceMonitor) executeArchiveAction(action CleanupAction, event *CleanupEvent) error {
	// Implementation would archive old session data
	rm.logger.Info().Msg("Archive action executed (placeholder)")
	return nil
}

func (rm *ResourceMonitor) executeAlertAction(action CleanupAction, event *CleanupEvent) error {
	// Implementation would send alerts/notifications
	rm.logger.Info().Msg("Alert action executed (placeholder)")
	return nil
}

func (rm *ResourceMonitor) findCleanupCandidates(sessions []SessionData, parameters map[string]interface{}) []SessionData {
	candidates := make([]SessionData, 0)

	maxAge, hasMaxAge := parameters["max_age_hours"].(float64)
	maxIdle, hasMaxIdle := parameters["max_idle_hours"].(float64)
	maxCount, hasMaxCount := parameters["max_count"].(float64)

	for _, session := range sessions {
		ageHours := time.Since(session.CreatedAt).Hours()
		idleHours := time.Since(session.UpdatedAt).Hours()

		// Check age criteria
		if hasMaxAge && ageHours > maxAge {
			candidates = append(candidates, session)
			continue
		}

		// Check idle criteria
		if hasMaxIdle && idleHours > maxIdle {
			candidates = append(candidates, session)
			continue
		}
	}

	// Limit by count if specified
	if hasMaxCount && len(candidates) > int(maxCount) {
		candidates = candidates[:int(maxCount)]
	}

	return candidates
}

// Helper methods for session management

func (rm *ResourceMonitor) getSessionDataList() []SessionData {
	// Get all sessions from the session manager
	allSessions, _ := rm.sessionManager.GetAllSessions()
	sessionDataList := make([]SessionData, 0, len(allSessions))

	for _, session := range allSessions {
		sessionDataList = append(sessionDataList, *session)
	}

	return sessionDataList
}

func (rm *ResourceMonitor) terminateSession(sessionID string) error {
	// Use the session manager's cleanup method
	ctx := context.WithValue(context.Background(), "cleanup", true)
	_, err := rm.sessionManager.ListSessions(ctx, map[string]interface{}{
		"session_id": sessionID,
		"action":     "terminate",
	})

	if err != nil {
		return fmt.Errorf("failed to terminate session %s: %w", sessionID, err)
	}

	return nil
}

func (rm *ResourceMonitor) initializeDefaultCleanupRules() {
	// Rule 1: Clean up very old sessions
	rm.cleanupRules = append(rm.cleanupRules, CleanupRule{
		Name:        "cleanup_old_sessions",
		Description: "Remove sessions older than 24 hours",
		Enabled:     true,
		Priority:    10,
		Conditions: []CleanupCondition{
			{
				Type:        "age",
				Operator:    "gt",
				Threshold:   24.0, // hours
				Scope:       "session",
				Description: "Session age > 24 hours",
			},
		},
		Actions: []CleanupAction{
			{
				Type:        "terminate",
				Target:      "old_sessions",
				Parameters:  map[string]interface{}{"max_age_hours": 24.0},
				Description: "Terminate sessions older than 24 hours",
			},
		},
		Cooldown: 1 * time.Hour,
	})

	// Rule 2: Clean up idle sessions when memory pressure is high
	rm.cleanupRules = append(rm.cleanupRules, CleanupRule{
		Name:        "cleanup_idle_high_memory",
		Description: "Remove idle sessions when memory usage > 80%",
		Enabled:     true,
		Priority:    8,
		Conditions: []CleanupCondition{
			{
				Type:        "memory",
				Operator:    "gt",
				Threshold:   80.0, // percent
				Scope:       "system",
				Description: "Memory usage > 80%",
			},
		},
		Actions: []CleanupAction{
			{
				Type:        "terminate",
				Target:      "idle_sessions",
				Parameters:  map[string]interface{}{"max_idle_hours": 2.0, "max_count": 5.0},
				Description: "Terminate up to 5 sessions idle > 2 hours",
			},
		},
		Cooldown: 30 * time.Minute,
	})

	// Rule 3: Emergency cleanup when critical resource pressure
	rm.cleanupRules = append(rm.cleanupRules, CleanupRule{
		Name:        "emergency_cleanup",
		Description: "Emergency cleanup when resource pressure > 95%",
		Enabled:     true,
		Priority:    15,
		Conditions: []CleanupCondition{
			{
				Type:        "memory",
				Operator:    "gt",
				Threshold:   95.0, // percent
				Scope:       "system",
				Description: "Memory usage > 95%",
			},
		},
		Actions: []CleanupAction{
			{
				Type:        "terminate",
				Target:      "oldest_sessions",
				Parameters:  map[string]interface{}{"max_count": 10.0},
				Description: "Terminate up to 10 oldest sessions",
			},
		},
		Cooldown: 5 * time.Minute,
	})
}

func (rm *ResourceMonitor) initializeDefaultAlertThresholds() {
	rm.alertThresholds["memory_pressure"] = AlertThreshold{
		Name:              "memory_pressure",
		ResourceType:      "memory",
		WarningThreshold:  70.0,
		CriticalThreshold: 90.0,
		Duration:          2 * time.Minute,
		Enabled:           true,
	}

	rm.alertThresholds["disk_pressure"] = AlertThreshold{
		Name:              "disk_pressure",
		ResourceType:      "disk",
		WarningThreshold:  75.0,
		CriticalThreshold: 90.0,
		Duration:          5 * time.Minute,
		Enabled:           true,
	}

	rm.alertThresholds["session_count"] = AlertThreshold{
		Name:              "session_count",
		ResourceType:      "sessions",
		WarningThreshold:  80.0,
		CriticalThreshold: 95.0,
		Duration:          1 * time.Minute,
		Enabled:           true,
	}
}
