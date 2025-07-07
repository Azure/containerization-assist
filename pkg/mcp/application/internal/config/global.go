package config

import (
	"sync"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

var (
	// globalConfig holds the global configuration instance
	globalConfig *ConfigManager

	// configMutex protects access to the global configuration
	configMutex sync.RWMutex

	// initOnce ensures the global configuration is initialized only once
	initOnce sync.Once
)

// Initialize initializes the global configuration manager
// This should be called once at application startup
func Initialize(configPath string) error {
	var initErr error

	initOnce.Do(func() {
		configMutex.Lock()
		defer configMutex.Unlock()

		globalConfig = NewConfigManager()
		initErr = globalConfig.LoadConfig(configPath)
	})

	return initErr
}

// Get returns the global configuration manager
// Returns an error if configuration hasn't been initialized
func Get() (*ConfigManager, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfig == nil {
		return nil, errors.NewError().Messagef("configuration not initialized - call config.Initialize() first").Build()
	}

	if !globalConfig.IsLoaded() {
		return nil, errors.NewError().Messagef("configuration not loaded").Build()
	}

	return globalConfig, nil
}

// GetWithDefault returns the global configuration or a default configuration if not initialized
func GetWithDefault() *ConfigManager {
	cfg, err := Get()
	if err != nil {
		// Return a minimal default configuration
		return NewConfigManager()
	}
	return cfg
}

// GetServer returns the server configuration
func GetServer() (*ServerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Server, nil
}

// GetServerWithDefault returns the server configuration or a default if not available
func GetServerWithDefault() *ServerConfig {
	return GetWithDefault().Server
}

// GetAnalyzer returns the analyzer configuration
func GetAnalyzer() (*AnalyzerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Analyzer, nil
}

// GetAnalyzerWithDefault returns the analyzer configuration or a default if not available
func GetAnalyzerWithDefault() *AnalyzerConfig {
	return GetWithDefault().Analyzer
}

// GetTransport returns the transport configuration
func GetTransport() (*TransportConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Transport, nil
}

// GetTransportWithDefault returns the transport configuration or a default if not available
func GetTransportWithDefault() *TransportConfig {
	return GetWithDefault().Transport
}

// GetDocker returns the Docker configuration
func GetDocker() (*DockerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.Docker, nil
}

// GetDockerWithDefault returns the Docker configuration or a default if not available
func GetDockerWithDefault() *DockerConfig {
	return GetWithDefault().Docker
}

// IsInitialized returns true if the global configuration has been initialized
func IsInitialized() bool {
	configMutex.RLock()
	defer configMutex.RUnlock()

	return globalConfig != nil && globalConfig.IsLoaded()
}

// Reset resets the global configuration (useful for testing)
func Reset() {
	configMutex.Lock()
	defer configMutex.Unlock()

	globalConfig = nil
	initOnce = sync.Once{}
}

// SetTestConfig sets a test configuration (useful for testing)
func SetTestConfig(cfg *ConfigManager) {
	configMutex.Lock()
	defer configMutex.Unlock()

	globalConfig = cfg
	initOnce = sync.Once{} // Reset to allow re-initialization
}
