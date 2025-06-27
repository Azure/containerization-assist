package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
	ID      int                    `json:"id"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ChatResult represents the chat tool result
type ChatResult struct {
	Success   bool                     `json:"success"`
	SessionID string                   `json:"session_id"`
	Message   string                   `json:"message"`
	Stage     string                   `json:"stage,omitempty"`
	Status    string                   `json:"status,omitempty"`
	Options   []map[string]interface{} `json:"options,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Start the MCP server
	cmd := exec.Command("./container-kit-mcp", "--transport", "stdio", "--conversation", "--log-level", "info")

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer cmd.Process.Kill()

	// Create JSON encoder/decoder
	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(stdout)

	// Read stderr in background (for logs)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "[SERVER] %s\n", scanner.Text())
		}
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Send initialize request first
	fmt.Println("=== Sending initialize request ===")
	initReq := MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
		ID: 1,
	}

	if err := encoder.Encode(initReq); err != nil {
		log.Printf("Failed to send init request: %v", err)
		return fmt.Errorf("failed to send init request: %w", err)
	}

	var initResp MCPResponse
	if err := decoder.Decode(&initResp); err != nil {
		log.Printf("Failed to read init response: %v", err)
		return fmt.Errorf("failed to read init response: %w", err)
	}

	fmt.Printf("Initialize response: %+v\n", initResp)

	// List available tools
	fmt.Println("\n=== Listing available tools ===")
	listReq := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  map[string]interface{}{},
		ID:      2,
	}

	if err := encoder.Encode(listReq); err != nil {
		log.Printf("Failed to send list tools request: %v", err)
		return fmt.Errorf("failed to send list tools request: %w", err)
	}

	var listResp MCPResponse
	if err := decoder.Decode(&listResp); err != nil {
		log.Printf("Failed to read list tools response: %v", err)
		return fmt.Errorf("failed to read list tools response: %w", err)
	}

	fmt.Printf("Tools list response: %s\n", string(listResp.Result))

	// Test conversation flow
	fmt.Println("\n=== Testing MCP Conversation Tool ===")

	// Test 1: Initial message
	fmt.Println("\nTest 1: Initial greeting")
	sessionID := testChat(encoder, decoder, "Hello, I want to containerize my Go application", "")

	// Test 2: Continue conversation
	if sessionID != "" {
		fmt.Println("\nTest 2: Continue conversation")
		testChat(encoder, decoder, "Yes, let's proceed", sessionID)

		// Test 3: Provide repository
		fmt.Println("\nTest 3: Provide repository URL")
		testChat(encoder, decoder, "https://github.com/golang/example", sessionID)
	}

	fmt.Println("\nTest completed!")
	return nil
}

var requestID = 2

func testChat(encoder *json.Encoder, decoder *json.Decoder, message, sessionID string) string {
	requestID++
	// Build request
	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "chat",
			"arguments": map[string]interface{}{
				"message": message,
			},
		},
		ID: requestID,
	}

	if sessionID != "" {
		req.Params["arguments"].(map[string]interface{})["session_id"] = sessionID
	}

	fmt.Printf("Sending: %s\n", message)

	// Send request
	if err := encoder.Encode(req); err != nil {
		log.Printf("Failed to send request: %v", err)
		return ""
	}

	// Read response
	var resp MCPResponse
	if err := decoder.Decode(&resp); err != nil {
		if err != io.EOF {
			log.Printf("Failed to read response: %v", err)
		}
		return ""
	}

	// Check for error
	if resp.Error != nil {
		fmt.Printf("Error: %s (code: %d)\n", resp.Error.Message, resp.Error.Code)
		return ""
	}

	// Debug: Show raw result
	fmt.Printf("Raw response: %s\n", string(resp.Result))

	// Parse the MCP content wrapper first
	type MCPContent struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	var mcpContent MCPContent
	if err := json.Unmarshal(resp.Result, &mcpContent); err != nil {
		fmt.Printf("Failed to parse MCP content: %v\n", err)
		return ""
	}

	if len(mcpContent.Content) == 0 || mcpContent.Content[0].Text == "" {
		fmt.Printf("No content in response\n")
		return ""
	}

	// Parse the actual result from the text content
	var result ChatResult
	if err := json.Unmarshal([]byte(mcpContent.Content[0].Text), &result); err != nil {
		fmt.Printf("Failed to parse result: %v\n", err)
		fmt.Printf("Raw text: %s\n", mcpContent.Content[0].Text)
		return ""
	}

	// Display result
	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("SessionID: %s\n", result.SessionID)
	fmt.Printf("Stage: %s\n", result.Stage)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Message: %s\n", result.Message)

	if len(result.Options) > 0 {
		fmt.Println("Options:")
		for _, opt := range result.Options {
			fmt.Printf("  - %v\n", opt)
		}
	}

	return result.SessionID
}
