package appstate

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LoggingObserver logs all state changes
type LoggingObserver struct {
	logger *slog.Logger
}

// NewLoggingObserver creates a new logging observer
func NewLoggingObserver(logger *slog.Logger) StateObserver {
	return &LoggingObserver{
		logger: logger.With("component", "state_observer"),
	}
}

// OnStateChange logs state changes
func (o *LoggingObserver) OnStateChange(event *StateEvent) error {
	o.logger.Info("State changed",
		slog.String("event_id", event.ID),
		slog.String("event_type", string(event.Type)),
		slog.String("state_type", string(event.StateType)),
		slog.String("state_id", event.StateID),
		slog.Time("timestamp", event.Timestamp),
		slog.Any("metadata", event.Metadata))
	return nil
}

// GetID returns the observer ID
func (o *LoggingObserver) GetID() string {
	return "logging_observer"
}

// IsActive returns whether the observer is active
func (o *LoggingObserver) IsActive() bool {
	return true
}

// MetricsObserver collects state change metrics
type MetricsObserver struct {
	metrics    map[string]*StateMetrics
	mu         sync.RWMutex
	windowSize time.Duration
}

// StateMetrics tracks metrics for a state type
type StateMetrics struct {
	TotalChanges   int64
	CreateCount    int64
	UpdateCount    int64
	DeleteCount    int64
	LastChangeTime time.Time
	ChangeRate     float64
	recentChanges  []time.Time
}

// NewMetricsObserver creates a new metrics observer
func NewMetricsObserver(windowSize time.Duration) *MetricsObserver {
	return &MetricsObserver{
		metrics:    make(map[string]*StateMetrics),
		windowSize: windowSize,
	}
}

// OnStateChange updates metrics based on state changes
func (o *MetricsObserver) OnStateChange(event *StateEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	key := string(event.StateType)
	metrics, exists := o.metrics[key]
	if !exists {
		metrics = &StateMetrics{
			recentChanges: make([]time.Time, 0),
		}
		o.metrics[key] = metrics
	}

	metrics.TotalChanges++
	switch event.Type {
	case StateEventCreated:
		metrics.CreateCount++
	case StateEventUpdated:
		metrics.UpdateCount++
	case StateEventDeleted:
		metrics.DeleteCount++
	}

	metrics.LastChangeTime = event.Timestamp
	metrics.recentChanges = append(metrics.recentChanges, event.Timestamp)

	cutoff := time.Now().Add(-o.windowSize)
	validChanges := make([]time.Time, 0)
	for _, t := range metrics.recentChanges {
		if t.After(cutoff) {
			validChanges = append(validChanges, t)
		}
	}
	metrics.recentChanges = validChanges

	if len(validChanges) > 0 {
		duration := time.Since(validChanges[0])
		if duration > 0 {
			metrics.ChangeRate = float64(len(validChanges)) / duration.Minutes()
		}
	}
	return nil
}

// GetID returns the observer ID
func (o *MetricsObserver) GetID() string {
	return "metrics_observer"
}

// IsActive returns whether the observer is active
func (o *MetricsObserver) IsActive() bool {
	return true
}

// GetMetrics returns metrics for a state type
func (o *MetricsObserver) GetMetrics(stateType StateType) *StateMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	metrics, exists := o.metrics[string(stateType)]
	if !exists {
		return nil
	}

	metricsCopy := *metrics
	return &metricsCopy
}

// GetAllMetrics returns all collected metrics
func (o *MetricsObserver) GetAllMetrics() map[string]*StateMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make(map[string]*StateMetrics)
	for k, v := range o.metrics {
		metricsCopy := *v
		result[k] = &metricsCopy
	}
	return result
}

// AlertingObserver sends alerts for specific state changes
type AlertingObserver struct {
	alertHandlers map[string]AlertHandler
	mu            sync.RWMutex
	logger        *slog.Logger
}

// AlertHandler handles alerts for state changes
type AlertHandler func(event *StateEvent) error

// NewAlertingObserver creates a new alerting observer
func NewAlertingObserver(logger *slog.Logger) *AlertingObserver {
	return &AlertingObserver{
		alertHandlers: make(map[string]AlertHandler),
		logger:        logger.With("component", "alerting_observer"),
	}
}

// RegisterAlert registers an alert handler for specific conditions
func (o *AlertingObserver) RegisterAlert(name string, handler AlertHandler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.alertHandlers[name] = handler
}

// OnStateChange checks for alert conditions
func (o *AlertingObserver) OnStateChange(event *StateEvent) error {
	o.mu.RLock()
	handlers := make(map[string]AlertHandler)
	for k, v := range o.alertHandlers {
		handlers[k] = v
	}
	o.mu.RUnlock()

	for name, handler := range handlers {
		go func(n string, h AlertHandler) {
			if err := h(event); err != nil {
				o.logger.Error("Alert handler failed",
					slog.String("error", err.Error()),
					slog.String("alert_name", n),
					slog.String("event_id", event.ID))
			}
		}(name, handler)
	}
	return nil
}

// GetID returns the observer ID
func (o *AlertingObserver) GetID() string {
	return "alerting_observer"
}

// IsActive returns whether the observer is active
func (o *AlertingObserver) IsActive() bool {
	return true
}

// AuditObserver maintains an audit trail of state changes
type AuditObserver struct {
	auditLog []AuditEntry
	maxSize  int
	mu       sync.RWMutex
	logger   *slog.Logger
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	EventID   string
	EventType StateEventType
	StateType StateType
	StateID   string
	Timestamp time.Time
	UserID    string
	SessionID string
	Changes   map[string]interface{}
	Metadata  map[string]interface{}
}

// NewAuditObserver creates a new audit observer
func NewAuditObserver(maxSize int, logger *slog.Logger) *AuditObserver {
	return &AuditObserver{
		auditLog: make([]AuditEntry, 0),
		maxSize:  maxSize,
		logger:   logger.With("component", "audit_observer"),
	}
}

// OnStateChange adds an audit entry for the state change
func (o *AuditObserver) OnStateChange(event *StateEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry := AuditEntry{
		EventID:   event.ID,
		EventType: event.Type,
		StateType: event.StateType,
		StateID:   event.StateID,
		Timestamp: event.Timestamp,
		Metadata:  event.Metadata,
	}

	if event.Metadata != nil {
		if userID, ok := event.Metadata["user_id"].(string); ok {
			entry.UserID = userID
		}
		if sessionID, ok := event.Metadata["session_id"].(string); ok {
			entry.SessionID = sessionID
		}
	}

	if event.Type == StateEventUpdated && event.OldValue != nil && event.NewValue != nil {
		entry.Changes = o.calculateChanges(event.OldValue, event.NewValue)
	}

	o.auditLog = append(o.auditLog, entry)

	if len(o.auditLog) > o.maxSize {
		o.auditLog = o.auditLog[len(o.auditLog)-o.maxSize:]
	}

	o.logger.Debug("Audit entry created",
		slog.String("event_id", entry.EventID),
		slog.String("state_type", string(entry.StateType)),
		slog.String("state_id", entry.StateID))
	return nil
}

// GetID returns the observer ID
func (o *AuditObserver) GetID() string {
	return "audit_observer"
}

// IsActive returns whether the observer is active
func (o *AuditObserver) IsActive() bool {
	return true
}

// GetAuditLog returns the audit log
func (o *AuditObserver) GetAuditLog(limit int) []AuditEntry {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if limit <= 0 || limit > len(o.auditLog) {
		limit = len(o.auditLog)
	}

	result := make([]AuditEntry, limit)
	copy(result, o.auditLog[len(o.auditLog)-limit:])
	return result
}

// calculateChanges calculates what changed between old and new values
func (o *AuditObserver) calculateChanges(oldValue, newValue interface{}) map[string]interface{} {
	changes := make(map[string]interface{})
	changes["old"] = fmt.Sprintf("%v", oldValue)
	changes["new"] = fmt.Sprintf("%v", newValue)
	return changes
}

// CompositeObserver combines multiple observers
type CompositeObserver struct {
	observers []StateObserver
}

// NewCompositeObserver creates a new composite observer
func NewCompositeObserver(observers ...StateObserver) StateObserver {
	return &CompositeObserver{
		observers: observers,
	}
}

// OnStateChange notifies all observers
func (o *CompositeObserver) OnStateChange(event *StateEvent) error {
	for _, observer := range o.observers {
		if err := observer.OnStateChange(event); err != nil {
			return err
		}
	}
	return nil
}

// GetID returns the observer ID
func (o *CompositeObserver) GetID() string {
	return "composite_observer"
}

// IsActive returns whether the observer is active
func (o *CompositeObserver) IsActive() bool {
	return true
}
