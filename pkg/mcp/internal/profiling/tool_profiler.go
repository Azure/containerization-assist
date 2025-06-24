package profiling

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ToolProfiler provides comprehensive performance profiling for tool execution
type ToolProfiler struct {
	logger   zerolog.Logger
	metrics  *MetricsCollector
	enabled  bool
	mu       sync.RWMutex
	sessions map[string]*ExecutionSession
}

// ExecutionSession tracks performance metrics for a single tool execution
type ExecutionSession struct {
	ToolName      string
	SessionID     string
	StartTime     time.Time
	EndTime       time.Time
	DispatchTime  time.Duration
	ExecutionTime time.Duration
	TotalTime     time.Duration

	// Resource metrics
	StartMemory    MemoryStats
	EndMemory      MemoryStats
	MemoryDelta    MemoryStats
	GoroutineCount int

	// Execution context
	Success   bool
	ErrorType string
	Stage     string
	Metadata  map[string]interface{}
}

// MemoryStats captures memory usage metrics
type MemoryStats struct {
	Alloc         uint64  // bytes allocated and not yet freed
	TotalAlloc    uint64  // bytes allocated (even if freed)
	Sys           uint64  // bytes obtained from system (sum of XxxSys below)
	Mallocs       uint64  // number of malloc calls
	Frees         uint64  // number of free calls
	HeapAlloc     uint64  // bytes allocated and not yet freed (same as Alloc above)
	HeapSys       uint64  // bytes obtained from system
	HeapIdle      uint64  // bytes in idle spans
	HeapInuse     uint64  // bytes in non-idle span
	GCCPUFraction float64 // fraction of CPU time used by GC
}

// ProfiledExecution represents the result of a profiled tool execution
type ProfiledExecution struct {
	Session *ExecutionSession
	Result  interface{}
	Error   error
}

// NewToolProfiler creates a new tool profiler instance
func NewToolProfiler(logger zerolog.Logger, enabled bool) *ToolProfiler {
	return &ToolProfiler{
		logger:   logger.With().Str("component", "tool_profiler").Logger(),
		metrics:  NewMetricsCollector(),
		enabled:  enabled,
		sessions: make(map[string]*ExecutionSession),
	}
}

// StartExecution begins profiling a tool execution
func (p *ToolProfiler) StartExecution(toolName, sessionID string) *ExecutionSession {
	if !p.enabled {
		return nil
	}

	session := &ExecutionSession{
		ToolName:       toolName,
		SessionID:      sessionID,
		StartTime:      time.Now(),
		StartMemory:    p.captureMemoryStats(),
		GoroutineCount: runtime.NumGoroutine(),
		Metadata:       make(map[string]interface{}),
	}

	sessionKey := p.sessionKey(toolName, sessionID)
	p.mu.Lock()
	p.sessions[sessionKey] = session
	p.mu.Unlock()

	p.logger.Debug().
		Str("tool", toolName).
		Str("session_id", sessionID).
		Time("start_time", session.StartTime).
		Uint64("start_memory", session.StartMemory.HeapAlloc).
		Int("goroutines", session.GoroutineCount).
		Msg("Started execution profiling")

	return session
}

// RecordDispatchComplete marks the end of tool dispatch phase
func (p *ToolProfiler) RecordDispatchComplete(toolName, sessionID string) {
	if !p.enabled {
		return
	}

	sessionKey := p.sessionKey(toolName, sessionID)
	p.mu.Lock()
	session, exists := p.sessions[sessionKey]
	p.mu.Unlock()

	if !exists {
		p.logger.Warn().
			Str("tool", toolName).
			Str("session_id", sessionID).
			Msg("Dispatch complete recorded for unknown session")
		return
	}

	session.DispatchTime = time.Since(session.StartTime)

	p.logger.Debug().
		Str("tool", toolName).
		Str("session_id", sessionID).
		Dur("dispatch_time", session.DispatchTime).
		Msg("Tool dispatch completed")
}

// EndExecution completes profiling and returns execution metrics
func (p *ToolProfiler) EndExecution(toolName, sessionID string, success bool, errorType string) *ExecutionSession {
	if !p.enabled {
		return nil
	}

	sessionKey := p.sessionKey(toolName, sessionID)
	p.mu.Lock()
	session, exists := p.sessions[sessionKey]
	if exists {
		delete(p.sessions, sessionKey)
	}
	p.mu.Unlock()

	if !exists {
		p.logger.Warn().
			Str("tool", toolName).
			Str("session_id", sessionID).
			Msg("End execution called for unknown session")
		return nil
	}

	// Complete session metrics
	session.EndTime = time.Now()
	session.TotalTime = session.EndTime.Sub(session.StartTime)
	session.ExecutionTime = session.TotalTime - session.DispatchTime
	session.EndMemory = p.captureMemoryStats()
	session.MemoryDelta = p.calculateMemoryDelta(session.StartMemory, session.EndMemory)
	session.Success = success
	session.ErrorType = errorType

	// Record metrics
	p.metrics.RecordExecution(session)

	p.logger.Info().
		Str("tool", toolName).
		Str("session_id", sessionID).
		Dur("total_time", session.TotalTime).
		Dur("dispatch_time", session.DispatchTime).
		Dur("execution_time", session.ExecutionTime).
		Uint64("memory_delta", session.MemoryDelta.HeapAlloc).
		Bool("success", success).
		Msg("Execution profiling completed")

	return session
}

// ProfileToolExecution wraps a tool execution with comprehensive profiling
func (p *ToolProfiler) ProfileToolExecution(
	ctx context.Context,
	toolName, sessionID string,
	execution func(context.Context) (interface{}, error),
) *ProfiledExecution {
	// Start profiling
	p.StartExecution(toolName, sessionID)

	// Record dispatch complete (assuming immediate execution)
	p.RecordDispatchComplete(toolName, sessionID)

	// Execute the tool
	result, err := execution(ctx)

	// End profiling
	success := err == nil
	errorType := ""
	if err != nil {
		errorType = "execution_error"
	}

	finalSession := p.EndExecution(toolName, sessionID, success, errorType)

	return &ProfiledExecution{
		Session: finalSession,
		Result:  result,
		Error:   err,
	}
}

// SetMetadata adds metadata to an active execution session
func (p *ToolProfiler) SetMetadata(toolName, sessionID, key string, value interface{}) {
	if !p.enabled {
		return
	}

	sessionKey := p.sessionKey(toolName, sessionID)
	p.mu.Lock()
	defer p.mu.Unlock()

	if session, exists := p.sessions[sessionKey]; exists {
		session.Metadata[key] = value
	}
}

// SetStage updates the current execution stage
func (p *ToolProfiler) SetStage(toolName, sessionID, stage string) {
	if !p.enabled {
		return
	}

	sessionKey := p.sessionKey(toolName, sessionID)
	p.mu.Lock()
	defer p.mu.Unlock()

	if session, exists := p.sessions[sessionKey]; exists {
		session.Stage = stage
		p.logger.Debug().
			Str("tool", toolName).
			Str("session_id", sessionID).
			Str("stage", stage).
			Msg("Execution stage updated")
	}
}

// GetMetrics returns the current metrics collector
func (p *ToolProfiler) GetMetrics() *MetricsCollector {
	return p.metrics
}

// IsEnabled returns whether profiling is currently enabled
func (p *ToolProfiler) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

// Enable enables or disables profiling
func (p *ToolProfiler) Enable(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = enabled

	p.logger.Info().
		Bool("enabled", enabled).
		Msg("Tool profiling state changed")
}

// captureMemoryStats captures current memory statistics
func (p *ToolProfiler) captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		GCCPUFraction: m.GCCPUFraction,
	}
}

// calculateMemoryDelta computes the difference between memory stats
func (p *ToolProfiler) calculateMemoryDelta(start, end MemoryStats) MemoryStats {
	return MemoryStats{
		Alloc:      end.Alloc - start.Alloc,
		TotalAlloc: end.TotalAlloc - start.TotalAlloc,
		Mallocs:    end.Mallocs - start.Mallocs,
		Frees:      end.Frees - start.Frees,
		HeapAlloc:  end.HeapAlloc - start.HeapAlloc,
	}
}

// sessionKey creates a unique key for tracking execution sessions
func (p *ToolProfiler) sessionKey(toolName, sessionID string) string {
	return toolName + ":" + sessionID
}
