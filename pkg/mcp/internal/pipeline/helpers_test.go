package pipeline

import (
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/stretchr/testify/assert"
)

func TestMetadataManager(t *testing.T) {
	// Create test metadata
	metadata := map[pipeline.MetadataKey]any{
		"string_key":   "test_value",
		"int_key":      42,
		"bool_key":     true,
		"duration_key": 5 * time.Minute,
	}

	manager := NewMetadataManager(metadata)

	// Test string retrieval
	value, exists := manager.GetString("string_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", value)

	// Test string with default
	defaultValue := manager.GetStringWithDefault("missing_key", "default")
	assert.Equal(t, "default", defaultValue)

	// Test int retrieval
	intValue, exists := manager.GetInt("int_key")
	assert.True(t, exists)
	assert.Equal(t, 42, intValue)

	// Test bool retrieval
	boolValue, exists := manager.GetBool("bool_key")
	assert.True(t, exists)
	assert.True(t, boolValue)

	// Test duration retrieval
	durationValue, exists := manager.GetDuration("duration_key")
	assert.True(t, exists)
	assert.Equal(t, 5*time.Minute, durationValue)

	// Test non-existent key
	_, exists = manager.GetString("non_existent")
	assert.False(t, exists)

	// Test setting value
	manager.Set("new_key", "new_value")
	value, exists = manager.GetString("new_key")
	assert.True(t, exists)
	assert.Equal(t, "new_value", value)
}

func TestAnalysisConverter(t *testing.T) {
	converter := NewAnalysisConverter()

	// Test valid analysis map
	analysisData := map[string]interface{}{
		"language":  "go",
		"framework": "gin",
		"port":      8080,
	}

	analysisMap, err := converter.ToMap(analysisData)
	assert.NoError(t, err)
	assert.Equal(t, analysisData, analysisMap)

	// Test extracting language
	language := converter.GetLanguage(analysisMap)
	assert.Equal(t, "go", language)

	// Test extracting framework
	framework := converter.GetFramework(analysisMap)
	assert.Equal(t, "gin", framework)

	// Test extracting port
	port := converter.GetPort(analysisMap)
	assert.Equal(t, 8080, port)

	// Test missing port
	analysisNoPort := map[string]interface{}{
		"language": "python",
	}
	port = converter.GetPort(analysisNoPort)
	assert.Equal(t, 0, port)
}

func TestInsightGenerator(t *testing.T) {
	generator := NewInsightGenerator()

	// Test repository insights with analysis
	metadata := map[pipeline.MetadataKey]any{
		pipeline.RepoAnalysisResultKey: map[string]interface{}{
			"language":  "javascript",
			"framework": "express",
		},
	}
	manager := NewMetadataManager(metadata)

	insights := generator.GenerateRepositoryInsights(manager)
	assert.Contains(t, insights, "Repository analysis completed successfully")
	assert.Contains(t, insights, "Detected javascript project")
	assert.Contains(t, insights, "Framework: express")

	// Test Docker insights
	dockerMetadata := map[pipeline.MetadataKey]any{
		"build_logs":     "Build successful",
		"build_duration": 1 * time.Minute,
	}
	dockerManager := NewMetadataManager(dockerMetadata)

	dockerInsights := generator.GenerateDockerInsights(dockerManager)
	assert.Contains(t, dockerInsights, "Container image built successfully")
	assert.Contains(t, dockerInsights, "Build logs available for review")
	assert.Contains(t, dockerInsights, "Fast build time achieved")

	// Test manifest insights
	manifestMetadata := map[pipeline.MetadataKey]any{
		"manifest_path": "/path/to/manifests",
	}
	manifestManager := NewMetadataManager(manifestMetadata)

	manifestInsights := generator.GenerateManifestInsights(manifestManager)
	assert.Contains(t, manifestInsights, "Kubernetes manifests generated successfully")
	assert.Contains(t, manifestInsights, "Manifests saved to /path/to/manifests")

	// Test common insights
	commonMetadata := map[pipeline.MetadataKey]any{
		"ai_token_usage": 150,
	}
	commonManager := NewMetadataManager(commonMetadata)

	commonInsights := generator.GenerateCommonInsights(commonManager)
	assert.Contains(t, commonInsights, "AI analysis used 150 tokens")
}
