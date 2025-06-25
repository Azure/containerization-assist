package observability_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/observability"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockK8sValidationClient for testing
type MockK8sValidationClient struct {
	validationResult  *ops.ValidationResult
	dryRunResult      *ops.DryRunResult
	supportedVersions []string
	shouldError       bool
}

func (m *MockK8sValidationClient) ValidateManifest(ctx context.Context, manifest []byte) (*ops.ValidationResult, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.validationResult, nil
}

func (m *MockK8sValidationClient) DryRunManifest(ctx context.Context, manifest []byte) (*ops.DryRunResult, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.dryRunResult, nil
}

func (m *MockK8sValidationClient) GetSupportedVersions(ctx context.Context) ([]string, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.supportedVersions, nil
}

func TestManifestValidator_ValidateManifestContent(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	t.Run("valid deployment manifest", func(t *testing.T) {
		mockClient := &MockK8sValidationClient{
			validationResult: &ops.ValidationResult{
				Valid:         true,
				APIVersion:    "apps/v1",
				Kind:          "Deployment",
				SchemaVersion: "1.27",
			},
			dryRunResult: &ops.DryRunResult{
				Accepted: true,
			},
		}

		validator := ops.NewManifestValidator(logger, mockClient)

		manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
`

		options := ops.ManifestValidationOptions{
			StrictValidation: true,
		}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.Valid)
		assert.Equal(t, "apps/v1", result.APIVersion)
		assert.Equal(t, "Deployment", result.Kind)
		assert.Equal(t, "test-app", result.Name)
		assert.Equal(t, "default", result.Namespace)
		assert.Equal(t, "1.27", result.SchemaVersion)
		assert.Empty(t, result.Errors)
	})

	t.Run("invalid yaml manifest", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		invalidManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  invalid yaml: [
`

		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(invalidManifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "INVALID_YAML", result.Errors[0].Code)
		assert.Equal(t, ops.SeverityCritical, result.Errors[0].Severity)
	})

	t.Run("missing required fields", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
# Missing spec field
`

		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Valid)
		assert.Greater(t, len(result.Errors), 0)

		// Should have error about missing spec
		hasSpecError := false
		for _, err := range result.Errors {
			if err.Code == "MISSING_DEPLOYMENT_SPEC" {
				hasSpecError = true
				break
			}
		}
		assert.True(t, hasSpecError, "Should have error about missing deployment spec")
	})

	t.Run("allowed kinds validation", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		manifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx
`

		options := ops.ManifestValidationOptions{
			AllowedKinds: []string{"Deployment", "Service"},
		}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "FORBIDDEN_KIND", result.Errors[0].Code)
		assert.Contains(t, result.Errors[0].Message, "Pod")
	})

	t.Run("required labels validation", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  labels:
    app: test-app
spec:
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx
`

		options := ops.ManifestValidationOptions{
			RequiredLabels: []string{"environment", "version"},
		}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 2) // Missing both required labels

		codes := make(map[string]bool)
		for _, err := range result.Errors {
			codes[err.Code] = true
		}
		assert.True(t, codes["MISSING_REQUIRED_LABEL"])
	})

	t.Run("forbidden fields validation", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		manifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  hostNetwork: true
  containers:
  - name: app
    image: nginx
`

		options := ops.ManifestValidationOptions{
			ForbiddenFields: []string{"spec.hostNetwork"},
		}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "FORBIDDEN_FIELD", result.Errors[0].Code)
	})

	t.Run("suggestions generation", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)

		manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  # No namespace specified
spec:
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx
`

		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Greater(t, len(result.Suggestions), 0)
		assert.Contains(t, result.Suggestions[0], "namespace")
	})
}

func TestManifestValidator_ValidateManifestDirectory(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	t.Run("validate directory with multiple manifests", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test manifests
		deploymentManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx:latest
`

		serviceManifest := `
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
`

		invalidManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: invalid-app
# Missing spec field
`

		// Write manifests to files
		err := os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(deploymentManifest), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tmpDir, "service.yaml"), []byte(serviceManifest), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte(invalidManifest), 0644)
		require.NoError(t, err)

		// Also create a non-YAML file that should be ignored
		err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("This is not YAML"), 0644)
		require.NoError(t, err)

		mockClient := &MockK8sValidationClient{
			validationResult: &ops.ValidationResult{
				Valid: true,
			},
			dryRunResult: &ops.DryRunResult{
				Accepted: true,
			},
		}

		validator := ops.NewManifestValidator(logger, mockClient)
		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestDirectory(context.Background(), tmpDir, options)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should validate 3 YAML files
		assert.Equal(t, 3, result.TotalManifests)
		assert.Len(t, result.Results, 3)

		// Check that results include our files
		assert.Contains(t, result.Results, "deployment.yaml")
		assert.Contains(t, result.Results, "service.yaml")
		assert.Contains(t, result.Results, "invalid.yaml")

		// Should not include non-YAML file
		assert.NotContains(t, result.Results, "readme.txt")

		// The invalid manifest should cause overall validation to fail
		assert.False(t, result.OverallValid)
		assert.Greater(t, result.ErrorCount, 0)

		// Check specific file results
		deploymentResult := result.Results["deployment.yaml"]
		assert.True(t, deploymentResult.Valid)
		assert.Equal(t, "Deployment", deploymentResult.Kind)
		assert.Equal(t, "test-app", deploymentResult.Name)

		serviceResult := result.Results["service.yaml"]
		assert.True(t, serviceResult.Valid)
		assert.Equal(t, "Service", serviceResult.Kind)
		assert.Equal(t, "test-service", serviceResult.Name)

		invalidResult := result.Results["invalid.yaml"]
		assert.False(t, invalidResult.Valid)
		assert.Greater(t, len(invalidResult.Errors), 0)
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		validator := ops.NewManifestValidator(logger, nil)
		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestDirectory(context.Background(), tmpDir, options)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, 0, result.TotalManifests)
		assert.Equal(t, 0, result.ValidManifests)
		assert.Equal(t, 0, result.ErrorCount)
		assert.Equal(t, 0, result.WarningCount)
		assert.True(t, result.OverallValid) // Empty directory is considered valid
		assert.Len(t, result.Results, 0)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		validator := ops.NewManifestValidator(logger, nil)
		options := ops.ManifestValidationOptions{}

		result, err := validator.ValidateManifestDirectory(context.Background(), "/non/existent/path", options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestManifestValidator_SpecificValidations(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	validator := ops.NewManifestValidator(logger, nil)

	t.Run("service validation", func(t *testing.T) {
		manifest := `
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  type: ClusterIP
  # Missing ports
`

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), ops.ManifestValidationOptions{})
		require.NoError(t, err)

		// Should have warning about missing ports
		hasPortWarning := false
		for _, warning := range result.Warnings {
			if warning.Code == "MISSING_SERVICE_PORTS" {
				hasPortWarning = true
				break
			}
		}
		assert.True(t, hasPortWarning)
	})

	t.Run("configmap validation", func(t *testing.T) {
		manifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
# No data or binaryData
`

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), ops.ManifestValidationOptions{})
		require.NoError(t, err)

		// Should have warning about empty ConfigMap
		hasEmptyWarning := false
		for _, warning := range result.Warnings {
			if warning.Code == "EMPTY_CONFIGMAP" {
				hasEmptyWarning = true
				break
			}
		}
		assert.True(t, hasEmptyWarning)
	})

	t.Run("secret validation", func(t *testing.T) {
		manifest := `
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: custom/unusual-type
# No data
`

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), ops.ManifestValidationOptions{})
		require.NoError(t, err)

		// Should have warnings about empty secret and unusual type
		codes := make(map[string]bool)
		for _, warning := range result.Warnings {
			codes[warning.Code] = true
		}
		assert.True(t, codes["EMPTY_SECRET"])
		assert.True(t, codes["UNUSUAL_SECRET_TYPE"])
	})

	t.Run("ingress validation", func(t *testing.T) {
		manifest := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
spec:
  rules: []
  # Empty rules
`

		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), ops.ManifestValidationOptions{})
		require.NoError(t, err)

		// Should have warning about empty rules
		hasEmptyRulesWarning := false
		for _, warning := range result.Warnings {
			if warning.Code == "EMPTY_INGRESS_RULES" {
				hasEmptyRulesWarning = true
				break
			}
		}
		assert.True(t, hasEmptyRulesWarning)
	})
}

func TestManifestValidator_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	validator := ops.NewManifestValidator(logger, nil)

	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 80
`

	options := ops.ManifestValidationOptions{}

	// Validate multiple times to test performance
	const iterations = 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		result, err := validator.ValidateManifestContent(context.Background(), []byte(manifest), options)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Valid)
	}

	duration := time.Since(start)
	avgDuration := duration / iterations

	t.Logf("Validated %d manifests in %v (avg: %v per manifest)", iterations, duration, avgDuration)

	// Performance expectation: should be under 10ms per manifest for basic validation
	assert.Less(t, avgDuration, 10*time.Millisecond, "Validation should be fast for basic manifests")
}
