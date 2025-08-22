package registrar

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Azure/containerization-assist/pkg/domain/resources"
	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	resourcesInfra "github.com/Azure/containerization-assist/pkg/infrastructure/core/resources"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWorkflowOrchestrator is a mock implementation of workflow.WorkflowOrchestrator
type MockWorkflowOrchestrator struct {
	executeCallCount int
	shouldFail       bool
}

func (m *MockWorkflowOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	m.executeCallCount++
	if m.shouldFail {
		return nil, assert.AnError
	}
	return &workflow.ContainerizeAndDeployResult{
		Success:  true,
		ImageRef: "test-image:latest",
	}, nil
}

// MockResourceStore is a mock implementation of resources.Store
type MockResourceStore struct {
	resources            []resources.Resource
	addResourceCallCount int
	shouldFail           bool
}

func (m *MockResourceStore) GetResource(ctx context.Context, uri string) (resources.Resource, error) {
	if m.shouldFail {
		return resources.Resource{}, assert.AnError
	}
	for _, r := range m.resources {
		if r.URI == uri {
			return r, nil
		}
	}
	return resources.Resource{}, assert.AnError
}

func (m *MockResourceStore) ListResources(ctx context.Context) ([]resources.Resource, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}
	return m.resources, nil
}

func (m *MockResourceStore) AddResource(ctx context.Context, resource resources.Resource) error {
	m.addResourceCallCount++
	if m.shouldFail {
		return assert.AnError
	}
	m.resources = append(m.resources, resource)
	return nil
}

func (m *MockResourceStore) RemoveResource(ctx context.Context, uri string) error {
	if m.shouldFail {
		return assert.AnError
	}
	for i, r := range m.resources {
		if r.URI == uri {
			m.resources = append(m.resources[:i], m.resources[i+1:]...)
			return nil
		}
	}
	return assert.AnError
}

func (m *MockResourceStore) RegisterProviders(mcpServer interface{}) error {
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func TestNewMCPRegistrar(t *testing.T) {
	logger := slog.Default()
	resourceStore := resourcesInfra.NewStore(logger)
	mockOrchestrator := &MockWorkflowOrchestrator{}

	config := workflow.DefaultServerConfig()
	registrar := NewMCPRegistrar(logger, resourceStore, mockOrchestrator, nil, config)

	assert.NotNil(t, registrar)
	assert.NotNil(t, registrar.toolRegistrar)
	assert.NotNil(t, registrar.resourceRegistrar)
}

func TestRegistrar_ComponentInitialization(t *testing.T) {
	logger := slog.Default()
	resourceStore := resourcesInfra.NewStore(logger)
	mockOrchestrator := &MockWorkflowOrchestrator{}

	config := workflow.DefaultServerConfig()
	registrar := NewMCPRegistrar(logger, resourceStore, mockOrchestrator, nil, config)

	// Verify internal components are properly initialized
	assert.NotNil(t, registrar.toolRegistrar, "ToolRegistrar should be initialized")
	assert.NotNil(t, registrar.resourceRegistrar, "ResourceRegistrar should be initialized")
}

func TestRegistrar_NilDependencies(t *testing.T) {
	// Test behavior with nil dependencies
	logger := slog.Default()

	// Should not panic with nil dependencies (defensive programming)
	assert.NotPanics(t, func() {
		config := workflow.DefaultServerConfig()
		registrar := NewMCPRegistrar(logger, nil, nil, nil, config)
		assert.NotNil(t, registrar)
	})
}

func TestNewToolRegistrar(t *testing.T) {
	logger := slog.Default()
	mockOrchestrator := &MockWorkflowOrchestrator{}

	config := workflow.DefaultServerConfig()
	toolRegistrar := NewToolRegistrar(logger, mockOrchestrator, nil, nil, config)

	assert.NotNil(t, toolRegistrar)
	assert.Equal(t, mockOrchestrator, toolRegistrar.orchestrator)
}

func TestNewResourceRegistrar(t *testing.T) {
	logger := slog.Default()
	resourceStore := resourcesInfra.NewStore(logger)

	resourceRegistrar := NewResourceRegistrar(logger, resourceStore)

	assert.NotNil(t, resourceRegistrar)
}

func TestMockWorkflowOrchestrator_Execute_Success(t *testing.T) {
	mock := &MockWorkflowOrchestrator{}

	result, err := mock.Execute(context.Background(), &mcp.CallToolRequest{}, &workflow.ContainerizeAndDeployArgs{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "test-image:latest", result.ImageRef)
	assert.Equal(t, 1, mock.executeCallCount)
}

func TestMockWorkflowOrchestrator_Execute_Failure(t *testing.T) {
	mock := &MockWorkflowOrchestrator{shouldFail: true}

	result, err := mock.Execute(context.Background(), &mcp.CallToolRequest{}, &workflow.ContainerizeAndDeployArgs{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, mock.executeCallCount)
}

func TestMockResourceStore_AddResource(t *testing.T) {
	mock := &MockResourceStore{}
	resource := resources.Resource{
		URI:  "test://resource/1",
		Name: "Test Resource",
	}

	err := mock.AddResource(context.Background(), resource)

	require.NoError(t, err)
	assert.Equal(t, 1, mock.addResourceCallCount)
	assert.Len(t, mock.resources, 1)
	assert.Equal(t, resource.URI, mock.resources[0].URI)
}

func TestMockResourceStore_GetResource(t *testing.T) {
	resource := resources.Resource{
		URI:  "test://resource/1",
		Name: "Test Resource",
	}
	mock := &MockResourceStore{resources: []resources.Resource{resource}}

	result, err := mock.GetResource(context.Background(), "test://resource/1")

	require.NoError(t, err)
	assert.Equal(t, resource.URI, result.URI)
	assert.Equal(t, resource.Name, result.Name)
}

func TestMockResourceStore_ListResources(t *testing.T) {
	resources_list := []resources.Resource{
		{URI: "test://resource/1", Name: "Resource 1"},
		{URI: "test://resource/2", Name: "Resource 2"},
	}
	mock := &MockResourceStore{resources: resources_list}

	result, err := mock.ListResources(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, resources_list[0].URI, result[0].URI)
	assert.Equal(t, resources_list[1].URI, result[1].URI)
}

func TestMockResourceStore_RemoveResource(t *testing.T) {
	resource := resources.Resource{URI: "test://resource/1", Name: "Test Resource"}
	mock := &MockResourceStore{resources: []resources.Resource{resource}}

	err := mock.RemoveResource(context.Background(), "test://resource/1")

	require.NoError(t, err)
	assert.Len(t, mock.resources, 0)
}

func TestMockResourceStore_RegisterProviders(t *testing.T) {
	mock := &MockResourceStore{}

	err := mock.RegisterProviders(nil)

	require.NoError(t, err)
}

// Test error scenarios
func TestMockResourceStore_ErrorScenarios(t *testing.T) {
	t.Run("AddResource fails", func(t *testing.T) {
		mock := &MockResourceStore{shouldFail: true}
		err := mock.AddResource(context.Background(), resources.Resource{})
		assert.Error(t, err)
	})

	t.Run("GetResource fails", func(t *testing.T) {
		mock := &MockResourceStore{shouldFail: true}
		_, err := mock.GetResource(context.Background(), "test")
		assert.Error(t, err)
	})

	t.Run("ListResources fails", func(t *testing.T) {
		mock := &MockResourceStore{shouldFail: true}
		_, err := mock.ListResources(context.Background())
		assert.Error(t, err)
	})

	t.Run("RegisterProviders fails", func(t *testing.T) {
		mock := &MockResourceStore{shouldFail: true}
		err := mock.RegisterProviders(nil)
		assert.Error(t, err)
	})
}
