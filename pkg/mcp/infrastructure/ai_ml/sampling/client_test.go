package sampling

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/sampling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	client := NewClient(logger)
	require.NotNil(t, client)

	// Verify client implements the domain interface
	var _ sampling.UnifiedSampler = client
}

func TestNewClientFromEnv(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Set environment variables
	os.Setenv("SAMPLING_MAX_TOKENS", "1024")
	os.Setenv("SAMPLING_TEMPERATURE", "0.5")
	os.Setenv("SAMPLING_RETRY_ATTEMPTS", "2")
	defer func() {
		os.Unsetenv("SAMPLING_MAX_TOKENS")
		os.Unsetenv("SAMPLING_TEMPERATURE")
		os.Unsetenv("SAMPLING_RETRY_ATTEMPTS")
	}()

	client, err := NewClientFromEnv(logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify client implements the domain interface
	var _ sampling.UnifiedSampler = client
}

func TestNewClientFromEnv_InvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Set invalid environment variable
	os.Setenv("SAMPLING_TEMPERATURE", "5.0") // Invalid: > 2.0
	defer os.Unsetenv("SAMPLING_TEMPERATURE")

	_, err := NewClientFromEnv(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestClient_Sample_NoMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	req := sampling.Request{
		Prompt: "Test prompt",
	}

	_, err := client.Sample(ctx, req)
	assert.Error(t, err)
	// Without proper MCP server context, sampling should fail
}

func TestCreateDomainClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	client := CreateDomainClient(logger)
	require.NotNil(t, client)

	// Verify client implements the domain interface
	var _ sampling.UnifiedSampler = client
}

func TestClient_AnalyzeDockerfile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	content := `FROM node:16
WORKDIR /app
COPY package.json .
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`

	// Without proper MCP server context, this should fail gracefully
	_, err := client.AnalyzeDockerfile(ctx, content)
	assert.Error(t, err)
}

func TestClient_AnalyzeKubernetesManifest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	content := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:latest`

	// Without proper MCP server context, this should fail gracefully
	_, err := client.AnalyzeKubernetesManifest(ctx, content)
	assert.Error(t, err)
}

func TestClient_AnalyzeSecurityScan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	scanResults := "Vulnerabilities found:\n- CVE-2023-1234: HIGH severity in nginx package"

	// Without proper MCP server context, this should fail gracefully
	_, err := client.AnalyzeSecurityScan(ctx, scanResults)
	assert.Error(t, err)
}

func TestClient_FixDockerfile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	content := "FROM ubuntu\nRUN apt-get update"
	buildErrors := []string{"Package not found: some-package"}

	// Without proper MCP server context, this should fail gracefully
	_, err := client.FixDockerfile(ctx, content, buildErrors)
	assert.Error(t, err)
}

func TestClient_FixKubernetesManifest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewClient(logger)

	ctx := context.Background()
	content := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`
	deploymentErrors := []string{"missing spec"}

	// Without proper MCP server context, this should fail gracefully
	_, err := client.FixKubernetesManifest(ctx, content, deploymentErrors)
	assert.Error(t, err)
}
