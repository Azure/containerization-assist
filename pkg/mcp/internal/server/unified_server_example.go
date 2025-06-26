package server

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

// ExampleUnifiedServer demonstrates how to set up and use the unified MCP server
func ExampleUnifiedServer() error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Open database
	db, err := bbolt.Open("/tmp/mcp-unified.db", 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create unified server in dual mode (both chat and workflow)
	server, err := NewUnifiedMCPServer(
		db,
		logger,
		ModeDual,
	)
	if err != nil {
		return fmt.Errorf("failed to create unified server: %w", err)
	}

	// Get server capabilities
	capabilities := server.GetCapabilities()
	logger.Info().
		Bool("chat_support", capabilities.ChatSupport).
		Bool("workflow_support", capabilities.WorkflowSupport).
		Interface("available_modes", capabilities.AvailableModes).
		Interface("shared_tools", capabilities.SharedTools).
		Msg("Server capabilities")

	// Example 1: Use chat mode
	ctx := context.Background()
	chatResponse, err := server.ExecuteTool(ctx, "chat", map[string]interface{}{
		"message":    "I want to containerize my Node.js application",
		"session_id": "example-session-1",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Chat tool execution failed")
	} else {
		logger.Info().
			Interface("response", chatResponse).
			Msg("Chat response received")
	}

	// Example 2: Use workflow mode
	workflowResponse, err := server.ExecuteTool(ctx, "execute_workflow", map[string]interface{}{
		"workflow_name": "containerization-pipeline",
		"variables": map[string]string{
			"repo_url": "https://github.com/example/nodejs-app",
			"registry": "myregistry.azurecr.io",
		},
		"options": map[string]interface{}{
			"dry_run":     false,
			"checkpoints": true,
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("Workflow execution failed")
	} else {
		logger.Info().
			Interface("response", workflowResponse).
			Msg("Workflow execution completed")
	}

	// Example 3: Use atomic tools directly
	atomicResponse, err := server.ExecuteTool(ctx, "analyze_repository_atomic", map[string]interface{}{
		"session_id": "example-session-2",
		"repo_url":   "https://github.com/example/python-app",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Atomic tool execution failed")
	} else {
		logger.Info().
			Interface("response", atomicResponse).
			Msg("Atomic tool execution completed")
	}

	// Example 4: List available workflows
	workflowList, err := server.ExecuteTool(ctx, "list_workflows", map[string]interface{}{
		"category": "security",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list workflows")
	} else {
		logger.Info().
			Interface("workflows", workflowList).
			Msg("Available workflows")
	}

	return nil
}

// ExampleWorkflowModeOnly demonstrates a workflow-only server
func ExampleWorkflowModeOnly() error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Open database
	db, err := bbolt.Open("/tmp/mcp-workflow-only.db", 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create legacy orchestrator
	// Create workflow-only server
	server, err := NewUnifiedMCPServer(
		db,
		logger,
		ModeWorkflow,
	)
	if err != nil {
		return fmt.Errorf("failed to create workflow server: %w", err)
	}

	ctx := context.Background()

	// This will work - workflow tool
	_, err = server.ExecuteTool(ctx, "execute_workflow", map[string]interface{}{
		"workflow_name": "security-focused-pipeline",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Workflow execution failed")
	}

	// This will fail - chat tool not available in workflow-only mode
	_, err = server.ExecuteTool(ctx, "chat", map[string]interface{}{
		"message": "Hello",
	})
	if err != nil {
		logger.Info().Err(err).Msg("Expected error: chat not available in workflow mode")
	}

	// Atomic tools are always available
	_, err = server.ExecuteTool(ctx, "build_image_atomic", map[string]interface{}{
		"session_id": "workflow-session",
		"image_name": "my-app",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Atomic tool execution failed")
	}

	return nil
}

// ExampleChatModeOnly demonstrates a chat-only server
func ExampleChatModeOnly() error {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Open database
	db, err := bbolt.Open("/tmp/mcp-chat-only.db", 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create chat-only server
	server, err := NewUnifiedMCPServer(
		db,
		logger,
		ModeChat,
	)
	if err != nil {
		return fmt.Errorf("failed to create chat server: %w", err)
	}

	ctx := context.Background()

	// This will work - chat tool
	_, err = server.ExecuteTool(ctx, "chat", map[string]interface{}{
		"message":    "Help me containerize my application",
		"session_id": "chat-session",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Chat execution failed")
	}

	// This will fail - workflow tools not available in chat-only mode
	_, err = server.ExecuteTool(ctx, "execute_workflow", map[string]interface{}{
		"workflow_name": "containerization-pipeline",
	})
	if err != nil {
		logger.Info().Err(err).Msg("Expected error: workflow not available in chat mode")
	}

	// Atomic tools are always available
	_, err = server.ExecuteTool(ctx, "scan_image_security_atomic", map[string]interface{}{
		"session_id": "chat-session",
		"image_ref":  "my-app:latest",
	})
	if err != nil {
		logger.Error().Err(err).Msg("Atomic tool execution failed")
	}

	return nil
}
