package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/container-copilot/pkg/mcp"
	"github.com/Azure/container-copilot/pkg/mcp/types"
)

// FileProcessorTool demonstrates rich error handling patterns
type FileProcessorTool struct {
	name string
}

// NewFileProcessorTool creates a new instance
func NewFileProcessorTool() *FileProcessorTool {
	return &FileProcessorTool{
		name: "file_processor",
	}
}

// Execute demonstrates various error scenarios and handling
func (t *FileProcessorTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	toolArgs, ok := args.(*FileProcessorArgs)
	if !ok {
		// Type assertion error
		return nil, types.NewRichError(
			"INVALID_ARGS_TYPE",
			"Invalid arguments type provided",
			fmt.Errorf("expected *FileProcessorArgs, got %T", args),
		).WithContext(map[string]interface{}{
			"tool":          t.name,
			"expected_type": "*FileProcessorArgs",
			"actual_type":   fmt.Sprintf("%T", args),
		})
	}

	// Simulate different error scenarios based on input
	switch toolArgs.Scenario {
	case "missing_file":
		return nil, t.handleMissingFileError(toolArgs.FilePath)
	
	case "permission_denied":
		return nil, t.handlePermissionError(toolArgs.FilePath)
	
	case "invalid_format":
		return nil, t.handleFormatError(toolArgs.FilePath, "json")
	
	case "timeout":
		return nil, t.handleTimeoutError(toolArgs.FilePath, 30)
	
	case "dependency_missing":
		return nil, t.handleDependencyError("jq", "1.6")
	
	case "success":
		return &FileProcessorResult{
			Status:       "success",
			FilePath:     toolArgs.FilePath,
			LinesRead:    100,
			BytesRead:    2048,
			ProcessingMs: 150,
		}, nil
	
	default:
		return nil, types.NewRichError(
			"UNKNOWN_SCENARIO",
			fmt.Sprintf("Unknown test scenario: %s", toolArgs.Scenario),
			nil,
		).WithContext(map[string]interface{}{
			"tool":     t.name,
			"scenario": toolArgs.Scenario,
			"hint":     "Valid scenarios: missing_file, permission_denied, invalid_format, timeout, dependency_missing, success",
		})
	}
}

// Error handling methods demonstrate different error patterns

func (t *FileProcessorTool) handleMissingFileError(filePath string) error {
	return types.NewRichError(
		"FILE_NOT_FOUND",
		fmt.Sprintf("File not found: %s", filePath),
		nil,
	).WithContext(map[string]interface{}{
		"tool":      t.name,
		"file_path": filePath,
		"phase":     "validation",
	}).WithRecovery([]string{
		"Verify the file path is correct",
		"Check if the file exists in the expected location",
		"Ensure you have read permissions for the directory",
	})
}

func (t *FileProcessorTool) handlePermissionError(filePath string) error {
	return types.NewRichError(
		"PERMISSION_DENIED",
		fmt.Sprintf("Permission denied accessing file: %s", filePath),
		nil,
	).WithContext(map[string]interface{}{
		"tool":      t.name,
		"file_path": filePath,
		"phase":     "file_access",
		"required":  "read",
	}).WithRecovery([]string{
		"Check file permissions with: ls -la " + filePath,
		"Ensure the user has read access",
		"Try running with appropriate permissions",
	})
}

func (t *FileProcessorTool) handleFormatError(filePath string, expectedFormat string) error {
	return types.NewRichError(
		"INVALID_FORMAT",
		"File format does not match expected format",
		nil,
	).WithContext(map[string]interface{}{
		"tool":            t.name,
		"file_path":       filePath,
		"expected_format": expectedFormat,
		"detected_format": "unknown",
		"phase":           "parsing",
	}).WithRecovery([]string{
		fmt.Sprintf("Ensure the file is valid %s format", expectedFormat),
		"Validate the file with a format checker",
		"Check for syntax errors in the file",
	})
}

func (t *FileProcessorTool) handleTimeoutError(filePath string, timeoutSeconds int) error {
	return types.NewRichError(
		"OPERATION_TIMEOUT",
		fmt.Sprintf("Operation timed out after %d seconds", timeoutSeconds),
		nil,
	).WithContext(map[string]interface{}{
		"tool":            t.name,
		"file_path":       filePath,
		"timeout_seconds": timeoutSeconds,
		"phase":           "processing",
	}).WithRecovery([]string{
		"Try processing a smaller file",
		"Increase the timeout limit",
		"Check system resources (CPU, memory)",
	})
}

func (t *FileProcessorTool) handleDependencyError(dependency string, requiredVersion string) error {
	return types.NewRichError(
		"DEPENDENCY_MISSING",
		fmt.Sprintf("Required dependency '%s' not found", dependency),
		nil,
	).WithContext(map[string]interface{}{
		"tool":             t.name,
		"dependency":       dependency,
		"required_version": requiredVersion,
		"phase":            "initialization",
	}).WithRecovery([]string{
		fmt.Sprintf("Install %s version %s or higher", dependency, requiredVersion),
		fmt.Sprintf("Run: apt-get install %s", dependency),
		"Check PATH environment variable",
	})
}

// GetMetadata provides tool information
func (t *FileProcessorTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "file_processor",
		Description: "Demonstrates rich error handling patterns",
		Version:     "1.0.0",
		Category:    "example",
		Requirements: []string{
			"file-access",
			"json-parser",
		},
		Parameters: map[string]string{
			"file_path": "required - Path to the file to process",
			"scenario":  "required - Test scenario to simulate",
		},
	}
}

// Validate checks input arguments
func (t *FileProcessorTool) Validate(ctx context.Context, args interface{}) error {
	toolArgs, ok := args.(*FileProcessorArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type")
	}

	if toolArgs.FilePath == "" {
		return types.NewRichError(
			"VALIDATION_FAILED",
			"file_path is required",
			nil,
		).WithContext(map[string]interface{}{
			"field": "file_path",
			"rule":  "required",
		})
	}

	if toolArgs.Scenario == "" {
		return types.NewRichError(
			"VALIDATION_FAILED",
			"scenario is required",
			nil,
		).WithContext(map[string]interface{}{
			"field":           "scenario",
			"rule":            "required",
			"valid_scenarios": []string{
				"missing_file",
				"permission_denied",
				"invalid_format",
				"timeout",
				"dependency_missing",
				"success",
			},
		})
	}

	return nil
}

// FileProcessorArgs defines input arguments
type FileProcessorArgs struct {
	FilePath string `json:"file_path" description:"Path to the file to process"`
	Scenario string `json:"scenario" description:"Test scenario to simulate"`
}

// FileProcessorResult defines the output
type FileProcessorResult struct {
	Status       string `json:"status"`
	FilePath     string `json:"file_path"`
	LinesRead    int    `json:"lines_read"`
	BytesRead    int    `json:"bytes_read"`
	ProcessingMs int    `json:"processing_ms"`
}

// Ensure compliance
var _ mcp.Tool = (*FileProcessorTool)(nil)

// Helper function to display rich errors
func displayRichError(err error) {
	if richErr, ok := err.(*types.RichError); ok {
		fmt.Printf("Error Code: %s\n", richErr.Code)
		fmt.Printf("Message: %s\n", richErr.Message)
		
		if richErr.Context != nil && len(richErr.Context) > 0 {
			fmt.Println("Context:")
			for k, v := range richErr.Context {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
		
		if richErr.Recovery != nil && len(richErr.Recovery) > 0 {
			fmt.Println("Recovery suggestions:")
			for i, suggestion := range richErr.Recovery {
				fmt.Printf("  %d. %s\n", i+1, suggestion)
			}
		}
		
		if richErr.Cause != nil {
			fmt.Printf("Underlying cause: %v\n", richErr.Cause)
		}
	} else {
		fmt.Printf("Standard error: %v\n", err)
	}
}

func main() {
	tool := NewFileProcessorTool()
	
	fmt.Println("=== Rich Error Handling Examples ===")
	fmt.Println()
	
	// Test scenarios
	scenarios := []struct {
		name string
		args *FileProcessorArgs
	}{
		{
			name: "Missing File Error",
			args: &FileProcessorArgs{
				FilePath: "/path/to/missing/file.json",
				Scenario: "missing_file",
			},
		},
		{
			name: "Permission Denied Error",
			args: &FileProcessorArgs{
				FilePath: "/etc/shadow",
				Scenario: "permission_denied",
			},
		},
		{
			name: "Invalid Format Error",
			args: &FileProcessorArgs{
				FilePath: "/tmp/data.json",
				Scenario: "invalid_format",
			},
		},
		{
			name: "Timeout Error",
			args: &FileProcessorArgs{
				FilePath: "/tmp/large_file.json",
				Scenario: "timeout",
			},
		},
		{
			name: "Dependency Missing Error",
			args: &FileProcessorArgs{
				FilePath: "/tmp/data.json",
				Scenario: "dependency_missing",
			},
		},
		{
			name: "Success Case",
			args: &FileProcessorArgs{
				FilePath: "/tmp/valid_file.json",
				Scenario: "success",
			},
		},
	}
	
	for i, scenario := range scenarios {
		fmt.Printf("Example %d: %s\n", i+1, scenario.name)
		fmt.Println(strings.Repeat("-", 50))
		
		// Validate
		if err := tool.Validate(context.Background(), scenario.args); err != nil {
			fmt.Println("Validation error:")
			displayRichError(err)
			fmt.Println()
			continue
		}
		
		// Execute
		result, err := tool.Execute(context.Background(), scenario.args)
		if err != nil {
			displayRichError(err)
		} else {
			if res, ok := result.(*FileProcessorResult); ok {
				fmt.Printf("✓ Success!\n")
				fmt.Printf("  Status: %s\n", res.Status)
				fmt.Printf("  Lines read: %d\n", res.LinesRead)
				fmt.Printf("  Bytes read: %d\n", res.BytesRead)
				fmt.Printf("  Processing time: %dms\n", res.ProcessingMs)
			}
		}
		fmt.Println()
	}
	
	// Example of validation error
	fmt.Println("Example: Validation Error")
	fmt.Println(strings.Repeat("-", 50))
	
	invalidArgs := &FileProcessorArgs{
		// Missing required fields
	}
	
	if err := tool.Validate(context.Background(), invalidArgs); err != nil {
		fmt.Println("Validation failed as expected:")
		displayRichError(err)
	}
}

// strings package helper
var strings = struct {
	Repeat func(s string, count int) string
}{
	Repeat: func(s string, count int) string {
		result := ""
		for i := 0; i < count; i++ {
			result += s
		}
		return result
	},
}