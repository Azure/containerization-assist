package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigManager_DefaultValues(t *testing.T) {
	cfg := NewConfigManager()

	// Test default server config
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}

	// Test default analyzer config
	if cfg.Analyzer.EnableAI != false {
		t.Errorf("Expected default EnableAI false, got %t", cfg.Analyzer.EnableAI)
	}

	if cfg.Analyzer.MaxAnalysisTime != 5*time.Minute {
		t.Errorf("Expected default MaxAnalysisTime 5m, got %v", cfg.Analyzer.MaxAnalysisTime)
	}

	// Test default docker config
	if cfg.Docker.Registry != "docker.io" {
		t.Errorf("Expected default registry 'docker.io', got '%s'", cfg.Docker.Registry)
	}
}

func TestConfigManager_EnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("MCP_SERVER_HOST", "testhost")
	os.Setenv("MCP_SERVER_PORT", "9090")
	os.Setenv("MCP_ANALYZER_ENABLE_AI", "true")
	os.Setenv("MCP_DOCKER_REGISTRY", "myregistry.com")

	defer func() {
		// Clean up
		os.Unsetenv("MCP_SERVER_HOST")
		os.Unsetenv("MCP_SERVER_PORT")
		os.Unsetenv("MCP_ANALYZER_ENABLE_AI")
		os.Unsetenv("MCP_DOCKER_REGISTRY")
	}()

	cfg := NewConfigManager()
	err := cfg.LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that environment variables override defaults
	if cfg.Server.Host != "testhost" {
		t.Errorf("Expected host 'testhost', got '%s'", cfg.Server.Host)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}

	if cfg.Analyzer.EnableAI != true {
		t.Errorf("Expected EnableAI true, got %t", cfg.Analyzer.EnableAI)
	}

	if cfg.Docker.Registry != "myregistry.com" {
		t.Errorf("Expected registry 'myregistry.com', got '%s'", cfg.Docker.Registry)
	}
}

func TestConfigManager_Validation(t *testing.T) {
	cfg := NewConfigManager()

	// Test valid configuration
	err := cfg.validate()
	if err != nil {
		t.Errorf("Expected valid default config, got error: %v", err)
	}

	// Test invalid port
	cfg.Server.Port = -1
	err = cfg.validate()
	if err == nil {
		t.Error("Expected validation error for invalid port, got none")
	}

	// Reset to valid value
	cfg.Server.Port = 8080

	// Test invalid timeout
	cfg.Server.ReadTimeout = -1 * time.Second
	err = cfg.validate()
	if err == nil {
		t.Error("Expected validation error for invalid timeout, got none")
	}
}

func TestGlobalConfig(t *testing.T) {
	// Reset global config for test
	Reset()

	// Test that Get() returns error before initialization
	_, err := Get()
	if err == nil {
		t.Error("Expected error when getting uninitialized config")
	}

	// Test initialization
	err = Initialize("")
	if err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	// Test that Get() works after initialization
	cfg, err := Get()
	if err != nil {
		t.Fatalf("Failed to get initialized config: %v", err)
	}

	if cfg == nil {
		t.Error("Expected non-nil config after initialization")
	}

	// Test that IsInitialized() returns true
	if !IsInitialized() {
		t.Error("Expected IsInitialized() to return true after initialization")
	}

	// Test helper functions
	serverCfg, err := GetServer()
	if err != nil {
		t.Fatalf("Failed to get server config: %v", err)
	}

	if serverCfg.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", serverCfg.Host)
	}
}

func TestMigrationHelper(t *testing.T) {
	// Create a test logger (you might need to adjust this based on your logger setup)
	logger := struct{}{}

	// For now, we'll skip the actual migration test since it requires zerolog
	// In a real implementation, you would test the migration helper here

	_ = logger // Avoid unused variable error
	t.Log("Migration helper test placeholder - implement when logger is available")
}
