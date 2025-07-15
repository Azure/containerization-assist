package degradation

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideDegradationServices provides all degradation-related services
func ProvideDegradationServices(logger *slog.Logger) DegradationServices {
	manager := ProvideDegradationManager(logger)

	// Register default health checkers
	registerDefaultHealthCheckers(manager)

	return DegradationServices{
		Manager: manager,
	}
}

// DegradationServices bundles all degradation services
type DegradationServices struct {
	Manager *DegradationManager
}

// ProvideDegradationManager provides a degradation manager instance
func ProvideDegradationManager(logger *slog.Logger) *DegradationManager {
	return NewDegradationManager(logger)
}

// ProvideGracefulOrchestrator provides a workflow orchestrator with graceful degradation
func ProvideGracefulOrchestrator(base workflow.WorkflowOrchestrator, manager *DegradationManager) workflow.WorkflowOrchestrator {
	return NewGracefulOrchestrator(base, manager)
}

// ProvideDegradableService provides a degradable service wrapper
func ProvideDegradableService(manager *DegradationManager, serviceName string) *DegradableService {
	return NewDegradableService(manager, serviceName)
}

// registerDefaultHealthCheckers registers default health checkers
func registerDefaultHealthCheckers(manager *DegradationManager) {
	// AI Service health checker
	manager.RegisterService("ai_service", func(ctx context.Context) error {
		// In a real implementation, this would check Azure OpenAI connectivity
		// For now, we'll assume it's healthy
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	// Docker service health checker
	manager.RegisterService("docker_service", func(ctx context.Context) error {
		// In a real implementation, this would check Docker daemon status
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return nil
		}
	})

	// Registry service health checker
	manager.RegisterService("registry_service", func(ctx context.Context) error {
		// In a real implementation, this would check registry connectivity
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})

	// Session store health checker
	manager.RegisterService("session_store", func(ctx context.Context) error {
		// In a real implementation, this would check BoltDB health
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
			return nil
		}
	})
}
