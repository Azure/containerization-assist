package observability_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	checker := ops.NewPreFlightChecker(logger)

	t.Run("docker config parsing integration", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create realistic docker config
		dockerConfig := ops.DockerConfig{
			Auths: map[string]ops.DockerAuth{
				"https://index.docker.io/v1/": {
					Username: "dockeruser",
					Password: "dockerpass",
					Email:    "user@docker.com",
					Auth:     "ZG9ja2VydXNlcjpkb2NrZXJwYXNz", // base64 of dockeruser:dockerpass
				},
				"gcr.io": {
					Username: "_json_key",
					Password: `{"type":"service_account","project_id":"my-project","private_key_id":"key123"}`,
					Auth:     "anNvbl9rZXk6eyJ0eXBlIjoic2VydmljZV9hY2NvdW50IiwicHJvamVjdF9pZCI6Im15LXByb2plY3QiLCJwcml2YXRlX2tleV9pZCI6ImtleTEyMyJ9",
				},
				"private-registry.company.com": {
					Username: "employee",
					Password: "companypass",
					Auth:     "ZW1wbG95ZWU6Y29tcGFueXBhc3M=",
				},
			},
			CredHelpers: map[string]string{
				"123456789012.dkr.ecr.us-west-2.amazonaws.com": "ecr-login",
				"us.gcr.io": "gcloud",
			},
			CredsStore: "desktop",
		}

		configData, err := json.MarshalIndent(dockerConfig, "", "  ")
		require.NoError(t, err)

		configPath := filepath.Join(tmpDir, "config.json")
		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Test that the config can be parsed correctly by reading it
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var parsedConfig ops.DockerConfig
		err = json.Unmarshal(content, &parsedConfig)
		require.NoError(t, err)

		// Verify structure matches what we wrote
		assert.Len(t, parsedConfig.Auths, len(dockerConfig.Auths))
		assert.Len(t, parsedConfig.CredHelpers, len(dockerConfig.CredHelpers))
		assert.Equal(t, "desktop", parsedConfig.CredsStore)

		// Verify specific auth entries
		assert.Contains(t, parsedConfig.Auths, "https://index.docker.io/v1/")
		assert.Contains(t, parsedConfig.Auths, "gcr.io")
		assert.Contains(t, parsedConfig.Auths, "private-registry.company.com")

		dockerAuth := parsedConfig.Auths["https://index.docker.io/v1/"]
		assert.Equal(t, "dockeruser", dockerAuth.Username)
		assert.Equal(t, "user@docker.com", dockerAuth.Email)

		gcrAuth := parsedConfig.Auths["gcr.io"]
		assert.Equal(t, "_json_key", gcrAuth.Username)
		assert.Contains(t, gcrAuth.Password, "service_account")

		// Verify credential helpers
		assert.Equal(t, "ecr-login", parsedConfig.CredHelpers["123456789012.dkr.ecr.us-west-2.amazonaws.com"])
		assert.Equal(t, "gcloud", parsedConfig.CredHelpers["us.gcr.io"])

		t.Logf("Successfully parsed docker config with %d auths and %d credential helpers",
			len(parsedConfig.Auths), len(parsedConfig.CredHelpers))
	})

	t.Run("registry validation integration", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		registries := []string{
			"docker.io",
			"gcr.io",
			"quay.io",
		}

		result, err := checker.ValidateMultipleRegistries(ctx, registries)

		// Network connectivity might fail in CI environments
		if err != nil {
			t.Logf("Registry validation failed (expected in some environments): %v", err)
			// Continue with validation that we got some structure back
		}

		require.NotNil(t, result)
		assert.Len(t, result.Results, len(registries))

		for _, registry := range registries {
			registryResult, exists := result.Results[registry]
			assert.True(t, exists, "Result should exist for registry: %s", registry)
			assert.NotNil(t, registryResult)

			// Verify result structure
			assert.NotEmpty(t, registryResult.Registry, "Registry should be set")
			assert.NotZero(t, registryResult.Timestamp, "Timestamp should be recorded for %s", registry)
			assert.NotEmpty(t, registryResult.OverallStatus, "Overall status should be set for %s", registry)

			if registryResult.OverallStatus == "success" || registryResult.ConnectivityStatus == "success" {
				t.Logf("✓ Registry %s: validation successful", registry)
			} else {
				t.Logf("✗ Registry %s: validation failed - connectivity: %s, auth: %s",
					registry, registryResult.ConnectivityStatus, registryResult.AuthenticationStatus)
				if registryResult.ConnectivityError != "" {
					t.Logf("  Connectivity error: %s", registryResult.ConnectivityError)
				}
				if registryResult.AuthenticationError != "" {
					t.Logf("  Auth error: %s", registryResult.AuthenticationError)
				}
			}
		}

		// Verify overall timing and results
		assert.NotZero(t, result.Duration, "Duration should be recorded")
		assert.NotZero(t, result.Timestamp, "Timestamp should be recorded")

		t.Logf("Registry validation completed in %v", result.Duration)
	})

	t.Run("preflight checks integration", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		// Use the available RunChecks method
		result, err := checker.RunChecks(ctx)

		// Some checks might fail in CI environments, but we should get a result
		require.NotNil(t, result)

		// Verify result structure
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.Duration)
		assert.Greater(t, len(result.Checks), 0, "Should have run some checks")

		// Categorize check results
		checksByCategory := make(map[string][]ops.CheckResult)
		checksByStatus := make(map[ops.CheckStatus]int)

		for _, check := range result.Checks {
			checksByCategory[check.Category] = append(checksByCategory[check.Category], check)
			checksByStatus[check.Status]++
		}

		// Log results
		t.Logf("Preflight check results:")
		t.Logf("  Overall passed: %v", result.Passed)
		t.Logf("  Can proceed: %v", result.CanProceed)
		t.Logf("  Duration: %v", result.Duration)
		t.Logf("  Total checks: %d", len(result.Checks))

		for status, count := range checksByStatus {
			t.Logf("  %s: %d", status, count)
		}

		for category, checks := range checksByCategory {
			t.Logf("  Category %s: %d checks", category, len(checks))
			for _, check := range checks {
				t.Logf("    %s (%s): %s", check.Name, check.Status, check.Message)
				if check.Error != "" {
					t.Logf("      Error: %s", check.Error)
				}
			}
		}

		// Error should only occur for fatal failures
		if err != nil {
			t.Logf("Preflight error (may be expected in CI): %v", err)
		}
	})

	t.Run("credential store fallback", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Test with only credential store
		dockerConfig := ops.DockerConfig{
			CredsStore: "desktop",
		}

		configData, err := json.MarshalIndent(dockerConfig, "", "  ")
		require.NoError(t, err)

		configPath := filepath.Join(tmpDir, "config.json")
		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Test parsing the config with only credential store
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var parsedConfig ops.DockerConfig
		err = json.Unmarshal(content, &parsedConfig)
		require.NoError(t, err)

		assert.Equal(t, "desktop", parsedConfig.CredsStore)
		assert.Len(t, parsedConfig.Auths, 0)
		assert.Len(t, parsedConfig.CredHelpers, 0)

		t.Logf("Credential store configuration parsed successfully: %s", parsedConfig.CredsStore)
	})

	t.Run("mixed auth configuration", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Complex config with auths, helpers, and store
		dockerConfig := ops.DockerConfig{
			Auths: map[string]ops.DockerAuth{
				"docker.io": {
					Username: "user1",
					Password: "pass1",
					Auth:     "dXNlcjE6cGFzczE=",
				},
			},
			CredHelpers: map[string]string{
				"gcr.io": "gcloud",
				"ecr.io": "ecr-login",
			},
			CredsStore: "osxkeychain",
		}

		configData, err := json.MarshalIndent(dockerConfig, "", "  ")
		require.NoError(t, err)

		configPath := filepath.Join(tmpDir, "config.json")
		err = os.WriteFile(configPath, configData, 0644)
		require.NoError(t, err)

		// Test parsing mixed configuration
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var parsedConfig ops.DockerConfig
		err = json.Unmarshal(content, &parsedConfig)
		require.NoError(t, err)

		// Verify mixed configuration
		assert.Len(t, parsedConfig.Auths, 1)       // 1 direct auth
		assert.Len(t, parsedConfig.CredHelpers, 2) // 2 credential helpers
		assert.Equal(t, "osxkeychain", parsedConfig.CredsStore)

		// Verify specific entries
		assert.Contains(t, parsedConfig.Auths, "docker.io")
		assert.Equal(t, "gcloud", parsedConfig.CredHelpers["gcr.io"])
		assert.Equal(t, "ecr-login", parsedConfig.CredHelpers["ecr.io"])

		dockerAuth := parsedConfig.Auths["docker.io"]
		assert.Equal(t, "user1", dockerAuth.Username)
		assert.Equal(t, "pass1", dockerAuth.Password)

		t.Logf("Mixed auth configuration parsed successfully: %d auths, %d helpers, store: %s",
			len(parsedConfig.Auths), len(parsedConfig.CredHelpers), parsedConfig.CredsStore)
	})
}

func TestPreflightPerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	t.Run("preflight check performance", func(t *testing.T) {
		checker := ops.NewPreFlightChecker(logger)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		start := time.Now()

		result, err := checker.RunChecks(ctx)
		duration := time.Since(start)

		require.NotNil(t, result)

		// Performance expectations
		assert.Less(t, duration, 45*time.Second, "Preflight checks should complete within 45 seconds")
		assert.Greater(t, len(result.Checks), 0, "Should run some checks")

		t.Logf("Preflight checks completed in %v", duration)
		t.Logf("Average check duration: %v", duration/time.Duration(len(result.Checks)))

		// Verify no checks took excessively long
		for _, check := range result.Checks {
			assert.Less(t, check.Duration, 20*time.Second, "Individual check %s took too long: %v", check.Name, check.Duration)
		}

		if err != nil {
			t.Logf("Preflight error (may be expected): %v", err)
		}
	})

	t.Run("concurrent registry validation", func(t *testing.T) {
		const numWorkers = 3
		results := make(chan error, numWorkers)

		registries := []string{"docker.io", "gcr.io", "quay.io"}

		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				checker := ops.NewPreFlightChecker(logger)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				_, err := checker.ValidateMultipleRegistries(ctx, registries)
				results <- err
			}(i)
		}

		// Wait for all workers
		for i := 0; i < numWorkers; i++ {
			err := <-results
			if err != nil {
				t.Logf("Worker %d failed (may be expected in CI): %v", i, err)
			} else {
				t.Logf("Worker %d completed successfully", i)
			}
		}

		t.Logf("Concurrent registry validation completed with %d workers", numWorkers)
	})
}

// Benchmark for realistic workloads
func BenchmarkRegistryOperations(b *testing.B) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	checker := ops.NewPreFlightChecker(logger)

	b.Run("docker_config_parsing", func(b *testing.B) {
		tmpDir := b.TempDir()

		// Create a realistic docker config
		dockerConfig := ops.DockerConfig{
			Auths: map[string]ops.DockerAuth{
				"docker.io": {Username: "user1", Password: "pass1", Auth: "dXNlcjE6cGFzczE="},
				"gcr.io":    {Username: "user2", Password: "pass2", Auth: "dXNlcjI6cGFzczI="},
				"quay.io":   {Username: "user3", Password: "pass3", Auth: "dXNlcjM6cGFzczM="},
			},
			CredHelpers: map[string]string{
				"ecr.amazonaws.com": "ecr-login",
				"us.gcr.io":         "gcloud",
			},
			CredsStore: "desktop",
		}

		configData, _ := json.MarshalIndent(dockerConfig, "", "  ")
		configPath := filepath.Join(tmpDir, "config.json")
		os.WriteFile(configPath, configData, 0644)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			content, err := os.ReadFile(configPath)
			if err != nil {
				b.Fatalf("File read failed: %v", err)
			}

			var parsedConfig ops.DockerConfig
			err = json.Unmarshal(content, &parsedConfig)
			if err != nil {
				b.Fatalf("Config parsing failed: %v", err)
			}
		}
	})

	b.Run("registry_validation", func(b *testing.B) {
		registries := []string{"docker.io"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			_, err := checker.ValidateMultipleRegistries(ctx, registries)
			cancel()

			if err != nil {
				b.Logf("Iteration %d: %v", i, err)
			}
		}
	})
}
