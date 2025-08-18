package integration_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	progresstest "github.com/Azure/containerization-assist/pkg/mcp/infrastructure/core/testutil/progress"
	"github.com/Azure/containerization-assist/pkg/mcp/infrastructure/messaging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectProgressIntegration tests the direct progress implementation in a workflow
func TestDirectProgressIntegration(t *testing.T) {
	// Create logger
	logger := slog.Default()

	// Create test progress factory
	testFactory := progresstest.NewTestDirectProgressFactory()

	// Create mock step provider with a simple test step
	mockProvider := &MockStepProvider{
		analyzeStep: &MockStep{
			name: "analyze",
			executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
				// Emit some progress during execution
				_ = state.ProgressEmitter.Emit(ctx, "processing", 50, "Processing repository")
				return &workflow.StepResult{Success: true}, nil
			},
		},
	}

	// Create orchestrator with mock provider and progress factory
	progressFactory := func(ctx context.Context, req *mcp.CallToolRequest) api.ProgressEmitter {
		return testFactory.CreateEmitter(ctx, req, 10)
	}
	orchestrator, err := workflow.NewOrchestrator(
		mockProvider,
		logger,
		progressFactory,
	)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Execute workflow
	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	args := &workflow.ContainerizeAndDeployArgs{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
	}

	result, err := orchestrator.Execute(ctx, req, args)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check progress updates
	emitter := testFactory.GetTestEmitter()
	updates := emitter.GetUpdates()

	// Debug: Print all updates
	t.Logf("Total updates: %d", len(updates))
	for i, update := range updates {
		t.Logf("Update %d: stage=%s, percent=%d, message=%s, status=%s",
			i, update.Stage, update.Percentage, update.Message, update.Status)
	}

	// Should have at least start and complete updates
	assert.GreaterOrEqual(t, len(updates), 2)

	// Check that Close was called
	assert.True(t, emitter.IsClosed())

	// Verify update content - DAG orchestrator uses different message format
	foundAnalyzeComplete := false
	foundProcessing := false
	foundCompleted := false
	for _, update := range updates {
		if update.Stage == "analyze" && update.Message == "Step analyze completed" {
			foundAnalyzeComplete = true
		}
		if update.Stage == "processing" && update.Message == "Processing repository" {
			foundProcessing = true
		}
		if update.Stage == "completed" && update.Message == "Workflow completed" {
			foundCompleted = true
		}
	}
	assert.True(t, foundAnalyzeComplete, "Should have completion update for analyze step")
	assert.True(t, foundProcessing, "Should have custom processing update")
	assert.True(t, foundCompleted, "Should have workflow completed update")
}

// TestDirectProgressFactoryWithMCPServer tests the factory creates MCP emitter when server is present
func TestDirectProgressFactoryWithMCPServer(t *testing.T) {
	logger := slog.Default()
	// Test without server - should get CLI emitter
	ctx := context.Background()
	emitter := messaging.CreateProgressEmitter(ctx, nil, 10, logger)
	_, isCLI := emitter.(*messaging.CLIDirectEmitter)
	assert.True(t, isCLI, "Should get CLI emitter without server")

	// Note: Testing with actual MCP server would require more setup
	// This is covered in the direct_integration_test.go
}

// MockStep implements workflow.Step for testing
type MockStep struct {
	name        string
	executeFunc func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error)
	maxRetries  int
}

func (m *MockStep) Name() string { return m.name }
func (m *MockStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, state)
	}
	return &workflow.StepResult{Success: true}, nil
}
func (m *MockStep) MaxRetries() int { return m.maxRetries }

// MockStepProvider implements workflow.StepProvider for testing
type MockStepProvider struct {
	analyzeStep    workflow.Step
	dockerfileStep workflow.Step
	buildStep      workflow.Step
	scanStep       workflow.Step
	tagStep        workflow.Step
	pushStep       workflow.Step
	manifestStep   workflow.Step
	clusterStep    workflow.Step
	deployStep     workflow.Step
	verifyStep     workflow.Step
}

func (m *MockStepProvider) GetAnalyzeStep() workflow.Step {
	if m.analyzeStep != nil {
		return m.analyzeStep
	}
	return &MockStep{name: "analyze", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetDockerfileStep() workflow.Step {
	if m.dockerfileStep != nil {
		return m.dockerfileStep
	}
	return &MockStep{name: "dockerfile", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetBuildStep() workflow.Step {
	if m.buildStep != nil {
		return m.buildStep
	}
	return &MockStep{name: "build", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetScanStep() workflow.Step {
	if m.scanStep != nil {
		return m.scanStep
	}
	return &MockStep{name: "scan", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetTagStep() workflow.Step {
	if m.tagStep != nil {
		return m.tagStep
	}
	return &MockStep{name: "tag", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetPushStep() workflow.Step {
	if m.pushStep != nil {
		return m.pushStep
	}
	return &MockStep{name: "push", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetManifestStep() workflow.Step {
	if m.manifestStep != nil {
		return m.manifestStep
	}
	return &MockStep{name: "manifest", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetClusterStep() workflow.Step {
	if m.clusterStep != nil {
		return m.clusterStep
	}
	return &MockStep{name: "cluster", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetDeployStep() workflow.Step {
	if m.deployStep != nil {
		return m.deployStep
	}
	return &MockStep{name: "deploy", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

func (m *MockStepProvider) GetVerifyStep() workflow.Step {
	if m.verifyStep != nil {
		return m.verifyStep
	}
	return &MockStep{name: "verify", executeFunc: func(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
		return &workflow.StepResult{Success: true}, nil
	}}
}

// GetStep implements workflow.StepProvider interface
func (m *MockStepProvider) GetStep(name string) (workflow.Step, error) {
	stepMap := map[string]workflow.Step{
		workflow.StepAnalyzeRepository:  m.GetAnalyzeStep(),
		workflow.StepGenerateDockerfile: m.GetDockerfileStep(),
		workflow.StepBuildImage:         m.GetBuildStep(),
		workflow.StepSecurityScan:       m.GetScanStep(),
		workflow.StepTagImage:           m.GetTagStep(),
		workflow.StepPushImage:          m.GetPushStep(),
		workflow.StepGenerateManifests:  m.GetManifestStep(),
		workflow.StepSetupCluster:       m.GetClusterStep(),
		workflow.StepDeployApplication:  m.GetDeployStep(),
		workflow.StepVerifyDeployment:   m.GetVerifyStep(),
	}

	if step, exists := stepMap[name]; exists {
		return step, nil
	}
	return nil, fmt.Errorf("unknown step: %s", name)
}

// ListSteps implements workflow.StepProvider interface
func (m *MockStepProvider) ListSteps() []string {
	return []string{
		workflow.StepAnalyzeRepository,
		workflow.StepGenerateDockerfile,
		workflow.StepBuildImage,
		workflow.StepSecurityScan,
		workflow.StepTagImage,
		workflow.StepPushImage,
		workflow.StepGenerateManifests,
		workflow.StepSetupCluster,
		workflow.StepDeployApplication,
		workflow.StepVerifyDeployment,
	}
}
