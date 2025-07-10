package pipeline

import "context"

// Monitor provides monitoring and statistics for the pipeline
type Monitor interface {
	// GetStatus returns the overall pipeline status
	GetStatus() (*Status, error)

	// GetWorkerStats returns statistics about workers
	GetWorkerStats() ManagerStats

	// GetJobStats returns statistics about jobs
	GetJobStats() OrchestratorStats

	// GetConfiguration returns the current pipeline configuration
	GetConfiguration() *PipelineConfig
}

// pipelineMonitor implements Monitor
type pipelineMonitor struct {
	service Service
}

// NewPipelineMonitor creates a new Monitor service
func NewPipelineMonitor(service Service) Monitor {
	return &pipelineMonitor{
		service: service,
	}
}

func (p *pipelineMonitor) GetStatus() (*Status, error) {
	status := p.service.GetStatus()
	return &status, nil
}

func (p *pipelineMonitor) GetWorkerStats() ManagerStats {
	return p.service.GetManagerStats(context.Background())
}

func (p *pipelineMonitor) GetJobStats() OrchestratorStats {
	return p.service.GetOrchestratorStats(context.Background())
}

func (p *pipelineMonitor) GetConfiguration() *PipelineConfig {
	return p.service.GetConfig()
}
