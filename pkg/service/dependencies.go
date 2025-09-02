package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/containerization-assist/pkg/api"
	domainevents "github.com/Azure/containerization-assist/pkg/domain/events"
	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/ai_ml/prompts"
	"github.com/Azure/containerization-assist/pkg/infrastructure/ai_ml/sampling"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
	"github.com/Azure/containerization-assist/pkg/service/bootstrap"
	"github.com/Azure/containerization-assist/pkg/service/lifecycle"
	"github.com/Azure/containerization-assist/pkg/service/session"
)

type Option func(*Dependencies)

type Dependencies struct {
	Logger         *slog.Logger
	Config         workflow.ServerConfig
	SessionManager session.OptimizedSessionManager
	ResourceStore  *core.Store

	EventPublisher domainevents.Publisher

	WorkflowOrchestrator workflow.WorkflowOrchestrator

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

type server struct {
	dependencies     *Dependencies
	lifecycleManager *lifecycle.LifecycleManager
	bootstrapper     *bootstrap.Bootstrapper
}

func (s *server) Start(ctx context.Context) error {
	return s.lifecycleManager.Start(ctx)
}

func (s *server) Stop(ctx context.Context) error {
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

	return &server{
		dependencies:     deps,
		lifecycleManager: lifecycleManager,
		bootstrapper:     bootstrapper,
	}, nil
}
