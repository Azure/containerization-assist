package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStep implements the Step interface for testing
type MockStep struct {
	mock.Mock
}

func (m *MockStep) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockStep) Execute(ctx context.Context, state *WorkflowState) (*StepResult, error) {
	args := m.Called(ctx, state)
	return args.Get(0).(*StepResult), args.Error(1)
}

func (m *MockStep) MaxRetries() int {
	args := m.Called()
	return args.Int(0)
}

// MockStepProvider implements StepProvider for testing
type MockStepProvider struct {
	analyzeStep    Step
	dockerfileStep Step
	buildStep      Step
	scanStep       Step
	tagStep        Step
	pushStep       Step
	manifestStep   Step
	clusterStep    Step
	deployStep     Step
	verifyStep     Step
}

// GetStep implements the consolidated StepProvider interface
func (p *MockStepProvider) GetStep(name string) (Step, error) {
	stepMap := map[string]Step{
		StepAnalyzeRepository:  p.analyzeStep,
		StepGenerateDockerfile: p.dockerfileStep,
		StepBuildImage:         p.buildStep,
		StepSecurityScan:       p.scanStep,
		StepTagImage:           p.tagStep,
		StepPushImage:          p.pushStep,
		StepGenerateManifests:  p.manifestStep,
		StepSetupCluster:       p.clusterStep,
		StepDeployApplication:  p.deployStep,
		StepVerifyDeployment:   p.verifyStep,
	}

	if step, exists := stepMap[name]; exists {
		return step, nil
	}
	return nil, fmt.Errorf("step %s not found", name)
}

// ListSteps returns all available step names
// mockProgressEmitter implements api.ProgressEmitter for testing
type mockProgressEmitter struct{}

func (m *mockProgressEmitter) Emit(ctx context.Context, stage string, percent int, message string) error {
	return nil
}

func (m *mockProgressEmitter) EmitDetailed(ctx context.Context, update api.ProgressUpdate) error {
	return nil
}

func (m *mockProgressEmitter) Close() error {
	return nil
}

func (p *MockStepProvider) ListSteps() []string {
	return []string{
		StepAnalyzeRepository,
		StepGenerateDockerfile,
		StepBuildImage,
		StepSecurityScan,
		StepTagImage,
		StepPushImage,
		StepGenerateManifests,
		StepSetupCluster,
		StepDeployApplication,
		StepVerifyDeployment,
	}
}

func TestNewOrchestrator(t *testing.T) {
	t.Run("creates orchestrator successfully", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		mockProgressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
			return &mockProgressEmitter{}
		}
		orchestrator, err := NewOrchestrator(stepProvider, logger, mockProgressFactory)
		require.NoError(t, err)
		assert.NotNil(t, orchestrator)
		assert.NotNil(t, orchestrator.steps)
		assert.Equal(t, logger, orchestrator.logger)
	})
}

func TestOrchestratorExecute(t *testing.T) {
	t.Run("executes workflow successfully", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up expectations for all steps
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*MockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(&StepResult{Success: true}, nil)
		}

		mockProgressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
			return &mockProgressEmitter{}
		}
		orchestrator, err := NewOrchestrator(stepProvider, logger, mockProgressFactory)
		require.NoError(t, err)

		ctx := context.Background()
		req := &mcp.CallToolRequest{}
		args := &ContainerizeAndDeployArgs{
			RepoPath: "/test/repo",
		}

		result, err := orchestrator.Execute(ctx, req, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Empty(t, result.Error)

		// Verify all steps were called
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*MockStep)
			mockStep.AssertExpectations(t)
		}
	})

	t.Run("handles step failure", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up analyze and dockerfile to succeed
		analyzeStep, _ := stepProvider.GetStep("analyze_repository")
		analyzeStepMock := analyzeStep.(*MockStep)
		analyzeStepMock.On("Execute", mock.Anything, mock.Anything).Return(&StepResult{Success: true}, nil)

		dockerfileStep, _ := stepProvider.GetStep("generate_dockerfile")
		dockerfileStepMock := dockerfileStep.(*MockStep)
		dockerfileStepMock.On("Execute", mock.Anything, mock.Anything).Return(&StepResult{Success: true}, nil)

		// Set up build step to fail
		buildStep, _ := stepProvider.GetStep("build_image")
		buildStepMock := buildStep.(*MockStep)
		buildStepMock.On("Execute", mock.Anything, mock.Anything).Return((*StepResult)(nil), errors.New("build failed"))

		mockProgressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
			return &mockProgressEmitter{}
		}
		orchestrator, err := NewOrchestrator(stepProvider, logger, mockProgressFactory)
		require.NoError(t, err)

		ctx := context.Background()
		req := &mcp.CallToolRequest{}
		args := &ContainerizeAndDeployArgs{
			RepoPath: "/test/repo",
		}

		result, err := orchestrator.Execute(ctx, req, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "step build failed")
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Workflow failed")

		// Verify only expected steps were called
		analyzeStepMock.AssertExpectations(t)
		dockerfileStepMock.AssertExpectations(t)
		buildStepMock.AssertExpectations(t)

		// Verify subsequent steps were not called
		scanStep, _ := stepProvider.GetStep("security_scan")
		scanStepMock := scanStep.(*MockStep)
		scanStepMock.AssertNotCalled(t, "Execute")
	})

	t.Run("uses existing workflow ID from context", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up all steps to succeed
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*MockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(&StepResult{Success: true}, nil)
		}

		mockProgressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
			return &mockProgressEmitter{}
		}
		orchestrator, err := NewOrchestrator(stepProvider, logger, mockProgressFactory)
		require.NoError(t, err)

		existingWorkflowID := "existing-workflow-123"
		ctx := WithWorkflowID(context.Background(), existingWorkflowID)
		req := &mcp.CallToolRequest{}
		args := &ContainerizeAndDeployArgs{
			RepoPath: "/test/repo",
		}

		result, err := orchestrator.Execute(ctx, req, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
	})

	t.Run("creates progress emitter when factory provided", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up all steps to succeed
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*MockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(&StepResult{Success: true}, nil)
		}

		mockProgressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
			return &mockProgressEmitter{}
		}
		orchestrator, err := NewOrchestrator(stepProvider, logger, mockProgressFactory)
		require.NoError(t, err)

		ctx := context.Background()
		req := &mcp.CallToolRequest{}
		args := &ContainerizeAndDeployArgs{
			RepoPath: "/test/repo",
		}

		result, err := orchestrator.Execute(ctx, req, args)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
	})
}

func TestBuildContainerizationSteps(t *testing.T) {
	t.Run("creates correct workflow step sequence", func(t *testing.T) {
		stepProvider := createMockStepProvider()

		steps, err := buildContainerizationSteps(stepProvider)
		require.NoError(t, err)
		assert.NotNil(t, steps)

		// Verify all steps are present in correct order
		assert.Len(t, steps, 10)
		assert.Equal(t, "analyze", steps[0].Name())
		assert.Equal(t, "dockerfile", steps[1].Name())
		assert.Equal(t, "build", steps[2].Name())
		assert.Equal(t, "scan", steps[3].Name())
		assert.Equal(t, "tag", steps[4].Name())
		assert.Equal(t, "push", steps[5].Name())
		assert.Equal(t, "manifest", steps[6].Name())
		assert.Equal(t, "cluster", steps[7].Name())
		assert.Equal(t, "deploy", steps[8].Name())
		assert.Equal(t, "verify", steps[9].Name())
	})
}

// Helper functions

func createMockStepProvider() *MockStepProvider {
	provider := &MockStepProvider{}

	// Create mock steps with names
	steps := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}

	for _, name := range steps {
		step := new(MockStep)
		step.On("Name").Return(name).Maybe()
		step.On("MaxRetries").Return(3).Maybe()

		switch name {
		case "analyze":
			provider.analyzeStep = step
		case "dockerfile":
			provider.dockerfileStep = step
		case "build":
			provider.buildStep = step
		case "scan":
			provider.scanStep = step
		case "tag":
			provider.tagStep = step
		case "push":
			provider.pushStep = step
		case "manifest":
			provider.manifestStep = step
		case "cluster":
			provider.clusterStep = step
		case "deploy":
			provider.deployStep = step
		case "verify":
			provider.verifyStep = step
		}
	}

	return provider
}

func getAllSteps(provider *MockStepProvider) []Step {
	return []Step{
		provider.analyzeStep,
		provider.dockerfileStep,
		provider.buildStep,
		provider.scanStep,
		provider.tagStep,
		provider.pushStep,
		provider.manifestStep,
		provider.clusterStep,
		provider.deployStep,
		provider.verifyStep,
	}
}
