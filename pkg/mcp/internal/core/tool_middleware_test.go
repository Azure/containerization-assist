package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTool for testing
type MockTool struct {
	mock.Mock
	metadata *mcp.ToolMetadata
}

func (m *MockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	called := m.Called(ctx, args)
	return called.Get(0), called.Error(1)
}

func (m *MockTool) GetMetadata() (*mcp.ToolMetadata, error) {
	if m.metadata != nil {
		return m.metadata, nil
	}
	return &mcp.ToolMetadata{Name: "mock_tool", Version: "1.0.0"}, nil
}

func (m *MockTool) Validate(args interface{}) error {
	called := m.Called(args)
	return called.Error(0)
}

// MockArgs for testing
type MockArgs struct {
	SessionID string `json:"session_id"`
	DryRun    bool   `json:"dry_run"`
}

func (m MockArgs) GetSessionID() string {
	return m.SessionID
}

func (m MockArgs) IsDryRun() bool {
	return m.DryRun
}

// MockValidationService for testing
type MockValidationService struct {
	mock.Mock
}

// MockErrorService for testing
type MockErrorService struct {
	mock.Mock
}

func (m *MockErrorService) HandleError(ctx context.Context, err error, errCtx ErrorContext) error {
	called := m.Called(ctx, err, errCtx)
	return called.Error(0)
}

// MockTelemetryService for testing
type MockTelemetryService struct {
	mock.Mock
}

func (m *MockTelemetryService) TrackToolExecution(ctx context.Context, execution ToolExecution) {
	m.Called(ctx, execution)
}

func (m *MockTelemetryService) CreatePerformanceTracker(tool, operation string) *MockPerformanceTracker {
	return &MockPerformanceTracker{}
}

// MockPerformanceTracker for testing
type MockPerformanceTracker struct {
	startTime time.Time
}

func (m *MockPerformanceTracker) Start() {
	m.startTime = time.Now()
}

func (m *MockPerformanceTracker) Finish() time.Duration {
	return time.Since(m.startTime)
}

func (m *MockPerformanceTracker) Record(metric string, value interface{}, unit string) {
	// Mock implementation
}

func setupTestMiddleware(t *testing.T) (*ToolMiddleware, *MockValidationService, *MockErrorService, *MockTelemetryService) {
	mockValidation := &MockValidationService{}
	mockError := &MockErrorService{}
	mockTelemetry := &MockTelemetryService{}

	logger := zerolog.New(nil).With().Timestamp().Logger()

	// Create a real telemetry service for testing
	telemetryService := NewTelemetryService(logger)

	middleware := NewToolMiddleware(
		nil,              // validation service
		nil,              // error service
		telemetryService, // real telemetry service
		logger,
	)

	return middleware, mockValidation, mockError, mockTelemetry
}

func TestNewToolMiddleware(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	assert.NotNil(t, middleware)
	assert.Equal(t, 0, len(middleware.middlewares))
}

func TestToolMiddleware_Use(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	// Create a mock middleware
	mockMiddleware := &LoggingMiddleware{logger: zerolog.New(nil)}

	middleware.Use(mockMiddleware)

	assert.Equal(t, 1, len(middleware.middlewares))
}

func TestToolMiddleware_ExecuteWithMiddleware_Success(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	// Set up mock tool
	mockTool := &MockTool{}
	mockTool.On("Execute", mock.Anything, mock.Anything).Return("test result", nil)

	ctx := context.Background()
	args := MockArgs{SessionID: "test-session"}

	result, err := middleware.ExecuteWithMiddleware(ctx, mockTool, args)

	assert.NoError(t, err)
	assert.Equal(t, "test result", result)
	mockTool.AssertExpectations(t)
}

func TestToolMiddleware_ExecuteWithMiddleware_Failure(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	// Set up mock tool to return error
	mockTool := &MockTool{}
	testError := errors.New("test error")
	mockTool.On("Execute", mock.Anything, mock.Anything).Return(nil, testError)

	ctx := context.Background()
	args := MockArgs{SessionID: "test-session"}

	result, err := middleware.ExecuteWithMiddleware(ctx, mockTool, args)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "test error")
	mockTool.AssertExpectations(t)
}

func TestToolMiddleware_ExecuteWithMiddleware_InvalidTool(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	// Use a tool that doesn't implement the Tool interface
	invalidTool := "not a tool"

	ctx := context.Background()
	args := MockArgs{}

	result, err := middleware.ExecuteWithMiddleware(ctx, invalidTool, args)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not implement Tool interface")
}

func TestValidationMiddleware_Success(t *testing.T) {
	logger := zerolog.New(nil)
	validationMiddleware := NewValidationMiddleware(nil, logger)

	// Create mock tool with validation
	mockTool := &MockTool{}
	mockTool.On("Validate", mock.Anything).Return(nil)

	// Create execution context
	ctx := &ExecutionContext{
		Context: context.Background(),
		Tool:    mockTool,
		Args:    MockArgs{},
	}

	// Mock next handler
	nextCalled := false
	next := func(ctx *ExecutionContext) (interface{}, error) {
		nextCalled = true
		return "success", nil
	}

	handler := validationMiddleware.Wrap(next)
	result, err := handler(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.True(t, nextCalled)
	mockTool.AssertExpectations(t)
}

func TestValidationMiddleware_ValidationFailure(t *testing.T) {
	logger := zerolog.New(nil)
	validationMiddleware := NewValidationMiddleware(nil, logger)

	// Create mock tool with validation that fails
	mockTool := &MockTool{}
	validationError := errors.New("validation failed")
	mockTool.On("Validate", mock.Anything).Return(validationError)

	// Create execution context
	ctx := &ExecutionContext{
		Context: context.Background(),
		Tool:    mockTool,
		Args:    MockArgs{},
	}

	// Mock next handler
	nextCalled := false
	next := func(ctx *ExecutionContext) (interface{}, error) {
		nextCalled = true
		return "success", nil
	}

	handler := validationMiddleware.Wrap(next)
	result, err := handler(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, validationError, err)
	assert.False(t, nextCalled)
	mockTool.AssertExpectations(t)
}

func TestValidationMiddleware_NoValidation(t *testing.T) {
	logger := zerolog.New(nil)
	validationMiddleware := NewValidationMiddleware(nil, logger)

	// Create a simple tool that doesn't implement ToolWithValidation
	simpleTool := struct {
		name string
	}{name: "simple_tool"}

	// Create execution context
	ctx := &ExecutionContext{
		Context: context.Background(),
		Tool:    simpleTool,
		Args:    MockArgs{},
	}

	// Mock next handler
	nextCalled := false
	next := func(ctx *ExecutionContext) (interface{}, error) {
		nextCalled = true
		return "success", nil
	}

	handler := validationMiddleware.Wrap(next)
	result, err := handler(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.True(t, nextCalled)
}

func TestLoggingMiddleware(t *testing.T) {
	logger := zerolog.New(nil)
	loggingMiddleware := NewLoggingMiddleware(logger)

	mockTool := &MockTool{}

	// Create execution context
	ctx := &ExecutionContext{
		Context:   context.Background(),
		Tool:      mockTool,
		StartTime: time.Now(),
	}

	// Test successful execution
	t.Run("Success", func(t *testing.T) {
		nextCalled := false
		next := func(ctx *ExecutionContext) (interface{}, error) {
			nextCalled = true
			return "success", nil
		}

		handler := loggingMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		assert.True(t, nextCalled)
	})

	// Test failed execution
	t.Run("Failure", func(t *testing.T) {
		nextCalled := false
		testError := errors.New("test error")
		next := func(ctx *ExecutionContext) (interface{}, error) {
			nextCalled = true
			return nil, testError
		}

		handler := loggingMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, testError, err)
		assert.True(t, nextCalled)
	})
}

func TestErrorHandlingMiddleware(t *testing.T) {
	logger := zerolog.New(nil)
	errorMiddleware := NewErrorHandlingMiddleware(nil, logger)

	mockTool := &MockTool{}

	// Test successful execution (no error handling needed)
	t.Run("Success", func(t *testing.T) {
		ctx := &ExecutionContext{
			Context: context.Background(),
			Tool:    mockTool,
			Metadata: map[string]interface{}{
				"session_id": "test-session",
			},
		}

		next := func(ctx *ExecutionContext) (interface{}, error) {
			return "success", nil
		}

		handler := errorMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	})

	// Test error handling (with nil service, should pass through original error)
	t.Run("Error", func(t *testing.T) {
		ctx := &ExecutionContext{
			Context: context.Background(),
			Tool:    mockTool,
			Metadata: map[string]interface{}{
				"session_id": "test-session",
			},
		}

		originalError := errors.New("original error")

		next := func(ctx *ExecutionContext) (interface{}, error) {
			return nil, originalError
		}

		handler := errorMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		// Since we're using nil service, should pass through original error or handle gracefully
		assert.NotNil(t, err)
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	logger := zerolog.New(nil)
	recoveryMiddleware := NewRecoveryMiddleware(logger)

	mockTool := &MockTool{}

	// Create execution context
	ctx := &ExecutionContext{
		Context: context.Background(),
		Tool:    mockTool,
	}

	// Test normal execution
	t.Run("Normal", func(t *testing.T) {
		next := func(ctx *ExecutionContext) (interface{}, error) {
			return "success", nil
		}

		handler := recoveryMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	})

	// Test panic recovery
	t.Run("Panic", func(t *testing.T) {
		next := func(ctx *ExecutionContext) (interface{}, error) {
			panic("test panic")
		}

		handler := recoveryMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "tool execution panicked")
		assert.Contains(t, err.Error(), "test panic")
	})
}

func TestContextMiddleware(t *testing.T) {
	logger := zerolog.New(nil)
	contextMiddleware := NewContextMiddleware(logger)

	mockTool := &MockTool{
		metadata: &mcp.ToolMetadata{
			Name:    "test_tool",
			Version: "1.2.3",
		},
	}

	// Create execution context with args that implement metadata interfaces
	ctx := &ExecutionContext{
		Context:  context.Background(),
		Tool:     mockTool,
		Args:     MockArgs{SessionID: "test-session", DryRun: true},
		Metadata: make(map[string]interface{}),
	}

	next := func(ctx *ExecutionContext) (interface{}, error) {
		return "success", nil
	}

	handler := contextMiddleware.Wrap(next)
	result, err := handler(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Check that metadata was extracted
	assert.Equal(t, "test-session", ctx.Metadata["session_id"])
	assert.Equal(t, true, ctx.Metadata["dry_run"])
	assert.Equal(t, "test_tool", ctx.Metadata["tool_name"])
	assert.Equal(t, "1.2.3", ctx.Metadata["tool_version"])
}

func TestTimeoutMiddleware(t *testing.T) {
	logger := zerolog.New(nil)

	// Test successful execution within timeout
	t.Run("Success", func(t *testing.T) {
		timeoutMiddleware := NewTimeoutMiddleware(100*time.Millisecond, logger)

		ctx := &ExecutionContext{
			Context: context.Background(),
			Tool:    &MockTool{},
		}

		next := func(ctx *ExecutionContext) (interface{}, error) {
			// Fast execution
			time.Sleep(10 * time.Millisecond)
			return "success", nil
		}

		handler := timeoutMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	})

	// Test timeout
	t.Run("Timeout", func(t *testing.T) {
		timeoutMiddleware := NewTimeoutMiddleware(50*time.Millisecond, logger)

		ctx := &ExecutionContext{
			Context: context.Background(),
			Tool:    &MockTool{},
		}

		next := func(ctx *ExecutionContext) (interface{}, error) {
			// Slow execution that should timeout
			time.Sleep(100 * time.Millisecond)
			return "success", nil
		}

		handler := timeoutMiddleware.Wrap(next)
		result, err := handler(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "timed out")
	})
}

func TestStandardMiddlewareChain(t *testing.T) {
	logger := zerolog.New(nil)

	// Create mock services
	validationService := &build.ValidationService{}
	errorService := &ErrorService{}
	telemetryService := &TelemetryService{}

	middleware := StandardMiddlewareChain(validationService, errorService, telemetryService, logger)

	assert.NotNil(t, middleware)
	// Should have 7 middleware: Recovery, Context, Timeout, Logging, Validation, ErrorHandling, Metrics
	assert.Equal(t, 7, len(middleware.middlewares))
}

func TestGetToolName(t *testing.T) {
	// Test with tool that has metadata
	t.Run("WithMetadata", func(t *testing.T) {
		mockTool := &MockTool{
			metadata: &mcp.ToolMetadata{Name: "test_tool"},
		}

		name := getToolName(mockTool)
		assert.Equal(t, "test_tool", name)
	})

	// Test with tool that doesn't have metadata
	t.Run("WithoutMetadata", func(t *testing.T) {
		simpleTool := struct{}{}

		name := getToolName(simpleTool)
		assert.Equal(t, "unknown", name)
	})
}

func TestGetToolMetadata(t *testing.T) {
	// Test with tool that has metadata
	t.Run("WithMetadata", func(t *testing.T) {
		expectedMetadata := &mcp.ToolMetadata{
			Name:    "test_tool",
			Version: "1.0.0",
		}
		mockTool := &MockTool{metadata: expectedMetadata}

		metadata := getToolMetadata(mockTool)
		assert.Equal(t, expectedMetadata, metadata)
	})

	// Test with tool that doesn't have metadata
	t.Run("WithoutMetadata", func(t *testing.T) {
		simpleTool := struct{}{}

		metadata := getToolMetadata(simpleTool)
		assert.Equal(t, "unknown", metadata.Name)
	})
}

func TestMiddlewareChaining(t *testing.T) {
	middleware, _, _, _ := setupTestMiddleware(t)

	// Add multiple middleware
	logger := zerolog.New(nil)
	middleware.Use(NewLoggingMiddleware(logger))
	middleware.Use(NewRecoveryMiddleware(logger))
	middleware.Use(NewContextMiddleware(logger))

	mockTool := &MockTool{}
	mockTool.On("Execute", mock.Anything, mock.Anything).Return("chained result", nil)

	ctx := context.Background()
	args := MockArgs{SessionID: "test-session"}

	result, err := middleware.ExecuteWithMiddleware(ctx, mockTool, args)

	assert.NoError(t, err)
	assert.Equal(t, "chained result", result)
	mockTool.AssertExpectations(t)
}
