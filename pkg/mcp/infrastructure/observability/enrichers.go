// Package observability provides log enrichers for the unified logging system
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// SystemEnricher enriches logs with system information
type SystemEnricher struct {
	includeGoroutines bool
	includeMemory     bool
	includeGC         bool
}

// NewSystemEnricher creates a new system enricher
func NewSystemEnricher(includeGoroutines, includeMemory, includeGC bool) *SystemEnricher {
	return &SystemEnricher{
		includeGoroutines: includeGoroutines,
		includeMemory:     includeMemory,
		includeGC:         includeGC,
	}
}

// Enrich adds system information to log records
func (se *SystemEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	if se.includeGoroutines {
		record.Properties["system_goroutines"] = runtime.NumGoroutine()
		record.GoroutinesDelta = runtime.NumGoroutine()
	}

	if se.includeMemory {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		record.Properties["system_memory_alloc"] = memStats.Alloc
		record.Properties["system_memory_sys"] = memStats.Sys
		record.Properties["system_memory_heap_alloc"] = memStats.HeapAlloc
		record.MemoryDelta = int64(memStats.Alloc)
	}

	if se.includeGC {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		record.Properties["system_gc_count"] = memStats.NumGC
		record.Properties["system_gc_pause_total"] = memStats.PauseTotalNs

		if memStats.NumGC > 0 {
			record.Properties["system_gc_pause_recent"] = memStats.PauseNs[(memStats.NumGC+255)%256]
		}
	}

	return nil
}

// Name returns the enricher name
func (se *SystemEnricher) Name() string {
	return "system"
}

// Priority returns the enricher priority (higher runs first)
func (se *SystemEnricher) Priority() int {
	return 100
}

// PerformanceEnricher enriches logs with performance information
type PerformanceEnricher struct {
	trackCaller     bool
	trackStackTrace bool
	maxStackDepth   int
}

// NewPerformanceEnricher creates a new performance enricher
func NewPerformanceEnricher(trackCaller, trackStackTrace bool, maxStackDepth int) *PerformanceEnricher {
	return &PerformanceEnricher{
		trackCaller:     trackCaller,
		trackStackTrace: trackStackTrace,
		maxStackDepth:   maxStackDepth,
	}
}

// Enrich adds performance information to log records
func (pe *PerformanceEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	if pe.trackCaller {
		// Get caller information
		if pc, file, line, ok := runtime.Caller(5); ok { // Skip enricher stack frames
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				record.Source = fmt.Sprintf("%s:%d", shortenFilePath(file), line)
				record.Properties["caller_function"] = fn.Name()
				record.Properties["caller_file"] = shortenFilePath(file)
				record.Properties["caller_line"] = line
			}
		}
	}

	if pe.trackStackTrace && record.Level >= LevelError {
		stack := debug.Stack()
		stackLines := strings.Split(string(stack), "\n")

		// Limit stack depth
		if pe.maxStackDepth > 0 && len(stackLines) > pe.maxStackDepth*2 {
			stackLines = stackLines[:pe.maxStackDepth*2]
		}

		// Skip the first few lines (goroutine info and panic handler)
		if len(stackLines) > 4 {
			stackLines = stackLines[4:]
		}

		record.StackTrace = strings.Join(stackLines, "\n")
		record.Properties["stack_trace_lines"] = len(stackLines)
	}

	return nil
}

// Name returns the enricher name
func (pe *PerformanceEnricher) Name() string {
	return "performance"
}

// Priority returns the enricher priority
func (pe *PerformanceEnricher) Priority() int {
	return 90
}

// TimestampEnricher enriches logs with additional timestamp information
type TimestampEnricher struct {
	includeUnix     bool
	includeISO      bool
	includeRelative bool
	startTime       time.Time
}

// NewTimestampEnricher creates a new timestamp enricher
func NewTimestampEnricher(includeUnix, includeISO, includeRelative bool) *TimestampEnricher {
	return &TimestampEnricher{
		includeUnix:     includeUnix,
		includeISO:      includeISO,
		includeRelative: includeRelative,
		startTime:       time.Now(),
	}
}

// Enrich adds timestamp information to log records
func (te *TimestampEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	if te.includeUnix {
		record.Properties["timestamp_unix"] = record.Time.Unix()
		record.Properties["timestamp_unix_nano"] = record.Time.UnixNano()
	}

	if te.includeISO {
		record.Properties["timestamp_iso"] = record.Time.Format(time.RFC3339Nano)
	}

	if te.includeRelative {
		elapsed := record.Time.Sub(te.startTime)
		record.Properties["timestamp_relative_ms"] = elapsed.Milliseconds()
		record.Properties["timestamp_relative"] = elapsed.String()
	}

	return nil
}

// Name returns the enricher name
func (te *TimestampEnricher) Name() string {
	return "timestamp"
}

// Priority returns the enricher priority
func (te *TimestampEnricher) Priority() int {
	return 80
}

// ContextEnricher enriches logs with context values
type ContextEnricher struct {
	contextKeys []string
}

// NewContextEnricher creates a new context enricher
func NewContextEnricher(contextKeys []string) *ContextEnricher {
	return &ContextEnricher{
		contextKeys: contextKeys,
	}
}

// Enrich adds context values to log records
func (ce *ContextEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	for _, key := range ce.contextKeys {
		if value := ctx.Value(key); value != nil {
			record.Properties[fmt.Sprintf("ctx_%s", key)] = value
		}
	}

	// Extract common context values
	if userID := ctx.Value("user_id"); userID != nil {
		record.Properties["user_id"] = userID
		record.Tags["user_id"] = fmt.Sprintf("%v", userID)
	}

	if requestID := ctx.Value("request_id"); requestID != nil {
		record.Properties["request_id"] = requestID
		record.Tags["request_id"] = fmt.Sprintf("%v", requestID)
	}

	if sessionID := ctx.Value("session_id"); sessionID != nil {
		record.SessionID = fmt.Sprintf("%v", sessionID)
		record.Properties["session_id"] = sessionID
	}

	if workflowID := ctx.Value("workflow_id"); workflowID != nil {
		record.WorkflowID = fmt.Sprintf("%v", workflowID)
		record.Properties["workflow_id"] = workflowID
	}

	return nil
}

// Name returns the enricher name
func (ce *ContextEnricher) Name() string {
	return "context"
}

// Priority returns the enricher priority
func (ce *ContextEnricher) Priority() int {
	return 70
}

// SecurityEnricher enriches logs with security-related information
type SecurityEnricher struct {
	enableIPTracking   bool
	enableUserTracking bool
	enableAuthTracking bool
	sensitivePatterns  []string
}

// NewSecurityEnricher creates a new security enricher
func NewSecurityEnricher(enableIPTracking, enableUserTracking, enableAuthTracking bool, sensitivePatterns []string) *SecurityEnricher {
	return &SecurityEnricher{
		enableIPTracking:   enableIPTracking,
		enableUserTracking: enableUserTracking,
		enableAuthTracking: enableAuthTracking,
		sensitivePatterns:  sensitivePatterns,
	}
}

// Enrich adds security information to log records
func (se *SecurityEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	// Check for sensitive data and redact if necessary
	se.redactSensitiveData(record)

	// Add security tags
	if se.enableIPTracking {
		if clientIP := ctx.Value("client_ip"); clientIP != nil {
			record.Properties["client_ip"] = clientIP
			record.Tags["client_ip"] = fmt.Sprintf("%v", clientIP)
		}
	}

	if se.enableUserTracking {
		if userID := ctx.Value("user_id"); userID != nil {
			record.Properties["user_id"] = userID
			record.Tags["user_id"] = fmt.Sprintf("%v", userID)
		}

		if username := ctx.Value("username"); username != nil {
			record.Properties["username"] = username
		}
	}

	if se.enableAuthTracking {
		if authMethod := ctx.Value("auth_method"); authMethod != nil {
			record.Properties["auth_method"] = authMethod
			record.Tags["auth_method"] = fmt.Sprintf("%v", authMethod)
		}

		if roles := ctx.Value("user_roles"); roles != nil {
			record.Properties["user_roles"] = roles
		}
	}

	// Add security classification
	record.Tags["security_level"] = se.classifySecurityLevel(record)

	return nil
}

// Name returns the enricher name
func (se *SecurityEnricher) Name() string {
	return "security"
}

// Priority returns the enricher priority
func (se *SecurityEnricher) Priority() int {
	return 60
}

// redactSensitiveData redacts sensitive information from log records
func (se *SecurityEnricher) redactSensitiveData(record *LogRecord) {
	// Check message for sensitive patterns
	for _, pattern := range se.sensitivePatterns {
		if strings.Contains(strings.ToLower(record.Message), strings.ToLower(pattern)) {
			record.Tags["contains_sensitive_data"] = "true"
			// Could implement actual redaction logic here
			break
		}
	}

	// Redact sensitive properties
	sensitiveKeys := []string{"password", "token", "secret", "key", "api_key", "auth_token"}
	for _, key := range sensitiveKeys {
		if _, exists := record.Properties[key]; exists {
			record.Properties[key] = "[REDACTED]"
		}
	}
}

// classifySecurityLevel determines the security level of a log record
func (se *SecurityEnricher) classifySecurityLevel(record *LogRecord) string {
	// High security events
	if strings.Contains(strings.ToLower(record.Message), "auth") ||
		strings.Contains(strings.ToLower(record.Message), "login") ||
		strings.Contains(strings.ToLower(record.Message), "permission") ||
		record.Level >= LevelError {
		return "high"
	}

	// Medium security events
	if strings.Contains(strings.ToLower(record.Message), "user") ||
		strings.Contains(strings.ToLower(record.Message), "access") {
		return "medium"
	}

	return "low"
}

// BusinessEnricher enriches logs with business context
type BusinessEnricher struct {
	enableMetrics     bool
	enableKPITracking bool
	businessEvents    map[string]string
}

// NewBusinessEnricher creates a new business enricher
func NewBusinessEnricher(enableMetrics, enableKPITracking bool, businessEvents map[string]string) *BusinessEnricher {
	if businessEvents == nil {
		businessEvents = make(map[string]string)
	}

	return &BusinessEnricher{
		enableMetrics:     enableMetrics,
		enableKPITracking: enableKPITracking,
		businessEvents:    businessEvents,
	}
}

// Enrich adds business context to log records
func (be *BusinessEnricher) Enrich(ctx context.Context, record *LogRecord) error {
	// Add business context from properties
	if customerID := record.Properties["customer_id"]; customerID != nil {
		record.Tags["customer_id"] = fmt.Sprintf("%v", customerID)
	}

	if tenantID := record.Properties["tenant_id"]; tenantID != nil {
		record.Tags["tenant_id"] = fmt.Sprintf("%v", tenantID)
	}

	if feature := record.Properties["feature"]; feature != nil {
		record.Tags["feature"] = fmt.Sprintf("%v", feature)
	}

	// Classify business events
	businessEventType := be.classifyBusinessEvent(record)
	if businessEventType != "" {
		record.Tags["business_event"] = businessEventType
		record.Properties["business_event_type"] = businessEventType
	}

	// Add KPI tracking
	if be.enableKPITracking {
		be.trackKPIs(record)
	}

	return nil
}

// Name returns the enricher name
func (be *BusinessEnricher) Name() string {
	return "business"
}

// Priority returns the enricher priority
func (be *BusinessEnricher) Priority() int {
	return 50
}

// classifyBusinessEvent classifies the business event type
func (be *BusinessEnricher) classifyBusinessEvent(record *LogRecord) string {
	message := strings.ToLower(record.Message)

	// Check predefined business events
	for pattern, eventType := range be.businessEvents {
		if strings.Contains(message, strings.ToLower(pattern)) {
			return eventType
		}
	}

	// Default classifications
	if strings.Contains(message, "deploy") || strings.Contains(message, "container") {
		return "deployment"
	}
	if strings.Contains(message, "workflow") || strings.Contains(message, "step") {
		return "workflow"
	}
	if strings.Contains(message, "user") || strings.Contains(message, "session") {
		return "user_activity"
	}
	if strings.Contains(message, "error") || strings.Contains(message, "fail") {
		return "error_event"
	}

	return ""
}

// trackKPIs adds KPI tracking information
func (be *BusinessEnricher) trackKPIs(record *LogRecord) {
	// Track deployment success rate
	if record.Tags["business_event"] == "deployment" {
		if record.Properties["success"] == true {
			record.Properties["kpi_deployment_success"] = 1
		} else {
			record.Properties["kpi_deployment_failure"] = 1
		}
	}

	// Track error rate
	if record.Level >= LevelError {
		record.Properties["kpi_error_count"] = 1
	}

	// Track response time for performance KPIs
	if duration, ok := record.Properties["duration_ms"]; ok {
		if durationInt, err := strconv.Atoi(fmt.Sprintf("%v", duration)); err == nil {
			record.Properties["kpi_response_time"] = durationInt
		}
	}
}

// Helper functions

func shortenFilePath(file string) string {
	// Keep only the last two path components
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return file
}

// LevelError is a convenience constant
const LevelError = slog.LevelError
