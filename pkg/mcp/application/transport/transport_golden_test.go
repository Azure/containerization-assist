package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var updateGolden = flag.Bool("update-golden", false, "Update golden files")

// StubTransport is a simple transport implementation for golden tests
type StubTransport struct{}

func (s *StubTransport) Serve(ctx context.Context, srv *server.MCPServer) error {
	// Simple stub that returns immediately
	return nil
}

// TestRegistryGoldenOutput tests registry behavior against golden files
func TestRegistryGoldenOutput(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Registry)
		operation func(*Registry) interface{}
		format    string // "json" or "yaml"
	}{
		{
			name: "registry_state_empty",
			setup: func(r *Registry) {
				// Empty registry
			},
			operation: func(r *Registry) interface{} {
				return captureRegistryState(r)
			},
			format: "json",
		},
		{
			name: "registry_state_with_transports",
			setup: func(r *Registry) {
				r.Register(TransportTypeStdio, &StubTransport{})
				r.Register(TransportTypeHTTP, &StubTransport{})
				r.Register("custom", &StubTransport{})
			},
			operation: func(r *Registry) interface{} {
				return captureRegistryState(r)
			},
			format: "json",
		},
		{
			name: "transport_lifecycle_events",
			setup: func(r *Registry) {
				// Use a simple stub transport that always returns nil
				r.Register(TransportTypeStdio, &StubTransport{})
			},
			operation: func(r *Registry) interface{} {
				events := []LifecycleEvent{}
				ctx := context.Background()
				mockServer := &server.MCPServer{}

				// Capture registration event
				events = append(events, LifecycleEvent{
					Type:      "registered",
					Transport: string(TransportTypeStdio),
					Success:   true,
				})

				// Attempt to start
				err := r.Start(ctx, TransportTypeStdio, mockServer)
				events = append(events, LifecycleEvent{
					Type:      "start_attempted",
					Transport: string(TransportTypeStdio),
					Success:   err == nil,
					Error:     errorString(err),
				})

				// Attempt to start unregistered transport
				err = r.Start(ctx, "unregistered", mockServer)
				events = append(events, LifecycleEvent{
					Type:      "start_attempted",
					Transport: "unregistered",
					Success:   err == nil,
					Error:     errorString(err),
				})

				return events
			},
			format: "yaml",
		},
		{
			name: "transport_error_scenarios",
			setup: func(r *Registry) {
				// Register various transports
				r.Register(TransportTypeStdio, &StubTransport{})
			},
			operation: func(r *Registry) interface{} {
				scenarios := []ErrorScenario{}
				ctx := context.Background()
				mockServer := &server.MCPServer{}

				// Test unsupported transport
				err := r.Start(ctx, "completely_unknown", mockServer)
				scenarios = append(scenarios, ErrorScenario{
					Name:        "unsupported_transport",
					Transport:   "completely_unknown",
					ErrorType:   classifyError(err),
					Message:     err.Error(),
					Recoverable: isRecoverable(err),
				})

				// Test empty transport type
				err = r.Start(ctx, "", mockServer)
				scenarios = append(scenarios, ErrorScenario{
					Name:        "empty_transport_type",
					Transport:   "",
					ErrorType:   classifyError(err),
					Message:     errorString(err),
					Recoverable: isRecoverable(err),
				})

				return scenarios
			},
			format: "json",
		},
		{
			name: "concurrent_registration_order",
			setup: func(r *Registry) {
				// Empty to start
			},
			operation: func(r *Registry) interface{} {
				// Register multiple transports concurrently
				done := make(chan string, 10)

				for i := 0; i < 10; i++ {
					go func(idx int) {
						name := fmt.Sprintf("transport_%d", idx)
						r.Register(TransportType(name), &StubTransport{})
						done <- name
					}(i)
				}

				// Collect registration order
				order := []string{}
				for i := 0; i < 10; i++ {
					order = append(order, <-done)
				}

				// Return deterministic view of final state
				return map[string]interface{}{
					"total_registered": len(captureRegistryState(r).RegisteredTransports),
					"all_registered":   len(order) == 10,
				}
			},
			format: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create registry with test logger
			logBuffer := &bytes.Buffer{}
			logger := slog.New(slog.NewTextHandler(logBuffer, nil))
			registry := NewRegistry(logger)

			// Setup
			tt.setup(registry)

			// Run operation
			result := tt.operation(registry)

			// Compare with golden file
			goldenPath := filepath.Join("testdata", "golden", tt.name+"."+tt.format)

			if *updateGolden {
				// Update golden file
				updateGoldenFile(t, goldenPath, result, tt.format)
			} else {
				// Compare with golden file
				compareWithGolden(t, goldenPath, result, tt.format)
			}
		})
	}
}

// TestTransportConfigGolden tests transport configuration serialization
func TestTransportConfigGolden(t *testing.T) {
	tests := []struct {
		name   string
		config interface{}
	}{
		{
			name: "stdio_config",
			config: StdioConfig{
				ReadBufferSize:  4096,
				WriteBufferSize: 4096,
				MaxMessageSize:  1048576,
			},
		},
		{
			name: "http_config",
			config: HTTPConfig{
				Port:           8080,
				Host:           "localhost",
				ReadTimeout:    30,
				WriteTimeout:   30,
				MaxHeaderBytes: 1048576,
				MaxRequestSize: 10485760,
				EnableCORS:     true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
			},
		},
		{
			name: "transport_metadata",
			config: TransportMetadata{
				Type:         "stdio",
				Version:      "1.0.0",
				Capabilities: []string{"streaming", "bidirectional", "progress"},
				Limits: map[string]int{
					"max_connections":  1,
					"max_message_size": 1048576,
					"buffer_size":      4096,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goldenPath := filepath.Join("testdata", "golden", "config_"+tt.name+".json")

			if *updateGolden {
				updateGoldenFile(t, goldenPath, tt.config, "json")
			} else {
				compareWithGolden(t, goldenPath, tt.config, "json")
			}
		})
	}
}

// Helper types for golden tests
type RegistryState struct {
	RegisteredTransports []string `json:"registered_transports"`
	TransportCount       int      `json:"transport_count"`
}

type LifecycleEvent struct {
	Type      string `yaml:"type"`
	Transport string `yaml:"transport"`
	Success   bool   `yaml:"success"`
	Error     string `yaml:"error,omitempty"`
}

type ErrorScenario struct {
	Name        string `json:"name"`
	Transport   string `json:"transport"`
	ErrorType   string `json:"error_type"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

type StdioConfig struct {
	ReadBufferSize  int `json:"read_buffer_size"`
	WriteBufferSize int `json:"write_buffer_size"`
	MaxMessageSize  int `json:"max_message_size"`
}

type HTTPConfig struct {
	Port           int      `json:"port"`
	Host           string   `json:"host"`
	ReadTimeout    int      `json:"read_timeout_seconds"`
	WriteTimeout   int      `json:"write_timeout_seconds"`
	MaxHeaderBytes int      `json:"max_header_bytes"`
	MaxRequestSize int64    `json:"max_request_size"`
	EnableCORS     bool     `json:"enable_cors"`
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
}

type TransportMetadata struct {
	Type         string         `json:"type"`
	Version      string         `json:"version"`
	Capabilities []string       `json:"capabilities"`
	Limits       map[string]int `json:"limits"`
}

// Helper functions
func captureRegistryState(r *Registry) RegistryState {
	// Get all transports from the registry
	allTransports := r.transports.All()

	transports := []string{}
	for name := range allTransports {
		transports = append(transports, name)
	}

	// Sort for deterministic output
	sort.Strings(transports)

	return RegistryState{
		RegisteredTransports: transports,
		TransportCount:       len(transports),
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	switch err {
	case context.Canceled:
		return "canceled"
	case context.DeadlineExceeded:
		return "timeout"
	default:
		if err.Error() == "unsupported transport type" {
			return "unsupported"
		}
		return "generic"
	}
}

func isRecoverable(err error) bool {
	if err == nil {
		return true
	}
	return err != context.Canceled && err != context.DeadlineExceeded
}

func updateGoldenFile(t *testing.T, path string, data interface{}, format string) {
	t.Helper()

	// Ensure directory exists
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err, "Failed to create golden directory")

	// Marshal data
	var content []byte
	switch format {
	case "json":
		content, err = json.MarshalIndent(data, "", "  ")
	case "yaml":
		content, err = yaml.Marshal(data)
	default:
		t.Fatalf("Unknown format: %s", format)
	}
	require.NoError(t, err, "Failed to marshal data")

	// Write file
	err = os.WriteFile(path, content, 0644)
	require.NoError(t, err, "Failed to write golden file")

	t.Logf("Updated golden file: %s", path)
}

func compareWithGolden(t *testing.T, path string, actual interface{}, format string) {
	t.Helper()

	// Read golden file
	golden, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("Golden file does not exist: %s\nRun with -update-golden to create it", path)
	}
	require.NoError(t, err, "Failed to read golden file")

	// Marshal actual data
	var actualContent []byte
	switch format {
	case "json":
		actualContent, err = json.MarshalIndent(actual, "", "  ")
	case "yaml":
		actualContent, err = yaml.Marshal(actual)
	default:
		t.Fatalf("Unknown format: %s", format)
	}
	require.NoError(t, err, "Failed to marshal actual data")

	// Compare
	assert.Equal(t, string(golden), string(actualContent),
		"Output does not match golden file.\nRun with -update-golden to update the golden file.")
}

// Benchmarks for golden test operations
func BenchmarkGoldenComparison(b *testing.B) {
	// Setup test data
	state := RegistryState{
		RegisteredTransports: []string{"stdio", "http", "websocket", "grpc"},
		TransportCount:       4,
	}

	// Marshal once for comparison
	golden, _ := json.MarshalIndent(state, "", "  ")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		actual, _ := json.MarshalIndent(state, "", "  ")
		_ = bytes.Equal(golden, actual)
	}
}
