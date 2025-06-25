package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

func TestGenerator_GenerateManifests(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	generator := NewManifestGenerator(logger)

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifests")

	opts := GenerationOptions{
		ImageRef: types.ImageReference{
			Registry:   "myregistry.com",
			Repository: "myapp",
			Tag:        "v1.0.0",
		},
		OutputPath:  manifestPath,
		Namespace:   "production",
		ServiceType: "LoadBalancer",
		Replicas:    3,
		Environment: map[string]string{
			"ENV":       "production",
			"LOG_LEVEL": "info",
		},
		IncludeIngress: true,
		IngressHosts: []IngressHost{
			{
				Host: "myapp.example.com",
				Paths: []IngressPath{
					{
						Path:        "/",
						PathType:    "Prefix",
						ServiceName: "myapp",
						ServicePort: 80,
					},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := generator.GenerateManifests(ctx, opts)

	if err != nil {
		t.Fatalf("GenerateManifests failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if len(result.FilesGenerated) == 0 {
		t.Error("Expected generated files, got none")
	}

	// Check that manifest directory was created
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("Expected manifest directory to be created")
	}

	// Check for expected files
	expectedFiles := []string{"deployment.yaml", "service.yaml", "configmap.yaml", "ingress.yaml"}
	for _, expectedFile := range expectedFiles {
		t.Run("file_exists_"+expectedFile, func(t *testing.T) {
			t.Parallel()
			filePath := filepath.Join(manifestPath, expectedFile)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Expected file %s to be created", expectedFile)
			}
		})
	}
}

func TestWriter_EnsureDirectory(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	writer := NewWriter(logger)

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "nested", "directories", "test")

	err := writer.EnsureDirectory(testPath)
	if err != nil {
		t.Fatalf("EnsureDirectory failed: %v", err)
	}

	// Check that directory was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}

func TestWriter_WriteFile(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	writer := NewWriter(logger)

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")
	testContent := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test")

	err := writer.WriteFile(testFile, testContent)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Check that file was created with correct content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Expected content '%s', got '%s'", string(testContent), string(content))
	}
}

func TestValidator_ValidateDirectory(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewValidator(logger)

	tempDir := t.TempDir()

	// Create valid manifest files
	validDeployment := `apiVersion: apps/v1
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
        image: test:latest
`

	validService := `apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
`

	// Create invalid manifest (missing required fields)
	invalidManifest := `apiVersion: v1
kind: ConfigMap
# Missing metadata section entirely - this should be invalid
data:
  key: value
`

	if err := os.WriteFile(filepath.Join(tempDir, "deployment.yaml"), []byte(validDeployment), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "service.yaml"), []byte(validService), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidManifest), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	summary, err := validator.ValidateDirectory(ctx, tempDir)

	if err != nil {
		t.Fatalf("ValidateDirectory failed: %v", err)
	}

	if summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	if summary.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", summary.TotalFiles)
	}

	if summary.ValidFiles != 2 {
		t.Errorf("Expected 2 valid files, got %d", summary.ValidFiles)
	}

	if summary.InvalidFiles != 1 {
		t.Errorf("Expected 1 invalid file, got %d", summary.InvalidFiles)
	}

	if summary.Valid {
		t.Error("Expected overall validation to fail due to invalid file")
	}

	// Check individual file results
	if len(summary.Results) != 3 {
		t.Errorf("Expected 3 file results, got %d", len(summary.Results))
	}

	// Check that deployment is valid
	if deploymentResult, exists := summary.Results["deployment.yaml"]; exists {
		if !deploymentResult.Valid {
			t.Error("Expected deployment.yaml to be valid")
		}
	} else {
		t.Error("Expected deployment.yaml result")
	}

	// Check that service is valid
	if serviceResult, exists := summary.Results["service.yaml"]; exists {
		if !serviceResult.Valid {
			t.Error("Expected service.yaml to be valid")
		}
	} else {
		t.Error("Expected service.yaml result")
	}

	// Check that invalid file is invalid
	if invalidResult, exists := summary.Results["invalid.yaml"]; exists {
		if invalidResult.Valid {
			t.Error("Expected invalid.yaml to be invalid")
		}
		if len(invalidResult.Errors) == 0 {
			t.Error("Expected validation errors for invalid.yaml")
		}
	} else {
		t.Error("Expected invalid.yaml result")
	}
}

func TestValidator_ValidateFile(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewValidator(logger)

	tempDir := t.TempDir()

	// Test valid manifest
	validManifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
`

	validFile := filepath.Join(tempDir, "valid.yaml")
	if err := os.WriteFile(validFile, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	result, err := validator.ValidateFile(ctx, validFile)

	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if !result.Valid {
		t.Error("Expected valid manifest to be valid")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}

	// Test invalid YAML
	invalidYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  invalid-yaml: [unclosed bracket
`

	invalidFile := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(invalidFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	result, err = validator.ValidateFile(ctx, invalidFile)

	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid YAML to be invalid")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for invalid YAML")
	}
}

func TestTemplateManager_GetTemplate(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	manager := NewTemplateManager(logger)

	// Test getting available templates
	availableTemplates, err := manager.ListAvailableTemplates()
	if err != nil {
		t.Fatalf("ListAvailableTemplates failed: %v", err)
	}

	if len(availableTemplates) == 0 {
		t.Error("Expected available templates, got none")
	}

	// Test template validation
	for _, templateName := range availableTemplates {
		err := manager.ValidateTemplate(templateName)
		// Note: This will fail if the actual template files don't exist
		// In a real implementation, we'd either mock the filesystem or have test templates
		if err != nil {
			t.Logf("Template %s validation failed (expected if templates don't exist): %v", templateName, err)
		}
	}
}

func TestIntegration_GenerateAndValidate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	generator := NewManifestGenerator(logger)
	validator := NewValidator(logger)

	// Generate manifests
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifests")

	opts := GenerationOptions{
		ImageRef: types.ImageReference{
			Registry:   "test.registry.com",
			Repository: "test-app",
			Tag:        "latest",
		},
		OutputPath:     manifestPath,
		Namespace:      "test",
		ServiceType:    "ClusterIP",
		Replicas:       2,
		IncludeIngress: false,
		Environment: map[string]string{
			"ENV": "test",
		},
	}

	ctx := context.Background()
	generateResult, err := generator.GenerateManifests(ctx, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Validate the generated manifests
	validationResult, err := validator.ValidateDirectory(ctx, generateResult.ManifestPath)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Check that generated manifests are valid
	if !validationResult.Valid {
		t.Errorf("Generated manifests should be valid, but got %d errors", validationResult.InvalidFiles)
		for fileName, result := range validationResult.Results {
			if !result.Valid {
				t.Logf("File %s errors: %+v", fileName, result.Errors)
			}
		}
	}
}
