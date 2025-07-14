package testutil

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// FixtureBuilder provides a fluent interface for building test fixtures
type FixtureBuilder struct {
	// Default values that can be overridden
	workflowID string
	repoURL    string
	branch     string
	timeout    time.Duration
}

// NewFixtureBuilder creates a new fixture builder with sensible defaults
func NewFixtureBuilder() *FixtureBuilder {
	return &FixtureBuilder{
		workflowID: "test-workflow-123",
		repoURL:    "https://github.com/test/repo",
		branch:     "main",
		timeout:    30 * time.Second,
	}
}

// WithWorkflowID sets the workflow ID
func (fb *FixtureBuilder) WithWorkflowID(id string) *FixtureBuilder {
	fb.workflowID = id
	return fb
}

// WithRepoURL sets the repository URL
func (fb *FixtureBuilder) WithRepoURL(url string) *FixtureBuilder {
	fb.repoURL = url
	return fb
}

// WithBranch sets the branch
func (fb *FixtureBuilder) WithBranch(branch string) *FixtureBuilder {
	fb.branch = branch
	return fb
}

// WithTimeout sets the timeout
func (fb *FixtureBuilder) WithTimeout(timeout time.Duration) *FixtureBuilder {
	fb.timeout = timeout
	return fb
}

// BuildContext creates a test context with common values
// Note: The caller is responsible for calling the returned cancel function
func (fb *FixtureBuilder) BuildContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, workflow.WorkflowIDKey, fb.workflowID)
	ctx = context.WithValue(ctx, workflow.TraceIDKey, "test-trace-"+fb.workflowID)
	ctx, cancel := context.WithTimeout(ctx, fb.timeout)
	return ctx, cancel
}

// BuildArgs creates test ContainerizeAndDeployArgs
func (fb *FixtureBuilder) BuildArgs() *workflow.ContainerizeAndDeployArgs {
	deployTrue := true
	return &workflow.ContainerizeAndDeployArgs{
		RepoURL:  fb.repoURL,
		Branch:   fb.branch,
		Scan:     true,
		Deploy:   &deployTrue,
		TestMode: true,
	}
}

// BuildWorkflowState creates a test WorkflowState
func (fb *FixtureBuilder) BuildWorkflowState() *workflow.WorkflowState {
	return &workflow.WorkflowState{
		WorkflowID:  fb.workflowID,
		Args:        fb.BuildArgs(),
		Result:      &workflow.ContainerizeAndDeployResult{},
		CurrentStep: 1,
		TotalSteps:  10,
	}
}

// BuildMCPRequest creates a test MCP CallToolRequest
// Note: The actual structure depends on the mcp-go library version
func (fb *FixtureBuilder) BuildMCPRequest() interface{} {
	// Return a generic request structure that can be adapted to the actual MCP request type
	return map[string]interface{}{
		"tool": "containerize_and_deploy",
		"arguments": map[string]interface{}{
			"repo_url":  fb.repoURL,
			"branch":    fb.branch,
			"test_mode": true,
		},
	}
}

// TestData provides common test data
type TestData struct {
	// Repository test data
	ValidRepoURL   string
	InvalidRepoURL string
	ValidBranch    string

	// Docker test data
	ValidImageRef   string
	InvalidImageRef string
	ValidRegistry   string

	// Kubernetes test data
	ValidNamespace   string
	InvalidNamespace string
	ValidDeployment  string

	// Common test values
	TestTimeout time.Duration
	TestUserID  string
}

// GetTestData returns a TestData instance with common test values
func GetTestData() TestData {
	return TestData{
		// Repository test data
		ValidRepoURL:   "https://github.com/test/repo",
		InvalidRepoURL: "not-a-url",
		ValidBranch:    "main",

		// Docker test data
		ValidImageRef:   "test/app:v1.0.0",
		InvalidImageRef: "invalid image ref",
		ValidRegistry:   "docker.io",

		// Kubernetes test data
		ValidNamespace:   "test-namespace",
		InvalidNamespace: "INVALID_NAMESPACE",
		ValidDeployment:  "test-deployment",

		// Common test values
		TestTimeout: 30 * time.Second,
		TestUserID:  "test-user-123",
	}
}

// AnalyzeResultFixture creates a test AnalyzeResult
func AnalyzeResultFixture() *workflow.AnalyzeResult {
	return &workflow.AnalyzeResult{
		Language:     "go",
		Framework:    "gin",
		Port:         8080,
		BuildCommand: "go build -o main .",
		StartCommand: "./main",
		Dependencies: []string{"gin-gonic/gin", "stretchr/testify"},
		RepoPath:     "/tmp/test-repo",
	}
}

// DockerfileResultFixture creates a test DockerfileResult
func DockerfileResultFixture() *workflow.DockerfileResult {
	return &workflow.DockerfileResult{
		Content: `FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]`,
		Path:      "Dockerfile",
		BaseImage: "golang:1.21-alpine",
	}
}

// BuildResultFixture creates a test BuildResult
func BuildResultFixture() *workflow.BuildResult {
	return &workflow.BuildResult{
		ImageID:   "sha256:1234567890abcdef",
		ImageRef:  "test/app:v1.0.0",
		BuildTime: "45s",
		ImageSize: 45 * 1024 * 1024, // 45MB in bytes
	}
}
