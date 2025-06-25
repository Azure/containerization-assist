package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPipelineAdapter implements mcptypes.PipelineOperations for testing
type testPipelineAdapter struct {
	workspaceDir       string
	analysisResult     *analysis.AnalysisResult
	analysisError      error
	cloneResult        *git.CloneResult
	cloneError         error
	shouldFailClone    bool
	shouldFailAnalysis bool
}

func (t *testPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	if t.workspaceDir != "" {
		return t.workspaceDir
	}
	return fmt.Sprintf("/workspace/%s", sessionID)
}

func (t *testPipelineAdapter) AnalyzeRepository(sessionID, repoPath string) (*analysis.AnalysisResult, error) {
	if t.shouldFailAnalysis {
		return nil, t.analysisError
	}
	if t.analysisResult != nil {
		return t.analysisResult, nil
	}
	// Default result
	return &analysis.AnalysisResult{
		Language:  "Go",
		Framework: "gin",
		Port:      8080,
		Dependencies: []analysis.Dependency{
			{Name: "github.com/gin-gonic/gin", Version: "v1.8.1"},
		},
	}, nil
}

func (t *testPipelineAdapter) CloneRepository(sessionID, repoURL, branch string) (*git.CloneResult, error) {
	if t.shouldFailClone {
		return t.cloneResult, t.cloneError
	}
	if t.cloneResult != nil {
		return t.cloneResult, nil
	}
	// Default result
	return &git.CloneResult{
		RepoPath:   fmt.Sprintf("/workspace/%s/repo", sessionID),
		Branch:     branch,
		CommitHash: "abc123",
		Duration:   2 * time.Second,
	}, nil
}

func (t *testPipelineAdapter) SaveAnalysisCache(sessionID string, result *analysis.AnalysisResult) error {
	return nil
}

func (t *testPipelineAdapter) GetCachedAnalysis(sessionID string) (*analysis.AnalysisResult, bool) {
	return nil, false
}

// Implement remaining required methods
func (t *testPipelineAdapter) GenerateDockerfile(sessionID, language, framework string) (string, error) {
	// Return a minimal happy-path Dockerfile for testing
	dockerfile := fmt.Sprintf(`FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .
EXPOSE 8080
CMD ["./main"]
# Generated for %s/%s in session %s`, language, framework, sessionID)
	return dockerfile, nil
}

func (t *testPipelineAdapter) BuildDockerImage(sessionID, imageName, dockerfilePath string) (*mcptypes.BuildResult, error) {
	// Return a minimal happy-path build result for testing
	return &mcptypes.BuildResult{
		Success:  true,
		ImageID:  "sha256:abc123def456",
		ImageRef: imageName,
		Logs:     "Step 1/5 : FROM golang:1.21-alpine\nSuccessfully built abc123def456",
	}, nil
}

func (t *testPipelineAdapter) PushDockerImage(sessionID, imageName string) error {
	// Return success for tests
	return nil
}

func (t *testPipelineAdapter) GenerateKubernetesManifests(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	// Return a minimal happy-path manifest generation result for testing
	return &mcptypes.KubernetesManifestResult{
		Success: true,
		Manifests: []mcptypes.GeneratedManifest{
			{Name: "deployment", Kind: "Deployment", Path: fmt.Sprintf("/workspace/%s/manifests/deployment.yaml", sessionID), Content: "apiVersion: apps/v1\nkind: Deployment"},
			{Name: "service", Kind: "Service", Path: fmt.Sprintf("/workspace/%s/manifests/service.yaml", sessionID), Content: "apiVersion: v1\nkind: Service"},
			{Name: "configmap", Kind: "ConfigMap", Path: fmt.Sprintf("/workspace/%s/manifests/configmap.yaml", sessionID), Content: "apiVersion: v1\nkind: ConfigMap"},
		},
	}, nil
}

func (t *testPipelineAdapter) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	// Return a minimal happy-path deployment result for testing
	return &mcptypes.KubernetesDeploymentResult{
		Success:     true,
		Namespace:   "default",
		Deployments: []string{"test-app"},
		Services:    []string{"test-app-svc"},
	}, nil
}

func (t *testPipelineAdapter) CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	// Return a minimal happy-path health check result for testing
	return &mcptypes.HealthCheckResult{
		Healthy: true,
		Status:  "All pods are running and ready",
		PodStatuses: []mcptypes.PodStatus{
			{Name: "test-app-1", Ready: true, Status: "Running", Reason: "Running"},
			{Name: "test-app-2", Ready: true, Status: "Running", Reason: "Running"},
		},
	}, nil
}

func (t *testPipelineAdapter) PreviewDeployment(sessionID, manifestPath, namespace string) (string, error) {
	// Return a minimal happy-path preview for testing
	preview := fmt.Sprintf(`Deployment Preview for session %s:

Namespace: %s
Manifest: %s

Resources to be created:
- Deployment: test-app (2 replicas)
- Service: test-app-svc (ClusterIP, port 8080)
- ConfigMap: test-app-config

Resource Summary:
- CPU Request: 100m, Limit: 500m
- Memory Request: 128Mi, Limit: 512Mi
- Health checks: enabled
- Rolling update strategy: 25%% max unavailable

Estimated deployment time: 60 seconds`, sessionID, namespace, manifestPath)
	return preview, nil
}

// Context management methods for testing
func (t *testPipelineAdapter) SetContext(sessionID string, ctx context.Context) {
	// No-op for tests
}

func (t *testPipelineAdapter) GetContext(sessionID string) context.Context {
	return context.Background()
}

func (t *testPipelineAdapter) ClearContext(sessionID string) {
	// No-op for tests
}

func (t *testPipelineAdapter) TagDockerImage(sessionID, sourceImage, targetImage string) error {
	// Return success for tests
	return nil
}

func (t *testPipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	// Return success for tests
	return nil
}

// AcquireResource manages resource allocation for a session (required by PipelineOperations interface)
func (t *testPipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	// Mock implementation for tests
	return nil
}

// ReleaseResource manages resource cleanup for a session (required by PipelineOperations interface)
func (t *testPipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	// Mock implementation for tests
	return nil
}

// ConvertToDockerState creates a simple Docker state for interface compatibility
func (t *testPipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	// Mock implementation for tests
	return &mcptypes.DockerState{
		Images:     []string{},
		Containers: []string{},
		Networks:   []string{},
		Volumes:    []string{},
	}, nil
}

// UpdateSessionFromDockerResults updates session state with Docker stage results (required by PipelineOperations interface)
func (t *testPipelineAdapter) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	// Mock implementation for tests
	return nil
}

// testSessionManager implements mcptypes.ToolSessionManager for testing
type testSessionManager struct {
	sessions         map[string]*sessiontypes.SessionState
	shouldFailCreate bool
	shouldFailGet    bool
}

func newTestSessionManager() *testSessionManager {
	return &testSessionManager{
		sessions: make(map[string]*sessiontypes.SessionState),
	}
}

func (t *testSessionManager) GetSession(sessionID string) (interface{}, error) {
	if t.shouldFailGet {
		return nil, fmt.Errorf("failed to get session")
	}
	if session, exists := t.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (t *testSessionManager) CreateSession() (*sessiontypes.SessionState, error) {
	if t.shouldFailCreate {
		return nil, fmt.Errorf("session store full")
	}
	sessionID := fmt.Sprintf("new-session-%d", len(t.sessions)+1)
	session := &sessiontypes.SessionState{
		SessionID:    sessionID,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		WorkspaceDir: fmt.Sprintf("/workspace/%s", sessionID),
	}
	t.sessions[sessionID] = session
	return session, nil
}

func (t *testSessionManager) SaveSession(session *sessiontypes.SessionState) error {
	t.sessions[session.SessionID] = session
	return nil
}

func (t *testSessionManager) UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error {
	session, exists := t.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}
	updateFunc(session)
	return nil
}

func (t *testSessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	if sessionID != "" {
		if session, exists := t.sessions[sessionID]; exists {
			return session, nil
		}
	}
	session, err := t.CreateSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Interface compatibility methods for ToolSessionManager

func (t *testSessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	return t.GetSession(sessionID)
}

func (t *testSessionManager) GetOrCreateSessionFromRepo(repoURL string) (interface{}, error) {
	session, err := t.CreateSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (t *testSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	delete(t.sessions, sessionID)
	return nil
}

func (t *testSessionManager) ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error) {
	var sessions []interface{}
	for _, session := range t.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (t *testSessionManager) FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error) {
	// Mock implementation - return first session or create new one
	for _, session := range t.sessions {
		return session, nil
	}
	session, err := t.CreateSession()
	if err != nil {
		return nil, err
	}
	return session, nil
}

func TestAtomicAnalyzeRepositoryTool_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(nil).Level(zerolog.Disabled)
	ctx := context.Background()

	tests := []struct {
		name           string
		args           AtomicAnalyzeRepositoryArgs
		setupTest      func(*testPipelineAdapter, *testSessionManager)
		expectedError  bool
		validateResult func(*testing.T, *AtomicAnalysisResult)
	}{
		{
			name: "successful analysis with new session",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "",
					DryRun:    false,
				},
				RepoURL: "/tmp/test-repo", // Will be replaced in setupTest
				Branch:  "main",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				// Default behavior is sufficient
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				if !result.Success {
					t.Errorf("Unexpected failure in result")
				}
				t.Logf("Result success: %v, SessionID: %s", result.Success, result.SessionID)
				assert.True(t, result.Success)
				assert.NotEmpty(t, result.SessionID)
				assert.NotEmpty(t, result.RepoURL) // Will be temp directory
				assert.Equal(t, "main", result.Branch)
				assert.NotNil(t, result.Analysis)
				assert.Equal(t, "unknown", result.Analysis.Language)
				assert.Equal(t, "", result.Analysis.Framework)
				assert.True(t, result.Success)
			},
		},
		{
			name: "successful analysis with existing session and cache",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "cached-session",
					DryRun:    false,
				},
				RepoURL: "/local/repo/path",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				// Create session with cache
				session := &sessiontypes.SessionState{
					SessionID: "cached-session",
					Metadata:  make(map[string]interface{}),
					ScanSummary: &types.RepositoryScanSummary{
						Language:         "Python",
						Framework:        "flask",
						Port:             5000,
						Dependencies:     []string{"flask==2.0.1", "requests==2.26.0"},
						RepoPath:         "/local/repo/path",
						CachedAt:         time.Now().Add(-30 * time.Minute),
						FilesAnalyzed:    42,
						ConfigFilesFound: []string{"requirements.txt", "setup.py"},
						EntryPointsFound: []string{"app.py", "main.py"},
						HasGitIgnore:     true,
						HasReadme:        true,
						RepositorySize:   1024000,
						AnalysisDuration: 1.5,
					},
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				sessionMgr.sessions["cached-session"] = session
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				assert.True(t, result.Success)
				assert.Equal(t, "cached-session", result.SessionID)
				assert.Equal(t, "/local/repo/path", result.CloneDir)
				assert.NotNil(t, result.Analysis)
				assert.Equal(t, "Python", result.Analysis.Language)
				assert.Equal(t, "flask", result.Analysis.Framework)
				assert.Equal(t, 42, result.AnalysisContext.FilesAnalyzed)
				assert.True(t, result.AnalysisContext.HasGitIgnore)
			},
		},
		{
			name: "dry run mode",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "dry-run-session",
					DryRun:    true,
				},
				RepoURL: "/tmp/test-dry-run",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				session := &sessiontypes.SessionState{
					SessionID:    "dry-run-session",
					Metadata:     make(map[string]interface{}),
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				sessionMgr.sessions["dry-run-session"] = session
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				assert.Equal(t, "dry-run-session", result.SessionID)
				assert.True(t, result.DryRun)
				assert.Contains(t, result.AnalysisContext.NextStepSuggestions[0], "dry-run")
			},
		},
		{
			name: "clone repository failure",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "clone-fail-session",
					DryRun:    false,
				},
				RepoURL: "/tmp/nonexistent-repo",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.shouldFailClone = true
				adapter.cloneError = fmt.Errorf("repository not found")
				adapter.cloneResult = &git.CloneResult{
					Duration: 1 * time.Second,
					Error: &git.GitError{
						Type:    "clone_error",
						Message: "repository not found",
						RepoURL: "/tmp/nonexistent-repo",
					},
				}

				session := &sessiontypes.SessionState{
					SessionID:    "clone-fail-session",
					Metadata:     make(map[string]interface{}),
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				sessionMgr.sessions["clone-fail-session"] = session
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				// Error scenarios return errors directly, result may be nil
				if result != nil {
					assert.False(t, result.Success)
				}
			},
		},
		{
			name: "analysis failure",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "analysis-fail-session",
					DryRun:    false,
				},
				RepoURL: "/local/invalid/path",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.shouldFailAnalysis = true
				adapter.analysisError = fmt.Errorf("no supported language detected")

				session := &sessiontypes.SessionState{
					SessionID:    "analysis-fail-session",
					Metadata:     make(map[string]interface{}),
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				sessionMgr.sessions["analysis-fail-session"] = session
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				assert.False(t, result.Success)
				assert.False(t, result.Success)
			},
		},
		{
			name: "session creation failure",
			args: AtomicAnalyzeRepositoryArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "",
					DryRun:    false,
				},
				RepoURL: "/tmp/test-repo",
			},
			setupTest: func(adapter *testPipelineAdapter, sessionMgr *testSessionManager) {
				sessionMgr.shouldFailCreate = true
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicAnalysisResult) {
				assert.False(t, result.Success)
				assert.False(t, result.Success)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test components
			adapter := &testPipelineAdapter{}
			sessionMgr := newTestSessionManager()

			if tt.setupTest != nil {
				tt.setupTest(adapter, sessionMgr)
			}

			// Replace RepoURL with temp directory if it's a test path
			args := tt.args
			if args.RepoURL == "/tmp/test-repo" {
				args.RepoURL = t.TempDir()
			}

			// Create tool
			tool := NewAtomicAnalyzeRepositoryTool(adapter, sessionMgr, logger)

			// Execute
			result, err := tool.Execute(ctx, args)

			// Verify
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validateResult != nil && result != nil {
				typedResult, ok := result.(*AtomicAnalysisResult)
				require.True(t, ok, "Result should be *AtomicAnalysisResult")
				tt.validateResult(t, typedResult)
			}
		})
	}
}

func TestAtomicAnalyzeRepositoryTool_ValidationHelpers(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	adapter := &testPipelineAdapter{}
	sessionMgr := newTestSessionManager()
	tool := NewAtomicAnalyzeRepositoryTool(adapter, sessionMgr, logger)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"http URL", "http://github.com/test/repo", true},
		{"https URL", "https://github.com/test/repo", true},
		{"git URL", "git@github.com:test/repo.git", true},
		{"ssh URL", "ssh://git@github.com/test/repo", true},
		{"local path absolute", "/home/user/projects/myapp", false},
		{"local path relative", "./myapp", false},
		{"local path with spaces", "/home/user/my projects/app", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.isURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicAnalyzeRepositoryTool_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(nil).Level(zerolog.Disabled)
	ctx := context.Background()

	t.Run("stale cache is ignored", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()

		adapter := &testPipelineAdapter{}
		sessionMgr := newTestSessionManager()

		// Create session with stale cache (2 hours old)
		session := &sessiontypes.SessionState{
			SessionID: "stale-cache-session",
			Metadata:  make(map[string]interface{}),
			ScanSummary: &types.RepositoryScanSummary{
				Language:  "JavaScript",
				Framework: "express",
				RepoPath:  tmpDir,
				CachedAt:  time.Now().Add(-2 * time.Hour),
			},
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		sessionMgr.sessions["stale-cache-session"] = session

		tool := NewAtomicAnalyzeRepositoryTool(adapter, sessionMgr, logger)

		resultInterface, err := tool.Execute(ctx, AtomicAnalyzeRepositoryArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "stale-cache-session",
				DryRun:    false,
			},
			RepoURL: tmpDir,
		})

		require.NoError(t, err)
		result, ok := resultInterface.(*AtomicAnalysisResult)
		require.True(t, ok, "Result should be *AtomicAnalysisResult")
		assert.True(t, result.Success)
		// Should have performed fresh analysis and detected no specific language
		assert.NotEqual(t, "JavaScript", result.Analysis.Language)
	})

	t.Run("resumed session context", func(t *testing.T) {
		adapter := &testPipelineAdapter{}
		sessionMgr := newTestSessionManager()

		// Create session with resume metadata
		session := &sessiontypes.SessionState{
			SessionID: "resumed-session",
			Metadata: map[string]interface{}{
				"resumed_from": map[string]interface{}{
					"old_session_id": "expired-123",
					"last_repo_url":  "/tmp/old-repo",
				},
			},
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		sessionMgr.sessions["resumed-session"] = session

		tool := NewAtomicAnalyzeRepositoryTool(adapter, sessionMgr, logger)

		resultInterface, err := tool.Execute(ctx, AtomicAnalyzeRepositoryArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "resumed-session",
				DryRun:    false,
			},
			RepoURL: t.TempDir(),
		})

		require.NoError(t, err)
		result, ok := resultInterface.(*AtomicAnalysisResult)
		require.True(t, ok, "Result should be *AtomicAnalysisResult")
		assert.True(t, result.Success)
		// Should have analysis completed successfully with suggestions
		assert.NotEmpty(t, result.AnalysisContext.NextStepSuggestions)
		// Note: The resume logic might not trigger as expected due to session ID differences
	})
}
