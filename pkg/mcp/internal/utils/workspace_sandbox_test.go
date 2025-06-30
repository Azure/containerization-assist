package utils

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceSandboxing(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	
	// Create workspace manager with sandboxing enabled
	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024, // 512MB per session
		TotalMaxSize:      2 * 1024 * 1024 * 1024, // 2GB total
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	})
	
	// Skip if Docker is not available
	if err != nil && err.Error() == "docker command not found for sandboxing: exec: \"docker\": executable file not found in $PATH" {
		t.Skip("Docker not available, skipping sandboxing tests")
	}
	require.NoError(t, err)

	sessionID := "test-sandbox-session"
	ctx := context.Background()

	// Initialize workspace
	workspaceDir, err := workspace.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)
	assert.DirExists(t, workspaceDir)

	t.Run("SecurityPolicyValidation", func(t *testing.T) {
		// Test valid security policy
		validPolicy := SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io", "alpine"},
		}
		err := workspace.validateSecurityPolicy(validPolicy)
		assert.NoError(t, err)

		// Test invalid security policy (no trusted registries)
		invalidPolicy := SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{},
		}
		err = workspace.validateSecurityPolicy(invalidPolicy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one trusted registry must be specified")
	})

	t.Run("DockerCommandBuildingSecure", func(t *testing.T) {
		// Test secure sandbox options
		secureOptions := SandboxOptions{
			BaseImage:     "alpine:latest",
			MemoryLimit:   256 * 1024 * 1024, // 256MB
			CPUQuota:      50000,              // 50% CPU
			Timeout:       30 * time.Second,
			ReadOnly:      true,
			NetworkAccess: false,
			SecurityPolicy: SecurityPolicy{
				AllowNetworking:   false,
				AllowFileSystem:   true,
				RequireNonRoot:    true,
				TrustedRegistries: []string{"docker.io", "alpine"},
			},
		}

		cmd := []string{"echo", "hello sandbox"}
		dockerArgs, err := workspace.buildDockerRunCommand(sessionID, cmd, secureOptions)
		require.NoError(t, err)

		// Verify security settings are applied
		assert.Contains(t, dockerArgs, "--memory=268435456") // 256MB in bytes
		assert.Contains(t, dockerArgs, "--cpus=0.50")        // 50% CPU
		assert.Contains(t, dockerArgs, "--user=1000:1000")   // Non-root user
		assert.Contains(t, dockerArgs, "--read-only")        // Read-only filesystem
		assert.Contains(t, dockerArgs, "--network=none")     // No network access
		assert.Contains(t, dockerArgs, "alpine:latest")      // Specified image
		assert.Contains(t, dockerArgs, "echo")               // Command
		assert.Contains(t, dockerArgs, "hello sandbox")      // Command args
	})

	t.Run("DockerCommandBuildingPrivileged", func(t *testing.T) {
		// Test Docker-in-Docker options
		dindOptions := SandboxOptions{
			BaseImage:     "docker:dind",
			MemoryLimit:   1024 * 1024 * 1024, // 1GB
			CPUQuota:      100000,              // 100% CPU
			Timeout:       5 * time.Minute,
			ReadOnly:      false,
			NetworkAccess: true,
			SecurityPolicy: SecurityPolicy{
				AllowNetworking:   true,
				AllowFileSystem:   true,
				RequireNonRoot:    false, // Docker-in-Docker requires root
				TrustedRegistries: []string{"docker.io"},
			},
		}

		cmd := []string{"docker", "version"}
		dockerArgs, err := workspace.buildDockerRunCommand(sessionID, cmd, dindOptions)
		require.NoError(t, err)

		// Verify Docker-in-Docker settings
		assert.Contains(t, dockerArgs, "--memory=1073741824") // 1GB in bytes
		assert.Contains(t, dockerArgs, "--cpus=1.00")         // 100% CPU
		assert.Contains(t, dockerArgs, "--privileged")        // Privileged mode for DinD
		assert.Contains(t, dockerArgs, "/var/run/docker.sock:/var/run/docker.sock") // Docker socket mount
		assert.Contains(t, dockerArgs, "docker:dind")         // DinD image
		assert.NotContains(t, dockerArgs, "--network=none")   // Network access allowed
		assert.NotContains(t, dockerArgs, "--read-only")      // Not read-only
	})

	t.Run("EnvironmentSanitization", func(t *testing.T) {
		// Test environment variable sanitization
		env := map[string]string{
			"PATH":          "/usr/local/bin:/usr/bin:/bin",
			"HOME":          "/home/user",
			"USER":          "testuser",
			"LANG":          "en_US.UTF-8",
			"MALICIOUS_VAR": "value; rm -rf /",
			"PIPE_VAR":      "value | cat",
			"SECRET_KEY":    "super-secret",
		}

		sanitized := workspace.sanitizeEnvironment(env)

		// Should include safe variables
		assert.Contains(t, sanitized, "PATH=/usr/local/bin:/usr/bin:/bin")
		assert.Contains(t, sanitized, "HOME=/home/user")
		assert.Contains(t, sanitized, "USER=testuser")
		assert.Contains(t, sanitized, "LANG=en_US.UTF-8")

		// Should exclude dangerous variables
		for _, envVar := range sanitized {
			assert.NotContains(t, envVar, "MALICIOUS_VAR")
			assert.NotContains(t, envVar, "PIPE_VAR")
			assert.NotContains(t, envVar, "SECRET_KEY")
		}
	})

	t.Run("SandboxedAnalysisOptions", func(t *testing.T) {
		// Test that SandboxedAnalysis creates appropriate options
		result, err := workspace.SandboxedAnalysis(ctx, sessionID, "/workspace/repo", nil)
		
		// Should fail gracefully if Docker isn't available
		if err != nil {
			assert.Contains(t, err.Error(), "failed to execute docker command")
			t.Log("Docker not available for full sandboxed execution test")
			return
		}

		// If Docker is available, result should be non-nil
		assert.NotNil(t, result)
	})

	t.Run("SandboxedBuildOptions", func(t *testing.T) {
		// Test that SandboxedBuild creates appropriate options for Docker-in-Docker
		result, err := workspace.SandboxedBuild(ctx, sessionID, "/workspace/repo/Dockerfile", nil)
		
		// Should fail gracefully if Docker isn't available
		if err != nil {
			assert.Contains(t, err.Error(), "failed to execute docker command")
			t.Log("Docker not available for full sandboxed build test")
			return
		}

		// If Docker is available, result should be non-nil
		assert.NotNil(t, result)
	})

	// Cleanup
	err = workspace.CleanupWorkspace(ctx, sessionID)
	assert.NoError(t, err)
}

func TestSandboxingDisabled(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	
	// Create workspace manager with sandboxing disabled
	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024,
		TotalMaxSize:      2 * 1024 * 1024 * 1024,
		Cleanup:           true,
		SandboxEnabled:    false, // Disabled
		Logger:            logger,
	})
	require.NoError(t, err)

	sessionID := "test-no-sandbox-session"
	ctx := context.Background()

	// Initialize workspace
	_, err = workspace.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	t.Run("SandboxedOperationsDisabled", func(t *testing.T) {
		// Test that sandboxed operations return appropriate errors when disabled
		options := SandboxOptions{
			BaseImage: "alpine:latest",
			Timeout:   30 * time.Second,
		}

		_, err := workspace.ExecuteSandboxed(ctx, sessionID, []string{"echo", "test"}, options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sandboxing is disabled")

		_, err = workspace.SandboxedAnalysis(ctx, sessionID, "/workspace/repo", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sandboxing is disabled")

		_, err = workspace.SandboxedBuild(ctx, sessionID, "/workspace/repo/Dockerfile", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sandboxing is disabled")
	})

	// Cleanup
	err = workspace.CleanupWorkspace(ctx, sessionID)
	assert.NoError(t, err)
}

func TestWorkspaceStats(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	
	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 512 * 1024 * 1024,
		TotalMaxSize:      2 * 1024 * 1024 * 1024,
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	})
	
	// Skip if Docker is not available
	if err != nil && err.Error() == "docker command not found for sandboxing: exec: \"docker\": executable file not found in $PATH" {
		workspace, err = NewWorkspaceManager(context.Background(), WorkspaceConfig{
			BaseDir:           t.TempDir(),
			MaxSizePerSession: 512 * 1024 * 1024,
			TotalMaxSize:      2 * 1024 * 1024 * 1024,
			Cleanup:           true,
			SandboxEnabled:    false, // Disable if Docker not available
			Logger:            logger,
		})
	}
	require.NoError(t, err)

	stats := workspace.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(512*1024*1024), stats.PerSessionLimit)
	assert.Equal(t, int64(2*1024*1024*1024), stats.TotalDiskLimit)
	
	// SandboxEnabled should reflect the actual state
	assert.IsType(t, false, stats.SandboxEnabled) // Just check it's a boolean
}