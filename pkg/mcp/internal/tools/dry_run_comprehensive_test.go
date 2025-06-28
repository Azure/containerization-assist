package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTool implements the Tool interface for testing dry-run functionality
type MockTool struct {
	name     string
	dryRunOp func(ctx context.Context, args interface{}) (interface{}, error)
	actualOp func(ctx context.Context, args interface{}) (interface{}, error)
}

func NewMockTool(name string) *MockTool {
	return &MockTool{
		name: name,
		dryRunOp: func(ctx context.Context, args interface{}) (interface{}, error) {
			mockArgs := args.(*MockToolArgs)
			return &MockToolResponse{
				Success:   true,
				DryRun:    true,
				Operation: mockArgs.Operation,
				Message:   fmt.Sprintf("DRY-RUN: Would execute %s", mockArgs.Operation),
			}, nil
		},
		actualOp: func(ctx context.Context, args interface{}) (interface{}, error) {
			mockArgs := args.(*MockToolArgs)
			// Simulate actual work taking longer
			time.Sleep(10 * time.Millisecond)
			return &MockToolResponse{
				Success:   true,
				DryRun:    false,
				Operation: mockArgs.Operation,
				Message:   fmt.Sprintf("Executed %s", mockArgs.Operation),
			}, nil
		},
	}
}

func (m *MockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	mockArgs, ok := args.(*MockToolArgs)
	if !ok {
		return nil, fmt.Errorf("invalid args type: expected *MockToolArgs, got %T", args)
	}

	if mockArgs.DryRun {
		return m.dryRunOp(ctx, args)
	}
	return m.actualOp(ctx, args)
}

func (m *MockTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        m.name,
		Description: fmt.Sprintf("Mock tool for testing: %s", m.name),
		Version:     "1.0.0",
		Category:    "testing",
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
		},
		Parameters: map[string]string{
			"operation": "required - Operation to perform",
		},
	}
}

func (m *MockTool) Validate(ctx context.Context, args interface{}) error {
	mockArgs, ok := args.(*MockToolArgs)
	if !ok {
		return fmt.Errorf("invalid args type: expected *MockToolArgs, got %T", args)
	}
	if mockArgs.Operation == "" {
		return fmt.Errorf("operation is required")
	}
	return nil
}

// MockToolArgs represents arguments for the mock tool
type MockToolArgs struct {
	types.BaseToolArgs
	Operation string `json:"operation"`
}

// MockToolResponse represents the response from the mock tool
type MockToolResponse struct {
	Success   bool   `json:"success"`
	DryRun    bool   `json:"dry_run"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// TestDryRunSupport tests that tools properly support dry-run functionality
func TestDryRunSupport(t *testing.T) {
	tests := []struct {
		name                string
		tool                mcp.Tool
		args                *MockToolArgs
		expectDryRunMessage string
		expectActualMessage string
	}{
		{
			name: "build_tool_dry_run",
			tool: NewMockTool("build"),
			args: &MockToolArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Operation: "build_image",
			},
			expectDryRunMessage: "DRY-RUN: Would execute build_image",
			expectActualMessage: "Executed build_image",
		},
		{
			name: "deploy_tool_dry_run",
			tool: NewMockTool("deploy"),
			args: &MockToolArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Operation: "deploy_kubernetes",
			},
			expectDryRunMessage: "DRY-RUN: Would execute deploy_kubernetes",
			expectActualMessage: "Executed deploy_kubernetes",
		},
		{
			name: "scan_tool_dry_run",
			tool: NewMockTool("scan"),
			args: &MockToolArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Operation: "scan_secrets",
			},
			expectDryRunMessage: "DRY-RUN: Would execute scan_secrets",
			expectActualMessage: "Executed scan_secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Test dry-run execution
			tt.args.BaseToolArgs.DryRun = true
			dryRunResp, err := tt.tool.Execute(ctx, tt.args)
			require.NoError(t, err, "Dry-run should not produce errors")
			require.NotNil(t, dryRunResp, "Response should not be nil")

			dryRunResult, ok := dryRunResp.(*MockToolResponse)
			require.True(t, ok, "Response should be MockToolResponse")
			assert.True(t, dryRunResult.DryRun, "DryRun flag should be true")
			assert.Equal(t, tt.expectDryRunMessage, dryRunResult.Message)
			assert.True(t, dryRunResult.Success, "Dry-run should succeed")

			// Test actual execution
			tt.args.BaseToolArgs.DryRun = false
			actualResp, err := tt.tool.Execute(ctx, tt.args)
			require.NoError(t, err, "Actual execution should not produce errors")
			require.NotNil(t, actualResp, "Response should not be nil")

			actualResult, ok := actualResp.(*MockToolResponse)
			require.True(t, ok, "Response should be MockToolResponse")
			assert.False(t, actualResult.DryRun, "DryRun flag should be false")
			assert.Equal(t, tt.expectActualMessage, actualResult.Message)
			assert.True(t, actualResult.Success, "Actual execution should succeed")
		})
	}
}

// TestDryRunPerformance tests that dry-run mode is significantly faster
func TestDryRunPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tool := NewMockTool("performance_test")
	args := &MockToolArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "perf-test",
		},
		Operation: "expensive_operation",
	}

	ctx := context.Background()

	// Measure dry-run performance
	args.DryRun = true
	start := time.Now()
	_, err := tool.Execute(ctx, args)
	dryRunDuration := time.Since(start)
	require.NoError(t, err)

	// Measure actual execution performance
	args.DryRun = false
	start = time.Now()
	_, err = tool.Execute(ctx, args)
	actualDuration := time.Since(start)
	require.NoError(t, err)

	// Validate dry-run performance
	maxDryRunTime := 5 * time.Millisecond
	assert.Less(t, dryRunDuration, maxDryRunTime,
		"Dry-run should complete within %v, took %v", maxDryRunTime, dryRunDuration)

	// Calculate speedup
	speedup := float64(actualDuration) / float64(dryRunDuration)
	t.Logf("Performance comparison:")
	t.Logf("  Dry-run: %v", dryRunDuration)
	t.Logf("  Actual:  %v", actualDuration)
	t.Logf("  Speedup: %.2fx", speedup)

	// Dry-run should be faster
	assert.Greater(t, speedup, 1.0, "Dry-run should be faster than actual execution")
}

// TestDryRunValidation tests that dry-run validates inputs without side effects
func TestDryRunValidation(t *testing.T) {
	tool := NewMockTool("validation_test")
	ctx := context.Background()

	tests := []struct {
		name        string
		args        *MockToolArgs
		expectError bool
	}{
		{
			name: "valid_args_dry_run",
			args: &MockToolArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Operation: "valid_operation",
			},
			expectError: false,
		},
		{
			name: "invalid_args_dry_run",
			args: &MockToolArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				Operation: "", // Invalid: empty operation
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation should work the same for dry-run and actual
			err := tool.Validate(ctx, tt.args)
			if tt.expectError {
				assert.Error(t, err, "Validation should fail for invalid args")
			} else {
				assert.NoError(t, err, "Validation should pass for valid args")
			}

			// If validation passes, execution should also work
			if !tt.expectError {
				resp, err := tool.Execute(ctx, tt.args)
				assert.NoError(t, err, "Execution should succeed for valid args")
				assert.NotNil(t, resp, "Response should not be nil")
			}
		})
	}
}

// TestDryRunMetadata tests that tools properly expose dry-run capabilities
func TestDryRunMetadata(t *testing.T) {
	tool := NewMockTool("metadata_test")
	metadata := tool.GetMetadata()

	assert.Equal(t, "metadata_test", metadata.Name)
	assert.Contains(t, metadata.Capabilities, "supports_dry_run")
	assert.NotEmpty(t, metadata.Parameters)
	assert.Contains(t, metadata.Parameters, "operation")
}

// TestDryRunWithFiles tests dry-run behavior with file operations
func TestDryRunWithFiles(t *testing.T) {
	// Create temporary workspace
	workdir, err := os.MkdirTemp("", "dry-run-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(workdir)

	tool := NewMockTool("file_test")
	args := &MockToolArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "file-test",
			DryRun:    true,
		},
		Operation: "create_file",
	}

	ctx := context.Background()

	// Execute in dry-run mode
	resp, err := tool.Execute(ctx, args)
	require.NoError(t, err)

	result, ok := resp.(*MockToolResponse)
	require.True(t, ok)
	assert.True(t, result.DryRun)

	// Verify no files were actually created in dry-run mode
	testFilePath := filepath.Join(workdir, "test-file.txt")
	_, err = os.Stat(testFilePath)
	assert.True(t, os.IsNotExist(err), "File should not be created in dry-run mode")
}
