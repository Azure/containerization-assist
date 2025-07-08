package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// ============================================================================
// Session Management & Middleware HTTP Handlers
// ============================================================================

// This file contains session management handlers and middleware components
// for the HTTP transport layer. The core request handlers are in
// http_handlers_core.go, and type extensions are in http_handlers_types.go.

// ============================================================================
// Session Management Handlers
// ============================================================================

// handleListSessions lists all active sessions in the system.
// This endpoint provides session management functionality for administrative purposes.
func (t *HTTPTransport) handleListSessions(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	handler, exists := t.tools["list_sessions"]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, "Session management not available")
		return
	}

	result, err := handler.Handler(r.Context(), map[string]interface{}{})
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

// handleGetSession retrieves details for a specific session by ID.
// This endpoint provides detailed information about a single session.
func (t *HTTPTransport) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	// Get session details tool - this needs to be a specific tool for getting a single session
	if getSessionTool, exists := t.tools["get_session"]; exists {
		response, err := getSessionTool.Handler(r.Context(), map[string]interface{}{
			"session_id": sessionID,
		})
		if err != nil {
			t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get session %s: %v", sessionID, err))
			return
		}

		t.sendJSON(w, http.StatusOK, response)
		return
	}

	// Fallback: try to use list_sessions and filter
	if listTool, exists := t.tools["list_sessions"]; exists {
		listResponse, err := listTool.Handler(r.Context(), map[string]interface{}{})
		if err != nil {
			t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
			return
		}

		// Extract sessions from response and find the one we want
		if respMap, ok := listResponse.(map[string]interface{}); ok {
			if sessions, ok := respMap["sessions"].([]map[string]interface{}); ok {
				for _, session := range sessions {
					if sid, ok := session["session_id"].(string); ok && sid == sessionID {
						t.sendJSON(w, http.StatusOK, session)
						return
					}
				}
			}
		}

		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Session %s not found", sessionID))
		return
	}

	t.sendError(w, http.StatusServiceUnavailable, "Session management not available")
}

// handleDeleteSession removes a session from the system.
// This endpoint allows for cleanup of expired or unwanted sessions.
func (t *HTTPTransport) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	t.toolsMutex.RLock()
	handler, exists := t.tools["delete_session"]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, "Session management not available")
		return
	}

	result, err := handler.Handler(r.Context(), map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

// ============================================================================
// HTTP Middleware Functions
// ============================================================================

// loggingMiddleware provides comprehensive request/response logging.
// This middleware captures request details, response status, and execution time.
func (t *HTTPTransport) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := middleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Prepare log attributes
		logAttrs := []any{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		}

		if t.logBodies {
			headers := make(map[string]string)
			for k, v := range r.Header {
				if k != "Authorization" && k != "Api-Key" {
					headers[k] = strings.Join(v, ", ")
				}
			}
			logAttrs = append(logAttrs, "request_headers", headers)
		}

		if t.logBodies && r.Body != nil {
			bodyReader := io.LimitReader(r.Body, t.maxBodyLogSize)
			requestBody, err := io.ReadAll(bodyReader)
			if err != nil {
				t.logger.Debug("Failed to read request body", "error", err)
			}
			if err := r.Body.Close(); err != nil {
				t.logger.Debug("Failed to close request body", "error", err)
			}

			r.Body = io.NopCloser(bytes.NewReader(requestBody))

			if len(requestBody) > 0 {
				logAttrs = append(logAttrs, "request_body", string(requestBody))
			}
		}

		t.logger.Info("HTTP request received", logAttrs...)

		wrapped := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			logBodies:      t.logBodies,
			maxSize:        t.maxBodyLogSize,
		}

		next.ServeHTTP(wrapped, r)

		// Prepare response log attributes
		responseAttrs := []any{
			"request_id", requestID,
			"status", wrapped.statusCode,
			"duration", time.Since(start),
			"response_size", wrapped.bytesWritten,
		}

		if t.logBodies && len(wrapped.body) > 0 {
			responseAttrs = append(responseAttrs, "response_body", string(wrapped.body))
		}

		t.logger.Info("HTTP response sent", responseAttrs...)

		if wrapped.statusCode >= 400 || r.Method != "GET" {
			t.logger.Warn("Security audit: Non-GET request or error response",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"status", wrapped.statusCode)
		}
	})
}

// authMiddleware handles API key authentication for protected endpoints.
// This middleware validates API keys and allows health check bypass.
func (t *HTTPTransport) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		if t.apiKey != "" {
			providedKey := r.Header.Get("X-API-Key")
			if providedKey == "" {
				providedKey = r.URL.Query().Get("api_key")
			}

			if providedKey != t.apiKey {
				t.sendError(w, http.StatusUnauthorized, "Invalid or missing API key")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware implements rate limiting based on client IP.
// This middleware prevents abuse by limiting requests per time window.
func (t *HTTPTransport) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = strings.Split(forwarded, ",")[0]
		}

		if !t.checkRateLimit(clientIP) {
			t.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================================================
// Logging Response Writer
// ============================================================================

// WriteHeader captures the HTTP status code for logging purposes.
func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures response body data for logging (up to configured limit).
func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.logBodies && int64(len(w.body)) < w.maxSize {
		remaining := w.maxSize - int64(len(w.body))
		if remaining > 0 {
			toCopy := int64(len(data))
			if toCopy > remaining {
				toCopy = remaining
			}
			w.body = append(w.body, data[:toCopy]...)
		}
	}

	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += n
	return n, err
}

// ============================================================================
// Utility Functions
// ============================================================================

// sendJSON sends a JSON response with the specified status code.
func (t *HTTPTransport) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		t.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// sendError sends a standardized error response with timestamp.
func (t *HTTPTransport) sendError(w http.ResponseWriter, status int, message string) {
	errorResponse := ErrorResponse{
		Code:    fmt.Sprintf("HTTP_%d", status),
		Message: message,
		Type:    "http_error",
	}

	response := struct {
		Error     ErrorResponse `json:"error"`
		Status    int           `json:"status"`
		Timestamp int64         `json:"timestamp"`
	}{
		Error:     errorResponse,
		Status:    status,
		Timestamp: time.Now().Unix(),
	}

	t.sendJSON(w, status, response)
}

// checkRateLimit checks if a client has exceeded the configured rate limit.
// This function implements a sliding window rate limiting algorithm.
func (t *HTTPTransport) checkRateLimit(clientIP string) bool {
	limiter, exists := t.rateLimiter[clientIP]
	if !exists {
		limiter = &rateLimiter{
			requests: make([]time.Time, 0),
		}
		t.rateLimiter[clientIP] = limiter
	}

	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	validRequests := make([]time.Time, 0)
	for _, reqTime := range limiter.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	if len(validRequests) >= t.rateLimit {
		return false
	}

	limiter.requests = append(validRequests, now)
	return true
}
