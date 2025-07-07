package scan

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemediationGenerator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	require.NotNil(t, rg)
	assert.NotNil(t, rg.logger)
}

func TestRemediationGenerator_GenerateRemediationPlan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	tests := []struct {
		name        string
		secrets     []ScannedSecret
		expectedLen int
	}{
		{
			name:        "no_secrets",
			secrets:     []ScannedSecret{},
			expectedLen: 0,
		},
		{
			name: "single_secret",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value",
					Line:  1,
				},
			},
			expectedLen: 1,
		},
		{
			name: "multiple_secrets_same_type",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value-1",
					Line:  1,
				},
				{
					Type:  "api-key",
					Value: "secret-value-2",
					Line:  2,
				},
			},
			expectedLen: 2,
		},
		{
			name: "multiple_secrets_different_types",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value-1",
					Line:  1,
				},
				{
					Type:  "database-password",
					Value: "secret-value-2",
					Line:  2,
				},
			},
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := rg.GenerateRemediationPlan(tt.secrets)

			require.NotNil(t, plan)
			assert.Equal(t, tt.expectedLen, len(plan.SecretReferences))
			assert.Equal(t, "kubernetes-secrets", plan.PreferredManager)
			assert.Greater(t, len(plan.ImmediateActions), 0)
			assert.Greater(t, len(plan.MigrationSteps), 0)
			assert.NotNil(t, plan.ConfigMapEntries)
		})
	}
}

func TestRemediationGenerator_GenerateKubernetesSecrets(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	tests := []struct {
		name          string
		secrets       []ScannedSecret
		sessionID     string
		expectedCount int
		expectedError bool
	}{
		{
			name:          "no_secrets",
			secrets:       []ScannedSecret{},
			sessionID:     "test-session",
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "single_secret",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value",
					Line:  1,
				},
			},
			sessionID:     "test-session",
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "multiple_secrets_same_type",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value-1",
					Line:  1,
				},
				{
					Type:  "api-key",
					Value: "secret-value-2",
					Line:  2,
				},
			},
			sessionID:     "test-session",
			expectedCount: 1, // Should group same type into one manifest
			expectedError: false,
		},
		{
			name: "multiple_secrets_different_types",
			secrets: []ScannedSecret{
				{
					Type:  "api-key",
					Value: "secret-value-1",
					Line:  1,
				},
				{
					Type:  "database-password",
					Value: "secret-value-2",
					Line:  2,
				},
			},
			sessionID:     "test-session",
			expectedCount: 2, // Should create separate manifests for different types
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifests, err := rg.GenerateKubernetesSecrets(tt.secrets, tt.sessionID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(manifests))

				// Verify manifest structure for non-empty results
				for _, manifest := range manifests {
					assert.NotEmpty(t, manifest.Name)
					assert.NotEmpty(t, manifest.Content)
					assert.NotEmpty(t, manifest.FilePath)
					assert.NotNil(t, manifest.Keys)
				}
			}
		})
	}
}

func TestRemediationGenerator_GenerateRemediationPlan_ImmediateActions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{
			Type:  "api-key",
			Value: "secret-value",
			Line:  1,
		},
	}

	plan := rg.GenerateRemediationPlan(secrets)

	expectedActions := []string{
		"Stop committing files with detected secrets",
		"Remove secrets from version control history if already committed",
		"Rotate any exposed credentials",
		"Review and update .gitignore to prevent future commits",
	}

	assert.Equal(t, expectedActions, plan.ImmediateActions)
}

func TestRemediationGenerator_GenerateRemediationPlan_MigrationSteps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{
			Type:  "api-key",
			Value: "secret-value",
			Line:  1,
		},
	}

	plan := rg.GenerateRemediationPlan(secrets)

	expectedSteps := []string{
		"Create Kubernetes Secret manifests for sensitive data",
		"Update Deployment manifests to reference secrets via secretKeyRef",
		"Test the application with externalized secrets",
		"Remove hardcoded secrets from source files",
		"Implement proper secret rotation procedures",
	}

	assert.Equal(t, expectedSteps, plan.MigrationSteps)
}

func TestRemediationGenerator_GenerateRemediationPlan_SecretReferences(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{
			Type:  "api-key",
			Value: "secret-value",
			Line:  1,
		},
	}

	plan := rg.GenerateRemediationPlan(secrets)

	require.Equal(t, 1, len(plan.SecretReferences))
	ref := plan.SecretReferences[0]

	assert.Equal(t, "app-api-key-secrets", ref.SecretName)
	assert.Equal(t, "api-key-1", ref.SecretKey)
	assert.Equal(t, "API-KEY-1_VAR", ref.OriginalEnvVar)
	assert.Equal(t, "secretKeyRef: {name: app-api-key-secrets, key: api-key-1}", ref.KubernetesRef)
}

func TestRemediationGenerator_GenerateKubernetesSecrets_EmptySessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{
			Type:  "api-key",
			Value: "secret-value",
			Line:  1,
		},
	}

	manifests, err := rg.GenerateKubernetesSecrets(secrets, "")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(manifests))
}

func TestRemediationGenerator_GenerateKubernetesSecrets_LongSessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{
			Type:  "api-key",
			Value: "secret-value",
			Line:  1,
		},
	}

	longSessionID := "very-long-session-id-that-exceeds-normal-length-limits"
	manifests, err := rg.GenerateKubernetesSecrets(secrets, longSessionID)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(manifests))
}

// BenchmarkRemediationGenerator_GenerateRemediationPlan benchmarks remediation plan generation
func BenchmarkRemediationGenerator_GenerateRemediationPlan(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{Type: "api-key", Value: "secret-1", Line: 1},
		{Type: "database-password", Value: "secret-2", Line: 2},
		{Type: "certificate", Value: "secret-3", Line: 3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan := rg.GenerateRemediationPlan(secrets)
		_ = plan
	}
}

// BenchmarkRemediationGenerator_GenerateKubernetesSecrets benchmarks secret manifest generation
func BenchmarkRemediationGenerator_GenerateKubernetesSecrets(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rg := NewRemediationGenerator(logger)

	secrets := []ScannedSecret{
		{Type: "api-key", Value: "secret-1", Line: 1},
		{Type: "database-password", Value: "secret-2", Line: 2},
		{Type: "certificate", Value: "secret-3", Line: 3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manifests, err := rg.GenerateKubernetesSecrets(secrets, "test-session")
		require.NoError(b, err)
		_ = manifests
	}
}
