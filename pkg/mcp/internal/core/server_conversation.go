package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	coreinterfaces "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/observability"
	"github.com/Azure/container-kit/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-kit/pkg/mcp/internal/runtime/conversation"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/runner"
)

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
	ch, err := d.transport.InvokeTool(ctx, "chat", payload, false)
	if err != nil {
		return "", err
	}

	// Read the response from the channel
	for msg := range ch {
		var response string
		if err := json.Unmarshal(msg, &response); err == nil {
			return response, nil
		}
		// If unmarshaling as string fails, return the raw message as string
		return string(msg), nil
	}

	return "", nil
}

// Use ConversationConfig from core package to avoid type mismatch

// ConversationComponents holds the conversation mode components
type ConversationComponents struct {
	Handler         *conversation.ConversationHandler // Concrete conversation handler
	PreferenceStore *utils.PreferenceStore
	Telemetry       *observability.TelemetryManager
}

// EnableConversationMode integrates the conversation components into the server
func (s *Server) EnableConversationMode(config coreinterfaces.ConversationConfig) error {
	s.logger.Info().Msg("Enabling conversation mode")

	// Initialize preference store
	prefsPath := config.PreferencesDBPath
	if prefsPath == "" {
		prefsPath = filepath.Join(s.config.WorkspaceDir, "preferences.db")
	}

	preferenceStore, err := utils.NewPreferenceStore(prefsPath, s.logger, config.PreferencesEncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create preference store: %w", err)
	}

	// Initialize telemetry if enabled
	var telemetryMgr *observability.TelemetryManager
	if config.EnableTelemetry {
		// Create OpenTelemetry configuration if enabled
		var otelConfig *observability.OTELConfig
		if config.EnableOTEL {
			serviceName := config.ServiceName
			if serviceName == "" {
				serviceName = "container-kit-mcp"
			}

			serviceVersion := config.ServiceVersion
			if serviceVersion == "" {
				serviceVersion = "1.0.0"
			}

			environment := config.Environment
			if environment == "" {
				environment = "development"
			}

			sampleRate := config.TraceSampleRate
			if sampleRate <= 0 {
				sampleRate = 1.0
			}

			otelConfig = &observability.OTELConfig{
				ServiceName:     serviceName,
				ServiceVersion:  serviceVersion,
				Environment:     environment,
				EnableOTLP:      config.OTELEndpoint != "",
				OTLPEndpoint:    config.OTELEndpoint,
				OTLPHeaders:     config.OTELHeaders,
				OTLPInsecure:    true, // Default to insecure for development
				OTLPTimeout:     10 * time.Second,
				TraceSampleRate: sampleRate,
				CustomAttributes: map[string]string{
					"service.component": "mcp-server",
				},
				Logger: s.logger,
			}

			// Validate configuration
			if err := otelConfig.Validate(); err != nil {
				s.logger.Error().Err(err).Msg("Invalid OpenTelemetry configuration")
				return fmt.Errorf("invalid OpenTelemetry configuration: %w", err)
			}

			s.logger.Info().
				Str("service_name", serviceName).
				Str("otlp_endpoint", config.OTELEndpoint).
				Bool("enable_otlp", config.OTELEndpoint != "").
				Float64("sample_rate", sampleRate).
				Msg("OpenTelemetry configuration created")
		}

		telemetryMgr = observability.NewTelemetryManager(observability.TelemetryConfig{
			MetricsPort:      config.TelemetryPort,
			P95Target:        2 * time.Second,
			Logger:           s.logger,
			EnableAutoExport: true,
			OTELConfig:       otelConfig,
		})

		s.logger.Info().
			Int("port", config.TelemetryPort).
			Bool("otel_enabled", config.EnableOTEL).
			Msg("Telemetry enabled - Prometheus metrics and OpenTelemetry available")
	}

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

		// For tool orchestrator, create minimal bridge to handle TokenUsage type difference
		bridgeAnalyzer := &coreAnalyzerBridge{analyzer: coreAnalyzer}
		if s.toolOrchestrator != nil {
			s.toolOrchestrator.SetAnalyzer(bridgeAnalyzer)
		}

		s.logger.Info().Msg("CallerAnalyzer enabled for conversation mode")
	} else {
		s.logger.Warn().Msg("Transport does not implement LLMTransport - using StubAnalyzer")
	}

	// Create pipeline operations
	pipelineOps := pipeline.NewOperations(
		s.sessionManager,
		mcpClients,
		s.logger,
	)

	// Use session manager directly - no adapter needed

	// Use the server's canonical orchestrator instead of creating parallel orchestration
	// This eliminates the tool registration conflicts and ensures single orchestration path
	conversationHandler, err := conversation.NewConversationHandler(conversation.ConversationHandlerConfig{
		SessionManager:     s.sessionManager,
		PreferenceStore:    preferenceStore,
		PipelineOperations: pipelineOps,
		ToolOrchestrator:   s.toolOrchestrator, // Use canonical orchestrator
		Transport:          s.transport,
		Logger:             s.logger,
		Telemetry:          telemetryMgr,
	})
	if err != nil {
		return fmt.Errorf("failed to create conversation handler: %w", err)
	}

	// Chat tool registration is handled by register_all_tools.go

	// Store references for shutdown
	s.conversationComponents = &ConversationComponents{
		Handler:         conversationHandler,
		PreferenceStore: preferenceStore,
		Telemetry:       telemetryMgr,
	}

	s.logger.Info().Msg("Conversation mode enabled successfully")

	return nil
}

// Add these fields to the Server struct (in server.go):
// conversationAdapter *conversation.ConversationAdapter
// preferenceStore     *utils.PreferenceStore
// telemetry          *observability.TelemetryManager

// ShutdownConversation gracefully shuts down conversation components
func (s *Server) ShutdownConversation() error {
	if s.conversationComponents == nil {
		return nil
	}

	var errs []error

	if s.conversationComponents.PreferenceStore != nil {
		if err := s.conversationComponents.PreferenceStore.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close preference store: %w", err))
		}
	}

	if s.conversationComponents.Telemetry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.conversationComponents.Telemetry.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown telemetry: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
