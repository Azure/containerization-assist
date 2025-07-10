package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// circuitBreaker implements a simple circuit breaker pattern
type circuitBreaker struct {
	mutex        sync.RWMutex
	failureCount int
	lastFailure  time.Time
	failureLimit int
	resetTimeout time.Duration
	isOpen       bool
}

// HTTPLLMTransport implements types.LLMTransport for HTTP transport
// It can invoke tools back to the hosting LLM via HTTP requests
type HTTPLLMTransport struct {
	client  *http.Client
	baseURL string
	apiKey  string
	logger  *slog.Logger
	cb      *circuitBreaker
	// metrics functionality removed
	connected bool
}

// HTTPLLMTransportConfig configures the HTTP LLM transport
type HTTPLLMTransportConfig struct {
	BaseURL string        // Base URL for the hosting LLM API
	APIKey  string        // API key for authentication
	Timeout time.Duration // HTTP timeout (default: 30s)
}

// NewHTTPLLMTransport creates a new HTTP LLM transport
func NewHTTPLLMTransport(config HTTPLLMTransportConfig, logger *slog.Logger) *HTTPLLMTransport {
	return NewHTTPLLMTransportWithMetrics(config, logger, nil)
}

// NewHTTPLLMTransportWithMetrics creates a new HTTP LLM transport with metrics
func NewHTTPLLMTransportWithMetrics(config HTTPLLMTransportConfig, logger *slog.Logger, metrics interface{}) *HTTPLLMTransport {
	// Store the metrics for potential future use
	_ = metrics
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &HTTPLLMTransport{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		logger:  logger.With("component", "http_llm_transport"),
		cb: &circuitBreaker{
			failureLimit: 5,
			resetTimeout: 60 * time.Second,
		},
		// metrics functionality removed
	}
}

// isCircuitOpen checks if the circuit breaker is open
func (cb *circuitBreaker) isCircuitOpen() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	if !cb.isOpen {
		return false
	}

	// Check if reset timeout has passed
	if time.Since(cb.lastFailure) > cb.resetTimeout {
		return false
	}

	return true
}

// recordFailure records a failure and potentially opens the circuit
func (cb *circuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.failureLimit {
		cb.isOpen = true
	}
}

// recordSuccess records a successful call and resets the circuit
func (cb *circuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0
	cb.isOpen = false
}

// InvokeTool implements types.LLMTransport
// For HTTP, this means making an HTTP request to the hosting LLM
func (h *HTTPLLMTransport) InvokeTool(ctx context.Context, name string, payload map[string]any, stream bool) (<-chan json.RawMessage, error) {
	h.logger.Debug("Invoking tool on hosting LLM via HTTP",
		"tool_name", name,
		"stream", stream,
		"base_url", h.baseURL)

	// Check circuit breaker
	if err := h.checkCircuitBreaker(); err != nil {
		return nil, err
	}

	responseCh := make(chan json.RawMessage, 1)
	go h.processHTTPRequest(ctx, name, payload, stream, responseCh)
	return responseCh, nil
}

// checkCircuitBreaker validates circuit breaker state
func (h *HTTPLLMTransport) checkCircuitBreaker() error {
	if h.cb.isCircuitOpen() {
		h.logger.Warn("Circuit breaker is open, rejecting request")
		// Metrics functionality removed
		return errors.NewError().Messagef("circuit breaker is open").WithLocation().Build()
	}
	return nil
}

// processHTTPRequest handles the HTTP request in a separate goroutine
func (h *HTTPLLMTransport) processHTTPRequest(ctx context.Context, name string, payload map[string]any, stream bool, responseCh chan<- json.RawMessage) {
	defer close(responseCh)

	// Validate configuration
	if err := h.validateConfiguration(); err != nil {
		h.sendErrorResponse(ctx, responseCh, err.Error())
		return
	}

	// Build and execute request
	req, err := h.buildHTTPRequest(ctx, name, payload, stream)
	if err != nil {
		h.sendErrorResponse(ctx, responseCh, fmt.Sprintf("Failed to build request: %v", err))
		return
	}

	// Execute the request
	resp, err := h.executeHTTPRequest(req)
	if err != nil {
		h.sendErrorResponse(ctx, responseCh, fmt.Sprintf("HTTP request failed: %v", err))
		return
	}
	defer h.closeResponse(resp)

	// Process response
	if err := h.processHTTPResponse(ctx, resp, responseCh); err != nil {
		h.logger.Error("Failed to process HTTP response", "error", err)
	}
}

// validateConfiguration checks if the HTTP transport is properly configured
func (h *HTTPLLMTransport) validateConfiguration() error {
	if h.baseURL == "" {
		return errors.NewError().Messagef("HTTP LLM transport not configured (missing base URL)").Build()
	}
	return nil
}

// buildHTTPRequest creates the HTTP request
func (h *HTTPLLMTransport) buildHTTPRequest(ctx context.Context, name string, payload map[string]any, stream bool) (*http.Request, error) {
	requestPayload := map[string]interface{}{
		"tool":    name,
		"payload": payload,
		"stream":  stream,
	}

	requestBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/tools/invoke", h.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBytes))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	return req, nil
}

// executeHTTPRequest performs the HTTP request
func (h *HTTPLLMTransport) executeHTTPRequest(req *http.Request) (*http.Response, error) {
	// Metrics recording removed

	resp, err := h.client.Do(req)
	if err != nil {
		h.cb.recordFailure()
		// Metrics functionality removed
		return nil, err
	}

	return resp, nil
}

// processHTTPResponse handles the HTTP response
func (h *HTTPLLMTransport) processHTTPResponse(ctx context.Context, resp *http.Response, responseCh chan<- json.RawMessage) error {
	// Read response with size limit (10MB)
	const maxResponseSize = 10 * 1024 * 1024 // 10MB
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	responseBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return err
	}

	// Check if we hit the limit
	if len(responseBytes) == maxResponseSize {
		h.logger.Warn("HTTP response truncated due to size limit",
			"max_size", maxResponseSize)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		h.cb.recordFailure()
		h.logger.Error("HTTP request returned error status",
			"status_code", resp.StatusCode,
			"response", string(responseBytes))

		errorMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(responseBytes))
		h.sendErrorResponse(ctx, responseCh, errorMsg)
		return nil
	}

	// Record successful response
	h.cb.recordSuccess()
	// Metrics recording removed

	h.logger.Debug("Received HTTP response from hosting LLM",
		"response_size", len(responseBytes))

	// Send response
	select {
	case responseCh <- json.RawMessage(responseBytes):
	case <-ctx.Done():
		h.logger.Debug("Debug message")
	}

	return nil
}

// sendErrorResponse sends an error response through the channel
func (h *HTTPLLMTransport) sendErrorResponse(ctx context.Context, responseCh chan<- json.RawMessage, errorMsg string) {
	errorResponse := struct {
		Error string `json:"error"`
	}{
		Error: errorMsg,
	}

	if responseBytes, err := json.Marshal(errorResponse); err == nil {
		select {
		case responseCh <- json.RawMessage(responseBytes):
		case <-ctx.Done():
		}
	}
}

// closeResponse safely closes the HTTP response body
func (h *HTTPLLMTransport) closeResponse(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		h.logger.Warn("Failed to close response body", "error", err)
	}
}

// Start implements types.LLMTransport
func (h *HTTPLLMTransport) Start(ctx context.Context) error {
	h.logger.Info("Starting HTTP LLM transport")
	h.connected = true
	return nil
}

// Stop implements types.LLMTransport
func (h *HTTPLLMTransport) Stop(ctx context.Context) error {
	h.logger.Info("Stopping HTTP LLM transport")
	h.connected = false
	return nil
}

// Send implements types.LLMTransport
func (h *HTTPLLMTransport) Send(ctx context.Context, message interface{}) error {
	h.logger.Debug("Debug message")
	// For HTTP transport, sending is handled via InvokeTool
	return nil
}

// Receive implements types.LLMTransport
func (h *HTTPLLMTransport) Receive(ctx context.Context) (interface{}, error) {
	h.logger.Debug("Debug message")
	// For HTTP transport, receiving is handled via InvokeTool response channels
	return nil, nil
}

// IsConnected implements types.LLMTransport
func (h *HTTPLLMTransport) IsConnected() bool {
	return h.connected
}

// TODO: Interface compliance check disabled due to internal package restriction
// var _ types.LLMTransport = (*HTTPLLMTransport)(nil)
