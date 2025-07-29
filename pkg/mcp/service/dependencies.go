package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/api"
	domainevents "github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/resources"
	"github.com/Azure/container-kit/pkg/mcp/service/bootstrap"
	"github.com/Azure/container-kit/pkg/mcp/service/lifecycle"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
)

type Option func(*Dependencies)

func WithLogger(logger *slog.Logger) Option {
	return func(d *Dependencies) {
		d.Logger = logger
	}
}

func WithConfig(config workflow.ServerConfig) Option {
	return func(d *Dependencies) {
		d.Config = config
	}
}

type Dependencies struct {
	Logger         *slog.Logger
	Config         workflow.ServerConfig
	SessionManager session.OptimizedSessionManager
	ResourceStore  *resources.Store

	EventPublisher domainevents.Publisher

	WorkflowOrchestrator   workflow.WorkflowOrchestrator
	EventAwareOrchestrator workflow.EventAwareOrchestrator

	SamplingClient *sampling.Client
	PromptManager  *prompts.Manager
}

func (d *Dependencies) Validate() error {
	var errs []error

	if d.Logger == nil {
		errs = append(errs, fmt.Errorf("logger is required"))
	}
	if d.SessionManager == nil {
		errs = append(errs, fmt.Errorf("session manager is required"))
	}
	if d.ResourceStore == nil {
		errs = append(errs, fmt.Errorf("resource store is required"))
	}

	if d.EventPublisher == nil {
		errs = append(errs, fmt.Errorf("event publisher is required"))
	}

	if d.WorkflowOrchestrator == nil {
		errs = append(errs, fmt.Errorf("workflow orchestrator is required"))
	}

	if d.SamplingClient == nil {
		errs = append(errs, fmt.Errorf("sampling client is required"))
	}
	if d.PromptManager == nil {
		errs = append(errs, fmt.Errorf("prompt manager is required"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("dependency validation failed: %v", errs)
	}
	return nil
}

type serverImpl struct {
	dependencies     *Dependencies
	lifecycleManager *lifecycle.LifecycleManager
	bootstrapper     *bootstrap.Bootstrapper
}

func (s *serverImpl) Start(ctx context.Context) error {
	return s.lifecycleManager.Start(ctx)
}

func (s *serverImpl) Stop(ctx context.Context) error {
	return s.lifecycleManager.Shutdown(ctx)
}

func NewMCPServerFromDeps(deps *Dependencies) (api.MCPServer, error) {
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	bootstrapper := bootstrap.NewBootstrapper(
		deps.Logger,
		deps.Config,
		deps.ResourceStore,
		deps.WorkflowOrchestrator,
		deps.SessionManager,
	)

	lifecycleManager := lifecycle.NewLifecycleManager(
		deps.Logger,
		deps.Config,
		deps.SessionManager,
		bootstrapper,
	)

	return &serverImpl{
		dependencies:     deps,
		lifecycleManager: lifecycleManager,
		bootstrapper:     bootstrapper,
	}, nil
}

func GetChatModeFunctions() []string {
	return []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"scan_image",
		"tag_image",
		"push_image",
		"generate_k8s_manifests",
		"prepare_cluster",
		"deploy_application",
		"verify_deployment",
		"start_workflow",
		"workflow_status",
		"list_tools",
	}
}
