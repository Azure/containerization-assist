package pipeline

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/genericutils"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/pipeline"
)

// MetadataManager provides type-safe access to pipeline metadata
type MetadataManager struct {
	metadata map[pipeline.MetadataKey]any
}

// NewMetadataManager creates a new metadata manager
func NewMetadataManager(metadata map[pipeline.MetadataKey]any) *MetadataManager {
	if metadata == nil {
		metadata = make(map[pipeline.MetadataKey]any)
	}
	return &MetadataManager{metadata: metadata}
}

// GetString safely retrieves a string value from metadata
func (m *MetadataManager) GetString(key string) (string, bool) {
	if value, exists := m.metadata[pipeline.MetadataKey(key)]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetStringWithDefault retrieves a string value with a default fallback
func (m *MetadataManager) GetStringWithDefault(key, defaultValue string) string {
	if value, exists := m.GetString(key); exists {
		return value
	}
	return defaultValue
}

// GetInt safely retrieves an int value from metadata
func (m *MetadataManager) GetInt(key string) (int, bool) {
	if value, exists := m.metadata[pipeline.MetadataKey(key)]; exists {
		if i, ok := value.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// GetDuration safely retrieves a time.Duration value from metadata
func (m *MetadataManager) GetDuration(key string) (time.Duration, bool) {
	if value, exists := m.metadata[pipeline.MetadataKey(key)]; exists {
		if d, ok := value.(time.Duration); ok {
			return d, true
		}
	}
	return 0, false
}

// GetBool safely retrieves a bool value from metadata
func (m *MetadataManager) GetBool(key string) (bool, bool) {
	if value, exists := m.metadata[pipeline.MetadataKey(key)]; exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// Set stores a value in metadata
func (m *MetadataManager) Set(key string, value any) {
	m.metadata[pipeline.MetadataKey(key)] = value
}

// ToStringMap converts metadata to a plain string map for compatibility
func (m *MetadataManager) ToStringMap() map[string]interface{} {
	result := make(map[string]interface{}, len(m.metadata))
	for k, v := range m.metadata {
		result[string(k)] = v
	}
	return result
}

// AnalysisConverter provides type-safe conversion for repository analysis
type AnalysisConverter struct{}

// NewAnalysisConverter creates a new analysis converter
func NewAnalysisConverter() *AnalysisConverter {
	return &AnalysisConverter{}
}

// ToMap safely converts repository analysis to map format
func (c *AnalysisConverter) ToMap(analysis interface{}) (map[string]interface{}, error) {
	analysisMap, err := genericutils.SafeCast[map[string]interface{}](analysis)
	if err != nil {
		return nil, errors.NewError().Message("failed to convert analysis to map").Cause(err).Build()
	}
	return analysisMap, nil
}

// GetLanguage extracts language from analysis map
func (c *AnalysisConverter) GetLanguage(analysisMap map[string]interface{}) string {
	return genericutils.MapGetWithDefault[string](analysisMap, "language", "")
}

// GetFramework extracts framework from analysis map
func (c *AnalysisConverter) GetFramework(analysisMap map[string]interface{}) string {
	return genericutils.MapGetWithDefault[string](analysisMap, "framework", "")
}

// GetPort extracts port from analysis map
func (c *AnalysisConverter) GetPort(analysisMap map[string]interface{}) int {
	if port, ok := genericutils.MapGet[int](analysisMap, "port"); ok {
		return port
	}
	return 0
}

// InsightGenerator generates insights from pipeline state
type InsightGenerator struct {
	analysisConverter *AnalysisConverter
}

// NewInsightGenerator creates a new insight generator
func NewInsightGenerator() *InsightGenerator {
	return &InsightGenerator{
		analysisConverter: NewAnalysisConverter(),
	}
}

// GenerateRepositoryInsights generates insights for repository analysis stage
func (g *InsightGenerator) GenerateRepositoryInsights(metadata *MetadataManager) []string {
	insights := []string{}

	if repoAnalysis, exists := metadata.metadata[pipeline.RepoAnalysisResultKey]; exists {
		insights = append(insights, "Repository analysis completed successfully")

		if analysisMap, err := g.analysisConverter.ToMap(repoAnalysis); err == nil {
			if language := g.analysisConverter.GetLanguage(analysisMap); language != "" {
				insights = append(insights, fmt.Sprintf("Detected %s project", language))
			}
			if framework := g.analysisConverter.GetFramework(analysisMap); framework != "" {
				insights = append(insights, fmt.Sprintf("Framework: %s", framework))
			}
		}
	}

	return insights
}

// GenerateDockerInsights generates insights for Docker build stage
func (g *InsightGenerator) GenerateDockerInsights(metadata *MetadataManager) []string {
	insights := []string{"Container image built successfully"}

	if buildLogs := metadata.GetStringWithDefault("build_logs", ""); buildLogs != "" {
		insights = append(insights, "Build logs available for review")
	}

	if duration, exists := metadata.GetDuration("build_duration"); exists {
		if duration < 2*time.Minute {
			insights = append(insights, "Fast build time achieved")
		}
	}

	return insights
}

// GenerateManifestInsights generates insights for manifest generation stage
func (g *InsightGenerator) GenerateManifestInsights(metadata *MetadataManager) []string {
	insights := []string{"Kubernetes manifests generated successfully"}

	if manifestPath, exists := metadata.GetString("manifest_path"); exists {
		insights = append(insights, fmt.Sprintf("Manifests saved to %s", manifestPath))
	}

	return insights
}

// GenerateCommonInsights generates common insights based on metadata
func (g *InsightGenerator) GenerateCommonInsights(metadata *MetadataManager) []string {
	insights := []string{}

	if tokenUsage, exists := metadata.GetInt("ai_token_usage"); exists && tokenUsage > 0 {
		insights = append(insights, fmt.Sprintf("AI analysis used %d tokens", tokenUsage))
	}

	return insights
}
