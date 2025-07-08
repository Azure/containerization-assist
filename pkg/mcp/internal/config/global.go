package config

import (
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// Global configuration instance
var (
	globalConfig     *ConfigManager
	globalConfigMu   sync.RWMutex
	globalConfigOnce sync.Once
	initialized      bool
)

// Reset resets the global configuration (useful for testing)
func Reset() {
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()

	globalConfig = nil
	initialized = false
	globalConfigOnce = sync.Once{}
}

// Initialize initializes the global configuration with a config path
func Initialize(configPath string) error {
	var initErr error

	globalConfigOnce.Do(func() {
		globalConfigMu.Lock()
		defer globalConfigMu.Unlock()

		globalConfig = NewConfigManager()
		initErr = globalConfig.LoadConfig(configPath)
		if initErr == nil {
			initialized = true
		}
	})

	return initErr
}

// Get returns the global configuration
func Get() (*ConfigManager, error) {
	globalConfigMu.RLock()
	defer globalConfigMu.RUnlock()

	if globalConfig == nil {
		return nil, errors.NewError().Message("configuration not initialized - call Initialize() first").Build()
	}

	if !initialized {
		return nil, errors.NewError().Message("configuration not loaded").Build()
	}

	return globalConfig, nil
}

// IsInitialized returns true if the global configuration has been initialized
func IsInitialized() bool {
	globalConfigMu.RLock()
	defer globalConfigMu.RUnlock()

	return globalConfig != nil && initialized
}

// GetServer returns the server configuration from the global config
func GetServer() (*ServerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Server, nil
}

// GetAnalyzer returns the analyzer configuration from the global config
func GetAnalyzer() (*AnalyzerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Analyzer, nil
}

// GetTransport returns the transport configuration from the global config
func GetTransport() (*TransportConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Transport, nil
}

// GetDocker returns the Docker configuration from the global config
func GetDocker() (*DockerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Docker, nil
}
