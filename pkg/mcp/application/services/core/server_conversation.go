package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/pipeline"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/runtime/conversation"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/processing"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/runner"
)

// toolOrchestratorAdapter adapts api.Orchestrator to core.ToolOrchestrator interface
type toolOrchestratorAdapter struct {
	orchestrator api.Orchestrator
}

// ExecuteTool implements core.ToolOrchestrator interface
func (a *toolOrchestratorAdapter) ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error) {
	input := api.ToolInput{
		Data: map[string]interface{}{"params": args},
	}
	output, err := a.orchestrator.ExecuteTool(ctx, toolName, input)
	if err != nil {
		return nil, err
	}
	return output, nil
}

// RegisterTool implements core.ToolOrchestrator interface
func (a *toolOrchestratorAdapter) RegisterTool(toolName string, tool api.Tool) error {
	return a.orchestrator.RegisterTool(toolName, tool)
}

// ValidateToolArgs implements core.ToolOrchestrator interface
func (a *toolOrchestratorAdapter) ValidateToolArgs(_ string, _ interface{}) error {
	// For now, delegate to the underlying orchestrator's validation if available
	return nil
}

// GetToolMetadata implements core.ToolOrchestrator interface
func (a *toolOrchestratorAdapter) GetToolMetadata(toolName string) (*api.ToolMetadata, error) {
	tool, exists := a.orchestrator.GetTool(toolName)
	if !exists {
		return nil, errors.NewError().Messagef("tool %s not found", toolName).WithLocation().Build()
	}
	schema := tool.Schema()
	return &api.ToolMetadata{
		Name:        schema.Name,
		Description: schema.Description,
		Version:     schema.Version,
	}, nil
}

// RegisterGenericTool implements core.ToolOrchestrator interface
func (a *toolOrchestratorAdapter) RegisterGenericTool(toolName string, tool interface{}) error {
	if apiTool, ok := tool.(api.Tool); ok {
		return a.orchestrator.RegisterTool(toolName, apiTool)
	}
	return errors.NewError().Messagef("tool does not implement api.Tool interface").WithLocation(

	// GetTypedToolMetadata implements core.ToolOrchestrator interface
	).Build()
}

func (a *toolOrchestratorAdapter) GetTypedToolMetadata(toolName string) (*api.ToolMetadata, error) {
	return a.GetToolMetadata(toolName)
}

// simpleAnalyzerBridge bridges mcptypes.AIAnalyzer to coreinterfaces.AIAnalyzer for TokenUsage type compatibility
type simpleAnalyzerBridge struct {
	analyzer mcptypes.AIAnalyzer
}

func (b *simpleAnalyzerBridge) Analyze(ctx context.Context, prompt string) (string, error) {
	return b.analyzer.Analyze(ctx, prompt)
}

func (b *simpleAnalyzerBridge) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return b.analyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
}

func (b *simpleAnalyzerBridge) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return b.analyzer.AnalyzeWithFormat(ctx, promptTemplate, args...)
}

func (b *simpleAnalyzerBridge) GetTokenUsage() coreinterfaces.TokenUsage {
	usage := b.analyzer.GetTokenUsage()
	return coreinterfaces.TokenUsage{
		CompletionTokens: usage.CompletionTokens,
		PromptTokens:     usage.PromptTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func (b *simpleAnalyzerBridge) ResetTokenUsage() {
	b.analyzer.ResetTokenUsage()
}

// directLLMTransport implements analyze.LLMTransport using types.LLMTransport
type directLLMTransport struct {
	transport types.LLMTransport
}

// SendPrompt implements analyze.LLMTransport by converting to InvokeTool call
func (d *directLLMTransport) SendPrompt(prompt string) (string, error) {
	ctx := context.Background()
	payload := map[string]any{
		"prompt": prompt,
	}

	// Call the chat tool with the prompt
	err := d.transport.Send(ctx, payload)
	if err != nil {
		return "", err
	}

	// Receive the response
	responseData, err := d.transport.Receive(ctx)
	if err != nil {
		return "", err
	}

	// Extract response content
	if responseStr, ok := responseData.(string); ok {
		return responseStr, nil
	}

	// Try to convert to JSON and extract content
	if responseBytes, err := json.Marshal(responseData); err == nil {
		return string(responseBytes), nil
	}

	return "", errors.NewError().Messagef("unable to process response").WithLocation(

	// Use ConsolidatedConversationConfig from core package to avoid type mismatch
	).Build()
}

// ConversationComponents holds the conversation mode components
type ConversationComponents struct {
	Handler         *conversation.ConversationHandler // Concrete conversation handler
	PreferenceStore *processing.PreferenceStore
}

// EnableConversationMode integrates the conversation components into the server
func (s *Server) EnableConversationMode(config coreinterfaces.ConsolidatedConversationConfig) error {
	s.logger.Info("Enabling conversation mode")

	// Initialize preference store
	prefsPath := config.PreferencesDBPath
	if prefsPath == "" {
		prefsPath = filepath.Join(s.config.WorkspaceDir, "preferences.db")
	}

	preferenceStore, err := processing.NewPreferenceStore(prefsPath, slog.New(slog.NewTextHandler(os.Stderr, nil)).With("component", "preferences"), config.PreferencesEncryptionKey)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to create preference store",
			err,
		)
		systemErr.Context["component"] = "preference_store"
		return systemErr
	}

	// Telemetry initialization removed

	// Create clients for pipeline adapter
	cmdRunner := &runner.DefaultCommandRunner{}
	mcpClients := mcptypes.NewMCPClients(
		docker.NewDockerCmdRunner(cmdRunner),
		kind.NewKindCmdRunner(cmdRunner),
		k8s.NewKubeCmdRunner(cmdRunner),
	)

	// In conversation mode, use CallerAnalyzer instead of StubAnalyzer
	// This requires the transport to be able to forward prompts to the LLM
	if transport, ok := s.transport.(types.LLMTransport); ok {
		// Create direct implementation of analyze.LLMTransport using the existing transport
		llmTransport := &directLLMTransport{transport: transport}

		// Create core analyzer for orchestrator and MCPClients
		coreAnalyzer := analyze.NewCallerAnalyzer(llmTransport, analyze.CallerAnalyzerOpts{
			ToolName:       "chat",
			SystemPrompt:   "You are an AI assistant helping with code analysis and fixing.",
			PerCallTimeout: 60 * time.Second,
		})

		// Use analyzer directly - CallerAnalyzer now implements mcptypes.AIAnalyzer correctly
		mcpClients.Analyzer = coreAnalyzer

		// Create bridge for tool orchestrator (handles TokenUsage type compatibility)
		if s.toolOrchestrator != nil {
			// TODO: SetAnalyzer is not part of api.Orchestrator interface
			// This functionality needs to be migrated to the new architecture
			_ = coreAnalyzer // Suppress unused variable warning
		}

		s.logger.Info("CallerAnalyzer enabled for conversation mode")
	} else {
		s.logger.Warn("Transport does not implement LLMTransport - using StubAnalyzer")
	}

	// Create pipeline operations - use concrete session manager for job tracking
	pipelineOps := pipeline.NewOperations(
		s.sessionManager,
		mcpClients,
		s.logger,
	)

	// Create adapter to bridge api.Orchestrator to core.ToolOrchestrator
	orchestratorAdapter := &toolOrchestratorAdapter{
		orchestrator: s.toolOrchestrator,
	}

	// Use the server's canonical orchestrator instead of creating parallel orchestration
	// This eliminates the tool registration conflicts and ensures single orchestration path
	conversationHandler, err := conversation.NewConversationHandler(conversation.ConversationHandlerConfig{
		SessionManager:     s.sessionManager,
		PreferenceStore:    preferenceStore,
		PipelineOperations: pipelineOps,
		ToolOrchestrator:   orchestratorAdapter, // Use canonical orchestrator via adapter
		Transport:          s.transport,
		Logger:             slog.New(slog.NewTextHandler(os.Stderr, nil)).With("component", "conversation_handler"),
	})
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to create conversation handler",
			err,
		)
		systemErr.Context["component"] = "conversation_handler"
		return systemErr
	}

	// Chat tool registration is handled by register_all_tools.go

	// Store references for shutdown
	s.conversationComponents = &ConversationComponents{
		Handler:         conversationHandler,
		PreferenceStore: preferenceStore,
	}

	s.logger.Info("Conversation mode enabled successfully")

	return nil
}

// Add these fields to the Server struct (in server.go):
// conversationAdapter *conversation.ConversationAdapter
// preferenceStore     *diagnostics.PreferenceStore

// ShutdownConversation gracefully shuts down conversation components
func (s *Server) ShutdownConversation() error {
	if s.conversationComponents == nil {
		return nil
	}

	var errs []error

	if s.conversationComponents.PreferenceStore != nil {
		if err := s.conversationComponents.PreferenceStore.Close(); err != nil {
			systemErr := errors.SystemError(
				codes.SYSTEM_ERROR,
				"Failed to close preference store",
				err,
			)
			systemErr.Context["component"] = "preference_store"
			errs = append(errs, systemErr)
		}
	}

	// Telemetry shutdown removed

	if len(errs) > 0 {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("Shutdown completed with %d errors", len(errs)),
			nil,
		)
		systemErr.Context["error_count"] = len(errs)
		systemErr.Context["github.com/Azure/container-kit/pkg/mcp/domain/errors"] = errs
		systemErr.Context["component"] = "conversation_shutdown"
		return systemErr
	}

	return nil
}
