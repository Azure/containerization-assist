package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// DAGMockStep implements the Step interface for testing
type DAGMockStep struct {
	mock.Mock
}

func (m *DAGMockStep) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *DAGMockStep) Execute(ctx context.Context, state *WorkflowState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *DAGMockStep) MaxRetries() int {
	args := m.Called()
	return args.Int(0)
}

// DAGMockStepProvider implements StepProvider for testing
type DAGMockStepProvider struct {
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
func (p *DAGMockStepProvider) GetStep(name string) (Step, error) {
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
func (p *DAGMockStepProvider) ListSteps() []string {
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

		orchestrator, err := NewOrchestrator(stepProvider, nil, logger)
		require.NoError(t, err)
		assert.NotNil(t, orchestrator)
		assert.NotNil(t, orchestrator.dag)
		assert.Equal(t, logger, orchestrator.logger)
	})
}

func TestOrchestratorExecute(t *testing.T) {
	t.Run("executes workflow successfully", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up expectations for all steps
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*DAGMockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)
		}

		orchestrator, err := NewOrchestrator(stepProvider, nil, logger)
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
			mockStep := step.(*DAGMockStep)
			mockStep.AssertExpectations(t)
		}
	})

	t.Run("handles step failure", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up analyze and dockerfile to succeed
		analyzeStep, _ := stepProvider.GetStep("analyze_repository")
		analyzeStepMock := analyzeStep.(*DAGMockStep)
		analyzeStepMock.On("Execute", mock.Anything, mock.Anything).Return(nil)

		dockerfileStep, _ := stepProvider.GetStep("generate_dockerfile")
		dockerfileStepMock := dockerfileStep.(*DAGMockStep)
		dockerfileStepMock.On("Execute", mock.Anything, mock.Anything).Return(nil)

		// Set up build step to fail
		buildStep, _ := stepProvider.GetStep("build_image")
		buildStepMock := buildStep.(*DAGMockStep)
		buildStepMock.On("Execute", mock.Anything, mock.Anything).Return(errors.New("build failed"))

		orchestrator, err := NewOrchestrator(stepProvider, nil, logger)
		require.NoError(t, err)

		ctx := context.Background()
		req := &mcp.CallToolRequest{}
		args := &ContainerizeAndDeployArgs{
			RepoPath: "/test/repo",
		}

		result, err := orchestrator.Execute(ctx, req, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Step 'build' failed")
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "Workflow failed")

		// Verify only expected steps were called
		analyzeStepMock.AssertExpectations(t)
		dockerfileStepMock.AssertExpectations(t)
		buildStepMock.AssertExpectations(t)

		// Verify subsequent steps were not called
		scanStep, _ := stepProvider.GetStep("security_scan")
		scanStepMock := scanStep.(*DAGMockStep)
		scanStepMock.AssertNotCalled(t, "Execute")
	})

	t.Run("uses existing workflow ID from context", func(t *testing.T) {
		stepProvider := createMockStepProvider()
		logger := slog.Default()

		// Set up all steps to succeed
		for _, step := range getAllSteps(stepProvider) {
			mockStep := step.(*DAGMockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)
		}

		orchestrator, err := NewOrchestrator(stepProvider, nil, logger)
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
			mockStep := step.(*DAGMockStep)
			mockStep.On("Execute", mock.Anything, mock.Anything).Return(nil)
		}

		// Create mock emitter factory
		mockEmitter := &NoOpEmitter{}
		emitterFactory := &mockEmitterFactory{emitter: mockEmitter}

		orchestrator, err := NewOrchestrator(stepProvider, emitterFactory, logger)
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

func TestBuildContainerizationDAG(t *testing.T) {
	t.Run("creates correct workflow DAG structure", func(t *testing.T) {
		stepProvider := createMockStepProvider()

		dag, err := buildContainerizationDAG(stepProvider)
		require.NoError(t, err)
		assert.NotNil(t, dag)

		// Verify all steps are present
		assert.Len(t, dag.steps, 10)
		assert.NotNil(t, dag.steps["analyze"])
		assert.NotNil(t, dag.steps["dockerfile"])
		assert.NotNil(t, dag.steps["build"])
		assert.NotNil(t, dag.steps["scan"])
		assert.NotNil(t, dag.steps["tag"])
		assert.NotNil(t, dag.steps["push"])
		assert.NotNil(t, dag.steps["manifest"])
		assert.NotNil(t, dag.steps["cluster"])
		assert.NotNil(t, dag.steps["deploy"])
		assert.NotNil(t, dag.steps["verify"])

		// Verify dependencies
		sorted, err := dag.TopologicalSort()
		require.NoError(t, err)
		assert.Equal(t, []string{
			"analyze", "dockerfile", "build", "scan", "tag",
			"push", "manifest", "cluster", "deploy", "verify",
		}, sorted)

	})
}

// mockEmitterFactory implements ProgressEmitterFactory for testing
type mockEmitterFactory struct {
	emitter api.ProgressEmitter
}

func (f *mockEmitterFactory) CreateEmitter(ctx context.Context, req *mcp.CallToolRequest, totalSteps int) api.ProgressEmitter {
	return f.emitter
}

// Helper functions

func createMockStepProvider() *DAGMockStepProvider {
	provider := &DAGMockStepProvider{}

	// Create mock steps with names
	steps := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}

	for _, name := range steps {
		step := new(DAGMockStep)
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

func getAllSteps(provider *DAGMockStepProvider) []Step {
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
