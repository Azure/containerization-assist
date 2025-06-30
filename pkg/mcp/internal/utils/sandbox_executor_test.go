package utils

import (
	"context"
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
		options := SandboxOptions{
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
			EnableMetrics: true,
			EnableAudit:   true,
		}

		cmd := []string{"echo", "Hello from advanced sandbox"}
		result, err := executor.ExecuteAdvanced(ctx, sessionID, cmd, options)

		if err != nil && err.Error() == "failed to execute docker command: exec: \"docker\": executable file not found in $PATH" {
			t.Skip("Docker not available in test environment")
		}

		require.NoError(t, err)
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Hello from advanced sandbox")
	})
}

func TestSandboxMetricsCollector(t *testing.T) {
	collector := NewSandboxMetricsCollector()

	t.Run("RecordManagement", func(t *testing.T) {
		// Add test record
		record := ExecutionRecord{
			ID:        "exec-1",
			SessionID: "test-session",
			Command:   []string{"echo", "test"},
			StartTime: time.Now().Add(-1 * time.Minute),
			EndTime:   time.Now(),
			ExitCode:  0,
		}
		collector.addRecord(record)

		// Verify record is stored
		collector.mutex.RLock()
		assert.Len(t, collector.history, 1)
		assert.Equal(t, "exec-1", collector.history[0].ID)
		collector.mutex.RUnlock()
	})
}
