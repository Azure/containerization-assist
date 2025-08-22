package prompts

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_LoadEmbeddedTemplates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	config := ManagerConfig{
		EnableHotReload: false,
		AllowOverride:   false,
	}

	manager, err := NewManager(logger, config)
	require.NoError(t, err)

	// Should have loaded embedded templates
	templates := manager.ListTemplates()
	assert.Greater(t, len(templates), 0, "Should load embedded templates")

	// Check specific templates exist
	dockerfileTemplate, err := manager.GetTemplate("dockerfile-fix")
	require.NoError(t, err)
	assert.Equal(t, "Dockerfile Build Error Fix", dockerfileTemplate.Name)
	assert.Equal(t, "docker", dockerfileTemplate.Category)

	k8sTemplate, err := manager.GetTemplate("kubernetes-manifest-fix")
	require.NoError(t, err)
	assert.Equal(t, "Kubernetes Manifest Error Fix", k8sTemplate.Name)
	assert.Equal(t, "kubernetes", k8sTemplate.Category)
}

func TestManager_GetTemplatesByCategory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	dockerTemplates := manager.GetTemplatesByCategory("docker")
	assert.Greater(t, len(dockerTemplates), 0)

	for _, template := range dockerTemplates {
		assert.Equal(t, "docker", template.Category)
	}

	k8sTemplates := manager.GetTemplatesByCategory("kubernetes")
	assert.Greater(t, len(k8sTemplates), 0)

	for _, template := range k8sTemplates {
		assert.Equal(t, "kubernetes", template.Category)
	}
}

func TestManager_GetTemplatesByTag(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	errorFixTemplates := manager.GetTemplatesByTag("error-fix")
	assert.Greater(t, len(errorFixTemplates), 0)

	for _, template := range errorFixTemplates {
		assert.Contains(t, template.Tags, "error-fix")
	}
}

func TestTemplate_Render(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	// Test rendering dockerfile-fix template
	data := TemplateData{
		"Language":          "java",
		"Framework":         "spring-boot",
		"Port":              8080,
		"DockerfileContent": "FROM openjdk:11\nCOPY . .\nRUN mvn package",
		"BuildError":        "mvn: command not found",
	}

	rendered, err := manager.RenderTemplate("dockerfile-fix", data)
	require.NoError(t, err)

	assert.Contains(t, rendered.Content, "java")
	assert.Contains(t, rendered.Content, "spring-boot")
	assert.Contains(t, rendered.Content, "8080")
	assert.Contains(t, rendered.Content, "mvn: command not found")
	assert.Equal(t, "dockerfile-fix", rendered.ID)
	assert.NotEmpty(t, rendered.SystemPrompt)
	assert.Greater(t, rendered.MaxTokens, int32(0))
}

func TestTemplate_RenderWithDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	// Test with minimal data, should use defaults
	data := TemplateData{
		"Language":          "java",
		"Framework":         "spring-boot",
		"DockerfileContent": "FROM openjdk:11",
		"BuildError":        "build failed",
		// Port is not provided, should use default
	}

	rendered, err := manager.RenderTemplate("dockerfile-fix", data)
	require.NoError(t, err)

	assert.Contains(t, rendered.Content, "java")
	assert.Contains(t, rendered.Content, "spring-boot")
	// Should contain default port
	assert.Contains(t, rendered.Content, "8080")
}

func TestTemplate_ValidateParameters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	// Test missing required parameter
	data := TemplateData{
		"Language": "java",
		// Missing Framework, DockerfileContent, BuildError
	}

	_, err = manager.RenderTemplate("dockerfile-fix", data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required parameter missing")
}

func TestManager_GetNonExistentTemplate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	_, err = manager.GetTemplate("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

func TestManager_GetStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	stats := manager.GetStats()

	assert.Contains(t, stats, "total_templates")
	assert.Contains(t, stats, "categories")
	assert.Contains(t, stats, "tags")

	totalTemplates := stats["total_templates"].(int)
	assert.Greater(t, totalTemplates, 0)

	categories := stats["categories"].(map[string]int)
	assert.Greater(t, len(categories), 0)

	tags := stats["tags"].(map[string]int)
	assert.Greater(t, len(tags), 0)
}

func TestTemplate_RenderComplexTemplate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	// Test rendering kubernetes manifest fix template
	data := TemplateData{
		"ManifestContent":   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
		"DeploymentError":   "ImagePullBackOff",
		"DockerfileContent": "FROM alpine:latest",
		"RepoAnalysis":      "Language: Go, Framework: none",
	}

	rendered, err := manager.RenderTemplate("kubernetes-manifest-fix", data)
	require.NoError(t, err)

	assert.Contains(t, rendered.Content, "ImagePullBackOff")
	assert.Contains(t, rendered.Content, "apiVersion: v1")
	assert.Contains(t, rendered.Content, "FROM alpine:latest")
	assert.Equal(t, float32(0.2), rendered.Temperature) // Should use template-specific temperature
}

func TestTemplate_RenderErrorAnalysis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	require.NoError(t, err)

	data := TemplateData{
		"Error":   "Connection refused",
		"Context": "Docker build failed when downloading dependencies",
	}

	rendered, err := manager.RenderTemplate("error-analysis", data)
	require.NoError(t, err)

	assert.Contains(t, rendered.Content, "Connection refused")
	assert.Contains(t, rendered.Content, "Docker build failed")
	assert.Contains(t, rendered.Content, "ROOT CAUSE:")
	assert.Contains(t, rendered.Content, "FIX STEPS:")
}

// Benchmark tests

func BenchmarkTemplate_Render(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	if err != nil {
		b.Fatal(err)
	}

	data := TemplateData{
		"Language":          "java",
		"Framework":         "spring-boot",
		"Port":              8080,
		"DockerfileContent": "FROM openjdk:11\nCOPY . .\nRUN mvn package",
		"BuildError":        "mvn: command not found",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.RenderTemplate("dockerfile-fix", data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkManager_GetTemplate(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	manager, err := NewManager(logger, ManagerConfig{})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetTemplate("dockerfile-fix")
		if err != nil {
			b.Fatal(err)
		}
	}
}
