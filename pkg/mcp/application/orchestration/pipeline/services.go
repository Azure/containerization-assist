package pipeline

import (
	"context"
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// PipelineServices provides access to all pipeline-related services
type PipelineServices interface {
	// Lifecycle returns the pipeline lifecycle service
	Lifecycle() Lifecycle

	// WorkerRegistry returns the worker registry service
	WorkerRegistry() WorkerRegistry

	// WorkerHealth returns the worker health monitoring service
	WorkerHealth() WorkerHealthMonitor

	// JobScheduler returns the job scheduling service
	JobScheduler() JobScheduler

	// Monitor returns the pipeline monitoring service
	Monitor() Monitor
}

// pipelineServices implements PipelineServices
type pipelineServices struct {
	lifecycle      Lifecycle
	workerRegistry WorkerRegistry
	workerHealth   WorkerHealthMonitor
	jobScheduler   JobScheduler
	monitor        Monitor
}

// NewPipelineServices creates a new PipelineServices container with all services
func NewPipelineServices(logger *slog.Logger) PipelineServices {
	// Create the pipeline service
	pipelineService := NewPipelineService(logger)

	// Create focused services wrapping the pipeline service
	return &pipelineServices{
		lifecycle:      NewPipelineLifecycle(pipelineService),
		workerRegistry: NewWorkerRegistry(pipelineService),
		workerHealth:   NewWorkerHealthMonitor(pipelineService),
		jobScheduler:   NewJobScheduler(pipelineService),
		monitor:        NewPipelineMonitor(pipelineService),
	}
}

// NewPipelineServicesFromService creates services from an existing service
// This is useful for gradual migration
func NewPipelineServicesFromService(service Service) PipelineServices {
	return &pipelineServices{
		lifecycle:      NewPipelineLifecycle(service),
		workerRegistry: NewWorkerRegistry(service),
		workerHealth:   NewWorkerHealthMonitor(service),
		jobScheduler:   NewJobScheduler(service),
		monitor:        NewPipelineMonitor(service),
	}
}

func (p *pipelineServices) Lifecycle() Lifecycle {
	return p.lifecycle
}

func (p *pipelineServices) WorkerRegistry() WorkerRegistry {
	return p.workerRegistry
}

func (p *pipelineServices) WorkerHealth() WorkerHealthMonitor {
	return p.workerHealth
}

func (p *pipelineServices) JobScheduler() JobScheduler {
	return p.jobScheduler
}

func (p *pipelineServices) Monitor() Monitor {
	return p.monitor
}

// ServiceManagerAdapter adapts a Service to provide Manager interface compatibility
// This allows the existing pipeline services to work with the new Service interface
type ServiceManagerAdapter struct {
	service Service
	logger  *slog.Logger
}

// All Manager methods delegate to the service
func (a *ServiceManagerAdapter) Start() error {
	return a.service.Start(context.Background())
}

func (a *ServiceManagerAdapter) Stop() error {
	return a.service.Stop(context.Background())
}

func (a *ServiceManagerAdapter) IsRunning() bool {
	return a.service.IsRunning()
}

func (a *ServiceManagerAdapter) GetStatus() Status {
	return a.service.GetStatus()
}

func (a *ServiceManagerAdapter) GetConfig() *PipelineConfig {
	return a.service.GetConfig()
}

func (a *ServiceManagerAdapter) UpdateConfig(config *PipelineConfig) error {
	return a.service.UpdateConfig(context.Background(), config)
}

func (a *ServiceManagerAdapter) RegisterWorker(worker BackgroundWorker) error {
	return a.service.RegisterWorker(context.Background(), worker)
}

func (a *ServiceManagerAdapter) UnregisterWorker(name string) error {
	return a.service.UnregisterWorker(context.Background(), name)
}

func (a *ServiceManagerAdapter) RestartWorker(name string) error {
	return a.service.RestartWorker(context.Background(), name)
}

func (a *ServiceManagerAdapter) RestartAllWorkers() error {
	return a.service.RestartAllWorkers(context.Background())
}

func (a *ServiceManagerAdapter) GetWorkerNames() []string {
	return a.service.GetWorkerNames()
}

func (a *ServiceManagerAdapter) GetWorkerHealth(name string) (WorkerHealth, error) {
	return a.service.GetWorkerHealth(context.Background(), name)
}

func (a *ServiceManagerAdapter) GetAllWorkerHealth() map[string]WorkerHealth {
	return a.service.GetAllWorkerHealth(context.Background())
}

func (a *ServiceManagerAdapter) GetWorkerStatus(name string) (WorkerStatus, error) {
	return a.service.GetWorkerStatus(context.Background(), name)
}

func (a *ServiceManagerAdapter) GetAllWorkerStatuses() map[string]WorkerStatus {
	return a.service.GetAllWorkerStatuses(context.Background())
}

func (a *ServiceManagerAdapter) IsHealthy() bool {
	return a.service.IsHealthy()
}

func (a *ServiceManagerAdapter) SubmitJob(job *Job) error {
	return a.service.SubmitJob(context.Background(), job)
}

func (a *ServiceManagerAdapter) GetJob(jobID string) (*Job, bool) {
	return a.service.GetJob(context.Background(), jobID)
}

func (a *ServiceManagerAdapter) ListJobs(status JobStatus) []*Job {
	return a.service.ListJobs(context.Background(), status)
}

func (a *ServiceManagerAdapter) CancelJob(jobID string) error {
	return a.service.CancelJob(context.Background(), jobID)
}

func (a *ServiceManagerAdapter) GetManagerStats() ManagerStats {
	return a.service.GetManagerStats(context.Background())
}

func (a *ServiceManagerAdapter) GetOrchestratorStats() OrchestratorStats {
	return a.service.GetOrchestratorStats(context.Background())
}

// NewPipelineServicesFromContainer creates pipeline services from a service container
// This demonstrates the service container approach in practice
func NewPipelineServicesFromContainer(_ services.ServiceContainer) PipelineServices {
	// For now, create pipeline services with a legacy approach
	// In the future, this would wire services from the container
	logger := slog.Default()
	return NewPipelineServices(logger)
}
