package config

import (
	"fmt"
	"sync"
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
		return nil, fmt.Errorf("configuration not initialized - call config.Initialize() first")
	}

	if !globalConfig.IsLoaded() {
		return nil, fmt.Errorf("configuration not loaded")
	}

	return globalConfig, nil
}

// MustGet returns the global configuration manager or panics if not initialized
// Use this only when you're certain the configuration has been initialized
func MustGet() *ConfigManager {
	cfg, err := Get()
	if err != nil {
		panic(fmt.Sprintf("configuration not available: %v", err))
	}
	return cfg
}

// GetServer returns the server configuration
func GetServer() (*ServerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.GetServerConfig(), nil
}

// MustGetServer returns the server configuration or panics
func MustGetServer() *ServerConfig {
	return MustGet().GetServerConfig()
}

// GetAnalyzer returns the analyzer configuration
func GetAnalyzer() (*AnalyzerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.GetAnalyzerConfig(), nil
}

// MustGetAnalyzer returns the analyzer configuration or panics
func MustGetAnalyzer() *AnalyzerConfig {
	return MustGet().GetAnalyzerConfig()
}

// GetTransport returns the transport configuration
func GetTransport() (*TransportConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.GetTransportConfig(), nil
}

// MustGetTransport returns the transport configuration or panics
func MustGetTransport() *TransportConfig {
	return MustGet().GetTransportConfig()
}

// GetObservability returns the observability configuration
func GetObservability() (*ObservabilityConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.GetObservabilityConfig(), nil
}

// MustGetObservability returns the observability configuration or panics
func MustGetObservability() *ObservabilityConfig {
	return MustGet().GetObservabilityConfig()
}

// GetDocker returns the Docker configuration
func GetDocker() (*DockerConfig, error) {
	cfg, err := Get()
	if err != nil {
		return nil, err
	}
	return cfg.GetDockerConfig(), nil
}

// MustGetDocker returns the Docker configuration or panics
func MustGetDocker() *DockerConfig {
	return MustGet().GetDockerConfig()
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
