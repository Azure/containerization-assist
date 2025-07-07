package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/rs/zerolog"
)

// CommunicationManager provides advanced communication patterns for tool coordination
type CommunicationManager struct {
	eventBus           *EventBus
	correlationTracker map[string]*RequestCorrelation
	circuitBreakers    map[string]*CircuitBreaker
	requestMetrics     map[string]*RequestMetrics
	mutex              sync.RWMutex
	logger             zerolog.Logger
}

// NewCommunicationManager creates a new communication manager
func NewCommunicationManager(logger zerolog.Logger) *CommunicationManager {
	return &CommunicationManager{
		eventBus:           NewEventBus(logger),
		correlationTracker: make(map[string]*RequestCorrelation),
		circuitBreakers:    make(map[string]*CircuitBreaker),
		requestMetrics:     make(map[string]*RequestMetrics),
		logger:             logger.With().Str("component", "communication_manager").Logger(),
	}
}

// ToolRequest represents a request to execute a tool
type ToolRequest struct {
	ID            string                 `json:"id"`
	ToolName      string                 `json:"tool_name"`
	SessionID     string                 `json:"session_id"`
	Parameters    map[string]interface{} `json:"parameters"`
	CorrelationID string                 `json:"correlation_id"`
	Timeout       time.Duration          `json:"timeout"`
	Timestamp     time.Time              `json:"timestamp"`
	RetryCount    int                    `json:"retry_count"`
	Context       map[string]interface{} `json:"context"`
}

// ToolResponse represents a response from tool execution
type ToolResponse struct {
	ID            string                 `json:"id"`
	ToolName      string                 `json:"tool_name"`
	SessionID     string                 `json:"session_id"`
	CorrelationID string                 `json:"correlation_id"`
	Success       bool                   `json:"success"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// RequestCorrelation tracks related requests for distributed tracing
type RequestCorrelation struct {
	ID            string                 `json:"id"`
	RootRequestID string                 `json:"root_request_id"`
	ParentID      string                 `json:"parent_id,omitempty"`
	SessionID     string                 `json:"session_id"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Status        string                 `json:"status"`
	ToolChain     []string               `json:"tool_chain"`
	Context       map[string]interface{} `json:"context"`
}

// CircuitBreaker provides circuit breaker pattern for tool failures
type CircuitBreaker struct {
	name         string
	maxFailures  int
	resetTimeout time.Duration
	failureCount int
	lastFailure  time.Time
	state        CircuitBreakerState
	mutex        sync.RWMutex
	logger       zerolog.Logger
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerOpen     CircuitBreakerState = "open"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
)

// RequestMetrics tracks metrics for tool requests
type RequestMetrics struct {
	ToolName        string        `json:"tool_name"`
	TotalRequests   int64         `json:"total_requests"`
	SuccessfulReqs  int64         `json:"successful_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	P95Latency      time.Duration `json:"p95_latency"`
	ErrorRate       float64       `json:"error_rate"`
	latencyHistory  []time.Duration
	maxHistorySize  int
}

// SendRequest sends a request with comprehensive communication patterns
func (cm *CommunicationManager) SendRequest(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
	startTime := time.Now()

	if request.CorrelationID == "" {
		request.CorrelationID = cm.generateCorrelationID()
	}
	correlation := &RequestCorrelation{
		ID:            request.CorrelationID,
		RootRequestID: request.CorrelationID,
		SessionID:     request.SessionID,
		StartTime:     startTime,
		Status:        "pending",
		ToolChain:     []string{request.ToolName},
		Context:       request.Context,
	}

	cm.mutex.Lock()
	cm.correlationTracker[request.CorrelationID] = correlation
	cm.mutex.Unlock()

	breaker := cm.getOrCreateCircuitBreaker(request.ToolName)
	if !breaker.Allow() {
		correlation.Status = "circuit_breaker_open"
		correlation.EndTime = &startTime

		cm.logger.Warn().
			Str("tool_name", request.ToolName).
			Str("correlation_id", request.CorrelationID).
			Msg("Request blocked by circuit breaker")

		systemErr := errors.SystemError(
			codes.SYSTEM_OVERLOADED,
			fmt.Sprintf("Circuit breaker open for tool %s", request.ToolName),
			nil,
		)
		systemErr.Context["tool"] = request.ToolName
		systemErr.Context["component"] = "circuit_breaker"
		systemErr.Suggestions = append(systemErr.Suggestions, "Wait for circuit breaker to reset or check tool health")
		return nil, systemErr
	}

	cm.eventBus.Publish(EventTypeToolRequestStarted, map[string]interface{}{
		"request":        request,
		"correlation_id": request.CorrelationID,
		"timestamp":      startTime,
	})

	cm.logger.Info().
		Str("tool_name", request.ToolName).
		Str("session_id", request.SessionID).
		Str("correlation_id", request.CorrelationID).
		Msg("Sending tool request")

	response, err := cm.sendWithRetry(ctx, request)
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	correlation.EndTime = &endTime
	if err != nil {
		correlation.Status = "failed"
		breaker.RecordFailure()
		cm.updateMetrics(request.ToolName, duration, false)

		cm.eventBus.Publish(EventTypeToolRequestFailed, map[string]interface{}{
			"request":        request,
			"error":          err.Error(),
			"duration":       duration,
			"correlation_id": request.CorrelationID,
			"timestamp":      endTime,
		})

		cm.logger.Error().
			Err(err).
			Str("tool_name", request.ToolName).
			Str("correlation_id", request.CorrelationID).
			Dur("duration", duration).
			Msg("Tool request failed")

		return nil, err
	}

	correlation.Status = "completed"
	breaker.RecordSuccess()
	cm.updateMetrics(request.ToolName, duration, true)

	cm.eventBus.Publish(EventTypeToolRequestCompleted, map[string]interface{}{
		"request":        request,
		"response":       response,
		"duration":       duration,
		"correlation_id": request.CorrelationID,
		"timestamp":      endTime,
	})

	cm.logger.Info().
		Str("tool_name", request.ToolName).
		Str("correlation_id", request.CorrelationID).
		Dur("duration", duration).
		Msg("Tool request completed successfully")

	return response, nil
}

// sendWithRetry implements retry logic with exponential backoff
func (cm *CommunicationManager) sendWithRetry(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))

			cm.logger.Debug().
				Str("tool_name", request.ToolName).
				Str("correlation_id", request.CorrelationID).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying tool request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := cm.executeToolRequest(ctx, request)
		if err == nil {
			return response, nil
		}

		if !cm.isRetryableError(err) || attempt == maxRetries {
			return nil, err
		}

		request.RetryCount = attempt + 1
	}

	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		fmt.Sprintf("Request failed after %d retries", maxRetries),
		nil,
	)
	systemErr.Context["max_retries"] = maxRetries
	systemErr.Context["component"] = "retry_manager"
	systemErr.Suggestions = append(systemErr.Suggestions, "Check system health and consider increasing retry limits")
	return nil, systemErr
}

// executeToolRequest simulates tool execution (placeholder)
func (cm *CommunicationManager) executeToolRequest(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		return &ToolResponse{
			ID:            cm.generateResponseID(),
			ToolName:      request.ToolName,
			SessionID:     request.SessionID,
			CorrelationID: request.CorrelationID,
			Success:       true,
			Result: map[string]interface{}{
				"tool_executed": request.ToolName,
				"parameters":    request.Parameters,
				"simulated":     true,
			},
			Duration:  50 * time.Millisecond,
			Timestamp: time.Now(),
			Metadata:  map[string]interface{}{"retry_count": request.RetryCount},
		}, nil
	}
}

// isRetryableError determines if an error is retryable
func (cm *CommunicationManager) isRetryableError(err error) bool {
	errStr := err.Error()
	retryableErrors := []string{"timeout", "connection", "temporary", "unavailable"}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// generateCorrelationID generates a unique correlation ID
func (cm *CommunicationManager) generateCorrelationID() string {
	return fmt.Sprintf("corr_%d", time.Now().UnixNano())
}

// generateResponseID generates a unique response ID
func (cm *CommunicationManager) generateResponseID() string {
	return fmt.Sprintf("resp_%d", time.Now().UnixNano())
}

// getOrCreateCircuitBreaker gets or creates a circuit breaker for a tool
func (cm *CommunicationManager) getOrCreateCircuitBreaker(toolName string) *CircuitBreaker {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if breaker, exists := cm.circuitBreakers[toolName]; exists {
		return breaker
	}

	breaker := &CircuitBreaker{
		name:         toolName,
		maxFailures:  5,
		resetTimeout: 60 * time.Second,
		state:        CircuitBreakerClosed,
		logger:       cm.logger.With().Str("circuit_breaker", toolName).Logger(),
	}

	cm.circuitBreakers[toolName] = breaker
	return breaker
}

// updateMetrics updates request metrics for a tool
func (cm *CommunicationManager) updateMetrics(toolName string, duration time.Duration, success bool) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	metrics, exists := cm.requestMetrics[toolName]
	if !exists {
		metrics = &RequestMetrics{
			ToolName:       toolName,
			maxHistorySize: 100,
			latencyHistory: make([]time.Duration, 0, 100),
		}
		cm.requestMetrics[toolName] = metrics
	}

	metrics.TotalRequests++
	metrics.LastRequestTime = time.Now()

	if success {
		metrics.SuccessfulReqs++
	} else {
		metrics.FailedRequests++
	}

	metrics.latencyHistory = append(metrics.latencyHistory, duration)
	if len(metrics.latencyHistory) > metrics.maxHistorySize {
		metrics.latencyHistory = metrics.latencyHistory[1:]
	}

	var total time.Duration
	for _, d := range metrics.latencyHistory {
		total += d
	}
	metrics.AverageLatency = total / time.Duration(len(metrics.latencyHistory))

	if len(metrics.latencyHistory) >= 20 {
		sorted := make([]time.Duration, len(metrics.latencyHistory))
		copy(sorted, metrics.latencyHistory)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		p95Index := int(float64(len(sorted)) * 0.95)
		metrics.P95Latency = sorted[p95Index]
	}

	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	}
}

// GetCorrelation retrieves correlation information
func (cm *CommunicationManager) GetCorrelation(correlationID string) (*RequestCorrelation, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	correlation, exists := cm.correlationTracker[correlationID]
	return correlation, exists
}

// GetMetrics retrieves metrics for a tool
func (cm *CommunicationManager) GetMetrics(toolName string) (*RequestMetrics, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	metrics, exists := cm.requestMetrics[toolName]
	return metrics, exists
}

// GetAllMetrics retrieves all tool metrics
func (cm *CommunicationManager) GetAllMetrics() map[string]*RequestMetrics {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	result := make(map[string]*RequestMetrics)
	for k, v := range cm.requestMetrics {
		result[k] = v
	}

	return result
}

// SubscribeToEvents subscribes to communication events
func (cm *CommunicationManager) SubscribeToEvents(eventType EventType, handler EventHandler) {
	cm.eventBus.Subscribe(eventType, handler)
}

// Close gracefully shuts down the communication manager
func (cm *CommunicationManager) Close() error {
	cm.eventBus.Close()
	return nil
}

// Allow checks if the circuit breaker allows the request
func (cb *CircuitBreaker) Allow() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == CircuitBreakerOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
				cb.state = CircuitBreakerHalfOpen
				cb.logger.Info().Msg("Circuit breaker transitioning to half-open")
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return cb.state == CircuitBreakerHalfOpen
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerClosed
		cb.failureCount = 0
		cb.logger.Info().Msg("Circuit breaker closed after successful request")
	case CircuitBreakerClosed:
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failureCount >= cb.maxFailures {
			cb.state = CircuitBreakerOpen
			cb.logger.Warn().
				Int("failure_count", cb.failureCount).
				Msg("Circuit breaker opened due to failures")
		}
	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerOpen
		cb.logger.Warn().Msg("Circuit breaker opened from half-open state")
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}
