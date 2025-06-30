package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants from constants.go
func TestConstants(t *testing.T) {
	// Test registry constants
	assert.Equal(t, "docker.io", DefaultRegistry)
	assert.Equal(t, "network_error", NetworkError)

	// Test language constants
	assert.Equal(t, "typescript", LanguageTypeScript)
	assert.Equal(t, "python", LanguagePython)
	assert.Equal(t, "javascript", LanguageJavaScript)
	assert.Equal(t, "java", LanguageJava)
	assert.Equal(t, "json", LanguageJSON)

	// Test build system constants
	assert.Equal(t, "maven", BuildSystemMaven)
	assert.Equal(t, "gradle", BuildSystemGradle)

	// Test application server constants
	assert.Equal(t, "tomcat", AppServerTomcat)

	// Test size constants
	assert.Equal(t, "small", SizeSmall)
	assert.Equal(t, "large", SizeLarge)

	// Test health status constants
	assert.Equal(t, "healthy", HealthStatusHealthy)
	assert.Equal(t, "unhealthy", HealthStatusUnhealthy)
	assert.Equal(t, "degraded", HealthStatusDegraded)
	assert.Equal(t, "pending", HealthStatusPending)
	assert.Equal(t, "failed", HealthStatusFailed)
}

// Test schema version constants from common.go
func TestSchemaVersionConstants(t *testing.T) {
	assert.Equal(t, "v1.0.0", CurrentSchemaVersion)
	assert.Equal(t, "2024.12.17", ToolAPIVersion)
}

// TestBaseToolResponse tests the BaseToolResponse struct
func TestBaseToolResponse(t *testing.T) {
	now := time.Now()
	response := BaseToolResponse{
		Version:   "v1.0.0",
		Tool:      "test_tool",
		Timestamp: now,
		SessionID: "session-123",
		DryRun:    true,
	}

	assert.Equal(t, "v1.0.0", response.Version)
	assert.Equal(t, "test_tool", response.Tool)
	assert.Equal(t, now, response.Timestamp)
	assert.Equal(t, "session-123", response.SessionID)
	assert.True(t, response.DryRun)
}

// TestBaseToolArgs tests the BaseToolArgs struct
func TestBaseToolArgs(t *testing.T) {
	args := BaseToolArgs{
		DryRun:    true,
		SessionID: "session-456",
	}

	assert.True(t, args.DryRun)
	assert.Equal(t, "session-456", args.SessionID)
}

// TestNewBaseResponse tests the NewBaseResponse function
func TestNewBaseResponse(t *testing.T) {
	beforeCall := time.Now()
	response := NewBaseResponse("build_tool", "session-789", false)
	afterCall := time.Now()

	assert.Equal(t, CurrentSchemaVersion, response.Version)
	assert.Equal(t, "build_tool", response.Tool)
	assert.Equal(t, "session-789", response.SessionID)
	assert.False(t, response.DryRun)

	// Timestamp should be between before and after the call
	assert.True(t, response.Timestamp.After(beforeCall) || response.Timestamp.Equal(beforeCall))
	assert.True(t, response.Timestamp.Before(afterCall) || response.Timestamp.Equal(afterCall))
}

// TestImageReference tests the ImageReference struct and its methods
func TestImageReference(t *testing.T) {
	tests := []struct {
		name     string
		imageRef ImageReference
		expected string
	}{
		{
			name: "simple repository and tag",
			imageRef: ImageReference{
				Repository: "nginx",
				Tag:        "latest",
			},
			expected: "nginx:latest",
		},
		{
			name: "with registry",
			imageRef: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.21",
			},
			expected: "docker.io/library/nginx:1.21",
		},
		{
			name: "with digest",
			imageRef: ImageReference{
				Repository: "nginx",
				Tag:        "latest",
				Digest:     "sha256:abc123",
			},
			expected: "nginx:latest@sha256:abc123",
		},
		{
			name: "full reference",
			imageRef: ImageReference{
				Registry:   "myregistry.io",
				Repository: "myapp/service",
				Tag:        "v1.2.3",
				Digest:     "sha256:def456",
			},
			expected: "myregistry.io/myapp/service:v1.2.3@sha256:def456",
		},
		{
			name: "no tag",
			imageRef: ImageReference{
				Repository: "alpine",
			},
			expected: "alpine",
		},
		{
			name: "empty tag",
			imageRef: ImageReference{
				Repository: "ubuntu",
				Tag:        "",
			},
			expected: "ubuntu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.imageRef.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestImageReference_JSONSerialization tests JSON serialization/deserialization
func TestImageReference_JSONSerialization(t *testing.T) {
	original := ImageReference{
		Registry:   "docker.io",
		Repository: "library/nginx",
		Tag:        "1.21",
		Digest:     "sha256:abc123def456",
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize from JSON
	var deserialized ImageReference
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.Registry, deserialized.Registry)
	assert.Equal(t, original.Repository, deserialized.Repository)
	assert.Equal(t, original.Tag, deserialized.Tag)
	assert.Equal(t, original.Digest, deserialized.Digest)

	// Verify string representation is the same
	assert.Equal(t, original.String(), deserialized.String())
}

// TestBaseToolResponse_JSONSerialization tests JSON serialization/deserialization
func TestBaseToolResponse_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON precision
	original := BaseToolResponse{
		Version:   "v2.0.0",
		Tool:      "test_tool",
		Timestamp: now,
		SessionID: "session-test-123",
		DryRun:    true,
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	assert.Equal(t, "v2.0.0", jsonMap["version"])
	assert.Equal(t, "test_tool", jsonMap["tool"])
	assert.Equal(t, "session-test-123", jsonMap["session_id"])
	assert.Equal(t, true, jsonMap["dry_run"])

	// Deserialize from JSON
	var deserialized BaseToolResponse
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.Version, deserialized.Version)
	assert.Equal(t, original.Tool, deserialized.Tool)
	assert.Equal(t, original.SessionID, deserialized.SessionID)
	assert.Equal(t, original.DryRun, deserialized.DryRun)
	assert.True(t, original.Timestamp.Equal(deserialized.Timestamp))
}

// TestBaseToolArgs_JSONSerialization tests JSON serialization/deserialization
func TestBaseToolArgs_JSONSerialization(t *testing.T) {
	original := BaseToolArgs{
		DryRun:    false,
		SessionID: "session-args-test",
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Verify JSON structure
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Check that omitempty works - dry_run should be omitted when false
	_, hasDryRun := jsonMap["dry_run"]
	assert.False(t, hasDryRun, "dry_run should be omitted when false due to omitempty")

	// But session_id should be present
	assert.Equal(t, "session-args-test", jsonMap["session_id"])

	// Deserialize from JSON
	var deserialized BaseToolArgs
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, original.DryRun, deserialized.DryRun)
	assert.Equal(t, original.SessionID, deserialized.SessionID)

	// Test with dry_run true
	originalWithDryRun := BaseToolArgs{
		DryRun:    true,
		SessionID: "session-dry-run",
	}

	data, err = json.Marshal(originalWithDryRun)
	require.NoError(t, err)

	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Now dry_run should be present
	assert.Equal(t, true, jsonMap["dry_run"])
	assert.Equal(t, "session-dry-run", jsonMap["session_id"])
}

// TestImageReference_EdgeCases tests edge cases for ImageReference
func TestImageReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		imageRef ImageReference
		expected string
	}{
		{
			name: "empty repository",
			imageRef: ImageReference{
				Repository: "",
				Tag:        "latest",
			},
			expected: ":latest",
		},
		{
			name: "special characters in repository",
			imageRef: ImageReference{
				Repository: "my-org/my-app_v2",
				Tag:        "1.0.0-beta.1",
			},
			expected: "my-org/my-app_v2:1.0.0-beta.1",
		},
		{
			name: "localhost registry",
			imageRef: ImageReference{
				Registry:   "localhost:5000",
				Repository: "test/app",
				Tag:        "dev",
			},
			expected: "localhost:5000/test/app:dev",
		},
		{
			name: "digest only",
			imageRef: ImageReference{
				Repository: "app",
				Digest:     "sha256:123456789abcdef",
			},
			expected: "app@sha256:123456789abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.imageRef.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConstantsUniqueness ensures all constants are unique where they should be
func TestConstantsUniqueness(t *testing.T) {
	// Test language constants are unique
	languages := []string{
		LanguageTypeScript,
		LanguagePython,
		LanguageJavaScript,
		LanguageJava,
		LanguageJSON,
	}

	languageSet := make(map[string]bool)
	for _, lang := range languages {
		assert.False(t, languageSet[lang], "Language constant %s should be unique", lang)
		languageSet[lang] = true
	}

	// Test health status constants are unique
	healthStatuses := []string{
		HealthStatusHealthy,
		HealthStatusUnhealthy,
		HealthStatusDegraded,
		HealthStatusPending,
		HealthStatusFailed,
	}

	healthSet := make(map[string]bool)
	for _, status := range healthStatuses {
		assert.False(t, healthSet[status], "Health status constant %s should be unique", status)
		healthSet[status] = true
	}

	// Test build system constants are unique
	buildSystems := []string{
		BuildSystemMaven,
		BuildSystemGradle,
	}

	buildSet := make(map[string]bool)
	for _, system := range buildSystems {
		assert.False(t, buildSet[system], "Build system constant %s should be unique", system)
		buildSet[system] = true
	}
}

// TestConstantsNotEmpty ensures constants are not empty strings
func TestConstantsNotEmpty(t *testing.T) {
	constants := map[string]string{
		"DefaultRegistry":       DefaultRegistry,
		"NetworkError":          NetworkError,
		"LanguageTypeScript":    LanguageTypeScript,
		"LanguagePython":        LanguagePython,
		"LanguageJavaScript":    LanguageJavaScript,
		"LanguageJava":          LanguageJava,
		"LanguageJSON":          LanguageJSON,
		"BuildSystemMaven":      BuildSystemMaven,
		"BuildSystemGradle":     BuildSystemGradle,
		"AppServerTomcat":       AppServerTomcat,
		"SizeSmall":             SizeSmall,
		"SizeLarge":             SizeLarge,
		"HealthStatusHealthy":   HealthStatusHealthy,
		"HealthStatusUnhealthy": HealthStatusUnhealthy,
		"HealthStatusDegraded":  HealthStatusDegraded,
		"HealthStatusPending":   HealthStatusPending,
		"HealthStatusFailed":    HealthStatusFailed,
		"CurrentSchemaVersion":  CurrentSchemaVersion,
		"ToolAPIVersion":        ToolAPIVersion,
	}

	for name, value := range constants {
		assert.NotEmpty(t, value, "Constant %s should not be empty", name)
	}
}

// TestImageReferenceComparisons tests comparison operations
func TestImageReferenceComparisons(t *testing.T) {
	ref1 := ImageReference{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
		Digest:     "sha256:abc123",
	}

	ref2 := ImageReference{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "latest",
		Digest:     "sha256:abc123",
	}

	ref3 := ImageReference{
		Registry:   "docker.io",
		Repository: "nginx",
		Tag:        "1.21",
		Digest:     "sha256:def456",
	}

	// Test equality
	assert.Equal(t, ref1, ref2)
	assert.NotEqual(t, ref1, ref3)

	// Test string representations
	assert.Equal(t, ref1.String(), ref2.String())
	assert.NotEqual(t, ref1.String(), ref3.String())
}

// TestBaseResponseTimestampPrecision tests timestamp precision in base response
func TestBaseResponseTimestampPrecision(t *testing.T) {
	response1 := NewBaseResponse("tool1", "session1", false)

	// Small delay to ensure different timestamps
	time.Sleep(1 * time.Millisecond)

	response2 := NewBaseResponse("tool2", "session2", true)

	assert.True(t, response2.Timestamp.After(response1.Timestamp))
	assert.NotEqual(t, response1.Timestamp, response2.Timestamp)
}

// Benchmark tests
func BenchmarkImageReference_String(b *testing.B) {
	ref := ImageReference{
		Registry:   "docker.io",
		Repository: "library/nginx",
		Tag:        "latest",
		Digest:     "sha256:abcdef123456789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ref.String()
	}
}

func BenchmarkNewBaseResponse(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBaseResponse("benchmark_tool", "benchmark_session", false)
	}
}

func BenchmarkImageReference_JSONMarshal(b *testing.B) {
	ref := ImageReference{
		Registry:   "docker.io",
		Repository: "library/nginx",
		Tag:        "latest",
		Digest:     "sha256:abcdef123456789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(ref)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkBaseToolResponse_JSONMarshal(b *testing.B) {
	response := BaseToolResponse{
		Version:   CurrentSchemaVersion,
		Tool:      "benchmark_tool",
		Timestamp: time.Now(),
		SessionID: "benchmark_session",
		DryRun:    false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(response)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}
