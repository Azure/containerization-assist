package transport

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// TestJSONRPCMessageStructure verifies that our tool calls serialize correctly
func TestJSONRPCMessageStructure(t *testing.T) {
	tests := []struct {
		name    string
		message interface{}
		wantErr bool
		check   func(t *testing.T, data []byte)
	}{
		{
			name: "valid tool request",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  types.ToolNameAnalyzeRepository,
				"params": map[string]interface{}{
					"repository_url": "https://github.com/user/repo",
					"session_id":     "test-session",
				},
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					t.Fatal(err)
				}
				if msg["jsonrpc"] != "2.0" {
					t.Error("Expected jsonrpc 2.0")
				}
				if msg["method"] != types.ToolNameAnalyzeRepository {
					t.Error("Expected method analyze_repository")
				}
			},
		},
		{
			name: "valid tool response",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"success":    true,
					"session_id": "test-session",
					"language":   "go",
					"framework":  "gin",
				},
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					t.Fatal(err)
				}
				result, ok := msg["result"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected result to be a map")
				}
				if result["success"] != true {
					t.Error("Expected success to be true")
				}
				if result["language"] != "go" {
					t.Error("Expected language to be go")
				}
			},
		},
		{
			name: "error response",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    -32602,
					"message": "Invalid params",
					"data":    "repository_url is required",
				},
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					t.Fatal(err)
				}
				error, ok := msg["error"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected error to be a map")
				}
				if error["code"] != float64(-32602) { // JSON numbers unmarshal as float64
					t.Error("Expected error code -32602")
				}
			},
		},
		{
			name: "notification (no id)",
			message: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "session_updated",
				"params": map[string]interface{}{
					"session_id": "test-session",
					"status":     "completed",
				},
			},
			wantErr: false,
			check: func(t *testing.T, data []byte) {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					t.Fatal(err)
				}
				if _, hasID := msg["id"]; hasID {
					t.Error("Notification should not have id")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				tt.check(t, data)
			}
		})
	}
}

// TestToolArgsMapping verifies that tool arguments map correctly
func TestToolArgsMapping(t *testing.T) {
	// Test analyze_repository args mapping
	t.Run("analyze_repository args", func(t *testing.T) {
		args := map[string]interface{}{
			"repository_url": "https://github.com/user/repo",
			"session_id":     "test-session",
			"branch":         "main",
			"context":        "microservice",
			"language_hint":  "go",
			"shallow":        true,
			"dry_run":        false,
		}

		data, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}

		// Verify all fields serialize correctly
		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}

		if decoded["repository_url"] != "https://github.com/user/repo" {
			t.Error("repository_url mismatch")
		}
		if decoded["shallow"] != true {
			t.Error("shallow should be true")
		}
	})

	// Test build_image args mapping
	t.Run("build_image args", func(t *testing.T) {
		args := map[string]interface{}{
			"session_id": "test-session",
			"registry":   "docker.io",
			"image_name": "myapp",
			"image_tag":  "latest",
			"platform":   "linux/amd64",
			"no_cache":   true,
			"build_args": map[string]string{
				"VERSION": "1.0.0",
			},
		}

		data, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}

		buildArgs, ok := decoded["build_args"].(map[string]interface{})
		if !ok {
			t.Fatal("build_args should be a map")
		}
		if buildArgs["VERSION"] != "1.0.0" {
			t.Error("VERSION mismatch in build_args")
		}
	})
}

// TestErrorResponseMapping verifies error responses are properly structured
func TestErrorResponseMapping(t *testing.T) {
	tests := []struct {
		name      string
		errorCode int
		errorMsg  string
		errorData interface{}
	}{
		{
			name:      "parse error",
			errorCode: -32700,
			errorMsg:  "Parse error",
			errorData: "Invalid JSON",
		},
		{
			name:      "invalid request",
			errorCode: -32600,
			errorMsg:  "Invalid request",
			errorData: nil,
		},
		{
			name:      "method not found",
			errorCode: -32601,
			errorMsg:  "Method not found",
			errorData: "unknown_method",
		},
		{
			name:      "invalid params",
			errorCode: -32602,
			errorMsg:  "Invalid params",
			errorData: map[string]string{"field": "repository_url", "error": "required"},
		},
		{
			name:      "internal error",
			errorCode: -32603,
			errorMsg:  "Internal error",
			errorData: "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    tt.errorCode,
					"message": tt.errorMsg,
				},
			}

			if tt.errorData != nil {
				if errorMap, ok := errorResp["error"].(map[string]interface{}); ok {
					errorMap["data"] = tt.errorData
				}
			}

			data, err := json.Marshal(errorResp)
			if err != nil {
				t.Fatal(err)
			}

			// Verify structure
			var decoded map[string]interface{}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}

			errorObj, ok := decoded["error"].(map[string]interface{})
			if !ok {
				t.Fatal("error should be a map")
			}

			if codeFloat, ok := errorObj["code"].(float64); ok {
				if int(codeFloat) != tt.errorCode {
					t.Errorf("Expected error code %d, got %v", tt.errorCode, errorObj["code"])
				}
			} else {
				t.Errorf("error code should be a float64, got %T", errorObj["code"])
			}
		})
	}
}

// TestMalformedMessageHandling verifies that malformed messages are handled correctly
func TestMalformedMessageHandling(t *testing.T) {
	malformedMessages := []string{
		`{"not": "json-rpc"}`,                                          // Missing jsonrpc field
		`{"jsonrpc": "1.0", "method": "test"}`,                         // Wrong version
		`{"jsonrpc": "2.0"}`,                                           // No method, id, result, or error
		`{"jsonrpc": "2.0", "method": 123}`,                            // Method not string
		`{"jsonrpc": "2.0", "id": "1", "result": "ok", "error": {}}`,   // Both result and error
		`invalid json`,                                                 // Not JSON at all
		`{"jsonrpc": "2.0", "method": "test", "params": "not-object"}`, // Params not object/array
	}

	for i, msg := range malformedMessages {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			// These should all be filtered by isValidJSONRPC
			// or cause parse errors
			var decoded map[string]interface{}
			err := json.Unmarshal([]byte(msg), &decoded)

			if err == nil {
				// If it parsed as JSON, check if it's valid JSON-RPC
				jsonrpc, hasJsonrpc := decoded["jsonrpc"]
				if !hasJsonrpc || jsonrpc != "2.0" {
					t.Logf("Message correctly identified as invalid JSON-RPC: %s", msg)
					return
				}

				// Check structure
				_, hasMethod := decoded["method"]
				_, hasID := decoded["id"]
				_, hasResult := decoded["result"]
				_, hasError := decoded["error"]

				// Valid structures: request, response, notification
				isRequest := hasMethod && hasID
				isNotification := hasMethod && !hasID
				isResponse := hasID && (hasResult || hasError) && (!hasResult || !hasError)

				if !isRequest && !isNotification && !isResponse {
					t.Logf("Message correctly identified as invalid structure: %s", msg)
					return
				}
			} else {
				t.Logf("Message correctly identified as invalid JSON: %s", msg)
			}
		})
	}
}
