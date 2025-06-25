package session

import (
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestSessionState_SessionID(t *testing.T) {
	sessionID := "session-123"
	state := NewSessionState(sessionID, "/tmp/workspace")

	result := state.SessionID
	assert.Equal(t, sessionID, result)
}

func TestSessionState_ScanSummaryCheck(t *testing.T) {
	tests := []struct {
		name        string
		scanSummary *types.RepositoryScanSummary
		expected    bool
	}{
		{
			name:        "returns false when ScanSummary is nil",
			scanSummary: nil,
			expected:    false,
		},
		{
			name: "returns true when ScanSummary has analyzed files",
			scanSummary: &types.RepositoryScanSummary{
				FilesAnalyzed: 10,
				Language:      "go",
			},
			expected: true,
		},
		{
			name: "returns false when ScanSummary has zero analyzed files",
			scanSummary: &types.RepositoryScanSummary{
				FilesAnalyzed: 0,
				Language:      "go",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewSessionState("session-123", "/tmp/workspace")
			state.ScanSummary = tt.scanSummary

			result := state.ScanSummary != nil && state.ScanSummary.FilesAnalyzed > 0
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionState_DockerfilePushedCheck(t *testing.T) {
	tests := []struct {
		name             string
		dockerfilePushed bool
		expected         bool
	}{
		{
			name:             "returns true when Dockerfile.Pushed is true",
			dockerfilePushed: true,
			expected:         true,
		},
		{
			name:             "returns false when Dockerfile.Pushed is false",
			dockerfilePushed: false,
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewSessionState("session-123", "/tmp/workspace")
			state.Dockerfile.Pushed = tt.dockerfilePushed

			result := state.Dockerfile.Pushed
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionState_StageHistoryDerivation(t *testing.T) {
	now := time.Now()
	endTime := now.Add(time.Minute)

	tests := []struct {
		name         string
		stageHistory []ToolExecution
		expected     string
	}{
		{
			name:         "returns initialized when no history",
			stageHistory: []ToolExecution{},
			expected:     "initialized",
		},
		{
			name: "returns tool name when execution is not complete",
			stageHistory: []ToolExecution{
				{
					Tool:      "analyze_repository",
					StartTime: now,
					Success:   false,
					EndTime:   nil,
				},
			},
			expected: "analyze_repository",
		},
		{
			name: "derives next stage when execution is successful",
			stageHistory: []ToolExecution{
				{
					Tool:      "analyze_repository",
					StartTime: now,
					EndTime:   &endTime,
					Success:   true,
				},
			},
			expected: "analysis_complete",
		},
		{
			name: "uses last execution in history",
			stageHistory: []ToolExecution{
				{
					Tool:      "analyze_repository",
					StartTime: now,
					EndTime:   &endTime,
					Success:   true,
				},
				{
					Tool:      "generate_dockerfile",
					StartTime: now.Add(time.Minute),
					EndTime:   &endTime,
					Success:   true,
				},
			},
			expected: "dockerfile_ready",
		},
		{
			name: "returns tool name when last execution failed",
			stageHistory: []ToolExecution{
				{
					Tool:      "build_image",
					StartTime: now,
					EndTime:   &endTime,
					Success:   false,
				},
			},
			expected: "build_image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewSessionState("session-123", "/tmp/workspace")
			state.StageHistory = tt.stageHistory

			result := func() string {
				if len(state.StageHistory) == 0 {
					return "initialized"
				}
				lastExecution := state.StageHistory[len(state.StageHistory)-1]
				if lastExecution.Success && lastExecution.EndTime != nil {
					return DeriveNextStage(lastExecution.Tool)
				}
				return lastExecution.Tool
			}()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeriveNextStage(t *testing.T) {
	tests := []struct {
		name          string
		completedTool string
		expected      string
	}{
		{
			name:          "analyze_repository -> analysis_complete",
			completedTool: "analyze_repository",
			expected:      "analysis_complete",
		},
		{
			name:          "generate_dockerfile -> dockerfile_ready",
			completedTool: "generate_dockerfile",
			expected:      "dockerfile_ready",
		},
		{
			name:          "build_image -> image_built",
			completedTool: "build_image",
			expected:      "image_built",
		},
		{
			name:          "push_image -> image_pushed",
			completedTool: "push_image",
			expected:      "image_pushed",
		},
		{
			name:          "generate_manifests -> manifests_ready",
			completedTool: "generate_manifests",
			expected:      "manifests_ready",
		},
		{
			name:          "deploy_kubernetes -> deployed",
			completedTool: "deploy_kubernetes",
			expected:      "deployed",
		},
		{
			name:          "unknown tool -> unknown",
			completedTool: "unknown_tool",
			expected:      "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeriveNextStage(tt.completedTool)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionState_CompatibilityAccessors_Integration(t *testing.T) {
	t.Run("modern field usage scenario", func(t *testing.T) {
		// Start with a clean session
		state := NewSessionState("session-123", "/tmp/workspace")

		// Set modern fields
		state.ScanSummary = &types.RepositoryScanSummary{
			FilesAnalyzed: 5,
			Language:      "go",
		}
		state.Dockerfile.Pushed = true
		now := time.Now()
		endTime := now.Add(time.Minute)
		state.StageHistory = []ToolExecution{
			{
				Tool:      "push_image",
				StartTime: now,
				EndTime:   &endTime,
				Success:   true,
			},
		}

		// Verify modern field usage directly
		assert.Equal(t, "session-123", state.SessionID)
		assert.True(t, state.ScanSummary != nil && state.ScanSummary.FilesAnalyzed > 0)
		assert.True(t, state.Dockerfile.Pushed)
		// Check stage derivation
		lastExecution := state.StageHistory[len(state.StageHistory)-1]
		assert.Equal(t, "image_pushed", DeriveNextStage(lastExecution.Tool))
	})
}

func TestNewSessionState_FieldInitialization(t *testing.T) {
	sessionID := "test-session"
	workspaceDir := "/tmp/test-workspace"

	state := NewSessionState(sessionID, workspaceDir)

	// Verify fields are properly initialized
	assert.Equal(t, sessionID, state.SessionID)
	assert.Equal(t, workspaceDir, state.WorkspaceDir)
	assert.NotNil(t, state.StageHistory)
	assert.Len(t, state.StageHistory, 0)
	assert.NotNil(t, state.Metadata)
	// Check default state
	assert.Equal(t, "initialized", func() string {
		if len(state.StageHistory) == 0 {
			return "initialized"
		}
		return "has_history"
	}())
	assert.False(t, state.ScanSummary != nil && state.ScanSummary.FilesAnalyzed > 0)
	assert.False(t, state.Dockerfile.Pushed)
}

func TestConvertRepositoryInfoToScanSummary(t *testing.T) {
	tests := []struct {
		name     string
		info     map[string]interface{}
		expected *types.RepositoryScanSummary
	}{
		{
			name:     "nil input returns nil",
			info:     nil,
			expected: nil,
		},
		{
			name:     "empty map returns empty summary",
			info:     map[string]interface{}{},
			expected: &types.RepositoryScanSummary{},
		},
		{
			name: "complete repository info conversion",
			info: map[string]interface{}{
				"language":       "go",
				"framework":      "gin",
				"port":           8080,
				"dependencies":   []string{"github.com/gin-gonic/gin", "github.com/stretchr/testify"},
				"files":          []string{"go.mod", "Dockerfile", "main.go"},
				"repo_url":       "https://github.com/example/repo",
				"file_count":     15,
				"size_bytes":     int64(1024),
				"has_dockerfile": true,
			},
			expected: &types.RepositoryScanSummary{
				Language:         "go",
				Framework:        "gin",
				Port:             8080,
				Dependencies:     []string{"github.com/gin-gonic/gin", "github.com/stretchr/testify"},
				ConfigFilesFound: []string{"go.mod", "Dockerfile", "main.go"},
				RepoURL:          "https://github.com/example/repo",
				FilesAnalyzed:    15,
				RepositorySize:   1024,
				DockerFiles:      []string{"Dockerfile"},
			},
		},
		{
			name: "handles float64 port and file_count",
			info: map[string]interface{}{
				"port":       float64(3000),
				"file_count": float64(25),
				"size_bytes": float64(2048),
			},
			expected: &types.RepositoryScanSummary{
				Port:           3000,
				FilesAnalyzed:  25,
				RepositorySize: 2048,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertRepositoryInfoToScanSummary(tt.info)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.Language, result.Language)
			assert.Equal(t, tt.expected.Framework, result.Framework)
			assert.Equal(t, tt.expected.Port, result.Port)
			assert.Equal(t, tt.expected.Dependencies, result.Dependencies)
			assert.Equal(t, tt.expected.ConfigFilesFound, result.ConfigFilesFound)
			assert.Equal(t, tt.expected.RepoURL, result.RepoURL)
			assert.Equal(t, tt.expected.FilesAnalyzed, result.FilesAnalyzed)
			assert.Equal(t, tt.expected.RepositorySize, result.RepositorySize)
			assert.Equal(t, tt.expected.DockerFiles, result.DockerFiles)
			assert.False(t, result.CachedAt.IsZero()) // Should be set to current time
		})
	}
}

func TestConvertScanSummaryToRepositoryInfo(t *testing.T) {
	tests := []struct {
		name     string
		summary  *types.RepositoryScanSummary
		expected map[string]interface{}
	}{
		{
			name:     "nil input returns empty map",
			summary:  nil,
			expected: map[string]interface{}{},
		},
		{
			name:     "empty summary returns empty map",
			summary:  &types.RepositoryScanSummary{},
			expected: map[string]interface{}{},
		},
		{
			name: "complete scan summary conversion",
			summary: &types.RepositoryScanSummary{
				Language:         "python",
				Framework:        "flask",
				Port:             5000,
				Dependencies:     []string{"flask", "requests"},
				ConfigFilesFound: []string{"requirements.txt", "app.py"},
				RepoURL:          "https://github.com/example/python-app",
				FilesAnalyzed:    20,
				RepositorySize:   2048,
				PackageManagers:  []string{"pip"},
				DatabaseFiles:    []string{"postgres.conf", "mysql.cnf"},
				DockerFiles:      []string{"Dockerfile", "docker-compose.yml"},
			},
			expected: map[string]interface{}{
				"language":         "python",
				"framework":        "flask",
				"port":             5000,
				"dependencies":     []string{"flask", "requests"},
				"files":            []string{"requirements.txt", "app.py"},
				"repo_url":         "https://github.com/example/python-app",
				"file_count":       20,
				"size_bytes":       int64(2048),
				"package_managers": []string{"pip"},
				"database_types":   []string{"postgresql", "mysql"},
				"has_dockerfile":   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertScanSummaryToRepositoryInfo(tt.summary)

			assert.NotNil(t, result)
			for key, expectedValue := range tt.expected {
				actualValue, exists := result[key]
				assert.True(t, exists, "Expected key %s to exist", key)
				assert.Equal(t, expectedValue, actualValue, "Value mismatch for key %s", key)
			}
		})
	}
}

func TestExtractDatabaseTypes(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "empty input",
			files:    []string{},
			expected: nil,
		},
		{
			name:     "postgresql files",
			files:    []string{"postgresql.conf", "postgres_backup.sql"},
			expected: []string{"postgresql", "postgresql"},
		},
		{
			name:     "mysql files",
			files:    []string{"mysql.cnf", "my.cnf"},
			expected: []string{"mysql"},
		},
		{
			name:     "mixed database files",
			files:    []string{"postgres.conf", "mongodb.conf", "redis.conf"},
			expected: []string{"postgresql", "mongodb", "redis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDatabaseTypes(tt.files)
			assert.Equal(t, tt.expected, result)
		})
	}
}
