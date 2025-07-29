package tools

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/service/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolConfigs(t *testing.T) {
	configs := GetToolConfigs()

	// Verify we have all 15 tools (10 workflow + 2 orchestration + 3 utility including diagnostics)
	assert.Len(t, configs, 15, "Should have 15 tool configurations")

	// Verify categories
	workflowCount := 0
	orchestrationCount := 0
	utilityCount := 0

	for _, config := range configs {
		switch config.Category {
		case CategoryWorkflow:
			workflowCount++
		case CategoryOrchestration:
			orchestrationCount++
		case CategoryUtility:
			utilityCount++
		default:
			t.Errorf("Unknown category for tool %s: %s", config.Name, config.Category)
		}
	}

	assert.Equal(t, 10, workflowCount, "Should have 10 workflow tools")
	assert.Equal(t, 2, orchestrationCount, "Should have 2 orchestration tools")
	assert.Equal(t, 3, utilityCount, "Should have 3 utility tools (list_tools, ping, server_status)")
}

func TestGetToolConfig(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		wantErr   bool
		checkFunc func(t *testing.T, config *ToolConfig)
	}{
		{
			name:     "valid workflow tool",
			toolName: "analyze_repository",
			wantErr:  false,
			checkFunc: func(t *testing.T, config *ToolConfig) {
				assert.Equal(t, CategoryWorkflow, config.Category)
				assert.Contains(t, config.RequiredParams, "repo_path")
				assert.Contains(t, config.RequiredParams, "session_id")
				assert.Equal(t, "generate_dockerfile", config.NextTool)
			},
		},
		{
			name:     "valid orchestration tool",
			toolName: "start_workflow",
			wantErr:  false,
			checkFunc: func(t *testing.T, config *ToolConfig) {
				assert.Equal(t, CategoryOrchestration, config.Category)
				assert.Contains(t, config.RequiredParams, "repo_path")
				assert.NotContains(t, config.RequiredParams, "session_id")
			},
		},
		{
			name:     "valid utility tool",
			toolName: "list_tools",
			wantErr:  false,
			checkFunc: func(t *testing.T, config *ToolConfig) {
				assert.Equal(t, CategoryUtility, config.Category)
				assert.Empty(t, config.RequiredParams)
			},
		},
		{
			name:     "invalid tool",
			toolName: "non_existent_tool",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetToolConfig(tt.toolName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, config)
				if tt.checkFunc != nil {
					tt.checkFunc(t, config)
				}
			}
		})
	}
}

func TestBuildToolSchema(t *testing.T) {
	tests := []struct {
		name   string
		config ToolConfig
		check  func(t *testing.T, schema mcp.ToolInputSchema)
	}{
		{
			name: "simple schema",
			config: ToolConfig{
				RequiredParams: []string{"param1", "param2"},
			},
			check: func(t *testing.T, schema mcp.ToolInputSchema) {
				assert.Equal(t, "object", schema.Type)
				assert.Len(t, schema.Required, 2)
				assert.Contains(t, schema.Required, "param1")
				assert.Contains(t, schema.Required, "param2")
				assert.Len(t, schema.Properties, 2)
			},
		},
		{
			name: "schema with optional params",
			config: ToolConfig{
				RequiredParams: []string{"required1"},
				OptionalParams: map[string]interface{}{
					"optional1": "string",
					"optional2": "array",
				},
			},
			check: func(t *testing.T, schema mcp.ToolInputSchema) {
				assert.Equal(t, "object", schema.Type)
				assert.Len(t, schema.Required, 1)
				assert.Contains(t, schema.Required, "required1")
				assert.Len(t, schema.Properties, 3)
				assert.Contains(t, schema.Properties, "optional1")
				assert.Contains(t, schema.Properties, "optional2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := BuildToolSchema(tt.config)
			tt.check(t, schema)
		})
	}
}

func TestValidateDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))

	tests := []struct {
		name    string
		config  ToolConfig
		deps    ToolDependencies
		wantErr bool
	}{
		{
			name: "all dependencies provided",
			config: ToolConfig{
				NeedsStepProvider:   true,
				NeedsSessionManager: true,
				NeedsLogger:         true,
			},
			deps: ToolDependencies{
				StepProvider:   &mockStepProvider{},
				SessionManager: &mockSessionManager{},
				Logger:         logger,
			},
			wantErr: false,
		},
		{
			name: "missing required dependency",
			config: ToolConfig{
				NeedsStepProvider: true,
			},
			deps: ToolDependencies{
				StepProvider: nil,
			},
			wantErr: true,
		},
		{
			name:    "no dependencies needed",
			config:  ToolConfig{},
			deps:    ToolDependencies{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDependencies(tt.config, tt.deps)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChainHints(t *testing.T) {
	// Test workflow chain
	var previousTool string
	for _, config := range toolConfigs {
		if config.Category == CategoryWorkflow {
			if previousTool != "" {
				// Verify the previous tool points to this one (except for the first)
				prevConfig, err := GetToolConfig(previousTool)
				if err == nil && prevConfig.NextTool != "" {
					assert.Equal(t, config.Name, prevConfig.NextTool,
						"Tool %s should point to %s", previousTool, config.Name)
				}
			}
			previousTool = config.Name
		}
	}
}

func TestToolRegistration(t *testing.T) {
	// Create a real MCP server for testing
	mcpServer := server.NewMCPServer("test-server", "1.0.0")

	// Create dependencies
	deps := ToolDependencies{
		StepProvider:   &mockStepProvider{},
		SessionManager: &mockSessionManager{},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Test registering a single tool
	config := ToolConfig{
		Name:                "test_tool",
		Description:         "Test tool",
		Category:            CategoryWorkflow,
		RequiredParams:      []string{"session_id"},
		NeedsStepProvider:   true,
		NeedsSessionManager: true,
		NeedsLogger:         true,
		StepGetterName:      "GetAnalyzeStep",
	}

	err := RegisterTool(mcpServer, config, deps)
	assert.NoError(t, err)

	// Tool registration succeeded if no error was returned
}

// Mock implementations for testing

type mockStepProvider struct{}

// GetStep implements the consolidated StepProvider interface
func (m *mockStepProvider) GetStep(name string) (workflow.Step, error) {
	return &mockStep{}, nil
}

// ListSteps returns all available step names
func (m *mockStepProvider) ListSteps() []string {
	return []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"security_scan",
		"tag_image",
		"push_image",
		"generate_manifests",
		"setup_cluster",
		"deploy_application",
		"verify_deployment",
	}
}

type mockStep struct{}

func (m *mockStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	return nil
}

func (m *mockStep) Name() string {
	return "mock-step"
}

func (m *mockStep) MaxRetries() int {
	return 3
}

type mockSessionManager struct{}

func (m *mockSessionManager) Get(ctx context.Context, id string) (*session.SessionState, error) {
	return &session.SessionState{}, nil
}

func (m *mockSessionManager) Update(ctx context.Context, sessionID string, updateFunc func(*session.SessionState) error) error {
	state := &session.SessionState{}
	return updateFunc(state)
}

func (m *mockSessionManager) GetOrCreate(ctx context.Context, id string) (*session.SessionState, error) {
	return &session.SessionState{}, nil
}

func (m *mockSessionManager) List(ctx context.Context) ([]*session.SessionState, error) {
	return []*session.SessionState{}, nil
}

func (m *mockSessionManager) Stats() *session.SessionStats {
	return &session.SessionStats{}
}

func (m *mockSessionManager) Stop(ctx context.Context) error {
	return nil
}

type mockMCPServer struct {
	server.MCPServer
	tools map[string]mcp.Tool
}

func (m *mockMCPServer) AddTool(tool mcp.Tool, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	m.tools[tool.Name] = tool
}
