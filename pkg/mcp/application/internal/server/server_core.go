package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/runtime/conversation"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/processing"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"go.etcd.io/bbolt"
)

// UnifiedMCPServer is the main server implementation
type UnifiedMCPServer struct {
	// Chat mode components
	promptManager  *conversation.PromptManager
	sessionManager *session.SessionManager

	// Unified session management
	unifiedSessionManager session.UnifiedSessionManager

	// Workflow mode components
	workflowExecutor services.WorkflowExecutor

	// Shared components
	toolRegistry     api.Registry
	toolOrchestrator api.Orchestrator

	// Tool management
	toolManager *ToolManager

	// Server state
	currentMode ServerMode
	logger      *slog.Logger
}

// getChatModeTools returns tools available in chat mode
func (s *UnifiedMCPServer) getChatModeTools() []ToolDefinition {
	if s.toolManager == nil {
		return []ToolDefinition{}
	}
	return s.toolManager.getChatModeTools()
}

// getWorkflowModeTools returns tools available in workflow mode
func (s *UnifiedMCPServer) getWorkflowModeTools() []ToolDefinition {
	if s.toolManager == nil {
		return []ToolDefinition{}
	}
	return s.toolManager.getWorkflowModeTools()
}

// isAtomicTool checks if a tool is an atomic tool
func (s *UnifiedMCPServer) isAtomicTool(toolName string) bool {
	if s.toolManager == nil {
		return false
	}
	return s.toolManager.isAtomicTool(toolName)
}

// buildInputSchema builds an input schema for a tool
func (s *UnifiedMCPServer) buildInputSchema(metadata *api.ToolMetadata) map[string]interface{} {
	if s.toolManager == nil {
		return map[string]interface{}{}
	}
	return s.toolManager.buildInputSchema(metadata)
}

// NewUnifiedMCPServer creates a new unified MCP server
func NewUnifiedMCPServer(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
) (*UnifiedMCPServer, error) {
	return createUnifiedMCPServer(db, logger, mode, nil)
}

// NewUnifiedMCPServerWithUnifiedSessionManager creates a new unified MCP server with unified session manager
func NewUnifiedMCPServerWithUnifiedSessionManager(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
	unifiedSessionManager session.UnifiedSessionManager,
) (*UnifiedMCPServer, error) {
	return createUnifiedMCPServer(db, logger, mode, unifiedSessionManager)
}

// createUnifiedMCPServer is the common server creation logic
func createUnifiedMCPServer(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
	unifiedSessionManager session.UnifiedSessionManager,
) (*UnifiedMCPServer, error) {
	unifiedRegistry := core.NewUnifiedRegistry(logger)
	toolRegistry := core.NewRegistryAdapter(unifiedRegistry)

	var actualSessionManager session.UnifiedSessionManager
	var concreteSessionManager *session.SessionManager // Keep reference for legacy components
	var err error

	if unifiedSessionManager != nil {
		logger.Info("Using provided unified session manager")
		actualSessionManager = unifiedSessionManager
		if sm, ok := unifiedSessionManager.(*session.SessionManager); ok {
			concreteSessionManager = sm
		}
	} else {
		concreteSessionManager, err = session.NewSessionManager(session.SessionManagerConfig{
			WorkspaceDir:      "/tmp/mcp-sessions",
			MaxSessions:       100,
			SessionTTL:        24 * time.Hour,
			MaxDiskPerSession: 1024 * 1024 * 1024,
			TotalDiskLimit:    10 * 1024 * 1024 * 1024,
			StorePath:         "/tmp/mcp-sessions.db",
			Logger:            logger,
		})
		if err != nil {
			return nil, errors.NewError().Message("failed to create session manager").Cause(err).Build()
		}
		actualSessionManager = concreteSessionManager
	}

	toolOrchestrator := orchestration.NewOrchestrator(
		orchestration.WithLogger(logger),
		orchestration.WithTimeout(10*time.Minute),
		orchestration.WithMetrics(true),
	)

	server := &UnifiedMCPServer{
		toolRegistry:          toolRegistry,
		toolOrchestrator:      toolOrchestrator,
		unifiedSessionManager: actualSessionManager,
		currentMode:           mode,
		logger:                logger.With("component", "unified_mcp_server"),
	}

	server.toolManager = NewToolManager(server)

	if mode == ModeDual || mode == ModeChat {
		preferenceStore, err := processing.NewPreferenceStore("/tmp/mcp-preferences.db", logger, "")
		if err != nil {
			return nil, errors.NewError().Message("failed to create preference store").Cause(err).Build()
		}

		if concreteSessionManager != nil {
			server.promptManager = conversation.NewPromptManager(conversation.PromptManagerConfig{
				SessionManager:   concreteSessionManager,
				ToolOrchestrator: toolOrchestrator,
				PreferenceStore:  preferenceStore,
				Logger:           logger,
			})
		} else {
			logger.Warn("Chat mode requested but no concrete session manager available")
		}
	}

	if mode == ModeDual || mode == ModeWorkflow {
		logger.Info("Workflow manager initialization skipped - not implemented yet")
	}

	server.logger.Info("Initialized unified MCP server",
		"mode", string(mode))

	return server, nil
}

// GetCapabilities returns the server's capabilities
func (s *UnifiedMCPServer) GetCapabilities() ServerCapabilities {
	capabilities := ServerCapabilities{
		AvailableModes: make([]string, 0),
		SharedTools:    make([]string, 0),
	}

	switch s.currentMode {
	case ModeDual:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"chat", "workflow", "dual"}
	case ModeChat:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = false
		capabilities.AvailableModes = []string{"chat"}
	case ModeWorkflow:
		capabilities.ChatSupport = false
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"workflow"}
	}

	if s.toolOrchestrator != nil {
		capabilities.SharedTools = s.toolOrchestrator.ListTools()
	}

	return capabilities
}

// GetAvailableTools returns all available tools
func (s *UnifiedMCPServer) GetAvailableTools() []ToolDefinition {
	if s.toolManager == nil {
		return []ToolDefinition{}
	}
	return s.toolManager.GetAvailableTools()
}

// ExecuteTool executes a tool with the given name and arguments
func (s *UnifiedMCPServer) ExecuteTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (interface{}, error) {
	if s.toolManager == nil {
		return nil, errors.NewError().Message("tool manager not initialized").Build()
	}
	return s.toolManager.ExecuteTool(ctx, toolName, args)
}

// ExecuteToolTyped executes a tool with typed arguments
func (s *UnifiedMCPServer) ExecuteToolTyped(
	ctx context.Context,
	toolName string,
	args TypedArgs,
) (TypedResult, error) {
	// Parse the typed arguments
	var parsedArgs map[string]interface{}
	if err := json.Unmarshal(args.Data, &parsedArgs); err != nil {
		return TypedResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse arguments: %v", err),
		}, nil
	}

	// Execute the tool
	result, err := s.ExecuteTool(ctx, toolName, parsedArgs)
	if err != nil {
		return TypedResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Serialize the result
	resultData, err := json.Marshal(result)
	if err != nil {
		return TypedResult{
			Success: false,
			Error:   fmt.Sprintf("failed to serialize result: %v", err),
		}, nil
	}

	return TypedResult{
		Success: true,
		Data:    resultData,
	}, nil
}

// GetMode returns the current server mode
func (s *UnifiedMCPServer) GetMode() ServerMode {
	return s.currentMode
}

// GetLogger returns the server logger
func (s *UnifiedMCPServer) GetLogger() *slog.Logger {
	return s.logger
}

// GetSessionManager returns the session manager
func (s *UnifiedMCPServer) GetSessionManager() session.UnifiedSessionManager {
	return s.unifiedSessionManager
}

// GetToolOrchestrator returns the tool orchestrator
func (s *UnifiedMCPServer) GetToolOrchestrator() api.Orchestrator {
	return s.toolOrchestrator
}

// GetWorkflowExecutor returns the workflow executor
func (s *UnifiedMCPServer) GetWorkflowExecutor() services.WorkflowExecutor {
	return s.workflowExecutor
}

// GetPromptManager returns the prompt manager
func (s *UnifiedMCPServer) GetPromptManager() *conversation.PromptManager {
	return s.promptManager
}

// Shutdown gracefully shuts down the server
func (s *UnifiedMCPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down unified MCP server")

	if s.workflowExecutor != nil {
		s.logger.Info("Shutting down workflow executor")
	}

	if s.promptManager != nil {
		s.logger.Info("Shutting down prompt manager")
	}

	if s.unifiedSessionManager != nil {
		if sm, ok := s.unifiedSessionManager.(*session.SessionManager); ok && sm != nil {
			if err := sm.Close(); err != nil {
				s.logger.Error("Failed to close session manager", "error", err)
			}
		} else {
			s.logger.Info("Session manager close skipped - type assertion failed or nil")
		}
	}

	s.logger.Info("Unified MCP server shutdown complete")
	return nil
}
