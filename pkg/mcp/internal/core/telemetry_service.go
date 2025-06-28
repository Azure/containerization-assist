package core

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type TelemetryService struct {
	logger     zerolog.Logger
	collectors []MetricsCollector
	events     chan Event
	stopCh     chan struct{}
	mu         sync.RWMutex
	metrics    *SystemMetrics
}

func NewTelemetryService(logger zerolog.Logger) *TelemetryService {
	service := &TelemetryService{
		logger:     logger.With().Str("service", "telemetry").Logger(),
		collectors: make([]MetricsCollector, 0),
		events:     make(chan Event, 1000),
		stopCh:     make(chan struct{}),
		metrics:    NewSystemMetrics(),
	}

	go service.processEvents()

	return service
}

func (s *TelemetryService) RegisterCollector(collector MetricsCollector) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.collectors = append(s.collectors, collector)
	s.logger.Debug().Str("collector", collector.GetName()).Msg("Metrics collector registered")
}

func (s *TelemetryService) TrackToolExecution(ctx context.Context, execution ToolExecution) {
	s.metrics.RecordToolExecution(execution)

	event := Event{
		Type:      EventTypeToolExecution,
		Timestamp: time.Now(),
		Data:      execution,
	}

	select {
	case s.events <- event:
	default:
		s.logger.Warn().Msg("Event queue full, dropping event")
	}
}

func (s *TelemetryService) TrackPerformance(ctx context.Context, metric PerformanceMetric) {
	s.metrics.RecordPerformance(metric)

	event := Event{
		Type:      EventTypePerformance,
		Timestamp: time.Now(),
		Data:      metric,
	}

	select {
	case s.events <- event:
	default:
		s.logger.Warn().Msg("Event queue full, dropping performance metric")
	}
}

func (s *TelemetryService) TrackEvent(ctx context.Context, eventType string, data interface{}) {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	select {
	case s.events <- event:
	default:
		s.logger.Warn().Msg("Event queue full, dropping custom event")
	}
}

func (s *TelemetryService) GetMetrics() *SystemMetrics {
	return s.metrics
}

func (s *TelemetryService) CreatePerformanceTracker(tool, operation string) *PerformanceTracker {
	return NewPerformanceTracker(tool, operation, s)
}

func (s *TelemetryService) Shutdown(ctx context.Context) error {
	close(s.stopCh)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return nil
	}
}

func (s *TelemetryService) processEvents() {
	for {
		select {
		case event := <-s.events:
			s.processEvent(event)
		case <-s.stopCh:
			for len(s.events) > 0 {
				event := <-s.events
				s.processEvent(event)
			}
			return
		}
	}
}

func (s *TelemetryService) processEvent(event Event) {
	s.mu.RLock()
	collectors := make([]MetricsCollector, len(s.collectors))
	copy(collectors, s.collectors)
	s.mu.RUnlock()

	for _, collector := range collectors {
		if err := collector.Collect(event); err != nil {
			s.logger.Error().Err(err).Str("collector", collector.GetName()).Msg("Failed to collect event")
		}
	}
}

type Event struct {
	Type      string
	Timestamp time.Time
	Data      interface{}
}

const (
	EventTypeToolExecution = "tool_execution"
	EventTypePerformance   = "performance"
	EventTypeError         = "error"
	EventTypeCustom        = "custom"
)

type ToolExecution struct {
	Tool      string
	Operation string
	SessionID string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Success   bool
	DryRun    bool
	Metadata  map[string]interface{}
}

type PerformanceMetric struct {
	Tool      string
	Operation string
	Metric    string
	Value     float64
	Unit      string
	Timestamp time.Time
	Tags      map[string]string
}

type MetricsCollector interface {
	GetName() string
	Collect(event Event) error
}

type SystemMetrics struct {
	ToolExecutions map[string]*ToolMetrics
	Performance    map[string]*PerformanceStats
	mu             sync.RWMutex
}

func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		ToolExecutions: make(map[string]*ToolMetrics),
		Performance:    make(map[string]*PerformanceStats),
	}
}

func (m *SystemMetrics) RecordToolExecution(execution ToolExecution) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := execution.Tool
	if metrics, exists := m.ToolExecutions[key]; exists {
		metrics.Update(execution)
	} else {
		m.ToolExecutions[key] = NewToolMetrics(execution)
	}
}

func (m *SystemMetrics) RecordPerformance(metric PerformanceMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := metric.Tool + "." + metric.Metric
	if stats, exists := m.Performance[key]; exists {
		stats.Update(metric.Value)
	} else {
		m.Performance[key] = NewPerformanceStats(metric.Value)
	}
}

func (m *SystemMetrics) GetToolMetrics(tool string) *ToolMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.ToolExecutions[tool]; exists {
		return metrics.Copy()
	}
	return nil
}

type ToolMetrics struct {
	Tool            string
	TotalExecs      int64
	SuccessfulExecs int64
	FailedExecs     int64
	TotalDuration   time.Duration
	AvgDuration     time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	LastExecution   time.Time
}

func NewToolMetrics(execution ToolExecution) *ToolMetrics {
	metrics := &ToolMetrics{
		Tool:          execution.Tool,
		TotalExecs:    1,
		TotalDuration: execution.Duration,
		AvgDuration:   execution.Duration,
		MinDuration:   execution.Duration,
		MaxDuration:   execution.Duration,
		LastExecution: execution.EndTime,
	}

	if execution.Success {
		metrics.SuccessfulExecs = 1
	} else {
		metrics.FailedExecs = 1
	}

	return metrics
}

func (m *ToolMetrics) Update(execution ToolExecution) {
	m.TotalExecs++
	m.TotalDuration += execution.Duration
	m.AvgDuration = m.TotalDuration / time.Duration(m.TotalExecs)

	if execution.Duration < m.MinDuration {
		m.MinDuration = execution.Duration
	}
	if execution.Duration > m.MaxDuration {
		m.MaxDuration = execution.Duration
	}

	if execution.EndTime.After(m.LastExecution) {
		m.LastExecution = execution.EndTime
	}

	if execution.Success {
		m.SuccessfulExecs++
	} else {
		m.FailedExecs++
	}
}

func (m *ToolMetrics) Copy() *ToolMetrics {
	return &ToolMetrics{
		Tool:            m.Tool,
		TotalExecs:      m.TotalExecs,
		SuccessfulExecs: m.SuccessfulExecs,
		FailedExecs:     m.FailedExecs,
		TotalDuration:   m.TotalDuration,
		AvgDuration:     m.AvgDuration,
		MinDuration:     m.MinDuration,
		MaxDuration:     m.MaxDuration,
		LastExecution:   m.LastExecution,
	}
}

type PerformanceStats struct {
	Count   int64
	Sum     float64
	Min     float64
	Max     float64
	Average float64
}

func NewPerformanceStats(initialValue float64) *PerformanceStats {
	return &PerformanceStats{
		Count:   1,
		Sum:     initialValue,
		Min:     initialValue,
		Max:     initialValue,
		Average: initialValue,
	}
}

func (s *PerformanceStats) Update(value float64) {
	s.Count++
	s.Sum += value
	s.Average = s.Sum / float64(s.Count)

	if value < s.Min {
		s.Min = value
	}
	if value > s.Max {
		s.Max = value
	}
}

type PerformanceTracker struct {
	tool         string
	operation    string
	startTime    time.Time
	service      *TelemetryService
	measurements map[string]float64
}

func NewPerformanceTracker(tool, operation string, service *TelemetryService) *PerformanceTracker {
	return &PerformanceTracker{
		tool:         tool,
		operation:    operation,
		startTime:    time.Now(),
		service:      service,
		measurements: make(map[string]float64),
	}
}

func (t *PerformanceTracker) Start() {
	t.startTime = time.Now()
}

func (t *PerformanceTracker) Record(metric string, value float64, unit string) {
	t.measurements[metric] = value

	perfMetric := PerformanceMetric{
		Tool:      t.tool,
		Operation: t.operation,
		Metric:    metric,
		Value:     value,
		Unit:      unit,
		Timestamp: time.Now(),
	}

	t.service.TrackPerformance(context.Background(), perfMetric)
}

func (t *PerformanceTracker) Finish() time.Duration {
	duration := time.Since(t.startTime)

	t.Record("duration", float64(duration.Milliseconds()), "ms")

	return duration
}

type LoggingCollector struct {
	logger zerolog.Logger
}

func NewLoggingCollector(logger zerolog.Logger) *LoggingCollector {
	return &LoggingCollector{
		logger: logger.With().Str("collector", "logging").Logger(),
	}
}

func (c *LoggingCollector) GetName() string {
	return "logging"
}

func (c *LoggingCollector) Collect(event Event) error {
	switch event.Type {
	case EventTypeToolExecution:
		if exec, ok := event.Data.(ToolExecution); ok {
			c.logger.Info().
				Str("tool", exec.Tool).
				Str("operation", exec.Operation).
				Str("session", exec.SessionID).
				Dur("duration", exec.Duration).
				Bool("success", exec.Success).
				Bool("dry_run", exec.DryRun).
				Msg("Tool execution completed")
		}
	case EventTypePerformance:
		if perf, ok := event.Data.(PerformanceMetric); ok {
			c.logger.Debug().
				Str("tool", perf.Tool).
				Str("operation", perf.Operation).
				Str("metric", perf.Metric).
				Float64("value", perf.Value).
				Str("unit", perf.Unit).
				Msg("Performance metric recorded")
		}
	default:
		c.logger.Debug().
			Str("type", event.Type).
			Interface("data", event.Data).
			Msg("Custom event recorded")
	}

	return nil
}

type MetricsCollectorChain struct {
	collectors []MetricsCollector
}

func NewMetricsCollectorChain(collectors ...MetricsCollector) *MetricsCollectorChain {
	return &MetricsCollectorChain{
		collectors: collectors,
	}
}

func (c *MetricsCollectorChain) GetName() string {
	return "chain"
}

func (c *MetricsCollectorChain) Collect(event Event) error {
	for _, collector := range c.collectors {
		if err := collector.Collect(event); err != nil {
			continue
		}
	}
	return nil
}
