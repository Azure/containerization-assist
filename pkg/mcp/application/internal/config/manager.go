package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/constants"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"gopkg.in/yaml.v3"
)

// ConfigManager provides centralized configuration management for MCP
type ConfigManager struct {
	Server    *ServerConfig    `yaml:"server" json:"server"`
	Analyzer  *AnalyzerConfig  `yaml:"analyzer" json:"analyzer"`
	Transport *TransportConfig `yaml:"transport" json:"transport"`
	Docker    *DockerConfig    `yaml:"docker" json:"docker"`
	loaded    bool
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host                  string        `yaml:"host" json:"host" env:"MCP_SERVER_HOST"`
	Port                  int           `yaml:"port" json:"port" env:"MCP_SERVER_PORT"`
	ReadTimeout           time.Duration `yaml:"read_timeout" json:"read_timeout" env:"MCP_SERVER_READ_TIMEOUT"`
	WriteTimeout          time.Duration `yaml:"write_timeout" json:"write_timeout" env:"MCP_SERVER_WRITE_TIMEOUT"`
	MaxRequestSize        int64         `yaml:"max_request_size" json:"max_request_size" env:"MCP_SERVER_MAX_REQUEST_SIZE"`
	EnableProfiling       bool          `yaml:"enable_profiling" json:"enable_profiling" env:"MCP_SERVER_ENABLE_PROFILING"`
	ProfilingPort         int           `yaml:"profiling_port" json:"profiling_port" env:"MCP_SERVER_PROFILING_PORT"`
	LogLevel              string        `yaml:"log_level" json:"log_level" env:"MCP_SERVER_LOG_LEVEL"`
	EnableMetrics         bool          `yaml:"enable_metrics" json:"enable_metrics" env:"MCP_SERVER_ENABLE_METRICS"`
	MetricsPort           int           `yaml:"metrics_port" json:"metrics_port" env:"MCP_SERVER_METRICS_PORT"`
	WorkspaceBase         string        `yaml:"workspace_base" json:"workspace_base" env:"MCP_SERVER_WORKSPACE_BASE"`
	MaxConcurrentSessions int           `yaml:"max_concurrent_sessions" json:"max_concurrent_sessions" env:"MCP_SERVER_MAX_CONCURRENT_SESSIONS"`
}

// AnalyzerConfig contains analyzer-related configuration
type AnalyzerConfig struct {
	EnableAI                 bool          `yaml:"enable_ai" json:"enable_ai" env:"MCP_ANALYZER_ENABLE_AI"`
	AIAnalyzerLogLevel       string        `yaml:"ai_log_level" json:"ai_log_level" env:"MCP_ANALYZER_AI_LOG_LEVEL"`
	MaxAnalysisTime          time.Duration `yaml:"max_analysis_time" json:"max_analysis_time" env:"MCP_ANALYZER_MAX_ANALYSIS_TIME"`
	EnableFileDetection      bool          `yaml:"enable_file_detection" json:"enable_file_detection" env:"MCP_ANALYZER_ENABLE_FILE_DETECTION"`
	EnableLanguageDetection  bool          `yaml:"enable_language_detection" json:"enable_language_detection" env:"MCP_ANALYZER_ENABLE_LANGUAGE_DETECTION"`
	EnableDependencyScanning bool          `yaml:"enable_dependency_scanning" json:"enable_dependency_scanning" env:"MCP_ANALYZER_ENABLE_DEPENDENCY_SCANNING"`
	CacheResults             bool          `yaml:"cache_results" json:"cache_results" env:"MCP_ANALYZER_CACHE_RESULTS"`
	CacheTTL                 time.Duration `yaml:"cache_ttl" json:"cache_ttl" env:"MCP_ANALYZER_CACHE_TTL"`
}

// TransportConfig contains transport-related configuration
type TransportConfig struct {
	Type              string        `yaml:"type" json:"type" env:"MCP_TRANSPORT_TYPE"`
	BufferSize        int           `yaml:"buffer_size" json:"buffer_size" env:"MCP_TRANSPORT_BUFFER_SIZE"`
	ReadTimeout       time.Duration `yaml:"read_timeout" json:"read_timeout" env:"MCP_TRANSPORT_READ_TIMEOUT"`
	WriteTimeout      time.Duration `yaml:"write_timeout" json:"write_timeout" env:"MCP_TRANSPORT_WRITE_TIMEOUT"`
	EnableCompression bool          `yaml:"enable_compression" json:"enable_compression" env:"MCP_TRANSPORT_ENABLE_COMPRESSION"`
	LogLevel          string        `yaml:"log_level" json:"log_level" env:"MCP_TRANSPORT_LOG_LEVEL"`
}

// DockerConfig contains Docker-related configuration
type DockerConfig struct {
	Username      string        `yaml:"username" json:"username" env:"MCP_DOCKER_USERNAME"`
	Password      string        `yaml:"password" json:"password" env:"MCP_DOCKER_PASSWORD"`
	Registry      string        `yaml:"registry" json:"registry" env:"MCP_DOCKER_REGISTRY"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout" env:"MCP_DOCKER_TIMEOUT"`
	EnableCache   bool          `yaml:"enable_cache" json:"enable_cache" env:"MCP_DOCKER_ENABLE_CACHE"`
	BuildTimeout  time.Duration `yaml:"build_timeout" json:"build_timeout" env:"MCP_DOCKER_BUILD_TIMEOUT"`
	PushTimeout   time.Duration `yaml:"push_timeout" json:"push_timeout" env:"MCP_DOCKER_PUSH_TIMEOUT"`
	PullTimeout   time.Duration `yaml:"pull_timeout" json:"pull_timeout" env:"MCP_DOCKER_PULL_TIMEOUT"`
	MaxConcurrent int           `yaml:"max_concurrent" json:"max_concurrent" env:"MCP_DOCKER_MAX_CONCURRENT"`
}

// WorkerConfig contains worker-related configuration for background processing
type WorkerConfig struct {
	MaxWorkers      int           `yaml:"max_workers" json:"max_workers" env:"MCP_WORKER_MAX_WORKERS"`
	WorkerTimeout   time.Duration `yaml:"worker_timeout" json:"worker_timeout" env:"MCP_WORKER_TIMEOUT"`
	QueueSize       int           `yaml:"queue_size" json:"queue_size" env:"MCP_WORKER_QUEUE_SIZE"`
	RetryAttempts   int           `yaml:"retry_attempts" json:"retry_attempts" env:"MCP_WORKER_RETRY_ATTEMPTS"`
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay" env:"MCP_WORKER_RETRY_DELAY"`
	EnableMetrics   bool          `yaml:"enable_metrics" json:"enable_metrics" env:"MCP_WORKER_ENABLE_METRICS"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout" env:"MCP_WORKER_SHUTDOWN_TIMEOUT"`
}

// NewConfigManager creates a new configuration manager with default values
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		Server:    defaultServerConfig(),
		Analyzer:  defaultAnalyzerConfig(),
		Transport: defaultTransportConfig(),
		Docker:    defaultDockerConfig(),
		loaded:    false,
	}
}

// LoadConfig loads configuration from multiple sources in priority order:
// 1. Environment variables (highest priority)
// 2. Configuration file
// 3. Defaults (lowest priority)
func (cm *ConfigManager) LoadConfig(configPath string) error {
	// Load from file if specified and exists
	if configPath != "" {
		if err := cm.loadFromFile(configPath); err != nil {
			return errors.NewError().Messagef("config_file_load_failed: Failed to load configuration file %s: %v", configPath, err).Build(

			// Try default locations
			)
		}
	} else {

		defaultPaths := []string{
			"./mcptypes.yaml",
			"./mcptypes.yml",
			os.Getenv("HOME") + "/.mcp/config.yaml",
			"/etc/mcp/config.yaml",
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				if err := cm.loadFromFile(path); err != nil {
					return errors.NewError().Message(fmt.Sprintf("failed to load config file %s", path)).Cause(err).Build()
				}
				break
			}
		}
	}

	// Override with environment variables (highest priority)
	if err := cm.loadFromEnv(); err != nil {
		return errors.NewError().Message("failed to load environment variables").Cause(err).WithLocation(

		// Validate configuration
		).Build()
	}

	if err := cm.validate(); err != nil {
		return errors.NewError().Message("configuration validation failed").Cause(err).Build()
	}

	cm.loaded = true
	return nil
}

// loadFromFile loads configuration from a YAML file
func (cm *ConfigManager) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Expand environment variables in the file content
	expanded := os.ExpandEnv(string(data))

	return yaml.Unmarshal([]byte(expanded), cm)
}

// loadFromEnv loads configuration from environment variables
func (cm *ConfigManager) loadFromEnv() error {
	// Load server config
	if err := loadEnvVars(cm.Server); err != nil {
		return fmt.Errorf("failed to load server config from env: %w", err)
	}

	// Load analyzer config
	if err := loadEnvVars(cm.Analyzer); err != nil {
		return fmt.Errorf("failed to load analyzer config from env: %w", err)
	}

	// Load transport config
	if err := loadEnvVars(cm.Transport); err != nil {
		return fmt.Errorf("failed to load transport config from env: %w", err)
	}

	// Load docker config
	if err := loadEnvVars(cm.Docker); err != nil {
		return fmt.Errorf("failed to load docker config from env: %w", err)
	}

	return nil
}

// validate validates the loaded configuration
func (cm *ConfigManager) validate() error {
	// Validate server config
	if cm.Server.Port <= 0 || cm.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cm.Server.Port)
	}

	if cm.Server.ReadTimeout <= 0 {
		return fmt.Errorf("invalid read timeout: %v", cm.Server.ReadTimeout)
	}

	if cm.Server.WriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v", cm.Server.WriteTimeout)
	}

	// Validate analyzer config
	if cm.Analyzer.MaxAnalysisTime <= 0 {
		return fmt.Errorf("invalid max analysis time: %v", cm.Analyzer.MaxAnalysisTime)
	}

	// Validate transport config
	if cm.Transport.BufferSize <= 0 {
		return fmt.Errorf("invalid transport buffer size: %d", cm.Transport.BufferSize)
	}

	// Validate docker config
	if cm.Docker.MaxConcurrent <= 0 {
		return fmt.Errorf("invalid docker max concurrent: %d", cm.Docker.MaxConcurrent)
	}

	return nil
}

// IsLoaded returns true if configuration has been loaded
func (cm *ConfigManager) IsLoaded() bool {
	return cm.loaded
}

// Note: Configuration fields are exported for direct access
// Use cm.Server, cm.Analyzer, cm.Transport, cm.Observability, cm.Docker directly

// Default configuration functions

func defaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:                  constants.DefaultHost,
		Port:                  constants.DefaultPort,
		ReadTimeout:           constants.DefaultTimeout,
		WriteTimeout:          constants.DefaultTimeout,
		MaxRequestSize:        constants.DefaultMaxRequestSize,
		EnableProfiling:       false,
		ProfilingPort:         constants.DefaultProfilingPort,
		LogLevel:              constants.DefaultLogLevel,
		EnableMetrics:         true,
		MetricsPort:           constants.DefaultMetricsPort,
		WorkspaceBase:         constants.DefaultWorkspaceBase,
		MaxConcurrentSessions: constants.DefaultMaxConcurrentSessions,
	}
}

func defaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		EnableAI:                 false,
		AIAnalyzerLogLevel:       constants.DefaultLogLevel,
		MaxAnalysisTime:          constants.DefaultMaxAnalysisTime,
		EnableFileDetection:      true,
		EnableLanguageDetection:  true,
		EnableDependencyScanning: true,
		CacheResults:             true,
		CacheTTL:                 constants.DefaultCacheTTL,
	}
}

func defaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		Type:              constants.DefaultTransportType,
		BufferSize:        constants.DefaultBufferSize,
		ReadTimeout:       constants.DefaultTimeout,
		WriteTimeout:      constants.DefaultTimeout,
		EnableCompression: false,
		LogLevel:          constants.DefaultLogLevel,
	}
}

func defaultDockerConfig() *DockerConfig {
	return &DockerConfig{
		Username:      "",
		Password:      "",
		Registry:      constants.DefaultDockerRegistry,
		Timeout:       constants.DefaultDockerTimeout,
		EnableCache:   true,
		BuildTimeout:  constants.DefaultDockerBuildTimeout,
		PushTimeout:   constants.DefaultDockerPushTimeout,
		PullTimeout:   constants.DefaultDockerPullTimeout,
		MaxConcurrent: constants.DefaultMaxConcurrentDocker,
	}
}

// Helper function to load environment variables into a struct using reflection
func loadEnvVars(configStruct interface{}) error {
	// This is a simplified version - in production, you might want to use reflection
	// or a more sophisticated approach for automatic env var loading

	switch v := configStruct.(type) {
	case *ServerConfig:
		loadServerEnvVars(v)
	case *AnalyzerConfig:
		loadAnalyzerEnvVars(v)
	case *TransportConfig:
		loadTransportEnvVars(v)
	case *DockerConfig:
		loadDockerEnvVars(v)
	}

	return nil
}

func loadServerEnvVars(cfg *ServerConfig) {
	if val := os.Getenv("MCP_SERVER_HOST"); val != "" {
		cfg.Host = val
	}
	if val := os.Getenv("MCP_SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Port = port
		}
	}
	if val := os.Getenv("MCP_SERVER_READ_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.ReadTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_SERVER_WRITE_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.WriteTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_SERVER_MAX_REQUEST_SIZE"); val != "" {
		if size, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxRequestSize = size
		}
	}
	if val := os.Getenv("MCP_SERVER_ENABLE_PROFILING"); val != "" {
		cfg.EnableProfiling = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_SERVER_PROFILING_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.ProfilingPort = port
		}
	}
	if val := os.Getenv("MCP_SERVER_LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}
	if val := os.Getenv("MCP_SERVER_ENABLE_METRICS"); val != "" {
		cfg.EnableMetrics = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_SERVER_METRICS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.MetricsPort = port
		}
	}
	if val := os.Getenv("MCP_SERVER_WORKSPACE_BASE"); val != "" {
		cfg.WorkspaceBase = val
	}
	if val := os.Getenv("MCP_SERVER_MAX_CONCURRENT_SESSIONS"); val != "" {
		if sessions, err := strconv.Atoi(val); err == nil {
			cfg.MaxConcurrentSessions = sessions
		}
	}
}

func loadAnalyzerEnvVars(cfg *AnalyzerConfig) {
	if val := os.Getenv("MCP_ANALYZER_ENABLE_AI"); val != "" {
		cfg.EnableAI = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_ANALYZER_AI_LOG_LEVEL"); val != "" {
		cfg.AIAnalyzerLogLevel = val
	}
	if val := os.Getenv("MCP_ANALYZER_MAX_ANALYSIS_TIME"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.MaxAnalysisTime = timeout
		}
	}
	if val := os.Getenv("MCP_ANALYZER_ENABLE_FILE_DETECTION"); val != "" {
		cfg.EnableFileDetection = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_ANALYZER_ENABLE_LANGUAGE_DETECTION"); val != "" {
		cfg.EnableLanguageDetection = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_ANALYZER_ENABLE_DEPENDENCY_SCANNING"); val != "" {
		cfg.EnableDependencyScanning = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_ANALYZER_CACHE_RESULTS"); val != "" {
		cfg.CacheResults = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_ANALYZER_CACHE_TTL"); val != "" {
		if ttl, err := time.ParseDuration(val); err == nil {
			cfg.CacheTTL = ttl
		}
	}
}

func loadTransportEnvVars(cfg *TransportConfig) {
	if val := os.Getenv("MCP_TRANSPORT_TYPE"); val != "" {
		cfg.Type = val
	}
	if val := os.Getenv("MCP_TRANSPORT_BUFFER_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			cfg.BufferSize = size
		}
	}
	if val := os.Getenv("MCP_TRANSPORT_READ_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.ReadTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_TRANSPORT_WRITE_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.WriteTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_TRANSPORT_ENABLE_COMPRESSION"); val != "" {
		cfg.EnableCompression = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_TRANSPORT_LOG_LEVEL"); val != "" {
		cfg.LogLevel = val
	}
}

func loadDockerEnvVars(cfg *DockerConfig) {
	if val := os.Getenv("MCP_DOCKER_USERNAME"); val != "" {
		cfg.Username = val
	}
	if val := os.Getenv("MCP_DOCKER_PASSWORD"); val != "" {
		cfg.Password = val
	}
	if val := os.Getenv("MCP_DOCKER_REGISTRY"); val != "" {
		cfg.Registry = val
	}
	if val := os.Getenv("MCP_DOCKER_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.Timeout = timeout
		}
	}
	if val := os.Getenv("MCP_DOCKER_ENABLE_CACHE"); val != "" {
		cfg.EnableCache = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("MCP_DOCKER_BUILD_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.BuildTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_DOCKER_PUSH_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.PushTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_DOCKER_PULL_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			cfg.PullTimeout = timeout
		}
	}
	if val := os.Getenv("MCP_DOCKER_MAX_CONCURRENT"); val != "" {
		if concurrent, err := strconv.Atoi(val); err == nil {
			cfg.MaxConcurrent = concurrent
		}
	}
}
