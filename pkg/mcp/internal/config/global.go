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
// DEPRECATED: Use GetWithDefault() or check Get() error instead
func MustGet() *ConfigManager {
	cfg, err := Get()
	if err != nil {
		// TODO: Replace panic with proper error handling
		panic(fmt.Sprintf("configuration not available: %v", err))
	}
	return cfg
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
	return cfg.GetServerConfig(), nil
}

// MustGetServer returns the server configuration or panics
// DEPRECATED: Use GetServer() error handling instead
func MustGetServer() *ServerConfig {
	return MustGet().GetServerConfig()
}

// GetServerWithDefault returns the server configuration or a default if not available
func GetServerWithDefault() *ServerConfig {
	return GetWithDefault().GetServerConfig()
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
// DEPRECATED: Use GetAnalyzer() error handling instead
func MustGetAnalyzer() *AnalyzerConfig {
	return MustGet().GetAnalyzerConfig()
}

// GetAnalyzerWithDefault returns the analyzer configuration or a default if not available
func GetAnalyzerWithDefault() *AnalyzerConfig {
	return GetWithDefault().GetAnalyzerConfig()
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
// DEPRECATED: Use GetTransport() error handling instead
func MustGetTransport() *TransportConfig {
	return MustGet().GetTransportConfig()
}

// GetTransportWithDefault returns the transport configuration or a default if not available
func GetTransportWithDefault() *TransportConfig {
	return GetWithDefault().GetTransportConfig()
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
// DEPRECATED: Use GetObservability() error handling instead
func MustGetObservability() *ObservabilityConfig {
	return MustGet().GetObservabilityConfig()
}

// GetObservabilityWithDefault returns the observability configuration or a default if not available
func GetObservabilityWithDefault() *ObservabilityConfig {
	return GetWithDefault().GetObservabilityConfig()
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
// DEPRECATED: Use GetDocker() error handling instead
func MustGetDocker() *DockerConfig {
	return MustGet().GetDockerConfig()
}

// GetDockerWithDefault returns the Docker configuration or a default if not available
func GetDockerWithDefault() *DockerConfig {
	return GetWithDefault().GetDockerConfig()
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
