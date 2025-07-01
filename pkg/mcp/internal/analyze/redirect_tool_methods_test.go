package analyze

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// TestAnalyzeRepositoryRedirectTool_Methods tests that the redirect tool implements all required methods
func TestAnalyzeRepositoryRedirectTool_Methods(t *testing.T) {
	logger := zerolog.Nop()
	atomicTool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)

	// Test GetName
	name := tool.GetName()
	assert.Equal(t, "analyze_repository", name, "Tool name should be 'analyze_repository'")

	// Test GetDescription
	description := tool.GetDescription()
	assert.NotEmpty(t, description, "Tool description should not be empty")
	assert.Contains(t, description, "Analyzes a repository", "Description should mention repository analysis")
	assert.Contains(t, description, "session", "Description should mention session management")

	// Test GetSchema
	schema := tool.GetSchema()
	assert.Equal(t, "analyze_repository", schema.Name, "Schema name should match tool name")
	assert.NotEmpty(t, schema.Description, "Schema description should not be empty")
	assert.Equal(t, "1.0.0", schema.Version, "Schema version should be 1.0.0")
	assert.NotNil(t, schema.ParamsSchema, "Schema should have parameter schema")
	assert.NotNil(t, schema.ResultSchema, "Schema should have result schema")

	// Test GetMetadata
	metadata := tool.GetMetadata()
	assert.Equal(t, "analyze_repository", metadata.Name, "Metadata name should match tool name")
	assert.NotEmpty(t, metadata.Description, "Metadata description should not be empty")
	assert.Equal(t, "1.0.0", metadata.Version, "Metadata version should be 1.0.0")
	assert.Equal(t, "analysis", metadata.Category, "Tool category should be 'analysis'")
	assert.Contains(t, metadata.Dependencies, "analyze_repository_atomic", "Should depend on atomic tool")
}
