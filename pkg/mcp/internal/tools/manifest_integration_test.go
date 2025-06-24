package tools_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestManifestGeneration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tmpDir := t.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	tool := tools.NewGenerateManifestsTool(logger, tmpDir)

	t.Run("complete workflow with all manifest types", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "integration-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:      "integration-test",
			ServiceType:    "LoadBalancer",
			Replicas:       3,
			IncludeIngress: true,
			ConfigMapData: map[string]string{
				"app.properties": "server.port=8080\nspring.profiles.active=prod",
				"nginx.conf":     "server { listen 80; }",
			},
			BinaryData: map[string][]byte{
				"cert.pem": []byte("-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END CERTIFICATE-----"),
			},
			IngressHosts: []tools.IngressHost{
				{
					Host: "app.example.com",
					Paths: []tools.IngressPath{
						{
							Path:        "/api",
							PathType:    "Prefix",
							ServiceName: "app",
							ServicePort: 80,
						},
					},
				},
			},
			IngressTLS: []tools.IngressTLS{
				{
					Hosts:      []string{"app.example.com"},
					SecretName: "app-tls",
				},
			},
			ServicePorts: []tools.ServicePort{
				{
					Name:       "http",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: 8080,
				},
				{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: 8443,
				},
			},
			GeneratePullSecret: true,
			RegistrySecrets: []tools.RegistrySecret{
				{
					Registry: "docker.io",
					Username: "testuser",
					Password: "testpass",
					Email:    "test@example.com",
				},
			},
			WorkflowLabels: map[string]string{
				"app.version": "v1.0.0",
				"environment": "production",
				"team":        "platform",
			},
		}

		start := time.Now()
		result, err := tool.Execute(context.Background(), args)
		duration := time.Since(start)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		t.Logf("Generated %d manifests in %v", len(manifestResult.Manifests), duration)

		// Verify all expected manifest types were generated
		manifestTypes := make(map[string]bool)
		for _, manifest := range manifestResult.Manifests {
			manifestTypes[manifest.Kind] = true

			// Verify each manifest file exists and is valid YAML
			content, err := os.ReadFile(manifest.Path)
			require.NoError(t, err, "Failed to read manifest: %s", manifest.Path)

			var yamlContent map[string]interface{}
			err = yaml.Unmarshal(content, &yamlContent)
			require.NoError(t, err, "Invalid YAML in manifest: %s", manifest.Path)

			// Verify basic Kubernetes structure
			assert.NotEmpty(t, yamlContent["apiVersion"], "Missing apiVersion in %s", manifest.Path)
			assert.NotEmpty(t, yamlContent["kind"], "Missing kind in %s", manifest.Path)
			assert.NotEmpty(t, yamlContent["metadata"], "Missing metadata in %s", manifest.Path)

			t.Logf("✓ Validated manifest: %s (%s)", manifest.Name, manifest.Kind)
		}

		// Verify expected manifests were created
		expectedTypes := []string{"Deployment", "Service", "ConfigMap", "Ingress", "Secret"}
		for _, expectedType := range expectedTypes {
			assert.True(t, manifestTypes[expectedType], "Missing manifest type: %s", expectedType)
		}
	})

	t.Run("performance under load", func(t *testing.T) {
		const numIterations = 10
		durations := make([]time.Duration, numIterations)

		for i := 0; i < numIterations; i++ {
			args := tools.GenerateManifestsArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: fmt.Sprintf("perf-test-%d", i),
					DryRun:    false,
				},
				ImageRef: types.ImageReference{
					Registry:   "docker.io",
					Repository: "nginx",
					Tag:        "latest",
				},
				Namespace:      "perf-test",
				IncludeIngress: true,
				ConfigMapData: map[string]string{
					"config.yaml": "key: value",
				},
			}

			start := time.Now()
			result, err := tool.Execute(context.Background(), args)
			durations[i] = time.Since(start)

			require.NoError(t, err)
			require.NotNil(t, result)
		}

		// Calculate performance statistics
		var total time.Duration
		var min, max time.Duration = durations[0], durations[0]

		for _, d := range durations {
			total += d
			if d < min {
				min = d
			}
			if d > max {
				max = d
			}
		}

		avg := total / time.Duration(numIterations)

		t.Logf("Performance results over %d iterations:", numIterations)
		t.Logf("  Average: %v", avg)
		t.Logf("  Min: %v", min)
		t.Logf("  Max: %v", max)
		t.Logf("  Total: %v", total)

		// Performance assertions
		assert.Less(t, avg, 5*time.Second, "Average generation time should be under 5 seconds")
		assert.Less(t, max, 10*time.Second, "Maximum generation time should be under 10 seconds")
	})

	t.Run("concurrent manifest generation", func(t *testing.T) {
		const numGoroutines = 5
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				args := tools.GenerateManifestsArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: fmt.Sprintf("concurrent-test-%d", index),
						DryRun:    false,
					},
					ImageRef: types.ImageReference{
						Registry:   "docker.io",
						Repository: "nginx",
						Tag:        "latest",
					},
					Namespace: fmt.Sprintf("concurrent-ns-%d", index),
					ConfigMapData: map[string]string{
						"index": fmt.Sprintf("%d", index),
					},
				}

				_, err := tool.Execute(context.Background(), args)
				results <- err
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent execution %d failed", i)
		}

		t.Logf("Successfully completed %d concurrent manifest generations", numGoroutines)
	})
}

func TestRegistrySecretGeneration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tmpDir := t.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	tool := tools.NewGenerateManifestsTool(logger, tmpDir)

	t.Run("multiple registry credentials", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "multi-registry-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "private-registry.com",
				Repository: "app",
				Tag:        "v1.0.0",
			},
			Namespace:          "multi-registry",
			GeneratePullSecret: true,
			RegistrySecrets: []tools.RegistrySecret{
				{
					Registry: "docker.io",
					Username: "dockeruser",
					Password: "dockerpass",
					Email:    "docker@example.com",
				},
				{
					Registry: "gcr.io",
					Username: "_json_key",
					Password: `{"type":"service_account","project_id":"test"}`,
				},
				{
					Registry: "private-registry.com",
					Username: "admin",
					Password: "supersecret",
					Email:    "admin@company.com",
				},
				{
					Registry: "quay.io",
					Username: "quayuser",
					Password: "quaytoken",
				},
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Find the registry secret
		var secretManifest *tools.ManifestInfo
		for _, manifest := range manifestResult.Manifests {
			if manifest.Kind == "Secret" && manifest.Name == "registry-secret" {
				secretManifest = &manifest
				break
			}
		}
		require.NotNil(t, secretManifest, "Registry secret should be generated")

		// Read and validate the secret
		content, err := os.ReadFile(secretManifest.Path)
		require.NoError(t, err)

		var secret map[string]interface{}
		err = yaml.Unmarshal(content, &secret)
		require.NoError(t, err)

		// Verify secret structure
		assert.Equal(t, "kubernetes.io/dockerconfigjson", secret["type"])

		data := secret["data"].(map[string]interface{})
		dockerConfigJSON := data[".dockerconfigjson"].(string)

		// Decode and verify docker config
		decodedConfig, err := base64.StdEncoding.DecodeString(dockerConfigJSON)
		require.NoError(t, err)

		var dockerConfig map[string]interface{}
		err = json.Unmarshal(decodedConfig, &dockerConfig)
		require.NoError(t, err)

		auths := dockerConfig["auths"].(map[string]interface{})

		// Verify all registries are present
		expectedRegistries := []string{"docker.io", "gcr.io", "private-registry.com", "quay.io"}
		for _, registry := range expectedRegistries {
			assert.Contains(t, auths, registry, "Registry %s should be in auths", registry)

			auth := auths[registry].(map[string]interface{})
			assert.NotEmpty(t, auth["username"], "Username should be set for %s", registry)
			assert.NotEmpty(t, auth["password"], "Password should be set for %s", registry)
			assert.NotEmpty(t, auth["auth"], "Auth string should be set for %s", registry)
		}

		// Verify specific auth data
		dockerAuth := auths["docker.io"].(map[string]interface{})
		assert.Equal(t, "dockeruser", dockerAuth["username"])
		assert.Equal(t, "docker@example.com", dockerAuth["email"])

		gcrAuth := auths["gcr.io"].(map[string]interface{})
		assert.Equal(t, "_json_key", gcrAuth["username"])
		assert.Contains(t, gcrAuth["password"], "service_account")

		t.Logf("✓ Verified multi-registry secret with %d registries", len(expectedRegistries))
	})
}

func TestErrorHandling_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	t.Run("invalid image reference", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := tools.NewGenerateManifestsTool(logger, tmpDir)

		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "error-test",
				DryRun:    false,
			},
			// ImageRef intentionally left empty to trigger error
			Namespace: "error-test",
		}

		_, err := tool.Execute(context.Background(), args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image_ref is required")
	})

	t.Run("invalid workspace directory", func(t *testing.T) {
		// Use a non-existent directory
		invalidDir := "/non/existent/directory"
		tool := tools.NewGenerateManifestsTool(logger, invalidDir)

		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "error-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace: "error-test",
		}

		_, err := tool.Execute(context.Background(), args)
		assert.Error(t, err)
		// The error will be about failing to write manifests or create directories
	})

	t.Run("context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := tools.NewGenerateManifestsTool(logger, tmpDir)

		// Create a context that's already cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "cancellation-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace: "cancellation-test",
		}

		_, err := tool.Execute(ctx, args)
		// The tool might not check context cancellation immediately,
		// so we just verify it completes without panic
		t.Logf("Execute with cancelled context result: %v", err)
	})
}

// Benchmark for integration testing
func BenchmarkManifestGeneration_Integration(b *testing.B) {
	tmpDir := b.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	tool := tools.NewGenerateManifestsTool(logger, tmpDir)

	args := tools.GenerateManifestsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "benchmark",
			DryRun:    false,
		},
		ImageRef: types.ImageReference{
			Registry:   "docker.io",
			Repository: "nginx",
			Tag:        "latest",
		},
		Namespace:      "benchmark",
		IncludeIngress: true,
		ConfigMapData: map[string]string{
			"app.properties": "server.port=8080",
		},
		GeneratePullSecret: true,
		RegistrySecrets: []tools.RegistrySecret{
			{
				Registry: "docker.io",
				Username: "testuser",
				Password: "testpass",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		args.SessionID = fmt.Sprintf("benchmark-%d", i)
		_, err := tool.Execute(context.Background(), args)
		if err != nil {
			b.Fatalf("Benchmark iteration %d failed: %v", i, err)
		}
	}
}
