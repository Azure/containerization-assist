package fixing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/analyzer"
	"github.com/Azure/container-copilot/pkg/mcp/internal/fixing"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAnalyzer provides a test implementation of the Analyzer interface
type MockAnalyzer struct {
	responses map[string]string
	calls     []string
}

func NewMockAnalyzer() *MockAnalyzer {
	return &MockAnalyzer{
		responses: make(map[string]string),
		calls:     make([]string, 0),
	}
}

func (m *MockAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	m.calls = append(m.calls, prompt)
	if response, exists := m.responses[prompt]; exists {
		return response, nil
	}
	return "Mock analysis response", nil
}

func (m *MockAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	m.calls = append(m.calls, prompt)

	// Return a mock fix strategy response
	if len(m.responses) > 0 {
		for _, response := range m.responses {
			return response, nil
		}
	}

	return `STRATEGY 1:
Name: Fix Dockerfile base image
Description: Update the base image to a valid one
Priority: 1
Type: dockerfile
Commands: 
FileChanges: Dockerfile:update:fix base image
Validation: Verify base image is pullable
EstimatedTime: 2 minutes

<FIXED_CONTENT>
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
</FIXED_CONTENT>`, nil
}

func (m *MockAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	m.calls = append(m.calls, promptTemplate)
	return "Mock formatted analysis response", nil
}

func (m *MockAnalyzer) GetTokenUsage() analyzer.TokenUsage {
	return analyzer.TokenUsage{}
}

func (m *MockAnalyzer) ResetTokenUsage() {
	// No-op for mock
}

func (m *MockAnalyzer) SetResponse(prompt, response string) {
	m.responses[prompt] = response
}

func TestIterativeFixerBasicFlow(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
	mockAnalyzer := NewMockAnalyzer()

	fixer := fixing.NewDefaultIterativeFixer(mockAnalyzer, logger)

	ctx := context.Background()
	fixingCtx := &fixing.FixingContext{
		SessionID:     "test-session",
		ToolName:      "atomic_build_image",
		OperationType: "build",
		OriginalError: assert.AnError,
		MaxAttempts:   2,
		BaseDir:       "/tmp/test",
	}

	result, err := fixer.AttemptFix(ctx, fixingCtx)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.TotalAttempts)
	assert.Greater(t, len(mockAnalyzer.calls), 0)
}

func TestContextSharer(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
	sharer := fixing.NewDefaultContextSharer(logger)

	ctx := context.Background()
	sessionID := "test-session"

	// Test sharing context
	testData := map[string]interface{}{
		"tool":      "build_image",
		"operation": "docker_build",
		"error":     "Build failed",
	}

	err := sharer.ShareContext(ctx, sessionID, "failure_context", testData)
	assert.NoError(t, err)

	// Test retrieving context
	retrieved, err := sharer.GetSharedContext(ctx, sessionID, "failure_context")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	retrievedMap := retrieved.(map[string]interface{})
	assert.Equal(t, "build_image", retrievedMap["tool"])
	assert.Equal(t, "docker_build", retrievedMap["operation"])
}

func TestBuildImageWithFixes(t *testing.T) {
	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "fixing_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test Dockerfile with an error
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	dockerfileContent := `FROM nonexistent:latest
WORKDIR /app
COPY . .
CMD ["echo", "hello"]`

	err = os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0600)
	require.NoError(t, err)

	// Setup mock analyzer
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
	mockAnalyzer := NewMockAnalyzer()

	buildTool := fixing.NewBuildImageWithFixes(mockAnalyzer, logger)

	ctx := context.Background()

	// Test the build with fixes
	// The AI fixing system should successfully fix the Dockerfile and complete the build
	err = buildTool.ExecuteWithFixes(ctx, "test-session", "test-image", dockerfilePath, tempDir)

	// The build should succeed after fixing
	assert.NoError(t, err)                        // Expected to succeed after AI fixes the Dockerfile
	assert.Greater(t, len(mockAnalyzer.calls), 0) // Should have called the analyzer

	// Check that backup was created (if the fix application ran)
	backupPath := dockerfilePath + ".backup"
	if _, err := os.Stat(backupPath); err == nil {
		// Backup exists, which means fix was applied
		t.Log("Backup created, indicating fix was applied")
	}
}

func TestAtomicToolFixingMixin(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
	mockAnalyzer := NewMockAnalyzer()

	mixin := fixing.NewAtomicToolFixingMixin(mockAnalyzer, "test_tool", logger)

	// Create a mock operation that fails once then succeeds
	attempts := 0
	operation := &MockOperation{
		executeFunc: func(ctx context.Context) error {
			attempts++
			if attempts == 1 {
				return assert.AnError
			}
			return nil
		},
	}

	ctx := context.Background()
	err := mixin.ExecuteWithRetry(ctx, "test-session", "/tmp", operation)

	// Should succeed on second attempt
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestFixingConfiguration(t *testing.T) {
	config := fixing.GetEnhancedConfiguration("atomic_build_image")

	assert.Equal(t, "atomic_build_image", config.ToolName)
	assert.Equal(t, 3, config.MaxAttempts)
	assert.True(t, config.EnableRouting)
	assert.Equal(t, "Medium", config.SeverityThreshold)
	assert.Contains(t, config.SpecializedPrompts, "dockerfile_analysis")

	// Test default configuration
	defaultConfig := fixing.GetEnhancedConfiguration("unknown_tool")
	assert.Equal(t, "unknown_tool", defaultConfig.ToolName)
	assert.Equal(t, 2, defaultConfig.MaxAttempts)
	assert.False(t, defaultConfig.EnableRouting)
}

// MockOperation implements mcptypes.FixableOperation for testing
type MockOperation struct {
	executeFunc         func(ctx context.Context) error
	failureAnalysisFunc func(ctx context.Context, err error) (*types.RichError, error)
	prepareRetryFunc    func(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error
}

func (m *MockOperation) ExecuteOnce(ctx context.Context) error {
	if m.executeFunc != nil {
		return m.executeFunc(ctx)
	}
	return nil
}

func (m *MockOperation) GetFailureAnalysis(ctx context.Context, err error) (*types.RichError, error) {
	if m.failureAnalysisFunc != nil {
		return m.failureAnalysisFunc(ctx, err)
	}

	return &types.RichError{
		Code:     "TEST_ERROR",
		Type:     "test_error",
		Severity: "Medium",
		Message:  err.Error(),
	}, nil
}

func (m *MockOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if m.prepareRetryFunc != nil {
		return m.prepareRetryFunc(ctx, fixAttempt)
	}
	return nil
}

func TestFixAttemptSerialization(t *testing.T) {
	fixAttempt := mcptypes.FixAttempt{
		AttemptNumber: 1,
		StartTime:     time.Now(),
		EndTime:       time.Now().Add(time.Minute),
		Duration:      time.Minute,
		mcptypes.FixStrategy: *mcptypes.FixStrategy{
			Name:        "Test Fix",
			Description: "A test fix strategy",
			Priority:    1,
			Type:        "dockerfile",
		},
		Success:        true,
		FixedContent:   "FROM node:18-alpine\nWORKDIR /app",
		AnalysisPrompt: "Analyze this error",
		AnalysisResult: "Error caused by invalid base image",
	}

	// Verify all fields are set correctly
	assert.Equal(t, 1, fixAttempt.AttemptNumber)
	assert.Equal(t, "Test Fix", fixAttempt.FixStrategy.Name)
	assert.True(t, fixAttempt.Success)
	assert.Equal(t, time.Minute, fixAttempt.Duration)
}

func TestFixStrategyValidation(t *testing.T) {
	strategy := mcptypes.FixStrategy{
		Name:          "Fix Base Image",
		Description:   "Update the Docker base image to a valid one",
		Priority:      1,
		Type:          "dockerfile",
		Commands:      []string{"docker pull node:18-alpine"},
		Validation:    "Verify image can be pulled",
		EstimatedTime: "2 minutes",
	}

	assert.Equal(t, "Fix Base Image", strategy.Name)
	assert.Equal(t, 1, strategy.Priority)
	assert.Equal(t, "dockerfile", strategy.Type)
	assert.Contains(t, strategy.Commands, "docker pull node:18-alpine")
}
