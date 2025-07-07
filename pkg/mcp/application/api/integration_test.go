package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MockGenericAnalyzeTool implements api.AnalyzeTool for testing
type MockGenericAnalyzeTool struct {
	name        string
	description string
	timeout     time.Duration
}

func NewMockGenericAnalyzeTool() *MockGenericAnalyzeTool {
	return &MockGenericAnalyzeTool{
		name:        "mock_analyze_tool",
		description: "Mock analyze tool for testing",
		timeout:     30 * time.Second,
	}
}

func (m *MockGenericAnalyzeTool) Name() string {
	return m.name
}

func (m *MockGenericAnalyzeTool) Description() string {
	return m.description
}

func (m *MockGenericAnalyzeTool) Execute(ctx context.Context, input *api.AnalyzeInput) (*api.AnalyzeOutput, error) {
	// Simulate analysis
	time.Sleep(10 * time.Millisecond)

	return &api.AnalyzeOutput{
		Success:   true,
		SessionID: input.SessionID,
		Language:  "Go",
		Framework: "gin",
		Dependencies: []api.Dependency{
			{Name: "gin", Version: "v1.7.0", Type: "direct"},
			{Name: "gorm", Version: "v1.21.0", Type: "direct"},
		},
		SecurityIssues: []api.SecurityIssue{
			{ID: "SEC-001", Severity: "low", Description: "Consider using HTTPS"},
		},
		BuildRecommendations: []string{
			"Use multi-stage Docker builds",
			"Add .dockerignore file",
		},
		AnalysisTime:  time.Since(time.Now().Add(-100 * time.Millisecond)),
		FilesAnalyzed: 42,
		Data: map[string]interface{}{
			"repo_url": input.RepoURL,
			"branch":   input.Branch,
		},
	}, nil
}

func (m *MockGenericAnalyzeTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{"type": "string"},
				"repo_url":   map[string]interface{}{"type": "string"},
			},
			"required": []string{"session_id", "repo_url"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success":   map[string]interface{}{"type": "boolean"},
				"language":  map[string]interface{}{"type": "string"},
				"framework": map[string]interface{}{"type": "string"},
			},
		},
		Category: api.CategoryAnalyze,
		Tags:     []string{"analysis", "repository", "mock"},
	}
}

func (m *MockGenericAnalyzeTool) Validate(ctx context.Context, input *api.AnalyzeInput) error {
	return input.Validate()
}

func (m *MockGenericAnalyzeTool) GetTimeout() time.Duration {
	return m.timeout
}

// TestGenericToolInterface tests the generic tool interface directly
func TestGenericToolInterface(t *testing.T) {
	// Create a generic tool
	genericTool := NewMockGenericAnalyzeTool()

	// Test basic interface compliance
	if genericTool.Name() != "mock_analyze_tool" {
		t.Errorf("Expected name 'mock_analyze_tool', got %s", genericTool.Name())
	}

	if genericTool.Description() != "Mock analyze tool for testing" {
		t.Errorf("Expected description to match, got %s", genericTool.Description())
	}

	// Test execution with generic interface
	ctx := context.Background()
	genericInput := &api.AnalyzeInput{
		SessionID: "test-session",
		RepoURL:   "https://github.com/example/repo",
		Branch:    "main",
	}

	genericOutput, err := genericTool.Execute(ctx, genericInput)
	if err != nil {
		t.Fatalf("Generic execution failed: %v", err)
	}

	if !genericOutput.Success {
		t.Error("Expected generic output to be successful")
	}

	// Verify the data was properly set
	if genericOutput.Language != "Go" {
		t.Errorf("Expected language 'Go', got %s", genericOutput.Language)
	}

	if genericOutput.Framework != "gin" {
		t.Errorf("Expected framework 'gin', got %s", genericOutput.Framework)
	}
}

// MockLegacyTool implements the legacy api.Tool interface
type MockLegacyTool struct {
	name        string
	description string
}

func NewMockLegacyTool() *MockLegacyTool {
	return &MockLegacyTool{
		name:        "mock_legacy_tool",
		description: "Mock legacy tool for testing",
	}
}

func (m *MockLegacyTool) Name() string {
	return m.name
}

func (m *MockLegacyTool) Description() string {
	return m.description
}

func (m *MockLegacyTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Simulate processing
	time.Sleep(5 * time.Millisecond)

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"session_id": input.SessionID,
			"language":   "Python",
			"framework":  "flask",
			"result":     "legacy analysis complete",
		},
	}, nil
}

func (m *MockLegacyTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
		Version:     "1.0.0",
		Category:    api.CategoryAnalyze,
	}
}

// TestLegacyToolInterface tests the legacy tool interface directly
func TestLegacyToolInterface(t *testing.T) {
	// Create a legacy tool
	legacyTool := NewMockLegacyTool()

	// Test basic interface compliance
	if legacyTool.Name() != "mock_legacy_tool" {
		t.Errorf("Expected name 'mock_legacy_tool', got %s", legacyTool.Name())
	}

	// Test execution with legacy interface
	ctx := context.Background()
	legacyInput := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"repo_url": "https://github.com/example/repo",
			"branch":   "main",
		},
	}

	legacyOutput, err := legacyTool.Execute(ctx, legacyInput)
	if err != nil {
		t.Fatalf("Legacy execution failed: %v", err)
	}

	if !legacyOutput.Success {
		t.Error("Expected legacy output to be successful")
	}

	// Verify the data
	if sessionID, ok := legacyOutput.Data["session_id"].(string); !ok || sessionID != "test-session" {
		t.Errorf("Expected session ID 'test-session', got %v", legacyOutput.Data["session_id"])
	}
}

// TestDirectRegistryUsage tests the registry functionality directly
func TestDirectRegistryUsage(t *testing.T) {
	// Create a mock registry
	registry := NewMockRegistry()

	// Create a legacy tool that implements api.Tool
	legacyTool := NewMockLegacyTool()

	// Test registration
	err := registry.Register(legacyTool)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}

	// Test listing
	tools := registry.List()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0] != legacyTool.Name() {
		t.Errorf("Expected tool name %s, got %s", legacyTool.Name(), tools[0])
	}

	// Test retrieval
	retrievedTool, err := registry.Get(legacyTool.Name())
	if err != nil {
		t.Fatalf("Tool retrieval failed: %v", err)
	}

	if retrievedTool.Name() != legacyTool.Name() {
		t.Errorf("Expected retrieved tool name %s, got %s", retrievedTool.Name(), retrievedTool.Name())
	}

	// Test execution
	ctx := context.Background()
	input := api.ToolInput{
		SessionID: "registry-test",
		Data: map[string]interface{}{
			"repo_url": "https://github.com/example/repo",
			"branch":   "main",
		},
	}

	output, err := registry.Execute(ctx, legacyTool.Name(), input)
	if err != nil {
		t.Fatalf("Registry execution failed: %v", err)
	}

	if !output.Success {
		t.Error("Expected registry execution to be successful")
	}
}

// MockRegistry implements the legacy Registry interface for testing
type MockRegistry struct {
	tools map[string]api.Tool
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		tools: make(map[string]api.Tool),
	}
}

func (m *MockRegistry) Register(tool api.Tool, opts ...api.RegistryOption) error {
	m.tools[tool.Name()] = tool
	return nil
}

func (m *MockRegistry) Unregister(name string) error {
	delete(m.tools, name)
	return nil
}

func (m *MockRegistry) Get(name string) (api.Tool, error) {
	if tool, exists := m.tools[name]; exists {
		return tool, nil
	}
	return nil, errors.ErrToolNotFound
}

func (m *MockRegistry) List() []string {
	var names []string
	for name := range m.tools {
		names = append(names, name)
	}
	return names
}

func (m *MockRegistry) ListByCategory(category api.ToolCategory) []string {
	var names []string
	for name, tool := range m.tools {
		if tool.Schema().Category == category {
			names = append(names, name)
		}
	}
	return names
}

func (m *MockRegistry) ListByTags(tags ...string) []string {
	var names []string
	for name, tool := range m.tools {
		schema := tool.Schema()
		for _, tag := range tags {
			for _, schemaTag := range schema.Tags {
				if tag == schemaTag {
					names = append(names, name)
					break
				}
			}
		}
	}
	return names
}

func (m *MockRegistry) Execute(ctx context.Context, name string, input api.ToolInput) (api.ToolOutput, error) {
	tool, err := m.Get(name)
	if err != nil {
		return api.ToolOutput{}, err
	}
	return tool.Execute(ctx, input)
}

func (m *MockRegistry) ExecuteWithRetry(ctx context.Context, name string, input api.ToolInput, policy api.RetryPolicy) (api.ToolOutput, error) {
	// Simple implementation without actual retry logic for testing
	return m.Execute(ctx, name, input)
}

func (m *MockRegistry) GetMetadata(name string) (api.ToolMetadata, error) {
	tool, err := m.Get(name)
	if err != nil {
		return api.ToolMetadata{}, err
	}

	schema := tool.Schema()
	return api.ToolMetadata{
		Name:         schema.Name,
		Description:  schema.Description,
		Version:      schema.Version,
		Category:     schema.Category,
		Tags:         schema.Tags,
		Status:       api.StatusActive,
		RegisteredAt: time.Now(),
	}, nil
}

func (m *MockRegistry) GetStatus(name string) (api.ToolStatus, error) {
	if _, exists := m.tools[name]; exists {
		return api.StatusActive, nil
	}
	return api.StatusInactive, errors.ErrToolNotFound
}

func (m *MockRegistry) SetStatus(name string, status api.ToolStatus) error {
	if _, exists := m.tools[name]; !exists {
		return errors.ErrToolNotFound
	}
	// In a real implementation, this would update the tool status
	return nil
}

func (m *MockRegistry) Close() error {
	m.tools = make(map[string]api.Tool)
	return nil
}

// TestTypeConstraints tests that type constraints work properly
func TestTypeConstraints(t *testing.T) {
	// Test that AnalyzeInput implements ToolInputConstraint
	input := &api.AnalyzeInput{
		SessionID: "constraint-test",
		RepoURL:   "https://github.com/example/repo",
	}

	// These calls should compile because AnalyzeInput implements ToolInputConstraint
	sessionID := input.GetSessionID()
	if sessionID != "constraint-test" {
		t.Errorf("Expected session ID 'constraint-test', got %s", sessionID)
	}

	err := input.Validate()
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	context := input.GetContext()
	if context == nil {
		t.Error("Expected non-nil context")
	}

	// Test that AnalyzeOutput implements ToolOutputConstraint
	output := &api.AnalyzeOutput{
		Success:   true,
		SessionID: "constraint-test",
		Language:  "Go",
		ErrorMsg:  "",
	}

	// These calls should compile because AnalyzeOutput implements ToolOutputConstraint
	if !output.IsSuccess() {
		t.Error("Expected output to be successful")
	}

	data := output.GetData()
	if data == nil {
		t.Error("Expected non-nil data")
	}

	if output.GetError() != "" {
		t.Errorf("Expected no error, got: %s", output.GetError())
	}
}

func TestCompileTimeTypeSafety(t *testing.T) {
	// This test demonstrates compile-time type safety
	// The following code would not compile if uncommented:

	// var analyzeTool api.AnalyzeTool
	// var buildInput api.BuildInput
	// analyzeTool.Execute(ctx, &buildInput) // COMPILE ERROR: type mismatch

	// var analyzeRegistry api.GenericRegistry[*api.AnalyzeInput, *api.AnalyzeOutput]
	// var buildTool api.BuildTool
	// analyzeRegistry.Register(buildTool) // COMPILE ERROR: type mismatch

	t.Log("Compile-time type safety is enforced by the Go compiler")
	t.Log("Wrong input/output types would cause compilation to fail")
}
