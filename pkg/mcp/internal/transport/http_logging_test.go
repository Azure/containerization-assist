package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestHTTPLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		logBodies      bool
		maxBodyLogSize int64
		requestBody    string
		responseBody   string
		expectReqBody  bool
		expectRespBody bool
	}{
		{
			name:           "log bodies enabled",
			logBodies:      true,
			maxBodyLogSize: 1024,
			requestBody:    `{"test": "request"}`,
			responseBody:   `{"result": "success"}`,
			expectReqBody:  true,
			expectRespBody: true,
		},
		{
			name:           "log bodies disabled",
			logBodies:      false,
			maxBodyLogSize: 1024,
			requestBody:    `{"test": "request"}`,
			responseBody:   `{"result": "success"}`,
			expectReqBody:  false,
			expectRespBody: false,
		},
		{
			name:           "body exceeds max size",
			logBodies:      true,
			maxBodyLogSize: 10,
			requestBody:    `{"test": "this is a very long request body that exceeds the limit"}`,
			responseBody:   `{"result": "this is a very long response that exceeds the limit"}`,
			expectReqBody:  true, // Should log truncated
			expectRespBody: true, // Should log truncated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture logs
			var logBuf bytes.Buffer
			logger := zerolog.New(&logBuf).With().Timestamp().Logger()

			// Create transport with test config
			transport := &HTTPTransport{
				logger:         logger,
				logBodies:      tt.logBodies,
				maxBodyLogSize: tt.maxBodyLogSize,
			}

			// Create a test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(tt.responseBody)); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
			})

			// Wrap with logging middleware
			wrappedHandler := transport.loggingMiddleware(handler)

			// Create test request
			req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")

			// Create response recorder
			recorder := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(recorder, req)

			// Check response
			if recorder.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", recorder.Code)
			}

			// Parse logs
			logs := logBuf.String()

			// Check if request body was logged
			if tt.expectReqBody {
				if !bytes.Contains([]byte(logs), []byte("request_body")) {
					t.Error("Expected request_body in logs")
				}
			} else {
				if bytes.Contains([]byte(logs), []byte("request_body")) {
					t.Error("Did not expect request_body in logs")
				}
			}

			// Check if response body was logged
			if tt.expectRespBody {
				if !bytes.Contains([]byte(logs), []byte("response_body")) {
					t.Error("Expected response_body in logs")
				}
			} else {
				if bytes.Contains([]byte(logs), []byte("response_body")) {
					t.Error("Did not expect response_body in logs")
				}
			}

			// Verify security audit trail for non-GET requests
			if bytes.Contains([]byte(logs), []byte("Security audit")) {
				// Good - security audit was logged for POST request
			} else {
				t.Error("Expected security audit log for POST request")
			}
		})
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	tests := []struct {
		name      string
		logBodies bool
		maxSize   int64
		writeData []string
		expectLog string
	}{
		{
			name:      "capture full response",
			logBodies: true,
			maxSize:   100,
			writeData: []string{"Hello", " ", "World"},
			expectLog: "Hello World",
		},
		{
			name:      "truncate large response",
			logBodies: true,
			maxSize:   5,
			writeData: []string{"Hello", " ", "World"},
			expectLog: "Hello",
		},
		{
			name:      "no capture when disabled",
			logBodies: false,
			maxSize:   100,
			writeData: []string{"Hello", " ", "World"},
			expectLog: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test response writer
			recorder := httptest.NewRecorder()

			// Create logging response writer
			lrw := &loggingResponseWriter{
				ResponseWriter: recorder,
				statusCode:     http.StatusOK,
				logBodies:      tt.logBodies,
				maxSize:        tt.maxSize,
			}

			// Write data
			for _, data := range tt.writeData {
				if _, err := lrw.Write([]byte(data)); err != nil {
					t.Logf("Failed to write data: %v", err)
				}
			}

			// Check captured body
			if string(lrw.body) != tt.expectLog {
				t.Errorf("Expected captured body '%s', got '%s'", tt.expectLog, string(lrw.body))
			}

			// Check actual response
			fullResponse := recorder.Body.String()
			expectedFull := ""
			for _, data := range tt.writeData {
				expectedFull += data
			}
			if fullResponse != expectedFull {
				t.Errorf("Expected full response '%s', got '%s'", expectedFull, fullResponse)
			}
		})
	}
}

func TestSecurityAuditLogging(t *testing.T) {
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).With().Timestamp().Logger()

	transport := &HTTPTransport{
		logger:         logger,
		logBodies:      false,
		maxBodyLogSize: 1024,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			t.Logf("Failed to encode response: %v", err)
		}
	})

	wrappedHandler := transport.loggingMiddleware(handler)

	tests := []struct {
		method      string
		path        string
		status      int
		expectAudit bool
	}{
		{"GET", "/test", http.StatusOK, false},
		{"POST", "/test", http.StatusOK, true},
		{"PUT", "/test", http.StatusOK, true},
		{"DELETE", "/test", http.StatusOK, true},
		{"GET", "/error", http.StatusBadRequest, true},
		{"POST", "/error", http.StatusBadRequest, true},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			logBuf.Reset()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(recorder, req)

			logs := logBuf.String()
			hasAudit := bytes.Contains([]byte(logs), []byte("Security audit"))

			if tt.expectAudit && !hasAudit {
				t.Error("Expected security audit log")
			} else if !tt.expectAudit && hasAudit {
				t.Error("Did not expect security audit log")
			}
		})
	}
}
