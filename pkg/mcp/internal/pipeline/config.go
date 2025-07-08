package pipeline

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/config"
	errors "github.com/Azure/container-kit/pkg/mcp/internal"
)

// ConfigLoader loads and validates pipeline configuration
type ConfigLoader struct {
	serverConfig *config.ServerConfig
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(serverConfig *config.ServerConfig) *ConfigLoader {
	return &ConfigLoader{
		serverConfig: serverConfig,
	}
}

// LoadPipelineConfig loads pipeline configuration from server config
func (cl *ConfigLoader) LoadPipelineConfig() (*PipelineConfig, error) {
	if cl.serverConfig == nil {
		return DefaultPipelineConfig(), nil
	}

	pipelineConfig := &PipelineConfig{
		WorkerPoolSize:      cl.serverConfig.MaxWorkers,
		MaxConcurrentJobs:   cl.serverConfig.MaxConcurrentRequests,
		JobTimeout:          cl.serverConfig.JobTTL,
		HealthCheckInterval: cl.serverConfig.HealthCheckInterval,
	}

	if err := cl.validatePipelineConfig(pipelineConfig); err != nil {
		return nil, errors.NewError().Message("invalid pipeline configuration").Cause(err).WithLocation().Build()
	}

	return pipelineConfig, nil
}

// LoadWorkerConfig loads worker configuration from server config
func (cl *ConfigLoader) LoadWorkerConfig() (*config.WorkerConfig, error) {
	if cl.serverConfig == nil {
		return cl.defaultWorkerConfig(), nil
	}

	workerConfig := &config.WorkerConfig{
		ShutdownTimeout:   cl.serverConfig.ShutdownTimeout,
		HealthCheckPeriod: cl.serverConfig.HealthCheckInterval,
		MaxRetries:        3,
	}

	if err := cl.validateWorkerConfig(workerConfig); err != nil {
		return nil, errors.NewError().Message("invalid worker configuration").Cause(err).WithLocation().Build()
	}

	return workerConfig, nil
}

// validatePipelineConfig validates pipeline configuration
func (cl *ConfigLoader) validatePipelineConfig(pipelineConfig *PipelineConfig) error {
	if pipelineConfig.WorkerPoolSize < 1 {
		return fmt.Errorf("worker pool size must be at least 1, got %d", pipelineConfig.WorkerPoolSize)
	}

	if pipelineConfig.WorkerPoolSize > config.MaxGoroutines {
		return fmt.Errorf("worker pool size (%d) cannot exceed max goroutines (%d)",
			pipelineConfig.WorkerPoolSize, config.MaxGoroutines)
	}

	if pipelineConfig.MaxConcurrentJobs < 1 {
		return fmt.Errorf("max concurrent jobs must be at least 1, got %d", pipelineConfig.MaxConcurrentJobs)
	}

	if pipelineConfig.JobTimeout <= 0 {
		return fmt.Errorf("job timeout must be positive, got %v", pipelineConfig.JobTimeout)
	}

	if pipelineConfig.HealthCheckInterval <= 0 {
		return fmt.Errorf("health check interval must be positive, got %v", pipelineConfig.HealthCheckInterval)
	}

	return nil
}

// validateWorkerConfig validates worker configuration
func (cl *ConfigLoader) validateWorkerConfig(workerConfig *config.WorkerConfig) error {
	if workerConfig.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive, got %v", workerConfig.ShutdownTimeout)
	}

	if workerConfig.HealthCheckPeriod <= 0 {
		return fmt.Errorf("health check period must be positive, got %v", workerConfig.HealthCheckPeriod)
	}

	if workerConfig.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative, got %d", workerConfig.MaxRetries)
	}

	return nil
}

// defaultWorkerConfig returns default worker configuration
func (cl *ConfigLoader) defaultWorkerConfig() *config.WorkerConfig {
	return &config.WorkerConfig{
		ShutdownTimeout:   config.DefaultShutdownTimeout,
		HealthCheckPeriod: config.DefaultHealthCheckInterval,
		MaxRetries:        3,
	}
}

// ExtendPipelineConfig extends the basic PipelineConfig with additional fields
type ExtendPipelineConfig struct {
	*PipelineConfig

	MaxGoroutines      int           `yaml:"max_goroutines" json:"max_goroutines"`
	WorkerRestartDelay time.Duration `yaml:"worker_restart_delay" json:"worker_restart_delay"`
	JobRetryAttempts   int           `yaml:"job_retry_attempts" json:"job_retry_attempts"`
	JobRetryDelay      time.Duration `yaml:"job_retry_delay" json:"job_retry_delay"`
	EnableMetrics      bool          `yaml:"enable_metrics" json:"enable_metrics"`
	EnableTracing      bool          `yaml:"enable_tracing" json:"enable_tracing"`
	LogLevel           string        `yaml:"log_level" json:"log_level"`

	MaxMemoryPerWorker int64   `yaml:"max_memory_per_worker" json:"max_memory_per_worker"`
	MaxCPUPerWorker    float64 `yaml:"max_cpu_per_worker" json:"max_cpu_per_worker"`
	MaxDiskPerWorker   int64   `yaml:"max_disk_per_worker" json:"max_disk_per_worker"`

	JobQueueSize    int           `yaml:"job_queue_size" json:"job_queue_size"`
	JobQueueTimeout time.Duration `yaml:"job_queue_timeout" json:"job_queue_timeout"`
	PriorityLevels  int           `yaml:"priority_levels" json:"priority_levels"`

	JobHistoryRetention    time.Duration `yaml:"job_history_retention" json:"job_history_retention"`
	WorkerMetricsRetention time.Duration `yaml:"worker_metrics_retention" json:"worker_metrics_retention"`
	CleanupInterval        time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// DefaultExtendedPipelineConfig returns extended configuration with defaults
func DefaultExtendedPipelineConfig() *ExtendPipelineConfig {
	base := DefaultPipelineConfig()

	return &ExtendPipelineConfig{
		PipelineConfig: base,

		MaxGoroutines:      config.MaxGoroutines,
		WorkerRestartDelay: 5 * time.Second,
		JobRetryAttempts:   3,
		JobRetryDelay:      time.Minute,
		EnableMetrics:      true,
		EnableTracing:      false,
		LogLevel:           config.DefaultLogLevel,

		MaxMemoryPerWorker: 512 * 1024 * 1024,
		MaxCPUPerWorker:    1.0,
		MaxDiskPerWorker:   1024 * 1024 * 1024,

		JobQueueSize:    config.DefaultCacheSize,
		JobQueueTimeout: 30 * time.Second,
		PriorityLevels:  3,

		JobHistoryRetention:    24 * time.Hour,
		WorkerMetricsRetention: 7 * 24 * time.Hour,
		CleanupInterval:        time.Hour,
	}
}

// ApplyConfig applies configuration to an existing pipeline manager
func ApplyConfig(manager *Manager, serverConfig *config.ServerConfig) error {
	loader := NewConfigLoader(serverConfig)

	pipelineConfig, err := loader.LoadPipelineConfig()
	if err != nil {
		return errors.NewError().Message("failed to load pipeline configuration").Cause(err).WithLocation().Build()
	}

	return manager.UpdateConfig(pipelineConfig)
}

// ConfigValidator provides configuration validation utilities
type ConfigValidator struct{}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidateConfiguration validates the entire pipeline configuration
func (cv *ConfigValidator) ValidateConfiguration(config *ExtendPipelineConfig) error {
	if config.PipelineConfig == nil {
		return errors.NewError().Messagef("base pipeline configuration is required").WithLocation().Build()
	}

	loader := NewConfigLoader(nil)
	if err := loader.validatePipelineConfig(config.PipelineConfig); err != nil {
		return errors.NewError().Message("base configuration validation failed").Cause(err).WithLocation().Build()
	}

	if err := cv.validateExtendedConfig(config); err != nil {
		return errors.NewError().Message("extended configuration validation failed").Cause(err).WithLocation().Build()
	}

	return nil
}

func (cv *ConfigValidator) validateExtendedConfig(extendedConfig *ExtendPipelineConfig) error {
	if extendedConfig.MaxGoroutines < extendedConfig.WorkerPoolSize {
		return fmt.Errorf("max goroutines (%d) must be >= worker pool size (%d)",
			extendedConfig.MaxGoroutines, extendedConfig.WorkerPoolSize)
	}

	if extendedConfig.JobRetryAttempts < 0 {
		return fmt.Errorf("job retry attempts cannot be negative, got %d", extendedConfig.JobRetryAttempts)
	}

	if extendedConfig.JobRetryDelay < 0 {
		return fmt.Errorf("job retry delay cannot be negative, got %v", extendedConfig.JobRetryDelay)
	}

	if extendedConfig.MaxMemoryPerWorker <= 0 {
		return fmt.Errorf("max memory per worker must be positive, got %d", extendedConfig.MaxMemoryPerWorker)
	}

	if extendedConfig.MaxCPUPerWorker <= 0 {
		return fmt.Errorf("max CPU per worker must be positive, got %f", extendedConfig.MaxCPUPerWorker)
	}

	if extendedConfig.JobQueueSize <= 0 {
		return fmt.Errorf("job queue size must be positive, got %d", extendedConfig.JobQueueSize)
	}

	if extendedConfig.PriorityLevels < 1 {
		return fmt.Errorf("priority levels must be at least 1, got %d", extendedConfig.PriorityLevels)
	}

	return nil
}

// GetConfigSummary returns a summary of the current configuration
func GetConfigSummary(config *ExtendPipelineConfig) ConfigSummary {
	return ConfigSummary{
		WorkerPoolSize:      config.WorkerPoolSize,
		MaxConcurrentJobs:   config.MaxConcurrentJobs,
		JobTimeout:          config.JobTimeout.String(),
		HealthCheckInterval: config.HealthCheckInterval.String(),
		MaxGoroutines:       config.MaxGoroutines,
		JobQueueSize:        config.JobQueueSize,
		EnableMetrics:       config.EnableMetrics,
		EnableTracing:       config.EnableTracing,
		ResourceLimits: ResourceLimitsSummary{
			MemoryPerWorker: formatBytes(config.MaxMemoryPerWorker),
			CPUPerWorker:    config.MaxCPUPerWorker,
			DiskPerWorker:   formatBytes(config.MaxDiskPerWorker),
		},
	}
}

// ConfigSummary provides a human-readable summary of configuration
type ConfigSummary struct {
	WorkerPoolSize      int                   `json:"worker_pool_size"`
	MaxConcurrentJobs   int                   `json:"max_concurrent_jobs"`
	JobTimeout          string                `json:"job_timeout"`
	HealthCheckInterval string                `json:"health_check_interval"`
	MaxGoroutines       int                   `json:"max_goroutines"`
	JobQueueSize        int                   `json:"job_queue_size"`
	EnableMetrics       bool                  `json:"enable_metrics"`
	EnableTracing       bool                  `json:"enable_tracing"`
	ResourceLimits      ResourceLimitsSummary `json:"resource_limits"`
}

// ResourceLimitsSummary provides a summary of resource limits
type ResourceLimitsSummary struct {
	MemoryPerWorker string  `json:"memory_per_worker"`
	CPUPerWorker    float64 `json:"cpu_per_worker"`
	DiskPerWorker   string  `json:"disk_per_worker"`
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
