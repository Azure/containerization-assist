package core

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
	"github.com/Azure/container-copilot/pkg/mcp/internal/analyzer"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/Azure/container-copilot/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime/conversation"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	"github.com/Azure/container-copilot/pkg/runner"
)

// ConversationConfig holds configuration for conversation mode
type ConversationConfig struct {
	EnableTelemetry          bool
	TelemetryPort            int
	PreferencesDBPath        string
	PreferencesEncryptionKey string // Optional encryption key for preference store

	// OpenTelemetry configuration
	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ConversationComponents holds the conversation mode components
type ConversationComponents struct {
	Handler         *conversation.ConversationHandler // Concrete conversation handler
	PreferenceStore *utils.PreferenceStore
	Telemetry       *ops.TelemetryManager
}

// EnableConversationMode integrates the conversation components into the server
func (s *Server) EnableConversationMode(config ConversationConfig) error {
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
	var telemetryMgr *ops.TelemetryManager
	if config.EnableTelemetry {
		// Create OpenTelemetry configuration if enabled
		var otelConfig *ops.OTELConfig
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

			otelConfig = &ops.OTELConfig{
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

		telemetryMgr = ops.NewTelemetryManager(ops.TelemetryConfig{
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
	mcpClients := adapter.NewMCPClients(
		docker.NewDockerCmdRunner(cmdRunner),
		kind.NewKindCmdRunner(cmdRunner),
		k8s.NewKubeCmdRunner(cmdRunner),
	)

	// In conversation mode, use CallerAnalyzer instead of StubAnalyzer
	// This requires the transport to be able to forward prompts to the LLM
	if transport, ok := s.transport.(contract.LLMTransport); ok {
		callerAnalyzer := analyzer.NewCallerAnalyzer(transport, analyzer.CallerAnalyzerOpts{
			ToolName:       "chat",
			SystemPrompt:   "You are an AI assistant helping with code analysis and fixing.",
			PerCallTimeout: 60 * time.Second,
		})
		mcpClients.SetAnalyzer(callerAnalyzer)

		// Also set the analyzer on the tool orchestrator for fixing capabilities
		if s.toolOrchestrator != nil {
			s.toolOrchestrator.SetAnalyzer(callerAnalyzer)
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
	sessionAdapter := s.sessionManager

	// Use the server's canonical orchestrator instead of creating parallel orchestration
	// This eliminates the tool registration conflicts and ensures single orchestration path
	conversationHandler, err := conversation.NewConversationHandler(conversation.ConversationHandlerConfig{
		SessionManager:     s.sessionManager,
		SessionAdapter:     sessionAdapter,
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
// telemetry          *ops.TelemetryManager

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
