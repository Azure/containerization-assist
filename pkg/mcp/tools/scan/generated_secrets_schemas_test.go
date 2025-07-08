package scan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicScanSecretsArgsSchema(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)
	assert.Equal(t, "Atomic Scan Secrets", schema.Title)
	assert.NotNil(t, schema.Properties)
	assert.NotNil(t, schema.AdditionalProperties)
	assert.False(t, *schema.AdditionalProperties)
}

func TestAtomicScanSecretsArgsSchema_Properties(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	expectedProperties := map[string]string{
		"scan_path":           "Path to scan (default: session workspace)",
		"file_patterns":       "File patterns to include in scan (e.g., '*.py', '*.js')",
		"exclude_patterns":    "File patterns to exclude from scan",
		"scan_dockerfiles":    "Include Dockerfiles in scan",
		"scan_manifests":      "Include Kubernetes manifests in scan",
		"scan_source_code":    "Include source code files in scan",
		"scan_env_files":      "Include .env files in scan",
		"suggest_remediation": "Provide remediation suggestions",
		"generate_secrets":    "Generate Kubernetes Secret manifests",
	}

	for property, expectedDesc := range expectedProperties {
		t.Run(property, func(t *testing.T) {
			prop, exists := schema.Properties[property]
			require.True(t, exists, "Property %s should exist", property)
			assert.Equal(t, expectedDesc, prop.Description)
		})
	}
}

func TestAtomicScanSecretsArgsSchema_PropertyTypes(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	expectedTypes := map[string]string{
		"scan_path":           "string",
		"file_patterns":       "array",
		"exclude_patterns":    "array",
		"scan_dockerfiles":    "boolean",
		"scan_manifests":      "boolean",
		"scan_source_code":    "boolean",
		"scan_env_files":      "boolean",
		"suggest_remediation": "boolean",
		"generate_secrets":    "boolean",
	}

	for property, expectedType := range expectedTypes {
		t.Run(property, func(t *testing.T) {
			prop, exists := schema.Properties[property]
			require.True(t, exists, "Property %s should exist", property)
			assert.Equal(t, expectedType, prop.Type)
		})
	}
}

func TestAtomicScanSecretsArgsSchema_RequiredFields(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	// Currently no required fields according to the schema
	assert.Equal(t, 0, len(schema.Required))
}

func TestAtomicScanSecretsArgsSchema_StringProperties(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	stringProperties := []string{"scan_path"}

	for _, property := range stringProperties {
		t.Run(property, func(t *testing.T) {
			prop, exists := schema.Properties[property]
			require.True(t, exists)
			assert.Equal(t, "string", prop.Type)
		})
	}
}

func TestAtomicScanSecretsArgsSchema_ArrayProperties(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	arrayProperties := []string{"file_patterns", "exclude_patterns"}

	for _, property := range arrayProperties {
		t.Run(property, func(t *testing.T) {
			prop, exists := schema.Properties[property]
			require.True(t, exists)
			assert.Equal(t, "array", prop.Type)
		})
	}
}

func TestAtomicScanSecretsArgsSchema_BooleanProperties(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	booleanProperties := []string{
		"scan_dockerfiles",
		"scan_manifests",
		"scan_source_code",
		"scan_env_files",
		"suggest_remediation",
		"generate_secrets",
	}

	for _, property := range booleanProperties {
		t.Run(property, func(t *testing.T) {
			prop, exists := schema.Properties[property]
			require.True(t, exists)
			assert.Equal(t, "boolean", prop.Type)
		})
	}
}

func TestAtomicScanSecretsArgsSchema_PropertyCount(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	// Verify expected number of properties
	expectedPropertyCount := 9
	assert.Equal(t, expectedPropertyCount, len(schema.Properties))
}

func TestAtomicScanSecretsArgsSchema_SchemaValidity(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	// Basic schema validation
	assert.NotEmpty(t, schema.Type)
	assert.NotEmpty(t, schema.Title)
	assert.NotNil(t, schema.Properties)

	// Ensure all properties have types
	for name, prop := range schema.Properties {
		assert.NotEmpty(t, prop.Type, "Property %s should have a type", name)
		assert.NotEmpty(t, prop.Description, "Property %s should have a description", name)
	}
}

func TestAtomicScanSecretsArgsSchema_Immutability(t *testing.T) {
	// Test that the schema can be accessed multiple times without modification
	schema1 := AtomicScanSecretsArgsSchema
	schema2 := AtomicScanSecretsArgsSchema

	assert.Equal(t, schema1, schema2)
	assert.Equal(t, schema1.Type, schema2.Type)
	assert.Equal(t, schema1.Title, schema2.Title)
	assert.Equal(t, len(schema1.Properties), len(schema2.Properties))
}

func TestAtomicScanSecretsArgsSchema_AdditionalPropertiesConfiguration(t *testing.T) {
	schema := AtomicScanSecretsArgsSchema

	require.NotNil(t, schema.AdditionalProperties)
	assert.False(t, *schema.AdditionalProperties, "Additional properties should be disabled")
}

// BenchmarkAtomicScanSecretsArgsSchema_Access benchmarks schema access performance
func BenchmarkAtomicScanSecretsArgsSchema_Access(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schema := AtomicScanSecretsArgsSchema
		_ = schema.Type
		_ = schema.Properties
		_ = schema.Required
	}
}

// BenchmarkAtomicScanSecretsArgsSchema_PropertyIteration benchmarks property iteration
func BenchmarkAtomicScanSecretsArgsSchema_PropertyIteration(b *testing.B) {
	schema := AtomicScanSecretsArgsSchema

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for name, prop := range schema.Properties {
			_ = name
			_ = prop.Type
			_ = prop.Description
		}
	}
}
