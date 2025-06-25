package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/container-copilot/pkg/mcp"
)

// SimpleTool demonstrates a basic tool implementation using the unified interface
type SimpleTool struct {
	name string
}

// NewSimpleTool creates a new instance of SimpleTool
func NewSimpleTool() *SimpleTool {
	return &SimpleTool{
		name: "simple_tool",
	}
}

// Execute implements the Tool interface - performs the tool's main function
func (t *SimpleTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assert the arguments
	toolArgs, ok := args.(*SimpleToolArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected *SimpleToolArgs, got %T", args)
	}

	// Simulate some work
	message := fmt.Sprintf("Hello, %s! You said: %s", toolArgs.Name, toolArgs.Message)

	// Return structured result
	result := &SimpleToolResult{
		Status:  "success",
		Message: message,
		Echo:    toolArgs.Message,
	}

	return result, nil
}

// GetMetadata implements the Tool interface - provides tool information
func (t *SimpleTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "simple_tool",
		Description: "A simple example tool that echoes messages",
		Version:     "1.0.0",
		Category:    "example",
		Capabilities: []string{
			"echo",
			"greeting",
		},
		Parameters: map[string]string{
			"name":    "required - The name to greet",
			"message": "required - The message to echo",
		},
		Examples: []mcp.ToolExample{
			{
				Description: "Basic greeting",
				Args: map[string]interface{}{
					"name":    "Alice",
					"message": "Hello from Container Kit!",
				},
			},
		},
	}
}

// Validate implements the Tool interface - validates input arguments
func (t *SimpleTool) Validate(ctx context.Context, args interface{}) error {
	toolArgs, ok := args.(*SimpleToolArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected *SimpleToolArgs, got %T", args)
	}

	// Validate required fields
	if toolArgs.Name == "" {
		return fmt.Errorf("name is required")
	}

	if toolArgs.Message == "" {
		return fmt.Errorf("message is required")
	}

	// Validate field constraints
	if len(toolArgs.Name) > 50 {
		return fmt.Errorf("name must be 50 characters or less")
	}

	if len(toolArgs.Message) > 200 {
		return fmt.Errorf("message must be 200 characters or less")
	}

	return nil
}

// SimpleToolArgs defines the input arguments for SimpleTool
type SimpleToolArgs struct {
	Name    string `json:"name" description:"The name to greet"`
	Message string `json:"message" description:"The message to echo"`
}

// SimpleToolResult defines the output of SimpleTool
type SimpleToolResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Echo    string `json:"echo"`
}

// Ensure SimpleTool implements the Tool interface
var _ mcp.Tool = (*SimpleTool)(nil)

func main() {
	// Create tool instance
	tool := NewSimpleTool()

	// Print metadata
	metadata := tool.GetMetadata()
	fmt.Printf("Tool: %s v%s\n", metadata.Name, metadata.Version)
	fmt.Printf("Description: %s\n", metadata.Description)
	fmt.Printf("Category: %s\n", metadata.Category)
	fmt.Println()

	// Example 1: Valid execution
	fmt.Println("Example 1: Valid execution")
	args1 := &SimpleToolArgs{
		Name:    "Alice",
		Message: "Hello from the unified interface!",
	}

	// Validate
	if err := tool.Validate(context.Background(), args1); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	fmt.Println("✓ Validation passed")

	// Execute
	result1, err := tool.Execute(context.Background(), args1)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	if res, ok := result1.(*SimpleToolResult); ok {
		fmt.Printf("✓ Execution successful: %s\n", res.Message)
		fmt.Printf("  Status: %s\n", res.Status)
		fmt.Printf("  Echo: %s\n", res.Echo)
	}
	fmt.Println()

	// Example 2: Invalid arguments (missing name)
	fmt.Println("Example 2: Invalid arguments (missing name)")
	args2 := &SimpleToolArgs{
		Message: "This will fail validation",
	}

	if err := tool.Validate(context.Background(), args2); err != nil {
		fmt.Printf("✓ Validation correctly failed: %v\n", err)
	}
	fmt.Println()

	// Example 3: Invalid arguments (name too long)
	fmt.Println("Example 3: Invalid arguments (name too long)")
	args3 := &SimpleToolArgs{
		Name:    "ThisNameIsWayTooLongAndExceedsThe50CharacterLimitThatWeSet",
		Message: "Test message",
	}

	if err := tool.Validate(context.Background(), args3); err != nil {
		fmt.Printf("✓ Validation correctly failed: %v\n", err)
	}
}
