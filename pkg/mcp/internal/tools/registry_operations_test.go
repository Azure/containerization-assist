package tools

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAtomicPipelineAdapter for testing atomic tools
type MockAtomicPipelineAdapter struct {
	mock.Mock
}

func (m *MockAtomicPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	args := m.Called(sessionID)
	return args.String(0)
}

func (m *MockAtomicPipelineAdapter) LoadPipelineState(sessionID string) (interface{}, error) {
	args := m.Called(sessionID)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) SavePipelineState(sessionID string, state interface{}) error {
	args := m.Called(sessionID, state)
	return args.Error(0)
}

// Implement all mcptypes.PipelineOperations methods (stubs for testing)
func (m *MockAtomicPipelineAdapter) AnalyzeRepository(sessionID, repoPath string) (*analysis.AnalysisResult, error) {
	args := m.Called(sessionID, repoPath)
	return args.Get(0).(*analysis.AnalysisResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) CloneRepository(sessionID, repoURL, branch string) (*git.CloneResult, error) {
	args := m.Called(sessionID, repoURL, branch)
	return args.Get(0).(*git.CloneResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) GenerateDockerfile(sessionID, language, framework string) (string, error) {
	args := m.Called(sessionID, language, framework)
	return args.String(0), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) BuildDockerImage(sessionID, imageName, dockerfilePath string) (*mcptypes.BuildResult, error) {
	args := m.Called(sessionID, imageName, dockerfilePath)
	return args.Get(0).(*mcptypes.BuildResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) PushDockerImage(sessionID, imageName string) error {
	args := m.Called(sessionID, imageName)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) GenerateKubernetesManifests(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	args := m.Called(sessionID, imageName, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcptypes.KubernetesManifestResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) DeployToKubernetes(sessionID string, manifestPaths []string) (*mcptypes.KubernetesDeploymentResult, error) {
	args := m.Called(sessionID, manifestPaths)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcptypes.KubernetesDeploymentResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) CheckApplicationHealth(sessionID, namespace, labelSelector string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	args := m.Called(sessionID, namespace, labelSelector, timeout)
	return args.Get(0).(*mcptypes.HealthCheckResult), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) PreviewDeployment(sessionID, manifestPath, namespace string) (string, error) {
	args := m.Called(sessionID, manifestPath, namespace)
	return args.String(0), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) SaveAnalysisCache(sessionID string, result *analysis.AnalysisResult) error {
	args := m.Called(sessionID, result)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) SetContext(sessionID string, ctx context.Context) {
	m.Called(sessionID, ctx)
}

func (m *MockAtomicPipelineAdapter) GetContext(sessionID string) context.Context {
	args := m.Called(sessionID)
	return args.Get(0).(context.Context)
}

func (m *MockAtomicPipelineAdapter) ClearContext(sessionID string) {
	m.Called(sessionID)
}

func (m *MockAtomicPipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	args := m.Called(sessionID, resourceType)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	args := m.Called(sessionID, resourceType)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	args := m.Called(sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcptypes.DockerState), args.Error(1)
}

func (m *MockAtomicPipelineAdapter) TagDockerImage(sessionID, sourceImage, targetImage string) error {
	args := m.Called(sessionID, sourceImage, targetImage)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	args := m.Called(sessionID, imageRef)
	return args.Error(0)
}

func (m *MockAtomicPipelineAdapter) UpdateSessionFromDockerResults(sessionID string, results interface{}) error {
	args := m.Called(sessionID, results)
	return args.Error(0)
}

// MockAtomicSessionManager for testing atomic tools
type MockAtomicSessionManager struct {
	mock.Mock
}

func (m *MockAtomicSessionManager) GetSession(sessionID string) (interface{}, error) {
	args := m.Called(sessionID)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicSessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	args := m.Called(sessionID)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicSessionManager) SaveSession(session *sessiontypes.SessionState) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *MockAtomicSessionManager) CreateSession() (*sessiontypes.SessionState, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sessiontypes.SessionState), args.Error(1)
}

func (m *MockAtomicSessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	args := m.Called(sessionID)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAtomicSessionManager) FindSessionByRepo(ctx context.Context, repoPath string) (interface{}, error) {
	args := m.Called(ctx, repoPath)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicSessionManager) GetOrCreateSessionFromRepo(repoPath string) (interface{}, error) {
	args := m.Called(repoPath)
	return args.Get(0), args.Error(1)
}

func (m *MockAtomicSessionManager) ListSessions(ctx context.Context, filters map[string]interface{}) ([]interface{}, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAtomicSessionManager) UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error {
	args := m.Called(sessionID, updateFunc)
	return args.Error(0)
}

// Helper to create a test session
func createTestSession(sessionID string) *sessiontypes.SessionState {
	return &sessiontypes.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/test-workspace",
		Metadata:     make(map[string]interface{}),
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
}

func TestAtomicPullImageTool(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled) // Disable logging for tests

	t.Run("successful pull", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		// Setup expectations
		session := createTestSession("test-session")
		mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
		mockSessionMgr.On("UpdateSession", mock.Anything, mock.Anything).Return(nil)
		mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

		// Mock successful pull - now returns only error
		mockAdapter.On("PullDockerImage", "test-session", "nginx:latest").Return(nil)

		// Create tool
		pullTool := NewAtomicPullImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args
		args := AtomicPullImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "test-session",
				DryRun:    false,
			},
			ImageRef: "nginx:latest",
		}

		// Execute
		resultInterface, err := pullTool.Execute(context.Background(), args)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resultInterface)
		result, ok := resultInterface.(*AtomicPullImageResult)
		assert.True(t, ok, "Result should be *AtomicPullImageResult")
		assert.True(t, result.Success)
		assert.Equal(t, "nginx:latest", result.ImageRef)
		assert.Equal(t, "docker.io", result.Registry)
		assert.NotNil(t, result.PullContext)
		assert.Equal(t, "successful", result.PullContext.PullStatus)

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
		mockAdapter.AssertExpectations(t)
	})

	t.Run("dry run", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		session := createTestSession("test-session")
		mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
		mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

		// Create tool
		pullTool := NewAtomicPullImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args
		args := AtomicPullImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "test-session",
				DryRun:    true,
			},
			ImageRef: "nginx:latest",
		}

		// Execute
		resultInterface, err := pullTool.Execute(context.Background(), args)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resultInterface)
		result, ok := resultInterface.(*AtomicPullImageResult)
		assert.True(t, ok, "Result should be *AtomicPullImageResult")
		assert.True(t, result.Success)
		assert.True(t, result.DryRun)
		assert.Equal(t, "nginx:latest", result.ImageRef)
		assert.Equal(t, "dry-run", result.PullContext.PullStatus)

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
		mockAdapter.AssertExpectations(t)
	})

	t.Run("session not found", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		// Setup expectations for session not found
		mockSessionMgr.On("GetSession", "invalid-session").Return(nil, assert.AnError)

		// Create tool
		pullTool := NewAtomicPullImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args
		args := AtomicPullImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "invalid-session",
				DryRun:    false,
			},
			ImageRef: "nginx:latest",
		}

		// Execute
		resultInterface, err := pullTool.Execute(context.Background(), args)

		// Assertions
		assert.Error(t, err) // Tool should return errors when session management fails
		// Result may be nil when errors occur during setup
		if resultInterface != nil {
			result, ok := resultInterface.(*AtomicPullImageResult)
			if ok {
				assert.False(t, result.Success)
			}
		}

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
	})
}

func TestAtomicTagImageTool(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled) // Disable logging for tests

	t.Run("successful tag", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		// Setup expectations
		session := createTestSession("test-session")
		mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
		mockSessionMgr.On("UpdateSession", mock.Anything, mock.Anything).Return(nil)
		mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

		// Mock TagDockerImage method - returns only error per interface
		mockAdapter.On("TagDockerImage", "test-session", "nginx:latest", "my-nginx:v1.0.0").Return(nil)

		// Create tool
		tagTool := NewAtomicTagImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args
		args := AtomicTagImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "test-session",
				DryRun:    false,
			},
			SourceImage: "nginx:latest",
			TargetImage: "my-nginx:v1.0.0",
		}

		// Execute
		resultInterface, err := tagTool.Execute(context.Background(), args)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resultInterface)
		result, ok := resultInterface.(*AtomicTagImageResult)
		assert.True(t, ok, "Result should be *AtomicTagImageResult")
		assert.True(t, result.Success)
		assert.Equal(t, "nginx:latest", result.SourceImage)
		assert.Equal(t, "my-nginx:v1.0.0", result.TargetImage)
		assert.NotNil(t, result.TagContext)
		assert.Equal(t, "successful", result.TagContext.TagStatus)

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
		mockAdapter.AssertExpectations(t)
	})

	t.Run("dry run", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		session := createTestSession("test-session")
		mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
		mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

		// Create tool
		tagTool := NewAtomicTagImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args
		args := AtomicTagImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "test-session",
				DryRun:    true,
			},
			SourceImage: "nginx:latest",
			TargetImage: "my-nginx:v1.0.0",
		}

		// Execute
		resultInterface, err := tagTool.Execute(context.Background(), args)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, resultInterface)
		result, ok := resultInterface.(*AtomicTagImageResult)
		assert.True(t, ok, "Result should be *AtomicTagImageResult")
		assert.True(t, result.Success)
		assert.True(t, result.DryRun)
		assert.Equal(t, "nginx:latest", result.SourceImage)
		assert.Equal(t, "my-nginx:v1.0.0", result.TargetImage)
		assert.Equal(t, "dry_run_successful", result.TagContext.TagStatus)

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
		mockAdapter.AssertExpectations(t)
	})

	t.Run("same source and target - should succeed", func(t *testing.T) {
		// Setup mocks
		mockAdapter := &MockAtomicPipelineAdapter{}
		mockSessionMgr := &MockAtomicSessionManager{}

		session := createTestSession("test-session")
		mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
		mockSessionMgr.On("UpdateSession", mock.Anything, mock.Anything).Return(nil)
		mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

		// Mock successful tag operation - Docker allows tagging same image with same name
		mockAdapter.On("TagDockerImage", "test-session", "nginx:latest", "nginx:latest").Return(nil)

		// Create tool
		tagTool := NewAtomicTagImageTool(mockAdapter, mockSessionMgr, logger)

		// Test args with same source and target
		args := AtomicTagImageArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "test-session",
				DryRun:    false,
			},
			SourceImage: "nginx:latest",
			TargetImage: "nginx:latest", // Same as source
		}

		// Execute
		resultInterface, err := tagTool.Execute(context.Background(), args)

		// Assertions - this should succeed as Docker allows it
		assert.NoError(t, err)
		assert.NotNil(t, resultInterface)
		result, ok := resultInterface.(*AtomicTagImageResult)
		assert.True(t, ok, "Result should be *AtomicTagImageResult")
		assert.True(t, result.Success)
		assert.Equal(t, "nginx:latest", result.SourceImage)
		assert.Equal(t, "nginx:latest", result.TargetImage)

		// Verify mocks
		mockSessionMgr.AssertExpectations(t)
		mockAdapter.AssertExpectations(t)
	})
}

func TestRegistryDetection(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	testCases := []struct {
		imageRef         string
		expectedRegistry string
	}{
		{"nginx:latest", "docker.io"},
		{"ubuntu:20.04", "docker.io"},
		{"myregistry.azurecr.io/app:v1.0.0", "myregistry.azurecr.io"},
		{"gcr.io/project/image:tag", "gcr.io"},
		{"localhost:5000/image:tag", "localhost:5000"},
	}

	for _, tc := range testCases {
		t.Run("registry detection for "+tc.imageRef, func(t *testing.T) {
			// Setup mocks
			mockAdapter := &MockAtomicPipelineAdapter{}
			mockSessionMgr := &MockAtomicSessionManager{}

			session := createTestSession("test-session")
			mockSessionMgr.On("GetSession", "test-session").Return(session, nil)
			mockAdapter.On("GetSessionWorkspace", "test-session").Return("/tmp/test-workspace")

			// Create tool
			pullTool := NewAtomicPullImageTool(mockAdapter, mockSessionMgr, logger)

			// Test args
			args := AtomicPullImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true, // Use dry run for faster execution
				},
				ImageRef: tc.imageRef,
			}

			// Execute
			resultInterface, err := pullTool.Execute(context.Background(), args)

			// Assertions
			assert.NoError(t, err)
			result, ok := resultInterface.(*AtomicPullImageResult)
			assert.True(t, ok, "Result should be *AtomicPullImageResult")
			assert.True(t, result.Success)
			assert.Equal(t, tc.expectedRegistry, result.Registry, "Failed for image: %s", tc.imageRef)

			// Verify mocks
			mockSessionMgr.AssertExpectations(t)
			mockAdapter.AssertExpectations(t)
		})
	}
}
