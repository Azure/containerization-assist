package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultServerConfigGeneration(t *testing.T) {
	tests := []struct {
		name           string
		setupEnv       func() (cleanup func())
		validateConfig func(t *testing.T, config ServerConfig)
	}{
		{
			name: "valid home directory",
			setupEnv: func() func() {
				originalHome := os.Getenv("HOME")
				tmpHome, _ := os.MkdirTemp("", "test-home-*")
				os.Setenv("HOME", tmpHome)
				return func() {
					os.Setenv("HOME", originalHome)
					os.RemoveAll(tmpHome)
				}
			},
			validateConfig: func(t *testing.T, config ServerConfig) {
				assert.Contains(t, config.WorkspaceDir, "container-kit")
				assert.Equal(t, 10, config.MaxSessions)
				assert.Equal(t, 24*time.Hour, config.SessionTTL)                 // Actual default is 24h
				assert.Equal(t, int64(1024*1024*1024), config.MaxDiskPerSession) // 1GB
				assert.Equal(t, int64(10*1024*1024*1024), config.TotalDiskLimit) // 10GB
				assert.Equal(t, "stdio", config.TransportType)
				assert.Equal(t, "info", config.LogLevel)
				assert.Equal(t, 8080, config.HTTPPort) // Default HTTP port
			},
		},
		{
			name: "no home directory - fallback to temp",
			setupEnv: func() func() {
				originalHome := os.Getenv("HOME")
				os.Unsetenv("HOME")
				return func() {
					if originalHome != "" {
						os.Setenv("HOME", originalHome)
					}
				}
			},
			validateConfig: func(t *testing.T, config ServerConfig) {
				// Should fallback to temp directory
				assert.Contains(t, config.WorkspaceDir, os.TempDir())
				assert.Contains(t, config.WorkspaceDir, "container-kit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			cleanup := tt.setupEnv()
			defer cleanup()

			// Get default config
			config := DefaultServerConfig()

			// Validate config
			tt.validateConfig(t, config)

			// Ensure storage path is set
			assert.NotEmpty(t, config.StorePath)
			assert.Contains(t, config.StorePath, "sessions")
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	config := DefaultServerConfig()

	// Test all expected defaults
	assert.Equal(t, 10, config.MaxSessions)
	assert.Equal(t, 24*time.Hour, config.SessionTTL)
	assert.Equal(t, int64(1024*1024*1024), config.MaxDiskPerSession) // 1GB
	assert.Equal(t, int64(10*1024*1024*1024), config.TotalDiskLimit) // 10GB
	assert.Equal(t, "stdio", config.TransportType)
	assert.Equal(t, "localhost", config.HTTPAddr)
	assert.Equal(t, 8080, config.HTTPPort)
	assert.Equal(t, []string{"*"}, config.CORSOrigins)
	assert.Equal(t, "", config.APIKey)
	assert.Equal(t, 60, config.RateLimit)
	assert.Equal(t, false, config.SandboxEnabled)
	assert.Equal(t, "info", config.LogLevel)
	assert.Equal(t, 1*time.Hour, config.CleanupInterval)
	assert.Equal(t, 5, config.MaxWorkers)
	assert.Equal(t, 1*time.Hour, config.JobTTL)

	// OpenTelemetry defaults
	assert.Equal(t, false, config.EnableOTEL)
	assert.Equal(t, "http://localhost:4318/v1/traces", config.OTELEndpoint)
	assert.NotNil(t, config.OTELHeaders)
	assert.Equal(t, "container-kit-mcp", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, 1.0, config.TraceSampleRate)
}

func TestConfigPaths(t *testing.T) {
	config := DefaultServerConfig()

	// Test workspace and storage paths are correctly set
	assert.NotEmpty(t, config.WorkspaceDir)
	assert.NotEmpty(t, config.StorePath)
	assert.Contains(t, config.WorkspaceDir, "workspaces")
	assert.Contains(t, config.StorePath, "sessions.db")

	// Test paths are under the same root directory
	workspaceParent := filepath.Dir(config.WorkspaceDir)
	storeParent := filepath.Dir(config.StorePath)
	assert.Equal(t, workspaceParent, storeParent)
}

func TestConfigConsistency(t *testing.T) {
	// Generate multiple configs and ensure they are consistent
	config1 := DefaultServerConfig()
	config2 := DefaultServerConfig()

	// Core settings should be identical
	assert.Equal(t, config1.MaxSessions, config2.MaxSessions)
	assert.Equal(t, config1.SessionTTL, config2.SessionTTL)
	assert.Equal(t, config1.TransportType, config2.TransportType)
	assert.Equal(t, config1.HTTPPort, config2.HTTPPort)
	assert.Equal(t, config1.LogLevel, config2.LogLevel)
}
