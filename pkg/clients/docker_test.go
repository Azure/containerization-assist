package clients

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestBuildDockerfileContent_Success(t *testing.T) {
	mockDocker := &MockDockerClient{
		buildResult: "",
		buildErr:    nil,
		infoResult:  "Docker info",
		infoErr:     nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	dockerfileContent := "FROM alpine:latest\nRUN echo 'Hello World'"
	targetDir := "/tmp/test"
	registry := "localhost:5000"
	imageName := "test-image"

	buildErrors, err := clients.BuildDockerfileContent(ctx, dockerfileContent, targetDir, registry, imageName)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if buildErrors != "" {
		t.Logf("Build completed with warnings/errors: %s", buildErrors)
	}
}

func TestBuildDockerfileContent_BuildError(t *testing.T) {
	mockDocker := &MockDockerClient{
		buildResult: "build failed: invalid instruction",
		buildErr:    errors.New("docker build failed"),
		infoResult:  "Docker info",
		infoErr:     nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	dockerfileContent := "INVALID DOCKERFILE CONTENT"
	targetDir := "/tmp/test"
	registry := ""
	imageName := "test-image"

	buildErrors, err := clients.BuildDockerfileContent(ctx, dockerfileContent, targetDir, registry, imageName)

	if err == nil {
		t.Errorf("Expected error from docker build failure")
	}

	if !strings.Contains(err.Error(), "docker build failed") {
		t.Errorf("Expected error to contain 'docker build failed', got %v", err)
	}

	if buildErrors != "build failed: invalid instruction" {
		t.Errorf("Expected build errors to be returned even on failure")
	}
}

func TestBuildDockerfileContent_NoRegistry(t *testing.T) {
	mockDocker := &MockDockerClient{
		buildResult: "",
		buildErr:    nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	dockerfileContent := "FROM alpine:latest"
	targetDir := "/tmp/test"
	registry := "" // No registry
	imageName := "test-image"

	_, err := clients.BuildDockerfileContent(ctx, dockerfileContent, targetDir, registry, imageName)

	if err != nil {
		t.Errorf("Expected no error when registry is empty, got %v", err)
	}
}

func TestBuildDockerfileContent_TempFileHandling(t *testing.T) {
	// Test that temporary files are created and cleaned up properly
	mockDocker := &MockDockerClient{
		buildResult: "",
		buildErr:    nil,
	}

	// Create a custom mock that captures the dockerfile path
	var capturedDockerfilePath string
	customMockDocker := &MockDockerClientWithCapture{
		MockDockerClient:       *mockDocker,
		capturedDockerfilePath: &capturedDockerfilePath,
	}

	clients := &Clients{
		Docker: customMockDocker,
	}

	ctx := context.Background()
	dockerfileContent := "FROM alpine:latest\nLABEL test=true"

	_, err := clients.BuildDockerfileContent(ctx, dockerfileContent, "/tmp", "", "test")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that the dockerfile path was captured
	if capturedDockerfilePath == "" {
		t.Errorf("Expected dockerfile path to be captured")
	}

	// Verify the path structure
	if !strings.Contains(capturedDockerfilePath, "docker-build-") {
		t.Errorf("Expected temp directory to contain 'docker-build-', got %s", capturedDockerfilePath)
	}

	if !strings.HasSuffix(capturedDockerfilePath, "Dockerfile") {
		t.Errorf("Expected path to end with 'Dockerfile', got %s", capturedDockerfilePath)
	}
}

func TestCheckDockerRunning_Success(t *testing.T) {
	mockDocker := &MockDockerClient{
		infoResult: "Docker version info",
		infoErr:    nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	err := clients.checkDockerRunning(ctx)

	if err != nil {
		t.Errorf("Expected no error when Docker is running, got %v", err)
	}
}

func TestCheckDockerRunning_Error(t *testing.T) {
	mockDocker := &MockDockerClient{
		infoResult: "connection refused",
		infoErr:    errors.New("Cannot connect to the Docker daemon"),
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	err := clients.checkDockerRunning(ctx)

	if err == nil {
		t.Errorf("Expected error when Docker is not running")
	}

	expectedError := "docker daemon is not running"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestPushDockerImage_Success(t *testing.T) {
	mockDocker := &MockDockerClient{
		pushResult: "push completed successfully",
		pushErr:    nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	image := "localhost:5000/test-image:latest"

	err := clients.PushDockerImage(ctx, image)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestPushDockerImage_Error(t *testing.T) {
	mockDocker := &MockDockerClient{
		pushResult: "unauthorized: authentication required",
		pushErr:    errors.New("push failed"),
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	image := "unauthorized-registry.com/test-image:latest"

	err := clients.PushDockerImage(ctx, image)

	if err == nil {
		t.Errorf("Expected error when push fails")
	}

	expectedError := "error pushing to registry"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestBuildDockerfileContent_ImageTagGeneration(t *testing.T) {
	tests := []struct {
		name        string
		registry    string
		imageName   string
		expectedTag string
	}{
		{
			name:        "with registry",
			registry:    "localhost:5000",
			imageName:   "my-app",
			expectedTag: "localhost:5000/my-app:latest",
		},
		{
			name:        "without registry",
			registry:    "",
			imageName:   "my-app",
			expectedTag: "my-app:latest",
		},
		{
			name:        "with different registry",
			registry:    "myregistry.azurecr.io",
			imageName:   "web-service",
			expectedTag: "myregistry.azurecr.io/web-service:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedTag string
			mockDocker := &MockDockerClientWithTagCapture{
				capturedTag: &capturedTag,
			}

			clients := &Clients{
				Docker: mockDocker,
			}

			ctx := context.Background()
			_, _ = clients.BuildDockerfileContent(ctx, "FROM alpine", "/tmp", tt.registry, tt.imageName)

			if capturedTag != tt.expectedTag {
				t.Errorf("Expected tag '%s', got '%s'", tt.expectedTag, capturedTag)
			}
		})
	}
}

// Helper mocks for testing

type MockDockerClientWithCapture struct {
	MockDockerClient
	capturedDockerfilePath *string
}

func (m *MockDockerClientWithCapture) Build(ctx context.Context, dockerfilePath, tag, contextDir string) (string, error) {
	if m.capturedDockerfilePath != nil {
		*m.capturedDockerfilePath = dockerfilePath
	}
	return m.MockDockerClient.Build(ctx, dockerfilePath, tag, contextDir)
}

type MockDockerClientWithTagCapture struct {
	capturedTag *string
}

func (m *MockDockerClientWithTagCapture) Build(_ context.Context, _, tag, _ string) (string, error) {
	if m.capturedTag != nil {
		*m.capturedTag = tag
	}
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Version(_ context.Context) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Info(_ context.Context) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Push(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Pull(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Tag(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Login(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) LoginWithToken(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) Logout(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *MockDockerClientWithTagCapture) IsLoggedIn(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func TestBuildDockerfileContent_TempDirCleanup(t *testing.T) {
	// Test that temporary directories are properly cleaned up
	mockDocker := &MockDockerClient{
		buildResult: "",
		buildErr:    nil,
	}

	clients := &Clients{
		Docker: mockDocker,
	}

	ctx := context.Background()
	dockerfileContent := "FROM alpine:latest"

	// Count temp directories before
	tempDir := os.TempDir()
	beforeFiles, _ := os.ReadDir(tempDir)
	beforeCount := 0
	for _, file := range beforeFiles {
		if strings.HasPrefix(file.Name(), "docker-build-") {
			beforeCount++
		}
	}

	// Execute the function
	_, err := clients.BuildDockerfileContent(ctx, dockerfileContent, "/tmp", "", "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Count temp directories after
	afterFiles, _ := os.ReadDir(tempDir)
	afterCount := 0
	for _, file := range afterFiles {
		if strings.HasPrefix(file.Name(), "docker-build-") {
			afterCount++
		}
	}

	// Should be the same (cleanup happened)
	if afterCount > beforeCount {
		t.Errorf("Temporary directories not cleaned up properly. Before: %d, After: %d", beforeCount, afterCount)
	}
}

func TestPushDockerImage_NilClient(t *testing.T) {
	clients := &Clients{
		Docker: nil,
	}

	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic with nil Docker client")
		}
	}()

	_ = clients.PushDockerImage(ctx, "test:latest")
}
