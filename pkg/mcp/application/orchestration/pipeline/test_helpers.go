package pipeline

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// TestOperations creates a properly initialized Operations instance for testing
func TestOperations(t *testing.T) *Operations {
	// Create temporary workspace for tests
	tempDir := filepath.Join(os.TempDir(), "pipeline-test-"+t.Name())
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Initialize SessionManager with memory store
	config := session.SessionManagerConfig{
		WorkspaceDir: tempDir,
		MaxSessions:  10,
		Logger:       slog.Default(),
	}

	sessionManager, err := session.NewSessionManager(config)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Create mock Docker client
	dockerClient := &mockDockerClient{}

	// Create MCP clients
	clients := &mcptypes.MCPClients{
		Docker: dockerClient,
	}

	// Create logger
	logger := slog.Default()

	return NewOperations(sessionManager, clients, logger)
}

// mockDockerClient provides a minimal mock implementation for testing
type mockDockerClient struct{}

func (m *mockDockerClient) Version(_ context.Context) (string, error) {
	return "Docker version 20.10.0", nil
}

func (m *mockDockerClient) Info(_ context.Context) (string, error) {
	return "Docker info mock", nil
}

func (m *mockDockerClient) Build(_ context.Context, _, imageTag, _ string) (string, error) {
	// Mock successful build
	return "Successfully built " + imageTag, nil
}

func (m *mockDockerClient) Push(_ context.Context, imageTag string) (string, error) {
	// Mock successful push
	return "Successfully pushed " + imageTag, nil
}

func (m *mockDockerClient) Pull(_ context.Context, imageRef string) (string, error) {
	// Mock successful pull
	return "Successfully pulled " + imageRef, nil
}

func (m *mockDockerClient) Tag(_ context.Context, sourceRef, targetRef string) (string, error) {
	// Mock successful tag
	return "Successfully tagged " + sourceRef + " as " + targetRef, nil
}

func (m *mockDockerClient) Login(_ context.Context, _, _, _ string) (string, error) {
	return "Login succeeded", nil
}

func (m *mockDockerClient) LoginWithToken(_ context.Context, _, _ string) (string, error) {
	return "Login succeeded", nil
}

func (m *mockDockerClient) Logout(_ context.Context, _ string) (string, error) {
	return "Logout succeeded", nil
}

func (m *mockDockerClient) IsLoggedIn(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// createBenchmarkOperations creates an Operations instance suitable for benchmarking
func createBenchmarkOperations(b *testing.B) *Operations {
	// Create temporary workspace for benchmarks
	tempDir := filepath.Join(os.TempDir(), "pipeline-bench-"+b.Name())
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	b.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Initialize SessionManager with memory store
	config := session.SessionManagerConfig{
		WorkspaceDir: tempDir,
		MaxSessions:  10,
		Logger:       slog.Default(), // Default logger for benchmarks
	}

	sessionManager, err := session.NewSessionManager(config)
	if err != nil {
		b.Fatalf("Failed to create session manager: %v", err)
	}

	// Create mock Docker client
	dockerClient := &mockDockerClient{}

	// Create MCP clients
	clients := &mcptypes.MCPClients{
		Docker: dockerClient,
	}

	// Create logger
	logger := slog.Default()

	return NewOperations(sessionManager, clients, logger)
}
