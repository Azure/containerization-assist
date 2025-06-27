package orchestration

import (
	"fmt"
	"testing"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Simplified tests focusing on conversion functions and helper methods
func TestRepositoryAnalyzerAdapter_ConvertToRepositoryInfo_Simple(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	// Test with valid analysis result
	analysisResult := &analyze.AtomicAnalysisResult{
		Success:   true,
		SessionID: "test-session",
		Analysis: &analysis.AnalysisResult{
			Language:  "go",
			Framework: "gin",
			Dependencies: []analysis.Dependency{
				{Name: "github.com/gin-gonic/gin", Version: "v1.8.1"},
			},
			EntryPoints: []string{"main.go"},
			Port:        8080,
		},
		RepoURL:      "/test/repo",
		Branch:       "main",
		WorkspaceDir: "/workspace",
	}

	repoInfo := adapter.convertToRepositoryInfo(analysisResult)

	assert.NotNil(t, repoInfo)
	assert.Equal(t, "go", repoInfo.Language)
	assert.Equal(t, "gin", repoInfo.Framework)
	assert.Len(t, repoInfo.Dependencies, 1)
	assert.Contains(t, repoInfo.Dependencies, "github.com/gin-gonic/gin")
	assert.Equal(t, "go", repoInfo.BuildSystem)
	assert.NotNil(t, repoInfo.Metadata)
	assert.Equal(t, "main.go", repoInfo.Metadata["entry_point"])
}

func TestRepositoryAnalyzerAdapter_ConvertToProjectMetadata_Simple(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	repoInfo := &build.RepositoryInfo{
		Language:     "python",
		Framework:    "django",
		Dependencies: []string{"django", "requests"},
		BuildSystem:  "pip",
		ProjectSize:  "medium",
		Complexity:   "high",
		Metadata: map[string]interface{}{
			"entry_point": "manage.py",
		},
	}

	metadata := adapter.convertToProjectMetadata(repoInfo)

	assert.NotNil(t, metadata)
	assert.Equal(t, "python", metadata.Language)
	assert.Equal(t, "django", metadata.Framework)
	assert.Len(t, metadata.Dependencies, 2)
	assert.Equal(t, "pip", metadata.BuildSystem)
	assert.Equal(t, "medium", metadata.ProjectSize)
	assert.Equal(t, "high", metadata.Complexity)
	assert.Equal(t, repoInfo.Metadata, metadata.Attributes)
}

func TestRepositoryAnalyzerAdapter_DetermineBuildSystem_Simple(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	tests := []struct {
		name          string
		language      string
		expectedBuild string
	}{
		{"Go project", "go", "go"},
		{"JavaScript project", "javascript", "npm"},
		{"Python project", "python", "python"},
		{"Rust project", "rust", "cargo"},
		{"Unknown language", "unknown", "make"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &analysis.AnalysisResult{
				Language: tt.language,
			}

			buildSystem := adapter.determineBuildSystem(analysis)
			assert.Equal(t, tt.expectedBuild, buildSystem)
		})
	}
}

func TestRepositoryAnalyzerAdapter_DetermineProjectSize_Simple(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	tests := []struct {
		name         string
		depCount     int
		expectedSize string
	}{
		{"Large project", 60, "large"},
		{"Medium project", 25, "medium"},
		{"Small project", 3, "small"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make([]analysis.Dependency, tt.depCount)
			for i := 0; i < tt.depCount; i++ {
				deps[i] = analysis.Dependency{Name: fmt.Sprintf("dep%d", i)}
			}

			analysisResult := &analysis.AnalysisResult{
				Dependencies: deps,
				Structure:    map[string]interface{}{"files": []string{"main.go", "utils.go"}},
			}

			size := adapter.determineProjectSize(analysisResult)
			assert.Equal(t, tt.expectedSize, size)
		})
	}
}

func TestRepositoryAnalyzerAdapter_DetermineComplexity_Simple(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	tests := []struct {
		name               string
		language           string
		framework          string
		depCount           int
		expectedComplexity string
	}{
		{"High complexity", "java", "spring", 25, "high"},
		{"Medium complexity", "go", "gin", 10, "medium"},
		{"Low complexity", "python", "flask", 2, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make([]analysis.Dependency, tt.depCount)
			for i := 0; i < tt.depCount; i++ {
				deps[i] = analysis.Dependency{Name: fmt.Sprintf("dep%d", i)}
			}

			analysisResult := &analysis.AnalysisResult{
				Language:     tt.language,
				Framework:    tt.framework,
				Dependencies: deps,
			}

			complexity := adapter.determineComplexity(analysisResult)
			assert.Equal(t, tt.expectedComplexity, complexity)
		})
	}
}

func TestRepositoryAnalyzerAdapter_HelperFunctions_Simple(t *testing.T) {
	t.Run("contains function", func(t *testing.T) {
		assert.True(t, contains("hello world", "hello"))
		assert.True(t, contains("hello world", "world"))
		assert.False(t, contains("hello world", "xyz"))
	})

	t.Run("countOccurrences function", func(t *testing.T) {
		assert.Equal(t, 0, countOccurrences("hello world", "xyz"))
		assert.Equal(t, 1, countOccurrences("hello world", "hello"))
		assert.Equal(t, 2, countOccurrences("hello hello", "hello"))
	})
}

func TestRepositoryAnalyzerAdapter_NilInputHandling(t *testing.T) {
	adapter := &RepositoryAnalyzerAdapter{
		logger: zerolog.Nop(),
	}

	// Test nil analysis result
	repoInfo := adapter.convertToRepositoryInfo(nil)
	assert.NotNil(t, repoInfo)
	assert.Equal(t, "unknown", repoInfo.Language)
	assert.Equal(t, "unknown", repoInfo.Framework)

	// Test empty analysis
	emptyResult := &analyze.AtomicAnalysisResult{
		Success:  true,
		Analysis: nil,
	}
	repoInfo2 := adapter.convertToRepositoryInfo(emptyResult)
	assert.NotNil(t, repoInfo2)
	assert.Equal(t, "unknown", repoInfo2.Language)

	// Test nil analysis for other functions
	buildSystem := adapter.determineBuildSystem(nil)
	assert.Equal(t, "unknown", buildSystem)

	projectSize := adapter.determineProjectSize(nil)
	assert.Equal(t, "unknown", projectSize)

	complexity := adapter.determineComplexity(nil)
	assert.Equal(t, "unknown", complexity)
}
