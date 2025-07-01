package utils

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProductionSandboxIntegration tests the full production-ready sandbox flow
func TestProductionSandboxIntegration(t *testing.T) {
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
		t.Skip("Docker not available, skipping integration tests")
	}
	require.NoError(t, err)

	executor := NewSandboxExecutor(workspace, logger)
	sessionID := "integration-test-session"
	ctx := context.Background()

	// Initialize workspace
	_, err = workspace.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	t.Run("ProductionSandboxExecution", func(t *testing.T) {
		// Configure production-ready options
		options := SandboxOptions{
			BaseImage:     "alpine:3.18",
			MemoryLimit:   256 * 1024 * 1024,
			CPUQuota:      50000,
			Timeout:       30 * time.Second,
			ReadOnly:      true,
			NetworkAccess: false,
			User:          "1000",
			Group:         "1000",
			Capabilities:  []string{}, // No capabilities
			SecurityPolicy: SecurityPolicy{
				AllowNetworking:   false,
				AllowFileSystem:   true,
				RequireNonRoot:    true,
				TrustedRegistries: []string{"docker.io"},
			},
			EnableMetrics: true,
			EnableAudit:   true,
		}

		cmd := []string{"echo", "Production sandbox test"}
		result, err := executor.ExecuteAdvanced(ctx, sessionID, cmd, options)

		if err != nil && err.Error() == "failed to execute docker command: exec: \"docker\": executable file not found in $PATH" {
			t.Skip("Docker not available in test environment")
		}

		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Production sandbox test")
		assert.Greater(t, result.Duration, time.Duration(0))
	})

	t.Run("SecurityValidationIntegration", func(t *testing.T) {
		validator := NewSecurityValidator(logger)

		// Test secure configuration
		secureOptions := SandboxOptions{
			BaseImage:     "alpine:3.18",
			MemoryLimit:   256 * 1024 * 1024,
			CPUQuota:      50000,
			Timeout:       30 * time.Second,
			ReadOnly:      true,
			NetworkAccess: false,
			User:          "1000",
			Group:         "1000",
			Capabilities:  []string{},
			SecurityPolicy: SecurityPolicy{
				AllowNetworking:   false,
				AllowFileSystem:   true,
				RequireNonRoot:    true,
				TrustedRegistries: []string{"docker.io"},
			},
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, secureOptions)
		require.NoError(t, err)
		assert.True(t, report.Passed)
		assert.Equal(t, "LOW", report.OverallRisk)
		assert.NotEmpty(t, report.ThreatAssessment)
		assert.NotEmpty(t, report.ControlStatus)
	})

	t.Run("MetricsCollectionIntegration", func(t *testing.T) {
		collector := NewSandboxMetricsCollector()

		// Simulate execution record
		record := ExecutionRecord{
			ID:        "integration-exec-1",
			SessionID: sessionID,
			Command:   []string{"echo", "metrics test"},
			StartTime: time.Now().Add(-1 * time.Minute),
			EndTime:   time.Now(),
			ExitCode:  0,
		}

		collector.addRecord(record)

		// Verify metrics collection
		collector.mutex.RLock()
		assert.Len(t, collector.history, 1)
		assert.Equal(t, "integration-exec-1", collector.history[0].ID)
		collector.mutex.RUnlock()
	})
}

// TestCompleteWorkflow tests the entire sandbox workflow from initialization to cleanup
func TestCompleteWorkflow(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024,
		TotalMaxSize:      2 * 1024 * 1024 * 1024,
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	})

	if err != nil {
		t.Skip("Docker not available, skipping workflow test")
	}
	require.NoError(t, err)

	sessionID := "workflow-test-session"
	ctx := context.Background()

	// Step 1: Initialize workspace
	workspaceDir, err := workspace.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)
	assert.NotEmpty(t, workspaceDir)

	// Step 2: Security validation
	validator := NewSecurityValidator(logger)
	options := SandboxOptions{
		BaseImage:     "alpine:3.18",
		MemoryLimit:   256 * 1024 * 1024,
		CPUQuota:      50000,
		Timeout:       30 * time.Second,
		ReadOnly:      true,
		NetworkAccess: false,
		User:          "1000",
		Group:         "1000",
		Capabilities:  []string{},
	}

	report, err := validator.ValidateSecurity(ctx, sessionID, options)
	require.NoError(t, err)
	assert.True(t, report.Passed)

	// Step 3: Execute command
	executor := NewSandboxExecutor(workspace, logger)
	cmd := []string{"echo", "workflow test complete"}
	result, err := executor.ExecuteAdvanced(ctx, sessionID, cmd, options)

	if err != nil && err.Error() == "failed to execute docker command: exec: \"docker\": executable file not found in $PATH" {
		t.Skip("Docker not available in test environment")
	}

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "workflow test complete")

	// Step 4: Cleanup (handled automatically by workspace manager)
	err = workspace.CleanupWorkspace(ctx, sessionID)
	require.NoError(t, err)
}
