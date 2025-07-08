package config

import (
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// ConfigService provides configuration management without global state
type ConfigService struct {
	mu       sync.RWMutex
	manager  *ConfigManager
	loaded   bool
	initOnce sync.Once
}

// NewConfigService creates a new configuration service
func NewConfigService() *ConfigService {
	return &ConfigService{
		manager: NewConfigManager(),
		loaded:  false,
	}
}

// Initialize initializes the configuration service with a config path
func (c *ConfigService) Initialize(configPath string) error {
	var initErr error

	c.initOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.manager = NewConfigManager()
		initErr = c.manager.LoadConfig(configPath)
		if initErr == nil {
			c.loaded = true
		}
	})

	return initErr
}

// Get returns the configuration manager
func (c *ConfigService) Get() (*ConfigManager, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.manager == nil {
		return nil, errors.NewError().Messagef("configuration not initialized - call Initialize() first").Build()
	}

	if !c.loaded {
		return nil, errors.NewError().Messagef("configuration not loaded").Build()
	}

	return c.manager, nil
}

// GetWithDefault returns the configuration manager or creates a default one
func (c *ConfigService) GetWithDefault() *ConfigManager {
	c.mu.RLock()
	if c.manager != nil && c.loaded {
		defer c.mu.RUnlock()
		return c.manager
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check after acquiring write lock
	if c.manager != nil && c.loaded {
		return c.manager
	}

	// Create minimal default configuration
	c.manager = NewConfigManager()
	return c.manager
}

// GetServer returns the server configuration
func (c *ConfigService) GetServer() (*ServerConfig, error) {
	cfg, err := c.Get()
	if err != nil {
		return nil, err
	}
	return cfg.Server, nil
}

// GetServerWithDefault returns the server configuration or a default
func (c *ConfigService) GetServerWithDefault() *ServerConfig {
	return c.GetWithDefault().Server
}

// GetAnalyzer returns the analyzer configuration
func (c *ConfigService) GetAnalyzer() (*AnalyzerConfig, error) {
	cfg, err := c.Get()
	if err != nil {
		return nil, err
	}
	return cfg.Analyzer, nil
}

// GetAnalyzerWithDefault returns the analyzer configuration or a default
func (c *ConfigService) GetAnalyzerWithDefault() *AnalyzerConfig {
	return c.GetWithDefault().Analyzer
}

// GetTransport returns the transport configuration
func (c *ConfigService) GetTransport() (*TransportConfig, error) {
	cfg, err := c.Get()
	if err != nil {
		return nil, err
	}
	return cfg.Transport, nil
}

// GetTransportWithDefault returns the transport configuration or a default
func (c *ConfigService) GetTransportWithDefault() *TransportConfig {
	return c.GetWithDefault().Transport
}

// GetDocker returns the Docker configuration
func (c *ConfigService) GetDocker() (*DockerConfig, error) {
	cfg, err := c.Get()
	if err != nil {
		return nil, err
	}
	return cfg.Docker, nil
}

// GetDockerWithDefault returns the Docker configuration or a default
func (c *ConfigService) GetDockerWithDefault() *DockerConfig {
	return c.GetWithDefault().Docker
}

// IsInitialized returns true if the configuration has been initialized
func (c *ConfigService) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.manager != nil && c.loaded
}

// Reset resets the configuration service (useful for testing)
func (c *ConfigService) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.manager = nil
	c.loaded = false
	c.initOnce = sync.Once{}
}

// SetTestConfig sets a test configuration (useful for testing)
func (c *ConfigService) SetTestConfig(cfg *ConfigManager) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.manager = cfg
	c.loaded = true
	c.initOnce = sync.Once{} // Reset to allow re-initialization
}
