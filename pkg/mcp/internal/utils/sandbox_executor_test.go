package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxExecutor(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	// Create workspace manager
	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024,
		TotalMaxSize:      2 * 1024 * 1024 * 1024,
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	})

	// Skip if Docker is not available
	if err != nil {
		t.Skip("Docker not available, skipping sandbox executor tests")
	}
	require.NoError(t, err)

	executor := NewSandboxExecutor(workspace, logger)
	sessionID := "test-advanced-sandbox"
	ctx := context.Background()

	// Initialize workspace
	_, err = workspace.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	t.Run("BasicExecution", func(t *testing.T) {
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:     "alpine:latest",
				MemoryLimit:   256 * 1024 * 1024,
				CPUQuota:      50000,
				Timeout:       30 * time.Second,
				ReadOnly:      true,
				NetworkAccess: false,
				SecurityPolicy: SecurityPolicy{
					AllowNetworking:   false,
					AllowFileSystem:   true,
					RequireNonRoot:    true,
					TrustedRegistries: []string{"docker.io"},
				},
			},
			EnableMetrics: true,
			EnableAudit:   true,
		}

		cmd := []string{"echo", "Hello from advanced sandbox"}
		result, err := executor.ExecuteAdvanced(ctx, sessionID, cmd, options)

		if err != nil && err.Error() == "failed to execute docker command: exec: \"docker\": executable file not found in $PATH" {
			t.Skip("Docker not available for execution")
		}

		assert.NoError(t, err)
		if result != nil {
			assert.Equal(t, 0, result.ExitCode)
			assert.Contains(t, result.Stdout, "Hello from advanced sandbox")
		}
	})

	t.Run("SecurityValidation", func(t *testing.T) {
		// Test with untrusted image
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "untrusted.registry/malicious:latest",
				SecurityPolicy: SecurityPolicy{
					TrustedRegistries: []string{"docker.io", "gcr.io"},
				},
			},
		}

		_, err := executor.ExecuteAdvanced(ctx, sessionID, []string{"echo", "test"}, options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not from trusted registry")

		// Verify security event was audited
		auditLog := executor.GetSecurityAuditLog(sessionID)
		assert.Greater(t, len(auditLog), 0)

		found := false
		for _, entry := range auditLog {
			if entry.EventType == "EXECUTION_BLOCKED" && entry.Action == "DENY" {
				found = true
				break
			}
		}
		assert.True(t, found, "Security block event should be audited")
	})

	t.Run("CapabilityValidation", func(t *testing.T) {
		// Test dangerous capability request
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:latest",
				SecurityPolicy: SecurityPolicy{
					RequireNonRoot:    true,
					TrustedRegistries: []string{"docker.io"},
				},
			},
			Capabilities: []string{"SYS_ADMIN"}, // Dangerous capability
		}

		_, err := executor.ExecuteAdvanced(ctx, sessionID, []string{"echo", "test"}, options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dangerous capability requested")
	})

	t.Run("ResourceLimitValidation", func(t *testing.T) {
		// Test exceeding memory limit
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:   "alpine:latest",
				MemoryLimit: 2 * 1024 * 1024 * 1024, // 2GB - exceeds default policy
				SecurityPolicy: SecurityPolicy{
					TrustedRegistries: []string{"docker.io"},
					ResourceLimits: ResourceLimits{
						Memory: 512 * 1024 * 1024, // 512MB limit
					},
				},
			},
		}

		_, err := executor.ExecuteAdvanced(ctx, sessionID, []string{"echo", "test"}, options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds policy limit")
	})

	t.Run("ExecutionHistory", func(t *testing.T) {
		// Execute a command to generate history
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:latest",
				Timeout:   10 * time.Second,
				SecurityPolicy: SecurityPolicy{
					TrustedRegistries: []string{"docker.io"},
				},
			},
			EnableMetrics: true,
		}

		executor.ExecuteAdvanced(ctx, sessionID, []string{"echo", "history test"}, options)

		// Check execution history
		history := executor.GetExecutionHistory(sessionID)
		assert.Greater(t, len(history), 0)

		if len(history) > 0 {
			lastExecution := history[len(history)-1]
			assert.Equal(t, sessionID, lastExecution.SessionID)
			assert.Equal(t, []string{"echo", "history test"}, lastExecution.Command)
			assert.NotZero(t, lastExecution.StartTime)
			assert.NotZero(t, lastExecution.EndTime)
		}
	})

	t.Run("ResourceMonitoring", func(t *testing.T) {
		// Get resource usage
		usage := executor.GetResourceUsage()
		assert.NotNil(t, usage)

		// After executions, there should be some resource tracking
		if len(usage) > 0 {
			for sessionID, resourceUsage := range usage {
				assert.NotEmpty(t, sessionID)
				assert.NotNil(t, resourceUsage)
				assert.GreaterOrEqual(t, resourceUsage.ContainerCount, 0)
			}
		}
	})

	t.Run("SecurityAuditLog", func(t *testing.T) {
		// Get full audit log
		fullLog := executor.GetSecurityAuditLog("")
		assert.NotNil(t, fullLog)

		// Get session-specific audit log
		sessionLog := executor.GetSecurityAuditLog(sessionID)
		assert.NotNil(t, sessionLog)
		assert.LessOrEqual(t, len(sessionLog), len(fullLog))

		// Verify audit entries have required fields
		for _, entry := range sessionLog {
			assert.NotZero(t, entry.Timestamp)
			assert.NotEmpty(t, entry.EventType)
			assert.NotEmpty(t, entry.Severity)
			assert.NotEmpty(t, entry.Action)
		}
	})

	t.Run("MetricsExport", func(t *testing.T) {
		// Export metrics
		metricsData, err := executor.ExportMetrics(ctx)
		assert.NoError(t, err)
		assert.NotEmpty(t, metricsData)

		// Verify it's valid JSON
		var exported map[string]interface{}
		err = json.Unmarshal(metricsData, &exported)
		assert.NoError(t, err)
		assert.Contains(t, exported, "timestamp")
		assert.Contains(t, exported, "metrics")
		assert.Contains(t, exported, "history")
		assert.Contains(t, exported, "resources")
	})

	// Cleanup
	err = workspace.CleanupWorkspace(ctx, sessionID)
	assert.NoError(t, err)
}

func TestAdvancedSecurityFeatures(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024,
		TotalMaxSize:      2 * 1024 * 1024 * 1024,
		Cleanup:           true,
		SandboxEnabled:    false, // Don't need actual Docker for these tests
		Logger:            logger,
	})
	require.NoError(t, err)

	executor := NewSandboxExecutor(workspace, logger)

	t.Run("ImageTrustValidation", func(t *testing.T) {
		trustedRegistries := []string{"docker.io", "gcr.io", "quay.io"}

		// Test trusted images
		trustedImages := []string{
			"alpine",               // Library image
			"docker.io/alpine",     // Explicit docker.io
			"gcr.io/project/image", // GCR image
			"quay.io/org/image",    // Quay image
		}

		for _, image := range trustedImages {
			assert.True(t, executor.isImageTrusted(image, trustedRegistries),
				"Image %s should be trusted", image)
		}

		// Test untrusted images
		untrustedImages := []string{
			"untrusted.registry/image",
			"malicious.com/bad-image",
			"192.168.1.1:5000/local-image",
		}

		for _, image := range untrustedImages {
			assert.False(t, executor.isImageTrusted(image, trustedRegistries),
				"Image %s should not be trusted", image)
		}
	})

	t.Run("SecurityPolicyEngine", func(t *testing.T) {
		engine := NewSecurityPolicyEngine()

		// Test default policy
		defaultPolicy := engine.getPolicy("unknown-session")
		assert.False(t, defaultPolicy.AllowNetworking)
		assert.True(t, defaultPolicy.RequireNonRoot)
		assert.Greater(t, len(defaultPolicy.TrustedRegistries), 0)

		// Add custom policy
		customPolicy := SecurityPolicy{
			AllowNetworking:   true,
			AllowFileSystem:   true,
			RequireNonRoot:    false,
			TrustedRegistries: []string{"custom.registry"},
		}

		engine.mutex.Lock()
		engine.policies["custom-session"] = customPolicy
		engine.mutex.Unlock()

		// Verify custom policy is returned
		retrievedPolicy := engine.getPolicy("custom-session")
		assert.True(t, retrievedPolicy.AllowNetworking)
		assert.False(t, retrievedPolicy.RequireNonRoot)
		assert.Contains(t, retrievedPolicy.TrustedRegistries, "custom.registry")
	})

	t.Run("ResourceMonitor", func(t *testing.T) {
		monitor := NewResourceMonitor()

		// Test initial state
		assert.NotNil(t, monitor.limits)
		assert.Equal(t, float64(0.8), monitor.alertThreshold)

		// Update usage
		usage := ResourceUsage{
			CPUTime:        5 * time.Second,
			MemoryPeak:     256 * 1024 * 1024,
			NetworkIO:      1024 * 1024,
			DiskIO:         512 * 1024,
			ContainerCount: 2,
		}

		monitor.updateUsage("test-session", usage)

		// Verify usage was recorded
		monitor.mutex.RLock()
		recorded := monitor.usage["test-session"]
		monitor.mutex.RUnlock()

		assert.NotNil(t, recorded)
		assert.Equal(t, usage.MemoryPeak, recorded.MemoryPeak)
		assert.Equal(t, usage.ContainerCount, recorded.ContainerCount)
	})
}

func TestSecureDockerCommandBuilding(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:        t.TempDir(),
		SandboxEnabled: false,
		Logger:         logger,
	})
	require.NoError(t, err)

	executor := NewSandboxExecutor(workspace, logger)
	sessionID := "test-docker-cmd"

	t.Run("SecurityOptions", func(t *testing.T) {
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:latest",
				SecurityPolicy: SecurityPolicy{
					RequireNonRoot: true,
				},
			},
			CustomSeccomp:   "/path/to/seccomp.json",
			AppArmorProfile: "docker-default",
			SELinuxContext:  "system_u:system_r:container_t:s0",
			Capabilities:    []string{"NET_BIND_SERVICE"},
		}

		args, err := executor.buildSecureDockerCommand(sessionID, []string{"echo", "test"}, options)
		assert.NoError(t, err)

		// Verify security options are included
		assert.Contains(t, args, "--security-opt")
		assert.Contains(t, args, "no-new-privileges:true")
		assert.Contains(t, args, "seccomp=/path/to/seccomp.json")
		assert.Contains(t, args, "apparmor=docker-default")
		assert.Contains(t, args, "label=system_u:system_r:container_t:s0")
		assert.Contains(t, args, "--cap-add")
		assert.Contains(t, args, "NET_BIND_SERVICE")
	})

	t.Run("ResourceLimits", func(t *testing.T) {
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:   "alpine:latest",
				MemoryLimit: 512 * 1024 * 1024, // 512MB
				CPUQuota:    75000,             // 75% CPU
			},
		}

		args, err := executor.buildSecureDockerCommand(sessionID, []string{"echo", "test"}, options)
		assert.NoError(t, err)

		// Verify resource limits
		assert.Contains(t, args, "--memory=536870912")
		assert.Contains(t, args, "--memory-swap=536870912") // No swap
		assert.Contains(t, args, "--cpus=0.75")
	})

	t.Run("NetworkConfiguration", func(t *testing.T) {
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:     "alpine:latest",
				NetworkAccess: true,
				SecurityPolicy: SecurityPolicy{
					AllowNetworking: true,
				},
			},
			DNSServers: []string{"8.8.8.8", "8.8.4.4"},
			ExtraHosts: map[string]string{
				"custom.host": "192.168.1.100",
				"api.local":   "10.0.0.1",
			},
		}

		args, err := executor.buildSecureDockerCommand(sessionID, []string{"echo", "test"}, options)
		assert.NoError(t, err)

		// Verify network configuration
		assert.NotContains(t, args, "--network=none") // Network is enabled
		assert.Contains(t, args, "--dns")
		assert.Contains(t, args, "8.8.8.8")
		assert.Contains(t, args, "8.8.4.4")
		assert.Contains(t, args, "--add-host")
		assert.Contains(t, args, "custom.host:192.168.1.100")
		assert.Contains(t, args, "api.local:10.0.0.1")
	})

	t.Run("UserAndGroupConfiguration", func(t *testing.T) {
		// Test custom user/group
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:latest",
			},
			User:  "www-data",
			Group: "www-data",
		}

		args, err := executor.buildSecureDockerCommand(sessionID, []string{"echo", "test"}, options)
		assert.NoError(t, err)
		assert.Contains(t, args, "--user")
		assert.Contains(t, args, "www-data:www-data")

		// Test default non-root user
		options2 := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:latest",
				SecurityPolicy: SecurityPolicy{
					RequireNonRoot: true,
				},
			},
		}

		args2, err := executor.buildSecureDockerCommand(sessionID, []string{"echo", "test"}, options2)
		assert.NoError(t, err)
		assert.Contains(t, args2, "--user")
		assert.Contains(t, args2, "1000:1000")
	})
}

func TestMetricsCollector(t *testing.T) {
	collector := NewSandboxMetricsCollector()

	t.Run("RecordManagement", func(t *testing.T) {
		// Add multiple records
		for i := 0; i < 10; i++ {
			record := ExecutionRecord{
				ID:        fmt.Sprintf("exec-%d", i),
				SessionID: "test-session",
				Command:   []string{"echo", fmt.Sprintf("test-%d", i)},
				StartTime: time.Now().Add(-time.Duration(i) * time.Minute),
				EndTime:   time.Now().Add(-time.Duration(i) * time.Minute).Add(10 * time.Second),
				ExitCode:  0,
			}
			collector.addRecord(record)
		}

		// Verify records are stored
		collector.mutex.RLock()
		assert.Equal(t, 10, len(collector.history))
		collector.mutex.RUnlock()
	})

	t.Run("HistoryLimit", func(t *testing.T) {
		// Add more than 1000 records to test limit
		for i := 0; i < 1100; i++ {
			record := ExecutionRecord{
				ID:        fmt.Sprintf("exec-overflow-%d", i),
				SessionID: "test-overflow",
				Command:   []string{"echo", "overflow"},
				StartTime: time.Now(),
				EndTime:   time.Now().Add(1 * time.Second),
			}
			collector.addRecord(record)
		}

		// Verify only last 1000 are kept
		collector.mutex.RLock()
		assert.LessOrEqual(t, len(collector.history), 1000)
		collector.mutex.RUnlock()
	})
}
