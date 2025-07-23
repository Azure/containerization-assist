package integration

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/progress"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectProgressIntegration tests the direct progress implementation in a workflow
func TestDirectProgressIntegration(t *testing.T) {
	// Create logger
	logger := slog.Default()

	// Create test progress factory
	testFactory := workflow.NewTestDirectProgressFactory()

	// Create mock step provider with a simple test step
	mockProvider := &MockStepProvider{
		analyzeStep: &MockStep{
			name: "analyze",
			executeFunc: func(ctx context.Context, state *workflow.WorkflowState) error {
				// Emit some progress during execution
				_ = state.ProgressEmitter.Emit(ctx, "processing", 50, "Processing repository")
				return nil
			},
		},
	}

	// Create step factory with mock provider
	stepFactory := workflow.NewStepFactory(mockProvider, nil, nil, logger)

	// Create orchestrator with direct progress factory
	orchestrator := workflow.NewBaseOrchestrator(
		stepFactory,
		testFactory,
		logger,
		workflow.WithMiddleware(workflow.ProgressMiddleware(workflow.SimpleProgress)),
	)

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

	// Verify update content - check for analyze step since that's what we provided
	foundStart := false
	foundComplete := false
	foundProcessing := false
	for _, update := range updates {
		if update.Stage == "analyze" && update.Message == "Starting analyze" {
			foundStart = true
		}
		if update.Stage == "analyze" && strings.HasPrefix(update.Message, "Completed analyze") {
			foundComplete = true
		}
		if update.Stage == "processing" && update.Message == "Processing repository" {
			foundProcessing = true
		}
	}
	assert.True(t, foundStart, "Should have start update for analyze step")
	assert.True(t, foundComplete, "Should have complete update for analyze step")
	assert.True(t, foundProcessing, "Should have custom processing update")
}

// TestDirectProgressFactoryWithMCPServer tests the factory creates MCP emitter when server is present
func TestDirectProgressFactoryWithMCPServer(t *testing.T) {
	logger := slog.Default()
	factory := progress.NewDirectProgressFactory(logger)

	// Test without server - should get CLI emitter
	ctx := context.Background()
	emitter := factory.CreateEmitter(ctx, nil, 10)
	_, isCLI := emitter.(*progress.CLIDirectEmitter)
	assert.True(t, isCLI, "Should get CLI emitter without server")

	// Note: Testing with actual MCP server would require more setup
	// This is covered in the direct_integration_test.go
}

// MockStep implements workflow.Step for testing
type MockStep struct {
	name        string
	executeFunc func(ctx context.Context, state *workflow.WorkflowState) error
	maxRetries  int
}

func (m *MockStep) Name() string { return m.name }
func (m *MockStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, state)
	}
	return nil
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
	return &MockStep{name: "analyze"}
}

func (m *MockStepProvider) GetDockerfileStep() workflow.Step {
	if m.dockerfileStep != nil {
		return m.dockerfileStep
	}
	return &MockStep{name: "dockerfile"}
}

func (m *MockStepProvider) GetBuildStep() workflow.Step {
	if m.buildStep != nil {
		return m.buildStep
	}
	return &MockStep{name: "build"}
}

func (m *MockStepProvider) GetScanStep() workflow.Step {
	if m.scanStep != nil {
		return m.scanStep
	}
	return &MockStep{name: "scan"}
}

func (m *MockStepProvider) GetTagStep() workflow.Step {
	if m.tagStep != nil {
		return m.tagStep
	}
	return &MockStep{name: "tag"}
}

func (m *MockStepProvider) GetPushStep() workflow.Step {
	if m.pushStep != nil {
		return m.pushStep
	}
	return &MockStep{name: "push"}
}

func (m *MockStepProvider) GetManifestStep() workflow.Step {
	if m.manifestStep != nil {
		return m.manifestStep
	}
	return &MockStep{name: "manifest"}
}

func (m *MockStepProvider) GetClusterStep() workflow.Step {
	if m.clusterStep != nil {
		return m.clusterStep
	}
	return &MockStep{name: "cluster"}
}

func (m *MockStepProvider) GetDeployStep() workflow.Step {
	if m.deployStep != nil {
		return m.deployStep
	}
	return &MockStep{name: "deploy"}
}

func (m *MockStepProvider) GetVerifyStep() workflow.Step {
	if m.verifyStep != nil {
		return m.verifyStep
	}
	return &MockStep{name: "verify"}
}
