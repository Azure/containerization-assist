package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHealthMonitor is a test double for health.Monitor
type mockHealthMonitor struct{}

func (m *mockHealthMonitor) RegisterChecker(checker health.Checker) {}

func (m *mockHealthMonitor) GetHealth(ctx context.Context) health.HealthReport {
	return health.HealthReport{
		Status:     health.StatusHealthy,
		Components: make(map[string]health.ComponentHealth),
		Metadata: map[string]interface{}{
			"version": "0.0.6",
		},
	}
}

func (m *mockHealthMonitor) GetComponentHealth(ctx context.Context, component string) (health.Status, error) {
	return health.StatusHealthy, nil
}

func TestHandler_NewHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}

	handler := NewHandler(logger, 8080, monitor)
	assert.NotNil(t, handler, "Handler should be created successfully")
}

func TestHandler_NewHandler_DefaultPort(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}

	handler := NewHandler(logger, 0, monitor) // Should default to 8080
	assert.NotNil(t, handler, "Handler should be created successfully")
}

func TestHandler_RPCEndpoint_Initialize(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	// Test initialize request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params":  map[string]interface{}{},
		"id":      1,
	}

	requestBody, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/rpc", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Should set JSON content type")

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "2.0", response["jsonrpc"], "Should have correct JSON-RPC version")
	assert.Equal(t, float64(1), response["id"], "Should have correct ID")
	assert.Contains(t, response, "result", "Should have result field")
}

func TestHandler_RPCEndpoint_ListTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	// Test tools/list request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      2,
	}

	requestBody, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/rpc", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "2.0", response["jsonrpc"], "Should have correct JSON-RPC version")
	assert.Equal(t, float64(2), response["id"], "Should have correct ID")

	result, ok := response["result"].(map[string]interface{})
	require.True(t, ok, "Should have result object")

	tools, ok := result["tools"].([]interface{})
	require.True(t, ok, "Should have tools array")
	assert.NotEmpty(t, tools, "Should have at least one tool")
}

func TestHandler_RPCEndpoint_MethodNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	// Test unknown method
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "unknown_method",
		"id":      3,
	}

	requestBody, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/rpc", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK (JSON-RPC error)")

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, "2.0", response["jsonrpc"], "Should have correct JSON-RPC version")
	assert.Equal(t, float64(3), response["id"], "Should have correct ID")

	errorObj, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "Should have error object")
	assert.Equal(t, float64(-32601), errorObj["code"], "Should have method not found error code")
}

func TestHandler_RPCEndpoint_InvalidMethod(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	// Test GET request (should be POST only)
	req := httptest.NewRequest("GET", "/rpc", nil)
	rr := httptest.NewRecorder()
	handler.handleRPC(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "Should return 405 Method Not Allowed")
}

func TestHandler_HealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.handleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Should set JSON content type")

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Contains(t, response, "status", "Should have status field")
	assert.Contains(t, response, "metadata", "Should have metadata field")

	// Check version in metadata
	if metadata, ok := response["metadata"].(map[string]interface{}); ok {
		assert.Contains(t, metadata, "version", "Should have version in metadata")
	} else {
		t.Error("Metadata should be a map")
	}
}

func TestHandler_RootEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.handleRoot(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "Should set JSON content type")

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Contains(t, response, "name", "Should have name field")
	assert.Contains(t, response, "version", "Should have version field")
	assert.Contains(t, response, "endpoints", "Should have endpoints field")
}

func TestHandler_CORS_Middleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	monitor := &mockHealthMonitor{}
	handler := NewHandler(logger, 8080, monitor)

	// Test OPTIONS request for CORS preflight
	req := httptest.NewRequest("OPTIONS", "/rpc", nil)
	rr := httptest.NewRecorder()

	// Use the middleware directly
	testHandler := handler.withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	testHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 OK for OPTIONS")
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"), "Should set CORS origin")
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST", "Should allow POST method")
}
