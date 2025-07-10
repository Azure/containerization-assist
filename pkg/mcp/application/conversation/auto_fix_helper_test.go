package conversation

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MockTool implements api.Tool for testing
type MockTool struct {
	name           string
	executeFunc    func(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
	executeResults []MockExecuteResult
	executeCount   int
}

type MockExecuteResult struct {
	Output api.ToolOutput
	Error  error
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return "Mock tool for testing"
}

func (m *MockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: "Mock tool for testing",
	}
}

func (m *MockTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}

	if m.executeCount < len(m.executeResults) {
		result := m.executeResults[m.executeCount]
		m.executeCount++
		return result.Output, result.Error
	}

	return api.ToolOutput{}, errors.NewError().
		Code(errors.CodeNotImplemented).
		Message("mock tool execute not implemented").
		Build()
}

// MockSessionStore implements services.SessionStore for testing
type MockSessionStore struct {
	sessions map[string]*api.Session
}

func (m *MockSessionStore) Get(_ context.Context, sessionID string) (*api.Session, error) {
	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, errors.NewError().
		Code(errors.RESOURCE_NOT_FOUND).
		Message("session not found").
		Build()
}

func (m *MockSessionStore) Create(_ context.Context, session *api.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) Update(_ context.Context, session *api.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) Delete(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockSessionStore) List(_ context.Context) ([]*api.Session, error) {
	var sessions []*api.Session
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// MockSessionState implements services.SessionState for testing
type MockSessionState struct {
	states        map[string]map[string]interface{}
	checkpoints   map[string][]string
	workspaceDirs map[string]string
	metadata      map[string]map[string]interface{}
}

func (m *MockSessionState) SaveState(_ context.Context, sessionID string, state map[string]interface{}) error {
	if m.states == nil {
		m.states = make(map[string]map[string]interface{})
	}
	m.states[sessionID] = state
	return nil
}

func (m *MockSessionState) GetState(_ context.Context, sessionID string) (map[string]interface{}, error) {
	if state, exists := m.states[sessionID]; exists {
		return state, nil
	}
	return nil, errors.NewError().
		Code(errors.RESOURCE_NOT_FOUND).
		Message("state not found").
		Build()
}

func (m *MockSessionState) CreateCheckpoint(_ context.Context, sessionID string, name string) error {
	if m.checkpoints == nil {
		m.checkpoints = make(map[string][]string)
	}
	m.checkpoints[sessionID] = append(m.checkpoints[sessionID], name)
	return nil
}

func (m *MockSessionState) RestoreCheckpoint(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *MockSessionState) ListCheckpoints(_ context.Context, sessionID string) ([]string, error) {
	if checkpoints, exists := m.checkpoints[sessionID]; exists {
		return checkpoints, nil
	}
	return []string{}, nil
}

func (m *MockSessionState) GetWorkspaceDir(_ context.Context, sessionID string) (string, error) {
	if dir, exists := m.workspaceDirs[sessionID]; exists {
		return dir, nil
	}
	return "/tmp/test-workspace", nil
}

func (m *MockSessionState) SetWorkspaceDir(_ context.Context, sessionID string, dir string) error {
	if m.workspaceDirs == nil {
		m.workspaceDirs = make(map[string]string)
	}
	m.workspaceDirs[sessionID] = dir
	return nil
}

func (m *MockSessionState) GetSessionMetadata(_ context.Context, sessionID string) (map[string]interface{}, error) {
	if metadata, exists := m.metadata[sessionID]; exists {
		return metadata, nil
	}
	return map[string]interface{}{}, nil
}

func (m *MockSessionState) UpdateSessionData(_ context.Context, sessionID string, data map[string]interface{}) error {
	if m.metadata == nil {
		m.metadata = make(map[string]map[string]interface{})
	}
	m.metadata[sessionID] = data
	return nil
}

// MockFileAccessService implements services.FileAccessService for testing
type MockFileAccessService struct{}

func (m *MockFileAccessService) ReadFile(_ context.Context, _, _ string) (string, error) {
	return "", errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

func (m *MockFileAccessService) ListDirectory(_ context.Context, _, _ string) ([]services.FileInfo, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

func (m *MockFileAccessService) FileExists(_ context.Context, _, _ string) (bool, error) {
	return false, errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

func (m *MockFileAccessService) GetFileTree(_ context.Context, _, _ string) (string, error) {
	return "", errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

func (m *MockFileAccessService) ReadFileWithMetadata(_ context.Context, _, _ string) (*services.FileContent, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

func (m *MockFileAccessService) SearchFiles(_ context.Context, _, _ string) ([]services.FileInfo, error) {
	return nil, errors.NewError().Code(errors.CodeNotImplemented).Message("mock not implemented").Build()
}

// Test setup helper
func setupAutoFixHelper(_ testing.TB) *AutoFixHelper {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sessionStore := &MockSessionStore{
		sessions: make(map[string]*api.Session),
	}

	sessionState := &MockSessionState{
		states:        make(map[string]map[string]interface{}),
		checkpoints:   make(map[string][]string),
		workspaceDirs: make(map[string]string),
		metadata:      make(map[string]map[string]interface{}),
	}

	fileAccess := &MockFileAccessService{}

	return NewAutoFixHelper(logger, sessionStore, sessionState, fileAccess)
}

// TestAutoFixHelper_BasicFunctionality tests basic auto-fix functionality
func TestAutoFixHelper_BasicFunctionality(t *testing.T) {
	helper := setupAutoFixHelper(t)

	// Test that helper is properly initialized
	if helper == nil {
		t.Fatal("AutoFixHelper should not be nil")
	}

	if len(helper.fixes) == 0 {
		t.Error("AutoFixHelper should have registered fix strategies")
	}

	// Test that chain executor is initialized
	if helper.chainExecutor == nil {
		t.Error("ChainExecutor should be initialized")
	}
}

// TestAutoFixHelper_DockerfileNotFound tests the dockerfile_not_found fix strategy
func TestAutoFixHelper_DockerfileNotFound(t *testing.T) {
	helper := setupAutoFixHelper(t)
	ctx := context.Background()

	// Create a mock tool that fails first, then succeeds
	tool := &MockTool{
		name: "build_image",
		executeResults: []MockExecuteResult{
			{Error: errors.NewError().Message("dockerfile not found at original path").Build()},
			{Output: api.ToolOutput{Success: true, Data: map[string]interface{}{"fixed": true}}},
		},
	}

	// Create test arguments
	args := map[string]interface{}{
		"session_id":      "test-session",
		"dockerfile_path": "original/Dockerfile",
		"context_path":    "/app",
	}

	// Create test error
	err := errors.NewError().Message("Dockerfile not found").Build()

	// Attempt fix
	result, fixErr := helper.AttemptFix(ctx, tool, args, err)

	// Verify results
	if fixErr != nil {
		t.Errorf("Expected no error, got: %v", fixErr)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Verify that the tool was called multiple times (original + fix attempt)
	if tool.executeCount < 2 {
		t.Errorf("Expected at least 2 tool executions, got: %d", tool.executeCount)
	}
}

// TestAutoFixHelper_InvalidPort tests the invalid_port fix strategy
func TestAutoFixHelper_InvalidPort(t *testing.T) {
	helper := setupAutoFixHelper(t)
	ctx := context.Background()

	// Track which ports were tried
	var triedPorts []int

	tool := &MockTool{
		name: "deploy_container",
		executeFunc: func(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
			if port, ok := input.Data["port"].(int); ok {
				triedPorts = append(triedPorts, port)

				// Succeed on port 8080
				if port == 8080 {
					return api.ToolOutput{Success: true, Data: map[string]interface{}{"port": port}}, nil
				}
			}

			return api.ToolOutput{}, errors.NewError().Message("invalid port").Build()
		},
	}

	args := map[string]interface{}{
		"session_id": "test-session",
		"port":       99999, // Invalid port
	}

	err := errors.NewError().Message("invalid port: port out of range").Build()

	result, fixErr := helper.AttemptFix(ctx, tool, args, err)

	if fixErr != nil {
		t.Errorf("Expected no error, got: %v", fixErr)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Verify that 8080 was tried
	found8080 := false
	for _, port := range triedPorts {
		if port == 8080 {
			found8080 = true
			break
		}
	}

	if !found8080 {
		t.Error("Expected port 8080 to be tried")
	}
}

// TestAutoFixHelper_NetworkError tests the network_error fix strategy
func TestAutoFixHelper_NetworkError(t *testing.T) {
	helper := setupAutoFixHelper(t)
	ctx := context.Background()

	tool := &MockTool{
		name: "push_image",
	}

	args := map[string]interface{}{
		"session_id": "test-session",
		"image_name": "test:latest",
	}

	err := errors.NewError().Message("network connection failed").Build()

	_, fixErr := helper.AttemptFix(ctx, tool, args, err)

	// Network errors should return a structured error with suggestions
	if fixErr == nil {
		t.Error("Expected error with network suggestions")
	}

	errorMsg := fixErr.Error()
	if !strings.Contains(strings.ToLower(errorMsg), "network") {
		t.Errorf("Expected network error message, got: %s", errorMsg)
	}
}

// TestAutoFixHelper_SessionContext tests context-aware fixes
func TestAutoFixHelper_SessionContext(t *testing.T) {
	helper := setupAutoFixHelper(t)
	ctx := context.Background()

	sessionID := "test-session"

	// Add some fix history
	helper.recordFixAttempt(sessionID, "build_image", "dockerfile error", "basic", false)
	helper.recordFixAttempt(sessionID, "build_image", "dockerfile error", "basic", false)
	helper.recordFixAttempt(sessionID, "build_image", "dockerfile error", "basic", false)

	_ = &MockTool{name: "build_image"}
	_ = map[string]interface{}{"session_id": sessionID}
	_ = errors.NewError().Message("dockerfile error").Build()

	// Build session context
	sessionCtx, ctxErr := helper.buildSessionContext(ctx, sessionID)
	if ctxErr != nil {
		t.Errorf("Expected no error building session context, got: %v", ctxErr)
	}

	if sessionCtx == nil {
		t.Error("Expected session context, got nil")
		return
	}

	if sessionCtx.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, sessionCtx.SessionID)
	}

	// Test that repeated fix detection works
	shouldSkip := helper.shouldSkipRepeatedFix(sessionCtx, "build_image", "dockerfile error")
	if !shouldSkip {
		t.Error("Expected to skip repeated fix after 3 failures")
	}
}

// TestAutoFixHelper_FixChaining tests fix strategy chaining
func TestAutoFixHelper_FixChaining(t *testing.T) {
	helper := setupAutoFixHelper(t)
	ctx := context.Background()

	sessionID := "test-session"

	// Add multiple recent failures to trigger chaining
	helper.recordFixAttempt(sessionID, "build_image", "docker build failed", "basic", false)
	helper.recordFixAttempt(sessionID, "build_image", "docker build failed", "basic", false)
	helper.recordFixAttempt(sessionID, "build_image", "docker build failed", "basic", false)

	tool := &MockTool{
		name: "build_image",
		executeFunc: func(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
			// Always succeed for chaining test
			return api.ToolOutput{Success: true, Data: map[string]interface{}{"chained": true}}, nil
		},
	}

	args := map[string]interface{}{"session_id": sessionID}
	err := errors.NewError().Message("docker build failed dockerfile syntax error").Build()

	// Build session context
	sessionCtx, _ := helper.buildSessionContext(ctx, sessionID)

	// Test chain decision logic
	shouldUseChain := helper.shouldUseFixChain(sessionCtx, tool, err)
	if !shouldUseChain {
		t.Error("Expected to use fix chain for complex error with multiple failures")
	}

	// Test actual fix attempt with chaining
	result, fixErr := helper.AttemptFix(ctx, tool, args, err)

	if fixErr != nil {
		t.Errorf("Expected no error with chaining, got: %v", fixErr)
	}

	if result == nil {
		t.Error("Expected result from chaining, got nil")
	}
}

// TestAutoFixHelper_GetFixChainStatus tests the fix chain status reporting
func TestAutoFixHelper_GetFixChainStatus(t *testing.T) {
	helper := setupAutoFixHelper(t)

	// Add some chain usage history
	helper.recordFixAttempt("session1", "build_image", "error1", "chain", true)
	helper.recordFixAttempt("session1", "deploy", "error2", "chain", false)
	helper.recordFixAttempt("session2", "push_image", "error3", "chain", true)

	status := helper.GetFixChainStatus()

	if status == nil {
		t.Error("Expected fix chain status, got nil")
	}

	// Check available chains
	if chains, ok := status["available_chains"].(map[string]string); ok {
		if len(chains) == 0 {
			t.Error("Expected available chains")
		}
	} else {
		t.Error("Expected available_chains in status")
	}

	// Check usage stats
	if stats, ok := status["usage_stats"].(map[string]interface{}); ok {
		if totalAttempts, ok := stats["total_chain_attempts"].(int); !ok || totalAttempts != 3 {
			t.Errorf("Expected 3 total chain attempts, got: %v", totalAttempts)
		}

		if successfulChains, ok := stats["successful_chains"].(int); !ok || successfulChains != 2 {
			t.Errorf("Expected 2 successful chains, got: %v", successfulChains)
		}

		if successRate, ok := stats["success_rate"].(float64); !ok || successRate < 66.0 || successRate > 67.0 {
			t.Errorf("Expected success rate around 66.67%%, got: %v", successRate)
		}
	} else {
		t.Error("Expected usage_stats in status")
	}
}

// TestAutoFixHelper_CalculateSuccessRate tests success rate calculation
func TestAutoFixHelper_CalculateSuccessRate(t *testing.T) {
	helper := setupAutoFixHelper(t)

	tests := []struct {
		successful int
		total      int
		expected   float64
	}{
		{0, 0, 0.0},
		{1, 1, 100.0},
		{2, 3, 66.66666666666667},
		{7, 10, 70.0},
		{0, 5, 0.0},
	}

	for _, test := range tests {
		result := helper.calculateSuccessRate(test.successful, test.total)
		// Use approximate comparison for floating point values
		diff := result - test.expected
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.0001 { // Allow small floating point differences
			t.Errorf("For %d successful out of %d total, expected %f, got %f",
				test.successful, test.total, test.expected, result)
		}
	}
}

// TestAutoFixHelper_ListFixes tests listing available fix strategies
func TestAutoFixHelper_ListFixes(t *testing.T) {
	helper := setupAutoFixHelper(t)

	fixes := helper.ListFixes()

	if len(fixes) == 0 {
		t.Error("Expected some fix strategies to be available")
	}

	// Check for some expected fixes
	expectedFixes := []string{
		"dockerfile_not_found",
		"invalid_port",
		"network_error",
		"missing_dependency",
		"image_not_found",
	}

	for _, expectedFix := range expectedFixes {
		found := false
		for _, fix := range fixes {
			if fix == expectedFix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected fix strategy '%s' to be available", expectedFix)
		}
	}
}

// TestAutoFixHelper_HasFix tests checking for specific fix strategies
func TestAutoFixHelper_HasFix(t *testing.T) {
	helper := setupAutoFixHelper(t)

	// Test existing fix
	if !helper.HasFix("dockerfile_not_found") {
		t.Error("Expected dockerfile_not_found fix to exist")
	}

	// Test non-existing fix
	if helper.HasFix("non_existent_fix") {
		t.Error("Expected non_existent_fix to not exist")
	}
}

// TestAutoFixHelper_RegisterCustomFix tests registering custom fix strategies
func TestAutoFixHelper_RegisterCustomFix(t *testing.T) {
	helper := setupAutoFixHelper(t)

	customFixCalled := false
	customFix := func(_ context.Context, _ api.Tool, _ interface{}, _ error) (interface{}, error) {
		customFixCalled = true
		return map[string]interface{}{"custom": true}, nil
	}

	// Register custom fix
	helper.RegisterFix("custom_test_fix", customFix)

	// Verify it's registered
	if !helper.HasFix("custom_test_fix") {
		t.Error("Expected custom fix to be registered")
	}

	// Test using the custom fix
	ctx := context.Background()
	tool := &MockTool{name: "test_tool"}
	args := map[string]interface{}{"session_id": "test"}
	err := errors.NewError().Message("test error").Build()

	// Manually call the custom fix
	if strategy, exists := helper.fixes["custom_test_fix"]; exists {
		result, fixErr := strategy(ctx, tool, args, err)

		if fixErr != nil {
			t.Errorf("Expected no error from custom fix, got: %v", fixErr)
		}

		if !customFixCalled {
			t.Error("Expected custom fix to be called")
		}

		if result == nil {
			t.Error("Expected result from custom fix")
		}
	} else {
		t.Error("Custom fix not found in fixes map")
	}
}

// Benchmark tests for performance
func BenchmarkAutoFixHelper_AttemptFix(b *testing.B) {
	helper := setupAutoFixHelper(b)
	ctx := context.Background()

	tool := &MockTool{
		name: "test_tool",
		executeFunc: func(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
			return api.ToolOutput{Success: true}, nil
		},
	}

	args := map[string]interface{}{"session_id": "bench-session"}
	err := errors.NewError().Message("network error").Build()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = helper.AttemptFix(ctx, tool, args, err)
	}
}

func BenchmarkAutoFixHelper_BuildSessionContext(b *testing.B) {
	helper := setupAutoFixHelper(b)
	ctx := context.Background()
	sessionID := "bench-session"

	// Add some history
	for i := 0; i < 10; i++ {
		helper.recordFixAttempt(sessionID, "tool", "error", "strategy", i%2 == 0)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = helper.buildSessionContext(ctx, sessionID)
	}
}
