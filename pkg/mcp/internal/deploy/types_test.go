package deploy

import (
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Test GenerationOptions type
func TestGenerationOptions(t *testing.T) {
	imageRef := types.ImageReference{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
	}

	options := GenerationOptions{
		ImageRef:       imageRef,
		OutputPath:     "/output",
		Namespace:      "default",
		ServiceType:    "ClusterIP",
		Replicas:       3,
		Resources:      ResourceRequests{CPU: "100m", Memory: "128Mi"},
		Environment:    map[string]string{"ENV": "prod"},
		Secrets:        []SecretRef{{Name: "secret1", Key: "key1"}},
		IncludeIngress: true,
		HelmTemplate:   false,
		ConfigMapData:  map[string]string{"config": "value"},
	}

	if options.ImageRef.Registry != "docker.io" {
		t.Errorf("Expected ImageRef.Registry to be 'docker.io', got '%s'", options.ImageRef.Registry)
	}
	if options.OutputPath != "/output" {
		t.Errorf("Expected OutputPath to be '/output', got '%s'", options.OutputPath)
	}
	if options.Namespace != "default" {
		t.Errorf("Expected Namespace to be 'default', got '%s'", options.Namespace)
	}
	if options.ServiceType != "ClusterIP" {
		t.Errorf("Expected ServiceType to be 'ClusterIP', got '%s'", options.ServiceType)
	}
	if options.Replicas != 3 {
		t.Errorf("Expected Replicas to be 3, got %d", options.Replicas)
	}
	if options.Resources.CPU != "100m" {
		t.Errorf("Expected Resources.CPU to be '100m', got '%s'", options.Resources.CPU)
	}
	if options.Environment["ENV"] != "prod" {
		t.Errorf("Expected Environment['ENV'] to be 'prod', got '%s'", options.Environment["ENV"])
	}
	if len(options.Secrets) != 1 {
		t.Errorf("Expected 1 secret, got %d", len(options.Secrets))
	}
	if !options.IncludeIngress {
		t.Error("Expected IncludeIngress to be true")
	}
	if options.HelmTemplate {
		t.Error("Expected HelmTemplate to be false")
	}
}

// Test ResourceRequests type
func TestResourceRequests(t *testing.T) {
	resources := ResourceRequests{
		CPU:     "500m",
		Memory:  "1Gi",
		Storage: "10Gi",
	}

	if resources.CPU != "500m" {
		t.Errorf("Expected CPU to be '500m', got '%s'", resources.CPU)
	}
	if resources.Memory != "1Gi" {
		t.Errorf("Expected Memory to be '1Gi', got '%s'", resources.Memory)
	}
	if resources.Storage != "10Gi" {
		t.Errorf("Expected Storage to be '10Gi', got '%s'", resources.Storage)
	}
}

// Test SecretRef type
func TestSecretRef(t *testing.T) {
	secret := SecretRef{
		Name: "my-secret",
		Key:  "database-password",
	}

	if secret.Name != "my-secret" {
		t.Errorf("Expected Name to be 'my-secret', got '%s'", secret.Name)
	}
	if secret.Key != "database-password" {
		t.Errorf("Expected Key to be 'database-password', got '%s'", secret.Key)
	}
}

// Test ServicePort type
func TestServicePort(t *testing.T) {
	port := ServicePort{
		Name:       "http",
		Port:       80,
		TargetPort: 8080,
	}

	if port.Name != "http" {
		t.Errorf("Expected Name to be 'http', got '%s'", port.Name)
	}
	if port.Port != 80 {
		t.Errorf("Expected Port to be 80, got %d", port.Port)
	}
	if port.TargetPort != 8080 {
		t.Errorf("Expected TargetPort to be 8080, got %d", port.TargetPort)
	}
}

// Test IngressHost type
func TestIngressHost(t *testing.T) {
	host := IngressHost{
		Host: "app.example.com",
		Paths: []IngressPath{
			{Path: "/", PathType: "Prefix", ServiceName: "app-service", ServicePort: 80},
		},
	}

	if host.Host != "app.example.com" {
		t.Errorf("Expected Host to be 'app.example.com', got '%s'", host.Host)
	}
	if len(host.Paths) != 1 {
		t.Errorf("Expected 1 path, got %d", len(host.Paths))
	}
	if host.Paths[0].Path != "/" {
		t.Errorf("Expected Path to be '/', got '%s'", host.Paths[0].Path)
	}
}

// Test IngressPath type
func TestIngressPath(t *testing.T) {
	path := IngressPath{
		Path:        "/api",
		PathType:    "Prefix",
		ServiceName: "api-service",
		ServicePort: 3000,
	}

	if path.Path != "/api" {
		t.Errorf("Expected Path to be '/api', got '%s'", path.Path)
	}
	if path.PathType != "Prefix" {
		t.Errorf("Expected PathType to be 'Prefix', got '%s'", path.PathType)
	}
	if path.ServiceName != "api-service" {
		t.Errorf("Expected ServiceName to be 'api-service', got '%s'", path.ServiceName)
	}
	if path.ServicePort != 3000 {
		t.Errorf("Expected ServicePort to be 3000, got %d", path.ServicePort)
	}
}

// Test IngressTLS type
func TestIngressTLS(t *testing.T) {
	tls := IngressTLS{
		Hosts:      []string{"app.example.com", "api.example.com"},
		SecretName: "tls-secret",
	}

	if len(tls.Hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(tls.Hosts))
	}
	if tls.Hosts[0] != "app.example.com" {
		t.Errorf("Expected first host to be 'app.example.com', got '%s'", tls.Hosts[0])
	}
	if tls.SecretName != "tls-secret" {
		t.Errorf("Expected SecretName to be 'tls-secret', got '%s'", tls.SecretName)
	}
}

// Test GenerationResult type
func TestGenerationResult(t *testing.T) {
	duration := time.Second * 30
	result := GenerationResult{
		Success:        true,
		ManifestPath:   "/manifests",
		FilesGenerated: []string{"deployment.yaml", "service.yaml"},
		Duration:       duration,
		Errors:         []string{},
		Warnings:       []string{"Use specific image tag"},
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.ManifestPath != "/manifests" {
		t.Errorf("Expected ManifestPath to be '/manifests', got '%s'", result.ManifestPath)
	}
	if len(result.FilesGenerated) != 2 {
		t.Errorf("Expected 2 files generated, got %d", len(result.FilesGenerated))
	}
	if result.Duration != duration {
		t.Errorf("Expected Duration to be %v, got %v", duration, result.Duration)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
}

// Test ValidationSummary type
func TestValidationSummary(t *testing.T) {
	summary := ValidationSummary{
		Valid:           true,
		TotalFiles:      3,
		ValidFiles:      3,
		InvalidFiles:    0,
		Results:         map[string]FileValidation{},
		OverallSeverity: "info",
	}

	if !summary.Valid {
		t.Error("Expected Valid to be true")
	}
	if summary.TotalFiles != 3 {
		t.Errorf("Expected TotalFiles to be 3, got %d", summary.TotalFiles)
	}
	if summary.ValidFiles != 3 {
		t.Errorf("Expected ValidFiles to be 3, got %d", summary.ValidFiles)
	}
	if summary.InvalidFiles != 0 {
		t.Errorf("Expected InvalidFiles to be 0, got %d", summary.InvalidFiles)
	}
	if summary.OverallSeverity != "info" {
		t.Errorf("Expected OverallSeverity to be 'info', got '%s'", summary.OverallSeverity)
	}
}

// Test FileValidation type
func TestFileValidation(t *testing.T) {
	validation := FileValidation{
		Valid:  true,
		Errors: []ValidationIssue{},
		Warnings: []ValidationIssue{
			{Severity: "warning", Message: "Consider using resource limits", Field: "spec.containers", Code: "W001"},
		},
		Info: []ValidationIssue{},
	}

	if !validation.Valid {
		t.Error("Expected Valid to be true")
	}
	if len(validation.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(validation.Errors))
	}
	if len(validation.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(validation.Warnings))
	}
	if validation.Warnings[0].Severity != "warning" {
		t.Errorf("Expected warning severity to be 'warning', got '%s'", validation.Warnings[0].Severity)
	}
}

// Test ValidationIssue type
func TestValidationIssue(t *testing.T) {
	issue := ValidationIssue{
		Severity: "error",
		Message:  "Required field missing",
		Field:    "metadata.name",
		Code:     "E001",
	}

	if issue.Severity != "error" {
		t.Errorf("Expected Severity to be 'error', got '%s'", issue.Severity)
	}
	if issue.Message != "Required field missing" {
		t.Errorf("Expected Message to be 'Required field missing', got '%s'", issue.Message)
	}
	if issue.Field != "metadata.name" {
		t.Errorf("Expected Field to be 'metadata.name', got '%s'", issue.Field)
	}
	if issue.Code != "E001" {
		t.Errorf("Expected Code to be 'E001', got '%s'", issue.Code)
	}
}

// Test TemplateContext type
func TestTemplateContext(t *testing.T) {
	context := TemplateContext{
		Language:       "go",
		Framework:      "gin",
		HasTests:       true,
		HasDatabase:    true,
		IsWebApp:       true,
		HasStaticFiles: false,
		Port:           8080,
	}

	if context.Language != "go" {
		t.Errorf("Expected Language to be 'go', got '%s'", context.Language)
	}
	if context.Framework != "gin" {
		t.Errorf("Expected Framework to be 'gin', got '%s'", context.Framework)
	}
	if !context.HasTests {
		t.Error("Expected HasTests to be true")
	}
	if !context.HasDatabase {
		t.Error("Expected HasDatabase to be true")
	}
	if !context.IsWebApp {
		t.Error("Expected IsWebApp to be true")
	}
	if context.HasStaticFiles {
		t.Error("Expected HasStaticFiles to be false")
	}
	if context.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", context.Port)
	}
}
