package deploy

import (
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

// Test GenerateManifestsRequest type
func TestGenerateManifestsRequest(t *testing.T) {
	request := GenerateManifestsRequest{
		SessionID:      "session-123",
		ImageReference: "nginx:latest",
		AppName:        "web-app",
		Port:           8080,
		Namespace:      "production",
		CPURequest:     "100m",
		MemoryRequest:  "128Mi",
		CPULimit:       "500m",
		MemoryLimit:    "512Mi",
		Environment:    []SecretValue{{Name: "ENV", Value: "prod"}},
		IncludeIngress: true,
		IngressHost:    "app.example.com",
	}

	if request.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", request.SessionID)
	}
	if request.ImageReference != "nginx:latest" {
		t.Errorf("Expected ImageReference to be 'nginx:latest', got '%s'", request.ImageReference)
	}
	if request.AppName != "web-app" {
		t.Errorf("Expected AppName to be 'web-app', got '%s'", request.AppName)
	}
	if request.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", request.Port)
	}
	if request.Namespace != "production" {
		t.Errorf("Expected Namespace to be 'production', got '%s'", request.Namespace)
	}
	if request.CPURequest != "100m" {
		t.Errorf("Expected CPURequest to be '100m', got '%s'", request.CPURequest)
	}
	if request.MemoryRequest != "128Mi" {
		t.Errorf("Expected MemoryRequest to be '128Mi', got '%s'", request.MemoryRequest)
	}
	if len(request.Environment) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(request.Environment))
	}
	if !request.IncludeIngress {
		t.Error("Expected IncludeIngress to be true")
	}
	if request.IngressHost != "app.example.com" {
		t.Errorf("Expected IngressHost to be 'app.example.com', got '%s'", request.IngressHost)
	}
}

// Test SecretValue type
func TestSecretValue(t *testing.T) {
	secret := SecretValue{
		Name:  "DATABASE_PASSWORD",
		Value: "secret123",
	}

	if secret.Name != "DATABASE_PASSWORD" {
		t.Errorf("Expected Name to be 'DATABASE_PASSWORD', got '%s'", secret.Name)
	}
	if secret.Value != "secret123" {
		t.Errorf("Expected Value to be 'secret123', got '%s'", secret.Value)
	}
}

// Test SecretInfo type
func TestSecretInfo(t *testing.T) {
	info := SecretInfo{
		Name:        "db-password",
		Value:       "hidden",
		Type:        "password",
		SecretName:  "app-secrets",
		SecretKey:   "db-password",
		IsSecret:    true,
		IsSensitive: true,
		Pattern:     "password_pattern",
		Confidence:  0.95,
		Reason:      "Contains password keyword",
	}

	if info.Name != "db-password" {
		t.Errorf("Expected Name to be 'db-password', got '%s'", info.Name)
	}
	if info.Value != "hidden" {
		t.Errorf("Expected Value to be 'hidden', got '%s'", info.Value)
	}
	if info.Type != "password" {
		t.Errorf("Expected Type to be 'password', got '%s'", info.Type)
	}
	if info.SecretName != "app-secrets" {
		t.Errorf("Expected SecretName to be 'app-secrets', got '%s'", info.SecretName)
	}
	if info.SecretKey != "db-password" {
		t.Errorf("Expected SecretKey to be 'db-password', got '%s'", info.SecretKey)
	}
	if !info.IsSecret {
		t.Error("Expected IsSecret to be true")
	}
	if !info.IsSensitive {
		t.Error("Expected IsSensitive to be true")
	}
	if info.Pattern != "password_pattern" {
		t.Errorf("Expected Pattern to be 'password_pattern', got '%s'", info.Pattern)
	}
	if info.Confidence != 0.95 {
		t.Errorf("Expected Confidence to be 0.95, got %f", info.Confidence)
	}
	if info.Reason != "Contains password keyword" {
		t.Errorf("Expected Reason to be 'Contains password keyword', got '%s'", info.Reason)
	}
}

// Test ManifestFile type
func TestManifestFile(t *testing.T) {
	manifest := ManifestFile{
		Kind:       "Deployment",
		Name:       "web-app",
		Content:    "apiVersion: apps/v1\nkind: Deployment",
		FilePath:   "/manifests/deployment.yaml",
		IsSecret:   false,
		SecretInfo: "",
	}

	if manifest.Kind != "Deployment" {
		t.Errorf("Expected Kind to be 'Deployment', got '%s'", manifest.Kind)
	}
	if manifest.Name != "web-app" {
		t.Errorf("Expected Name to be 'web-app', got '%s'", manifest.Name)
	}
	if len(manifest.Content) == 0 {
		t.Error("Expected Content to not be empty")
	}
	if manifest.FilePath != "/manifests/deployment.yaml" {
		t.Errorf("Expected FilePath to be '/manifests/deployment.yaml', got '%s'", manifest.FilePath)
	}
	if manifest.IsSecret {
		t.Error("Expected IsSecret to be false")
	}
}

// Test ValidationResult type
func TestValidationResult(t *testing.T) {
	result := api.ValidationResult{
		Valid:  true,
		Errors: []api.ValidationError{},
		Warnings: []api.ValidationWarning{
			{
				Code:    "SPECIFIC_TAG",
				Message: "Consider using specific image tag",
			},
		},
		Metadata: validation.Metadata{
			ValidatedAt:      time.Now(),
			ValidatorName:    "TestValidator",
			ValidatorVersion: "1.0.0",
		},
		Details: map[string]interface{}{
			"manifest_path": "deployment.yaml",
		},
	}

	if manifestPath, ok := result.Details["manifest_path"].(string); !ok || manifestPath != "deployment.yaml" {
		t.Errorf("Expected manifest_path to be 'deployment.yaml', got '%v'", result.Details["manifest_path"])
	}
	if !result.Valid {
		t.Error("Expected Valid to be true")
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

// Test CommonManifestContext type
func TestCommonManifestContext(t *testing.T) {
	context := CommonManifestContext{
		ManifestsGenerated:  3,
		SecretsDetected:     2,
		SecretsExternalized: 1,
		ResourceTypes:       []string{"Deployment", "Service", "Ingress"},
		DeploymentStrategy:  "rolling",
	}

	if context.ManifestsGenerated != 3 {
		t.Errorf("Expected ManifestsGenerated to be 3, got %d", context.ManifestsGenerated)
	}
	if context.SecretsDetected != 2 {
		t.Errorf("Expected SecretsDetected to be 2, got %d", context.SecretsDetected)
	}
	if context.SecretsExternalized != 1 {
		t.Errorf("Expected SecretsExternalized to be 1, got %d", context.SecretsExternalized)
	}
	if len(context.ResourceTypes) != 3 {
		t.Errorf("Expected 3 resource types, got %d", len(context.ResourceTypes))
	}
	if context.ResourceTypes[0] != "Deployment" {
		t.Errorf("Expected first resource type to be 'Deployment', got '%s'", context.ResourceTypes[0])
	}
	if context.DeploymentStrategy != "rolling" {
		t.Errorf("Expected DeploymentStrategy to be 'rolling', got '%s'", context.DeploymentStrategy)
	}
}
