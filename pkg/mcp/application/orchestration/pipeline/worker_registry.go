package pipeline

import "context"

// WorkerRegistry manages registration and discovery of workers
type WorkerRegistry interface {
	// Register adds a new worker to the registry
	Register(worker BackgroundWorker) error

	// Unregister removes a worker from the registry
	Unregister(name string) error

	// Get retrieves a worker by name
	Get(name string) (BackgroundWorker, error)

	// List returns all registered workers
	List() []string
}

// workerRegistry implements WorkerRegistry
type workerRegistry struct {
	service Service
}

// NewWorkerRegistry creates a new WorkerRegistry service
func NewWorkerRegistry(service Service) WorkerRegistry {
	return &workerRegistry{
		service: service,
	}
}

func (w *workerRegistry) Register(worker BackgroundWorker) error {
	return w.service.RegisterWorker(context.Background(), worker)
}

func (w *workerRegistry) Unregister(name string) error {
	return w.service.UnregisterWorker(context.Background(), name)
}

func (w *workerRegistry) Get(_ string) (BackgroundWorker, error) {
	// This method needs to be added to the manager or worker manager
	// For now, return an error indicating it's not implemented
	return nil, ErrWorkerNotFound
}

func (w *workerRegistry) List() []string {
	return w.service.GetWorkerNames()
}
