package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/analyze"
	"github.com/Azure/container-copilot/pkg/mcp/internal/core"
	orchestrationtestutil "github.com/Azure/container-copilot/pkg/mcp/internal/orchestration/testutil"
	profilingtestutil "github.com/Azure/container-copilot/pkg/mcp/internal/profiling/testutil"
	"github.com/rs/zerolog"
)

// IntegrationTestSuite provides a comprehensive test suite for integration testing
type IntegrationTestSuite struct {
	t                   *testing.T
	logger              zerolog.Logger
	sessionManager      *TestSessionManager
	mockPipelineAdapter *MockPipelineAdapter
	mockClients         *mcptypes.MCPClients
	orchestratorCapture *orchestrationtestutil.ExecutionCapture
	profilingTestSuite  *profilingtestutil.ProfiledTestSuite
	testStartTime       time.Time
	cleanupFunctions    []func()
	mu                  sync.RWMutex
}

// NewIntegrationTestSuite creates a comprehensive integration test suite
func NewIntegrationTestSuite(t *testing.T, logger zerolog.Logger) *IntegrationTestSuite {
	testLogger := logger.With().
		Str("test", t.Name()).
		Str("component", "integration_test_suite").
		Logger()

	return &IntegrationTestSuite{
		t:                   t,
		logger:              testLogger,
		sessionManager:      NewTestSessionManager(testLogger),
		mockPipelineAdapter: NewMockPipelineAdapter(testLogger),
		mockClients:         NewTestClientSets(),
		orchestratorCapture: orchestrationtestutil.NewExecutionCapture(testLogger),
		profilingTestSuite:  profilingtestutil.NewProfiledTestSuite(t, testLogger),
		testStartTime:       time.Now(),
		cleanupFunctions:    make([]func(), 0),
	}
}

// GetSessionManager returns the test session manager
func (its *IntegrationTestSuite) GetSessionManager() *TestSessionManager {
	return its.sessionManager
}

// GetPipelineAdapter returns the mock pipeline adapter
func (its *IntegrationTestSuite) GetPipelineAdapter() *MockPipelineAdapter {
	return its.mockPipelineAdapter
}

// GetClients returns the test client sets
func (its *IntegrationTestSuite) GetClients() *mcptypes.MCPClients {
	return its.mockClients
}

// GetExecutionCapture returns the orchestrator execution capture
func (its *IntegrationTestSuite) GetExecutionCapture() *orchestrationtestutil.ExecutionCapture {
	return its.orchestratorCapture
}

// GetProfilingTestSuite returns the profiling test suite
func (its *IntegrationTestSuite) GetProfilingTestSuite() *profilingtestutil.ProfiledTestSuite {
	return its.profilingTestSuite
}

// CreateTestOrchestrator creates a test orchestrator with all necessary dependencies
func (its *IntegrationTestSuite) CreateTestOrchestrator() *orchestrationtestutil.MockToolOrchestrator {
	// For integration testing, use a mock orchestrator instead of the real one
	// This avoids complex dependency setup and focuses on integration logic
	mockOrchestrator := orchestrationtestutil.NewMockToolOrchestrator()

	// Configure mock with realistic behavior
	mockOrchestrator.ExecuteFunc = func(ctx context.Context, toolName string, args interface{}, session interface{}) (interface{}, error) {
		// Delegate to the mock pipeline adapter for realistic responses
		switch toolName {
		case "analyze_repository_atomic":
			if argsMap, ok := args.(map[string]interface{}); ok {
				if repoPath, exists := argsMap["repository_path"]; exists {
					return its.mockPipelineAdapter.AnalyzeRepository("test-session", repoPath.(string))
				}
			}
		case "build_image_atomic":
			if argsMap, ok := args.(map[string]interface{}); ok {
				if imageName, exists := argsMap["image_name"]; exists {
					return its.mockPipelineAdapter.BuildDockerImage("test-session", imageName.(string), "/tmp/Dockerfile")
				}
			}
		}

		// Default mock response
		return map[string]interface{}{
			"tool":     toolName,
			"success":  true,
			"mock":     true,
			"executed": true,
		}, nil
	}

	// Add cleanup for orchestrator
	its.AddCleanup(func() {
		mockOrchestrator.Clear()
	})

	return mockOrchestrator
}

// CreateProfiledOrchestrator creates a profiled orchestrator for performance testing
func (its *IntegrationTestSuite) CreateProfiledOrchestrator() *profilingtestutil.MockProfiler {
	// For integration testing, use a mock profiler
	mockProfiler := profilingtestutil.NewMockProfiler()

	// Add cleanup for profiled orchestrator
	its.AddCleanup(func() {
		// Log mock profiling results
		its.logger.Info().
			Int("total_executions", len(mockProfiler.GetExecutionsForTool(""))).
			Msg("Mock profiling completed for test")
	})

	return mockProfiler
}

// SetupFullWorkflow configures the test suite for end-to-end workflow testing
func (its *IntegrationTestSuite) SetupFullWorkflow() *WorkflowTestContext {
	// Create all necessary components
	orchestrator := its.CreateTestOrchestrator()
	profiler := its.CreateProfiledOrchestrator()

	// Setup workflow context
	context := &WorkflowTestContext{
		suite:             its,
		orchestrator:      orchestrator,
		profiler:          profiler,
		sessionID:         generateTestSessionID(),
		workflowStartTime: time.Now(),
	}

	// Create a test session
	its.sessionManager.CreateTestSession(context.sessionID, map[string]interface{}{
		"workflow_test": true,
		"created_at":    context.workflowStartTime,
	})

	// Add cleanup for workflow
	its.AddCleanup(func() {
		// Cleanup test session - simplified for mock
	})

	return context
}

// AddCleanup adds a cleanup function to be called at test end
func (its *IntegrationTestSuite) AddCleanup(cleanup func()) {
	its.mu.Lock()
	defer its.mu.Unlock()
	its.cleanupFunctions = append(its.cleanupFunctions, cleanup)
}

// Cleanup runs all registered cleanup functions
func (its *IntegrationTestSuite) Cleanup() {
	its.mu.RLock()
	cleanupFuncs := make([]func(), len(its.cleanupFunctions))
	copy(cleanupFuncs, its.cleanupFunctions)
	its.mu.RUnlock()

	// Run cleanup functions in reverse order
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					its.logger.Error().
						Interface("panic", r).
						Msg("Panic during test cleanup")
				}
			}()
			cleanupFuncs[i]()
		}()
	}
}

// WorkflowTestContext provides context for end-to-end workflow testing
type WorkflowTestContext struct {
	suite             *IntegrationTestSuite
	orchestrator      *orchestrationtestutil.MockToolOrchestrator
	profiler          *profilingtestutil.MockProfiler
	sessionID         string
	workflowStartTime time.Time
	currentStage      string
}

// ExecuteTool executes a tool through the orchestrator with capture
func (ctx *WorkflowTestContext) ExecuteTool(toolName string, args interface{}) (interface{}, error) {
	ctx.currentStage = toolName

	return ctx.suite.orchestratorCapture.CaptureExecution(
		context.Background(),
		toolName,
		args,
		ctx.sessionID,
		func() (interface{}, error) {
			return ctx.orchestrator.ExecuteTool(context.Background(), toolName, args, ctx.sessionID)
		},
	)
}

// ExecuteToolWithProfiling executes a tool with profiling enabled
func (ctx *WorkflowTestContext) ExecuteToolWithProfiling(toolName string, args interface{}) (interface{}, error) {
	ctx.currentStage = toolName

	// Use the mock profiler to profile the execution
	return ctx.profiler.ProfileExecution(toolName, ctx.sessionID, func(context.Context) (interface{}, error) {
		return ctx.orchestrator.ExecuteTool(context.Background(), toolName, args, ctx.sessionID)
	})
}

// BenchmarkTool runs a benchmark for a specific tool
func (ctx *WorkflowTestContext) BenchmarkTool(toolName string, args interface{}, iterations int) profilingtestutil.MockBenchmark {
	ctx.currentStage = "benchmark_" + toolName

	// Run mock benchmark
	return ctx.profiler.RunBenchmark(toolName, iterations, 1, func(context.Context) (interface{}, error) {
		return ctx.orchestrator.ExecuteTool(context.Background(), toolName, args, ctx.sessionID)
	})
}

// GetSessionID returns the test session ID
func (ctx *WorkflowTestContext) GetSessionID() string {
	return ctx.sessionID
}

// GetCurrentStage returns the current workflow stage
func (ctx *WorkflowTestContext) GetCurrentStage() string {
	return ctx.currentStage
}

// GetWorkflowDuration returns the total workflow duration so far
func (ctx *WorkflowTestContext) GetWorkflowDuration() time.Duration {
	return time.Since(ctx.workflowStartTime)
}

// TestSessionManager provides a pre-configured session manager for tests
type TestSessionManager struct {
	logger       zerolog.Logger
	testSessions map[string]map[string]interface{}
	mu           sync.RWMutex
}

// NewTestSessionManager creates a new test session manager
func NewTestSessionManager(logger zerolog.Logger) *TestSessionManager {
	return &TestSessionManager{
		logger:       logger.With().Str("component", "test_session_manager").Logger(),
		testSessions: make(map[string]map[string]interface{}),
	}
}

// CreateTestSession creates a session specifically for testing
func (tsm *TestSessionManager) CreateTestSession(sessionID string, metadata map[string]interface{}) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	// Store test-specific metadata
	tsm.testSessions[sessionID] = metadata

	tsm.logger.Info().Str("session_id", sessionID).Msg("Created test session")
}

// GetTestSessionMetadata retrieves test-specific metadata for a session
func (tsm *TestSessionManager) GetTestSessionMetadata(sessionID string) (map[string]interface{}, bool) {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	metadata, exists := tsm.testSessions[sessionID]
	return metadata, exists
}

// DeleteSession deletes a session and its test metadata
func (tsm *TestSessionManager) DeleteSession(sessionID string) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	// Delete test metadata
	delete(tsm.testSessions, sessionID)

	tsm.logger.Info().Str("session_id", sessionID).Msg("Deleted test session")
	return nil
}

// MockPipelineAdapter provides a controllable adapter mock with predictable behavior
type MockPipelineAdapter struct {
	mu                    sync.RWMutex
	logger                zerolog.Logger
	analyzeRepositoryFunc func(sessionID, repoPath string) (interface{}, error)
	buildImageFunc        func(sessionID, imageName, dockerfilePath string) (interface{}, error)
	generateManifestsFunc func(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (interface{}, error)
	operations            []MockOperation
}

// MockOperation represents a captured operation
type MockOperation struct {
	Operation string
	SessionID string
	Args      []interface{}
	Result    interface{}
	Error     error
	Timestamp time.Time
}

// NewMockPipelineAdapter creates a new mock pipeline adapter
func NewMockPipelineAdapter(logger zerolog.Logger) *MockPipelineAdapter {
	return &MockPipelineAdapter{
		logger:     logger.With().Str("component", "mock_pipeline_adapter").Logger(),
		operations: make([]MockOperation, 0),
	}
}

// AnalyzeRepository mocks repository analysis
func (mpa *MockPipelineAdapter) AnalyzeRepository(sessionID, repoPath string) (interface{}, error) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()

	var result interface{}
	var err error

	if mpa.analyzeRepositoryFunc != nil {
		result, err = mpa.analyzeRepositoryFunc(sessionID, repoPath)
	} else {
		// Default mock result
		result = map[string]interface{}{
			"language":      "go",
			"framework":     "standard",
			"port":          8080,
			"dependencies":  []string{"github.com/rs/zerolog"},
			"analysis_time": time.Now(),
		}
	}

	// Record operation
	operation := MockOperation{
		Operation: "AnalyzeRepository",
		SessionID: sessionID,
		Args:      []interface{}{repoPath},
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
	mpa.operations = append(mpa.operations, operation)

	return result, err
}

// BuildDockerImage mocks Docker image building
func (mpa *MockPipelineAdapter) BuildDockerImage(sessionID, imageName, dockerfilePath string) (interface{}, error) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()

	var result interface{}
	var err error

	if mpa.buildImageFunc != nil {
		result, err = mpa.buildImageFunc(sessionID, imageName, dockerfilePath)
	} else {
		// Default mock result
		result = map[string]interface{}{
			"image_id":   "sha256:abc123def456",
			"image_name": imageName,
			"build_time": time.Now(),
			"size_bytes": 104857600, // 100MB
			"layers":     []string{"layer1", "layer2", "layer3"},
		}
	}

	// Record operation
	operation := MockOperation{
		Operation: "BuildDockerImage",
		SessionID: sessionID,
		Args:      []interface{}{imageName, dockerfilePath},
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
	mpa.operations = append(mpa.operations, operation)

	return result, err
}

// GenerateKubernetesManifests mocks Kubernetes manifest generation
func (mpa *MockPipelineAdapter) GenerateKubernetesManifests(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (interface{}, error) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()

	var result interface{}
	var err error

	if mpa.generateManifestsFunc != nil {
		result, err = mpa.generateManifestsFunc(sessionID, imageName, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	} else {
		// Default mock result
		result = map[string]interface{}{
			"manifests": []map[string]interface{}{
				{
					"kind":     "Deployment",
					"name":     appName + "-deployment",
					"replicas": 1,
					"image":    imageName,
					"port":     port,
				},
				{
					"kind": "Service",
					"name": appName + "-service",
					"port": port,
				},
			},
			"generation_time": time.Now(),
		}
	}

	// Record operation
	operation := MockOperation{
		Operation: "GenerateKubernetesManifests",
		SessionID: sessionID,
		Args:      []interface{}{imageName, appName, port, cpuRequest, memoryRequest, cpuLimit, memoryLimit},
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
	mpa.operations = append(mpa.operations, operation)

	return result, err
}

// SetAnalyzeRepositoryFunc sets a custom function for repository analysis
func (mpa *MockPipelineAdapter) SetAnalyzeRepositoryFunc(fn func(sessionID, repoPath string) (interface{}, error)) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()
	mpa.analyzeRepositoryFunc = fn
}

// SetBuildImageFunc sets a custom function for image building
func (mpa *MockPipelineAdapter) SetBuildImageFunc(fn func(sessionID, imageName, dockerfilePath string) (interface{}, error)) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()
	mpa.buildImageFunc = fn
}

// SetGenerateManifestsFunc sets a custom function for manifest generation
func (mpa *MockPipelineAdapter) SetGenerateManifestsFunc(fn func(sessionID, imageName, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (interface{}, error)) {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()
	mpa.generateManifestsFunc = fn
}

// GetOperations returns all recorded operations
func (mpa *MockPipelineAdapter) GetOperations() []MockOperation {
	mpa.mu.RLock()
	defer mpa.mu.RUnlock()

	operations := make([]MockOperation, len(mpa.operations))
	copy(operations, mpa.operations)
	return operations
}

// GetOperationsForSession returns operations for a specific session
func (mpa *MockPipelineAdapter) GetOperationsForSession(sessionID string) []MockOperation {
	mpa.mu.RLock()
	defer mpa.mu.RUnlock()

	var sessionOperations []MockOperation
	for _, op := range mpa.operations {
		if op.SessionID == sessionID {
			sessionOperations = append(sessionOperations, op)
		}
	}
	return sessionOperations
}

// Clear resets the mock adapter state
func (mpa *MockPipelineAdapter) Clear() {
	mpa.mu.Lock()
	defer mpa.mu.Unlock()
	mpa.operations = make([]MockOperation, 0)
}

// NewTestClientSets creates pre-configured client mocks for testing
func NewTestClientSets() *mcptypes.MCPClients {
	// Create mock clients with test implementations
	return &mcptypes.MCPClients{
		Docker:   nil,                        // Mock docker client can be injected as needed
		Kind:     nil,                        // Mock kind runner can be injected as needed
		Kube:     nil,                        // Mock kube runner can be injected as needed
		Analyzer: analyzer.NewStubAnalyzer(), // Use stub analyzer for testing
	}
}

// EndToEndTestHelpers provides utilities for full workflow testing
type EndToEndTestHelpers struct {
	suite *IntegrationTestSuite
}

// NewEndToEndTestHelpers creates new end-to-end test helpers
func NewEndToEndTestHelpers(suite *IntegrationTestSuite) *EndToEndTestHelpers {
	return &EndToEndTestHelpers{suite: suite}
}

// RunFullContainerizationWorkflow runs a complete containerization workflow test
func (e2e *EndToEndTestHelpers) RunFullContainerizationWorkflow(repoPath, imageName string) (*WorkflowResult, error) {
	ctx := e2e.suite.SetupFullWorkflow()

	startTime := time.Now()
	result := &WorkflowResult{
		SessionID: ctx.GetSessionID(),
		StartTime: startTime,
		Stages:    make([]WorkflowStage, 0),
	}

	// Stage 1: Repository Analysis
	stageStart := time.Now()
	analysisResult, err := ctx.ExecuteToolWithProfiling("analyze_repository_atomic", map[string]interface{}{
		"session_id":      ctx.GetSessionID(),
		"repository_path": repoPath,
	})
	if err != nil {
		return result, err
	}

	result.Stages = append(result.Stages, WorkflowStage{
		Name:      "repository_analysis",
		StartTime: stageStart,
		EndTime:   time.Now(),
		Result:    analysisResult,
		Success:   true,
	})

	// Stage 2: Dockerfile Generation
	stageStart = time.Now()
	dockerfileResult, err := ctx.ExecuteToolWithProfiling("generate_dockerfile_atomic", map[string]interface{}{
		"session_id": ctx.GetSessionID(),
	})
	if err != nil {
		return result, err
	}

	result.Stages = append(result.Stages, WorkflowStage{
		Name:      "dockerfile_generation",
		StartTime: stageStart,
		EndTime:   time.Now(),
		Result:    dockerfileResult,
		Success:   true,
	})

	// Stage 3: Image Build
	stageStart = time.Now()
	buildResult, err := ctx.ExecuteToolWithProfiling("build_image_atomic", map[string]interface{}{
		"session_id": ctx.GetSessionID(),
		"image_name": imageName,
		"dockerfile": "/tmp/Dockerfile",
		"build_args": map[string]string{},
	})
	if err != nil {
		return result, err
	}

	result.Stages = append(result.Stages, WorkflowStage{
		Name:      "image_build",
		StartTime: stageStart,
		EndTime:   time.Now(),
		Result:    buildResult,
		Success:   true,
	})

	// Stage 4: Manifest Generation
	stageStart = time.Now()
	manifestResult, err := ctx.ExecuteToolWithProfiling("generate_manifests_atomic", map[string]interface{}{
		"session_id": ctx.GetSessionID(),
		"image_name": imageName,
		"app_name":   "test-app",
		"port":       8080,
	})
	if err != nil {
		return result, err
	}

	result.Stages = append(result.Stages, WorkflowStage{
		Name:      "manifest_generation",
		StartTime: stageStart,
		EndTime:   time.Now(),
		Result:    manifestResult,
		Success:   true,
	})

	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)
	result.Success = true

	return result, nil
}

// WorkflowResult represents the result of a complete workflow test
type WorkflowResult struct {
	SessionID     string
	StartTime     time.Time
	EndTime       time.Time
	TotalDuration time.Duration
	Success       bool
	Stages        []WorkflowStage
	Error         error
}

// WorkflowStage represents a single stage in the workflow
type WorkflowStage struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Result    interface{}
	Success   bool
	Error     error
}

// Utility functions

func generateTestSessionID() string {
	return "test-session-" + time.Now().Format("20060102-150405")
}

// getNoReflectDispatcher extracts the no-reflect dispatcher (helper function)
func getNoReflectDispatcher(orchestrator interface{}) interface{} {
	// This would need to be implemented based on the actual orchestrator structure
	// For now, return nil to indicate mock usage
	return nil
}
