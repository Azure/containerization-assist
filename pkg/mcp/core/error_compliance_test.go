package core

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// TestBuildImageParamsValidation tests that BuildImageParams.Validate uses RichError
func TestBuildImageParamsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		params    BuildImageParams
		expectErr bool
		errorCode errors.ErrorCode
	}{
		{
			name: "valid parameters",
			params: BuildImageParams{
				SessionID:      "test-session",
				DockerfilePath: "Dockerfile",
				ImageName:      "test:latest",
			},
			expectErr: false,
		},
		{
			name: "missing session_id",
			params: BuildImageParams{
				DockerfilePath: "Dockerfile",
				ImageName:      "test:latest",
			},
			expectErr: true,
			errorCode: errors.CodeMissingParameter,
		},
		{
			name: "missing dockerfile_path",
			params: BuildImageParams{
				SessionID: "test-session",
				ImageName: "test:latest",
			},
			expectErr: true,
			errorCode: errors.CodeMissingParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.params.Validate()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}

				richErr, ok := err.(*errors.RichError)
				if !ok {
					t.Errorf("Expected *errors.RichError, got %T", err)
					return
				}

				if richErr.Code != tt.errorCode {
					t.Errorf("Expected error code %s, got %s", tt.errorCode, richErr.Code)
				}

				if richErr.Type != errors.ErrTypeValidation {
					t.Errorf("Expected error type %s, got %s", errors.ErrTypeValidation, richErr.Type)
				}

				if richErr.Severity != errors.SeverityMedium {
					t.Errorf("Expected severity %s, got %s", errors.SeverityMedium, richErr.Severity)
				}

				if richErr.Location == nil {
					t.Error("Expected non-nil location in RichError")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestDeployParamsValidation tests that DeployParams.Validate uses RichError
func TestDeployParamsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		params    DeployParams
		expectErr bool
		errorCode errors.ErrorCode
	}{
		{
			name: "valid parameters",
			params: DeployParams{
				SessionID:     "test-session",
				ManifestPaths: []string{"manifest.yaml"},
				Namespace:     "default",
			},
			expectErr: false,
		},
		{
			name: "missing session_id",
			params: DeployParams{
				ManifestPaths: []string{"manifest.yaml"},
				Namespace:     "default",
			},
			expectErr: true,
			errorCode: errors.CodeMissingParameter,
		},
		{
			name: "empty manifest_paths",
			params: DeployParams{
				SessionID: "test-session",
				Namespace: "default",
			},
			expectErr: true,
			errorCode: errors.CodeMissingParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.params.Validate()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}

				richErr, ok := err.(*errors.RichError)
				if !ok {
					t.Errorf("Expected *errors.RichError, got %T", err)
					return
				}

				if richErr.Code != tt.errorCode {
					t.Errorf("Expected error code %s, got %s", tt.errorCode, richErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestConsolidatedScanParamsValidation tests that ConsolidatedScanParams.Validate uses RichError
func TestConsolidatedScanParamsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		params    ConsolidatedScanParams
		expectErr bool
		errorCode errors.ErrorCode
	}{
		{
			name: "valid parameters",
			params: ConsolidatedScanParams{
				SessionID: "test-session",
				ImageRef:  "nginx:latest",
			},
			expectErr: false,
		},
		{
			name: "valid with image_ref only",
			params: ConsolidatedScanParams{
				ImageRef: "nginx:latest",
			},
			expectErr: false,
		},
		{
			name: "missing image_ref",
			params: ConsolidatedScanParams{
				SessionID: "test-session",
			},
			expectErr: true,
			errorCode: errors.CodeMissingParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.params.Validate()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}

				richErr, ok := err.(*errors.RichError)
				if !ok {
					t.Errorf("Expected *errors.RichError, got %T", err)
					return
				}

				if richErr.Code != tt.errorCode {
					t.Errorf("Expected error code %s, got %s", tt.errorCode, richErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// mockToolOutput is a mock implementation of ToolOutput
type mockToolOutput struct {
	success bool
	data    interface{}
}

func (m *mockToolOutput) IsSuccess() bool {
	return m.success
}

func (m *mockToolOutput) GetData() interface{} {
	return m.data
}

// TestRegistryErrorCompliance tests that registry operations use RichError
func TestRegistryErrorCompliance(t *testing.T) {
	t.Parallel()
	registry := NewUnifiedRegistry(testLogger())

	mockTool := &mockUnifiedTool{name: "", description: ""}
	err := registry.Register(mockTool)

	if err == nil {
		t.Error("Expected error for empty tool name")
		return
	}

	richErr, ok := err.(*errors.RichError)
	if !ok {
		t.Errorf("Expected *errors.RichError, got %T", err)
		return
	}

	if richErr.Code != errors.CodeMissingParameter {
		t.Errorf("Expected code %s, got %s", errors.CodeMissingParameter, richErr.Code)
	}

	if richErr.Type != errors.ErrTypeValidation {
		t.Errorf("Expected type %s, got %s", errors.ErrTypeValidation, richErr.Type)
	}

	if richErr.Location == nil {
		t.Error("Expected non-nil location")
	}

	validTool := &mockUnifiedTool{name: "test-tool", description: "Test tool"}
	err = registry.Register(validTool)
	if err != nil {
		t.Fatalf("Failed to register valid tool: %v", err)
	}

	err = registry.Register(validTool)
	if err == nil {
		t.Error("Expected error for duplicate tool registration")
		return
	}

	richErr, ok = err.(*errors.RichError)
	if !ok {
		t.Errorf("Expected *errors.RichError, got %T", err)
		return
	}

	if richErr.Code != errors.CodeToolAlreadyRegistered {
		t.Errorf("Expected code %s, got %s", errors.CodeToolAlreadyRegistered, richErr.Code)
	}

	_, err = registry.Get("nonexistent-tool")
	if err == nil {
		t.Error("Expected error for nonexistent tool")
		return
	}

	richErr, ok = err.(*errors.RichError)
	if !ok {
		t.Errorf("Expected *errors.RichError, got %T", err)
		return
	}

	if richErr.Code != errors.CodeToolNotFound {
		t.Errorf("Expected code %s, got %s", errors.CodeToolNotFound, richErr.Code)
	}

	if richErr.Context == nil || len(richErr.Context) == 0 {
		t.Error("Expected non-empty context in tool not found error")
	}

	if len(richErr.Suggestions) == 0 {
		t.Error("Expected suggestions in tool not found error")
	}
}

// mockUnifiedTool is a mock implementation for testing
type mockUnifiedTool struct {
	name        string
	description string
}

func (m *mockUnifiedTool) Name() string {
	return m.name
}

func (m *mockUnifiedTool) Description() string {
	return m.description
}

func (m *mockUnifiedTool) Execute(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "mock success"},
	}, nil
}

func (m *mockUnifiedTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
		Version:     "1.0.0",
	}
}

// testLogger returns a test logger instance
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
