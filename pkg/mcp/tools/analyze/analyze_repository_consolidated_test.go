package analyze

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockServiceContainer for testing
type MockServiceContainer struct{}

func (m *MockServiceContainer) SessionStore() services.SessionStore         { return &MockSessionStore{} }
func (m *MockServiceContainer) SessionState() services.SessionState         { return &MockSessionState{} }
func (m *MockServiceContainer) BuildExecutor() services.BuildExecutor       { return nil }
func (m *MockServiceContainer) ToolRegistry() services.ToolRegistry         { return nil }
func (m *MockServiceContainer) WorkflowExecutor() services.WorkflowExecutor { return nil }
func (m *MockServiceContainer) Scanner() services.Scanner                   { return nil }
func (m *MockServiceContainer) ConfigValidator() services.ConfigValidator   { return nil }
func (m *MockServiceContainer) ErrorReporter() services.ErrorReporter       { return nil }
func (m *MockServiceContainer) StateManager() services.StateManager         { return nil }
func (m *MockServiceContainer) KnowledgeBase() services.KnowledgeBase       { return nil }
func (m *MockServiceContainer) K8sClient() services.K8sClient               { return nil }
func (m *MockServiceContainer) Analyzer() services.Analyzer                 { return nil }
func (m *MockServiceContainer) Logger() *slog.Logger                        { return slog.Default() }

// MockSessionStore for testing
type MockSessionStore struct{}

func (m *MockSessionStore) Create(ctx context.Context, session *api.Session) error { return nil }
func (m *MockSessionStore) Get(ctx context.Context, sessionID string) (*api.Session, error) {
	return &api.Session{ID: sessionID}, nil
}
func (m *MockSessionStore) Update(ctx context.Context, session *api.Session) error { return nil }
func (m *MockSessionStore) Delete(ctx context.Context, sessionID string) error     { return nil }
func (m *MockSessionStore) List(ctx context.Context) ([]*api.Session, error)       { return nil, nil }

// MockSessionState for testing
type MockSessionState struct{}

func (m *MockSessionState) SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error {
	return nil
}
func (m *MockSessionState) GetState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *MockSessionState) CreateCheckpoint(ctx context.Context, sessionID string, name string) error {
	return nil
}
func (m *MockSessionState) RestoreCheckpoint(ctx context.Context, sessionID string, name string) error {
	return nil
}
func (m *MockSessionState) ListCheckpoints(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

// MockPipelineAdapter for testing
type MockPipelineAdapter struct{}

func TestAnalyzeRepositoryTool_Name(t *testing.T) {
	tool := &AnalyzeRepositoryTool{}
	assert.Equal(t, "analyze_repository", tool.Name())
}

func TestAnalyzeRepositoryTool_Description(t *testing.T) {
	tool := &AnalyzeRepositoryTool{}
	description := tool.Description()
	assert.Contains(t, description, "Comprehensive repository analysis")
	assert.Contains(t, description, "containerization assessment")
}

func TestAnalyzeRepositoryTool_Schema(t *testing.T) {
	tool := &AnalyzeRepositoryTool{}
	schema := tool.Schema()

	assert.Equal(t, "analyze_repository", schema.Name)
	assert.Equal(t, "2.0.0", schema.Version)
	assert.NotNil(t, schema.InputSchema)
	assert.NotNil(t, schema.OutputSchema)
}

func TestAnalyzeRepositoryInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   AnalyzeRepositoryInput
		wantErr bool
	}{
		{
			name: "valid input with repo_url",
			input: AnalyzeRepositoryInput{
				RepoURL: "https://github.com/user/repo",
				Branch:  "main",
			},
			wantErr: false,
		},
		{
			name: "valid input with repo_path alias",
			input: AnalyzeRepositoryInput{
				RepoPath: "/path/to/repo",
			},
			wantErr: false,
		},
		{
			name: "valid input with path alias",
			input: AnalyzeRepositoryInput{
				Path: "/path/to/repo",
			},
			wantErr: false,
		},
		{
			name:    "invalid input without repo URL",
			input:   AnalyzeRepositoryInput{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnalyzeRepositoryInput_getRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		input    AnalyzeRepositoryInput
		expected string
	}{
		{
			name: "repo_url takes precedence",
			input: AnalyzeRepositoryInput{
				RepoURL:  "https://github.com/user/repo",
				RepoPath: "/path/to/repo",
				Path:     "/another/path",
			},
			expected: "https://github.com/user/repo",
		},
		{
			name: "repo_path fallback",
			input: AnalyzeRepositoryInput{
				RepoPath: "/path/to/repo",
				Path:     "/another/path",
			},
			expected: "/path/to/repo",
		},
		{
			name: "path fallback",
			input: AnalyzeRepositoryInput{
				Path: "/another/path",
			},
			expected: "/another/path",
		},
		{
			name:     "empty when no URLs provided",
			input:    AnalyzeRepositoryInput{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.getRepoURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzeRepositoryTool_parseInput(t *testing.T) {
	tool := &AnalyzeRepositoryTool{}

	tests := []struct {
		name    string
		input   api.ToolInput
		wantErr bool
	}{
		{
			name: "valid map input",
			input: api.ToolInput{
				Arguments: map[string]interface{}{
					"repo_url": "https://github.com/user/repo",
					"branch":   "main",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid input type",
			input: api.ToolInput{
				Arguments: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.parseInput(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAnalyzeRepositoryTool_isLocalPath(t *testing.T) {
	tool := &AnalyzeRepositoryTool{}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "http URL",
			path:     "http://github.com/user/repo",
			expected: false,
		},
		{
			name:     "https URL",
			path:     "https://github.com/user/repo",
			expected: false,
		},
		{
			name:     "git SSH URL",
			path:     "git@github.com:user/repo.git",
			expected: false,
		},
		{
			name:     "absolute local path",
			path:     "/tmp",
			expected: true,
		},
		{
			name:     "relative local path",
			path:     "./test",
			expected: false, // This would fail os.Stat in our implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.isLocalPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSimpleAnalyzeRepositoryTool_Name(t *testing.T) {
	tool := &SimpleAnalyzeRepositoryTool{}
	assert.Equal(t, "analyze_repository_simple", tool.Name())
}

func TestSimpleAnalyzeRepositoryTool_Description(t *testing.T) {
	tool := &SimpleAnalyzeRepositoryTool{}
	description := tool.Description()
	assert.Contains(t, description, "Lightweight repository analysis")
	assert.Contains(t, description, "basic language")
}

func TestSimpleAnalyzeRepositoryTool_detectLanguage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_repo")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tool := &SimpleAnalyzeRepositoryTool{
		logger: slog.Default(),
	}

	tests := []struct {
		name         string
		setupFiles   []string
		expectedLang string
	}{
		{
			name:         "javascript project",
			setupFiles:   []string{"package.json"},
			expectedLang: "javascript",
		},
		{
			name:         "python project",
			setupFiles:   []string{"requirements.txt"},
			expectedLang: "python",
		},
		{
			name:         "java project",
			setupFiles:   []string{"pom.xml"},
			expectedLang: "java",
		},
		{
			name:         "go project",
			setupFiles:   []string{"go.mod"},
			expectedLang: "go",
		},
		{
			name:         "unknown project",
			setupFiles:   []string{"README.md"},
			expectedLang: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			for _, file := range tt.setupFiles {
				f, err := os.Create(tempDir + "/" + file)
				require.NoError(t, err)
				f.Close()
			}

			// Test language detection
			lang, err := tool.detectLanguage(tempDir)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLang, lang)

			// Clean up test files
			for _, file := range tt.setupFiles {
				os.Remove(tempDir + "/" + file)
			}
		})
	}
}

func TestSimpleAnalyzeRepositoryTool_detectFramework(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_repo")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	tool := &SimpleAnalyzeRepositoryTool{
		logger: slog.Default(),
	}

	tests := []struct {
		name              string
		language          string
		setupFiles        []string
		expectedFramework string
	}{
		{
			name:              "django project",
			language:          "python",
			setupFiles:        []string{"manage.py"},
			expectedFramework: "django",
		},
		{
			name:              "flask project",
			language:          "python",
			setupFiles:        []string{"app.py"},
			expectedFramework: "flask",
		},
		{
			name:              "maven project",
			language:          "java",
			setupFiles:        []string{"pom.xml"},
			expectedFramework: "maven",
		},
		{
			name:              "gradle project",
			language:          "java",
			setupFiles:        []string{"build.gradle"},
			expectedFramework: "gradle",
		},
		{
			name:              "node project",
			language:          "javascript",
			setupFiles:        []string{"package.json"},
			expectedFramework: "node",
		},
		{
			name:              "no framework",
			language:          "unknown",
			setupFiles:        []string{},
			expectedFramework: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			for _, file := range tt.setupFiles {
				f, err := os.Create(tempDir + "/" + file)
				require.NoError(t, err)
				f.Close()
			}

			// Test framework detection
			framework := tool.detectFramework(tempDir, tt.language)
			assert.Equal(t, tt.expectedFramework, framework)

			// Clean up test files
			for _, file := range tt.setupFiles {
				os.Remove(tempDir + "/" + file)
			}
		})
	}
}

func TestSimpleAnalyzeRepositoryTool_generateBuildCommands(t *testing.T) {
	tool := &SimpleAnalyzeRepositoryTool{}

	tests := []struct {
		name             string
		language         string
		framework        string
		expectedCommands []string
	}{
		{
			name:             "javascript project",
			language:         "javascript",
			framework:        "node",
			expectedCommands: []string{"npm install", "npm run build"},
		},
		{
			name:             "python project",
			language:         "python",
			framework:        "",
			expectedCommands: []string{"pip install -r requirements.txt"},
		},
		{
			name:             "java maven project",
			language:         "java",
			framework:        "maven",
			expectedCommands: []string{"mvn clean package"},
		},
		{
			name:             "java gradle project",
			language:         "java",
			framework:        "gradle",
			expectedCommands: []string{"gradle build"},
		},
		{
			name:             "go project",
			language:         "go",
			framework:        "",
			expectedCommands: []string{"go build"},
		},
		{
			name:             "unknown project",
			language:         "unknown",
			framework:        "",
			expectedCommands: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := tool.generateBuildCommands(tt.language, tt.framework)
			assert.Equal(t, tt.expectedCommands, commands)
		})
	}
}

func TestSimpleAnalyzeRepositoryTool_generateRunCommand(t *testing.T) {
	tool := &SimpleAnalyzeRepositoryTool{}

	tests := []struct {
		name            string
		language        string
		framework       string
		expectedCommand string
	}{
		{
			name:            "javascript project",
			language:        "javascript",
			framework:       "node",
			expectedCommand: "npm start",
		},
		{
			name:            "python django project",
			language:        "python",
			framework:       "django",
			expectedCommand: "python manage.py runserver",
		},
		{
			name:            "python flask project",
			language:        "python",
			framework:       "flask",
			expectedCommand: "python app.py",
		},
		{
			name:            "java project",
			language:        "java",
			framework:       "maven",
			expectedCommand: "java -jar target/app.jar",
		},
		{
			name:            "go project",
			language:        "go",
			framework:       "",
			expectedCommand: "./main",
		},
		{
			name:            "unknown project",
			language:        "unknown",
			framework:       "",
			expectedCommand: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := tool.generateRunCommand(tt.language, tt.framework)
			assert.Equal(t, tt.expectedCommand, command)
		})
	}
}

func TestSimpleAnalyzeRepositoryTool_detectPort(t *testing.T) {
	tool := &SimpleAnalyzeRepositoryTool{}

	tests := []struct {
		name         string
		language     string
		framework    string
		expectedPort int
	}{
		{
			name:         "javascript project",
			language:     "javascript",
			framework:    "node",
			expectedPort: 3000,
		},
		{
			name:         "python django project",
			language:     "python",
			framework:    "django",
			expectedPort: 8000,
		},
		{
			name:         "python flask project",
			language:     "python",
			framework:    "flask",
			expectedPort: 5000,
		},
		{
			name:         "java project",
			language:     "java",
			framework:    "maven",
			expectedPort: 8080,
		},
		{
			name:         "go project",
			language:     "go",
			framework:    "",
			expectedPort: 8080,
		},
		{
			name:         "unknown project",
			language:     "unknown",
			framework:    "",
			expectedPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := tool.detectPort("", tt.language, tt.framework)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestCacheManager_GetSet(t *testing.T) {
	cacheManager := NewCacheManager(slog.Default())

	ctx := context.Background()
	repoURL := "https://github.com/user/repo"
	branch := "main"

	// Test cache miss
	_, err := cacheManager.Get(ctx, repoURL, branch)
	assert.Error(t, err)

	// Test cache set and hit
	result := &AnalyzeRepositoryOutput{
		Success:  true,
		Language: "javascript",
	}
	cacheManager.Set(ctx, repoURL, branch, result)

	cached, err := cacheManager.Get(ctx, repoURL, branch)
	assert.NoError(t, err)
	assert.Equal(t, result, cached)
}

func TestCacheManager_ExpiredEntries(t *testing.T) {
	cacheManager := NewCacheManager(slog.Default())
	cacheManager.ttl = 1 * time.Millisecond // Very short TTL for testing

	ctx := context.Background()
	repoURL := "https://github.com/user/repo"
	branch := "main"

	// Set cache entry
	result := &AnalyzeRepositoryOutput{
		Success:  true,
		Language: "javascript",
	}
	cacheManager.Set(ctx, repoURL, branch, result)

	// Wait for expiration
	time.Sleep(2 * time.Millisecond)

	// Test cache miss due to expiration
	_, err := cacheManager.Get(ctx, repoURL, branch)
	assert.Error(t, err)
}

func TestDeprecatedTools_GetMigrationGuide(t *testing.T) {
	guide := GetMigrationGuide("generic_analyze_tool")
	assert.NotNil(t, guide)
	assert.Equal(t, "generic_analyze_tool", guide.OldTool)
	assert.Equal(t, "analyze_repository", guide.NewTool)
	assert.NotEmpty(t, guide.ParameterChanges)
	assert.NotEmpty(t, guide.Examples)
}

func TestDeprecatedTools_GetDeprecationNotice(t *testing.T) {
	notice := GetDeprecationNotice("generic_analyze_tool")
	assert.NotNil(t, notice)
	assert.Equal(t, "generic_analyze_tool", notice.Tool)
	assert.Equal(t, "deprecated", notice.Status)
	assert.Equal(t, "analyze_repository", notice.Replacement)
	assert.NotNil(t, notice.MigrationGuide)
}

// Integration test for the consolidated tool
func TestAnalyzeRepositoryTool_Integration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_repo")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	packageJSON := `{
		"name": "test-app",
		"version": "1.0.0",
		"dependencies": {
			"express": "^4.18.0"
		}
	}`
	err = os.WriteFile(tempDir+"/package.json", []byte(packageJSON), 0644)
	require.NoError(t, err)

	// Create tool with mock services
	tool := NewAnalyzeRepositoryTool(
		&MockPipelineAdapter{},
		&MockServiceContainer{},
		slog.Default(),
	)

	// Test execution
	input := api.ToolInput{
		Arguments: map[string]interface{}{
			"repo_url":             tempDir,
			"include_dependencies": true,
			"use_cache":            false,
		},
	}

	output, err := tool.Execute(context.Background(), input)
	assert.NoError(t, err)
	assert.True(t, output.Success)
	assert.NotNil(t, output.Data)

	// Verify result structure
	resultData, ok := output.Data["result"]
	assert.True(t, ok)
	assert.NotNil(t, resultData)
}
