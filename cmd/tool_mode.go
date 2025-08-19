package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/service/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// handleToolMode handles execution when the binary is called in tool mode
// Usage: container-kit-mcp tool <tool-name> --json
func handleToolMode() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s tool <tool-name> [--json]\n", os.Args[0])
		os.Exit(1)
	}

	toolName := os.Args[2]

	// Get tool parameters from environment variable
	paramsJSON := os.Getenv("TOOL_PARAMS")
	if paramsJSON == "" {
		fmt.Fprintf(os.Stderr, "Error: TOOL_PARAMS environment variable not set\n")
		os.Exit(1)
	}

	// Parse parameters
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing TOOL_PARAMS: %v\n", err)
		os.Exit(1)
	}

	// Execute the tool and output result
	result, err := executeToolDirectly(toolName, params)
	if err != nil {
		// Output error as JSON
		errorResult := tools.ToolResult{
			Success: false,
			Error:   err.Error(),
		}
		output, _ := json.Marshal(errorResult)
		fmt.Println(string(output))
		os.Exit(1)
	}

	// Output successful result as JSON
	fmt.Println(result)
}

// executeToolDirectly executes a tool without the full server
func executeToolDirectly(toolName string, params map[string]interface{}) (string, error) {
	// Get the tool configuration
	toolConfig, err := tools.GetToolConfig(toolName)
	if err != nil {
		return "", fmt.Errorf("tool %s not found", toolName)
	}

	// Note: Logger would be used for workflow tools if they were executable in standalone mode
	_ = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// For utility tools that don't need dependencies
	if toolConfig.Category == tools.CategoryUtility {
		return executeUtilityTool(toolName, toolConfig, params)
	}

	// For tools that need session/workflow capabilities
	// We'll return a placeholder for now, as full implementation requires server setup
	// In practice, these would need the full dependency chain
	result := tools.ToolResult{
		Success: false,
		Error:   fmt.Sprintf("Tool %s requires server context. Please use MCP server mode.", toolName),
		Data: map[string]interface{}{
			"tool":     toolName,
			"category": string(toolConfig.Category),
			"hint":     "This tool requires session management and cannot run standalone",
		},
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

// executeUtilityTool handles execution of utility tools that don't need dependencies
func executeUtilityTool(toolName string, config *tools.ToolConfig, params map[string]interface{}) (string, error) {
	ctx := context.Background()

	// Create MCP request
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: params,
		},
	}

	var result *mcp.CallToolResult
	var err error

	// Handle specific utility tools
	switch toolName {
	case "list_tools":
		handler := tools.CreateListToolsHandler()
		result, err = handler(ctx, request)

	case "ping":
		// Simple ping implementation
		message, _ := params["message"].(string)
		response := "pong"
		if message != "" {
			response = "pong: " + message
		}
		result = &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"response":"%s","timestamp":"%s"}`, response, time.Now().Format(time.RFC3339)),
				},
			},
		}

	case "server_status":
		// Simple status without server context
		status := map[string]interface{}{
			"status":  "tool_mode",
			"version": getVersion(),
			"mode":    "standalone",
		}
		statusJSON, _ := json.Marshal(status)
		result = &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(statusJSON),
				},
			},
		}

	default:
		return "", fmt.Errorf("utility tool %s not implemented in standalone mode", toolName)
	}

	if err != nil {
		return "", err
	}

	// Extract text content from result
	if result != nil && len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			return textContent.Text, nil
		}
	}

	// Fallback
	output, _ := json.Marshal(result)
	return string(output), nil
}
