package tools_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tmpDir := t.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	tool := tools.NewGenerateManifestsTool(logger, tmpDir)

	t.Run("validation enabled with valid manifests", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace: "test-namespace",
			ConfigMapData: map[string]string{
				"app.properties": "key=value",
			},
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				StrictValidation: false,
				SkipDryRun:       true, // Skip dry-run to avoid kubectl dependency
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Verify validation was performed
		assert.NotNil(t, manifestResult.ValidationResult)
		assert.True(t, manifestResult.ValidationResult.Enabled)
		assert.Greater(t, manifestResult.ValidationResult.TotalFiles, 0)

		t.Logf("Validation Summary: %d/%d files valid, %d errors, %d warnings",
			manifestResult.ValidationResult.ValidFiles,
			manifestResult.ValidationResult.TotalFiles,
			manifestResult.ValidationResult.ErrorCount,
			manifestResult.ValidationResult.WarningCount)

		// Log individual file results
		for fileName, fileResult := range manifestResult.ValidationResult.Results {
			t.Logf("File %s: valid=%v, kind=%s, errors=%d, warnings=%d",
				fileName, fileResult.Valid, fileResult.Kind,
				fileResult.ErrorCount, fileResult.WarningCount)

			if !fileResult.Valid {
				for _, err := range fileResult.Errors {
					t.Logf("  Error: %s - %s", err.Code, err.Message)
				}
			}
		}
	})

	t.Run("validation with allowed kinds restriction", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-allowed-kinds",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			IncludeIngress:    true, // This will generate an Ingress
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				AllowedKinds: []string{"Deployment", "Service"}, // Exclude Ingress
				SkipDryRun:   true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Validation should fail due to forbidden Ingress
		assert.NotNil(t, manifestResult.ValidationResult)
		assert.False(t, manifestResult.ValidationResult.OverallValid)
		assert.Greater(t, manifestResult.ValidationResult.ErrorCount, 0)

		// Find the Ingress validation result
		ingressResult := findValidationResult(manifestResult.ValidationResult.Results, "Ingress")
		require.NotNil(t, ingressResult, "Should have Ingress validation result")
		assert.False(t, ingressResult.Valid)

		// Should have FORBIDDEN_KIND error
		hasForbiddenKindError := false
		for _, err := range ingressResult.Errors {
			if err.Code == "FORBIDDEN_KIND" {
				hasForbiddenKindError = true
				break
			}
		}
		assert.True(t, hasForbiddenKindError, "Should have FORBIDDEN_KIND error for Ingress")
	})

	t.Run("validation with required labels", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-required-labels",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				RequiredLabels: []string{"environment", "version"},
				SkipDryRun:     true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Validation should fail due to missing required labels
		assert.NotNil(t, manifestResult.ValidationResult)
		assert.False(t, manifestResult.ValidationResult.OverallValid)
		assert.Greater(t, manifestResult.ValidationResult.ErrorCount, 0)

		// Check that all manifests have missing label errors
		labelErrorCount := 0
		for _, fileResult := range manifestResult.ValidationResult.Results {
			for _, err := range fileResult.Errors {
				if err.Code == "MISSING_REQUIRED_LABEL" {
					labelErrorCount++
				}
			}
		}
		assert.Greater(t, labelErrorCount, 0, "Should have missing required label errors")
	})

	t.Run("validation with workflow labels satisfies requirements", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-workflow-labels",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace: "test-namespace",
			WorkflowLabels: map[string]string{
				"environment": "production",
				"version":     "v1.0.0",
				"team":        "platform",
			},
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				RequiredLabels: []string{"environment", "version"},
				SkipDryRun:     true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Validation should pass since workflow labels satisfy requirements
		assert.NotNil(t, manifestResult.ValidationResult)
		// Note: There might still be some validation warnings, but no critical errors

		// Check that no missing label errors exist
		labelErrorCount := 0
		for _, fileResult := range manifestResult.ValidationResult.Results {
			for _, err := range fileResult.Errors {
				if err.Code == "MISSING_REQUIRED_LABEL" {
					labelErrorCount++
				}
			}
		}
		assert.Equal(t, 0, labelErrorCount, "Should not have missing required label errors when workflow labels are provided")
	})

	t.Run("validation disabled", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-disabled",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			ValidateManifests: false, // Validation disabled
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Validation should not be performed
		assert.Nil(t, manifestResult.ValidationResult)
	})

	t.Run("validation performance test", func(t *testing.T) {
		const iterations = 5

		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "validation-performance",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:      "test-namespace",
			IncludeIngress: true,
			ConfigMapData: map[string]string{
				"config1": "value1",
				"config2": "value2",
			},
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				SkipDryRun: true,
			},
		}

		totalDuration := time.Duration(0)
		totalValidationDuration := time.Duration(0)

		for i := 0; i < iterations; i++ {
			args.SessionID = fmt.Sprintf("validation-performance-%d", i)

			start := time.Now()
			result, err := tool.Execute(context.Background(), args)
			duration := time.Since(start)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Type assert the result to the expected type
			manifestResult, ok := result.(*tools.GenerateManifestsResult)
			require.True(t, ok, "result should be of type *GenerateManifestsResult")
			require.NotNil(t, manifestResult.ValidationResult)

			totalDuration += duration
			totalValidationDuration += manifestResult.ValidationResult.Duration
		}

		avgDuration := totalDuration / iterations
		avgValidationDuration := totalValidationDuration / iterations

		t.Logf("Performance results over %d iterations:", iterations)
		t.Logf("  Average total duration: %v", avgDuration)
		t.Logf("  Average validation duration: %v", avgValidationDuration)
		t.Logf("  Validation overhead: %.1f%%",
			float64(avgValidationDuration)/float64(avgDuration)*100)

		// Validation should not add significant overhead
		assert.Less(t, avgValidationDuration, 500*time.Millisecond,
			"Validation should be fast")
		assert.Less(t, float64(avgValidationDuration)/float64(avgDuration), 0.5,
			"Validation should not be more than 50% of total time")
	})
}

// Helper function to find validation result by kind
func findValidationResult(results map[string]tools.FileValidation, kind string) *tools.FileValidation {
	for _, result := range results {
		if result.Kind == kind {
			return &result
		}
	}
	return nil
}

func TestValidationOptions_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tmpDir := t.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	tool := tools.NewGenerateManifestsTool(logger, tmpDir)

	t.Run("forbidden fields validation", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "forbidden-fields-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				ForbiddenFields: []string{"spec.hostNetwork", "spec.privileged"},
				SkipDryRun:      true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Should pass since we don't generate manifests with forbidden fields
		assert.NotNil(t, manifestResult.ValidationResult)

		// Check that no forbidden field errors exist
		forbiddenFieldErrors := 0
		for _, fileResult := range manifestResult.ValidationResult.Results {
			for _, err := range fileResult.Errors {
				if err.Code == "FORBIDDEN_FIELD" {
					forbiddenFieldErrors++
				}
			}
		}
		assert.Equal(t, 0, forbiddenFieldErrors, "Should not have forbidden field errors in generated manifests")
	})

	t.Run("strict validation mode", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "strict-validation-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				StrictValidation: true,
				SkipDryRun:       true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Verify strict validation was applied
		assert.NotNil(t, manifestResult.ValidationResult)
		assert.True(t, manifestResult.ValidationResult.Enabled)

		t.Logf("Strict validation results: %d total files, %d valid, %d errors, %d warnings",
			manifestResult.ValidationResult.TotalFiles,
			manifestResult.ValidationResult.ValidFiles,
			manifestResult.ValidationResult.ErrorCount,
			manifestResult.ValidationResult.WarningCount)
	})

	t.Run("k8s version specification", func(t *testing.T) {
		args := tools.GenerateManifestsArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "k8s-version-test",
				DryRun:    false,
			},
			ImageRef: types.ImageReference{
				Registry:   "docker.io",
				Repository: "nginx",
				Tag:        "latest",
			},
			Namespace:         "test-namespace",
			ValidateManifests: true,
			ValidationOptions: tools.ValidationOptions{
				K8sVersion: "1.27.0",
				SkipDryRun: true,
			},
		}

		result, err := tool.Execute(context.Background(), args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Type assert the result to the expected type
		manifestResult, ok := result.(*tools.GenerateManifestsResult)
		require.True(t, ok, "result should be of type *GenerateManifestsResult")

		// Verify validation was performed
		assert.NotNil(t, manifestResult.ValidationResult)
		assert.True(t, manifestResult.ValidationResult.Enabled)

		t.Logf("K8s version validation completed for version: %s",
			manifestResult.ValidationResult.K8sVersion)
	})
}
