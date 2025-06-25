package build

import (
	"context"
	"fmt"
	"time"

	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// testPipelineAdapter implements mcptypes.PipelineOperations for testing
type testPipelineAdapter struct {
	workspaceDir string
}

func (t *testPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	if t.workspaceDir != "" {
		return t.workspaceDir
	}
	return fmt.Sprintf("/workspace/%s", sessionID)
}

// UpdateSessionFromDockerResults implements PipelineOperations
func (t *testPipelineAdapter) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	return nil
}

// BuildDockerImage implements PipelineOperations
func (t *testPipelineAdapter) BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*mcptypes.BuildResult, error) {
	return &mcptypes.BuildResult{
		Success:  true,
		ImageRef: imageRef,
		ImageID:  "sha256:abcd1234",
	}, nil
}

// PullDockerImage implements PipelineOperations
func (t *testPipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	return nil
}

// PushDockerImage implements PipelineOperations
func (t *testPipelineAdapter) PushDockerImage(sessionID, imageRef string) error {
	return nil
}

// TagDockerImage implements PipelineOperations
func (t *testPipelineAdapter) TagDockerImage(sessionID, sourceRef, targetRef string) error {
	return nil
}

// ConvertToDockerState implements PipelineOperations
func (t *testPipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	return &mcptypes.DockerState{
		Images: []string{"test-image"},
	}, nil
}

// GenerateKubernetesManifests implements PipelineOperations
func (t *testPipelineAdapter) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	return &mcptypes.KubernetesManifestResult{
		Success: true,
	}, nil
}

// DeployToKubernetes implements PipelineOperations
func (t *testPipelineAdapter) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	return &mcptypes.KubernetesDeploymentResult{
		Success: true,
	}, nil
}

// CheckApplicationHealth implements PipelineOperations
func (t *testPipelineAdapter) CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	return &mcptypes.HealthCheckResult{
		Healthy: true,
	}, nil
}

// AcquireResource implements PipelineOperations
func (t *testPipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	return nil
}

// ReleaseResource implements PipelineOperations
func (t *testPipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	return nil
}

// testSessionManager implements mcptypes.ToolSessionManager for testing
type testSessionManager struct {
	sessions map[string]*sessiontypes.SessionState
}

func newTestSessionManager() *testSessionManager {
	return &testSessionManager{
		sessions: make(map[string]*sessiontypes.SessionState),
	}
}

func (t *testSessionManager) GetSession(sessionID string) (interface{}, error) {
	if session, exists := t.sessions[sessionID]; exists {
		return session, nil
	}
	// Create a default session
	session := &sessiontypes.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: fmt.Sprintf("/workspace/%s", sessionID),
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
	t.sessions[sessionID] = session
	return session, nil
}

func (t *testSessionManager) CreateSession(workspaceDir string) (string, interface{}, error) {
	sessionID := fmt.Sprintf("test-session-%d", time.Now().Unix())
	session := &sessiontypes.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: workspaceDir,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
	t.sessions[sessionID] = session
	return sessionID, session, nil
}

func (t *testSessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	return t.GetSession(sessionID)
}

func (t *testSessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	if session, exists := t.sessions[sessionID]; exists {
		return session, nil
	}
	return t.GetSession(sessionID) // Will create one
}

func (t *testSessionManager) GetOrCreateSessionFromRepo(repoURL string) (interface{}, error) {
	// Simple implementation - just create a new session
	_, session, err := t.CreateSession(fmt.Sprintf("/workspace/repo-%d", time.Now().Unix()))
	return session, err
}

func (t *testSessionManager) UpdateSession(sessionID string, updateFunc func(*sessiontypes.SessionState)) error {
	if session, exists := t.sessions[sessionID]; exists {
		updateFunc(session)
		return nil
	}
	return fmt.Errorf("session not found: %s", sessionID)
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

func (t *testSessionManager) Cleanup(olderThan time.Duration) error {
	return nil
}

func (t *testSessionManager) FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error) {
	// Simple implementation for testing
	for _, session := range t.sessions {
		if session.RepoURL == repoURL {
			return session, nil
		}
	}
	return nil, fmt.Errorf("session not found for repo: %s", repoURL)
}