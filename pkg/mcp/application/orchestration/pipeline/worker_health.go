package pipeline

import "context"

// WorkerHealthMonitor monitors and manages worker health
type WorkerHealthMonitor interface {
	// GetHealth returns the health status of a specific worker
	GetHealth(workerName string) (WorkerHealth, error)

	// GetAllHealth returns health status for all workers
	GetAllHealth() map[string]WorkerHealth

	// RestartWorker restarts a specific worker
	RestartWorker(workerName string) error

	// IsHealthy returns true if all workers are healthy
	IsHealthy() bool
}

// workerHealthMonitor implements WorkerHealthMonitor
type workerHealthMonitor struct {
	service Service
}

// NewWorkerHealthMonitor creates a new WorkerHealthMonitor service
func NewWorkerHealthMonitor(service Service) WorkerHealthMonitor {
	return &workerHealthMonitor{
		service: service,
	}
}

func (w *workerHealthMonitor) GetHealth(workerName string) (WorkerHealth, error) {
	return w.service.GetWorkerHealth(context.Background(), workerName)
}

func (w *workerHealthMonitor) GetAllHealth() map[string]WorkerHealth {
	return w.service.GetAllWorkerHealth(context.Background())
}

func (w *workerHealthMonitor) RestartWorker(workerName string) error {
	return w.service.RestartWorker(context.Background(), workerName)
}

func (w *workerHealthMonitor) IsHealthy() bool {
	return w.service.IsHealthy()
}
