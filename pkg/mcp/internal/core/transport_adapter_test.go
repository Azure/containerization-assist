package core

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/transport"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockInternalTransport for testing
type MockInternalTransport struct {
	mock.Mock
}

func (m *MockInternalTransport) Serve(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockInternalTransport) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockInternalTransport) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockInternalTransport) SetHandler(handler transport.LocalRequestHandler) {
	m.Called(handler)
}

// MockInternalRequestHandler for testing
type MockInternalRequestHandler struct {
	mock.Mock
}

func (m *MockInternalRequestHandler) HandleRequest(ctx context.Context, request interface{}) (interface{}, error) {
	args := m.Called(ctx, request)
	return args.Get(0), args.Error(1)
}

// mockLocalRequestHandler implements transport.LocalRequestHandler for testing
type mockLocalRequestHandler struct {
	mock.Mock
}

func (m *mockLocalRequestHandler) HandleRequest(ctx context.Context, req *mcptypes.MCPRequest) (*mcptypes.MCPResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcptypes.MCPResponse), args.Error(1)
}

func TestNewTransportAdapter_ValidTransport(t *testing.T) {
	mockTransport := &MockInternalTransport{}

	adapter := NewTransportAdapter(mockTransport)

	assert.NotNil(t, adapter)
	assert.IsType(t, &TransportAdapter{}, adapter)
}

func TestNewTransportAdapter_InvalidTransport(t *testing.T) {
	// Test with an object that doesn't implement the required interface
	invalidTransport := "not a transport"

	adapter := NewTransportAdapter(invalidTransport)

	assert.Nil(t, adapter)
}

func TestNewTransportAdapter_PartialInterface(t *testing.T) {
	// Test with an object that only partially implements the interface
	partialTransport := struct {
		Serve func(ctx context.Context) error
	}{
		Serve: func(ctx context.Context) error { return nil },
	}

	adapter := NewTransportAdapter(partialTransport)

	assert.Nil(t, adapter)
}

func TestTransportAdapter_Serve(t *testing.T) {
	mockTransport := &MockInternalTransport{}
	adapter := &TransportAdapter{internal: mockTransport}

	ctx := context.Background()
	expectedError := errors.New("serve error")

	// Test successful serve
	t.Run("Success", func(t *testing.T) {
		mockTransport.On("Serve", ctx).Return(nil).Once()

		err := adapter.Serve(ctx)

		assert.NoError(t, err)
		mockTransport.AssertExpectations(t)
	})

	// Test serve with error
	t.Run("Error", func(t *testing.T) {
		mockTransport.On("Serve", ctx).Return(expectedError).Once()

		err := adapter.Serve(ctx)

		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		mockTransport.AssertExpectations(t)
	})
}

func TestTransportAdapter_Stop(t *testing.T) {
	mockTransport := &MockInternalTransport{}
	adapter := &TransportAdapter{internal: mockTransport}

	ctx := context.Background()
	expectedError := errors.New("stop error")

	// Test successful stop
	t.Run("Success", func(t *testing.T) {
		mockTransport.On("Stop", ctx).Return(nil).Once()

		err := adapter.Stop(ctx)

		assert.NoError(t, err)
		mockTransport.AssertExpectations(t)
	})

	// Test stop with error
	t.Run("Error", func(t *testing.T) {
		mockTransport.On("Stop", ctx).Return(expectedError).Once()

		err := adapter.Stop(ctx)

		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		mockTransport.AssertExpectations(t)
	})
}

func TestTransportAdapter_Name(t *testing.T) {
	mockTransport := &MockInternalTransport{}
	adapter := &TransportAdapter{internal: mockTransport}

	expectedName := "test-transport"
	mockTransport.On("Name").Return(expectedName)

	name := adapter.Name()

	assert.Equal(t, expectedName, name)
	mockTransport.AssertExpectations(t)
}

func TestTransportAdapter_SetHandler(t *testing.T) {
	mockTransport := &MockInternalTransport{}
	adapter := &TransportAdapter{internal: mockTransport}

	// Create a mock handler that implements LocalRequestHandler interface
	mockHandler := &mockLocalRequestHandler{}

	mockTransport.On("SetHandler", mock.Anything).Once()

	adapter.SetHandler(mockHandler)

	mockTransport.AssertExpectations(t)
}

func TestRequestHandlerAdapter_HandleRequest_Success(t *testing.T) {
	mockHandler := &MockInternalRequestHandler{}
	adapter := &requestHandlerAdapter{handler: mockHandler}

	ctx := context.Background()
	req := &mcptypes.MCPRequest{
		Method: "test_method",
		Params: map[string]interface{}{"key": "value"},
	}

	// Test when handler returns MCPResponse
	t.Run("ReturnsValidMCPResponse", func(t *testing.T) {
		expectedResponse := &mcptypes.MCPResponse{
			Result: map[string]interface{}{"success": true},
		}

		mockHandler.On("HandleRequest", ctx, req).Return(expectedResponse, nil).Once()

		response, err := adapter.HandleRequest(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, expectedResponse, response)
		mockHandler.AssertExpectations(t)
	})

	// Test when handler returns other result that needs wrapping
	t.Run("ReturnsOtherResult", func(t *testing.T) {
		otherResult := map[string]interface{}{"data": "test"}

		mockHandler.On("HandleRequest", ctx, req).Return(otherResult, nil).Once()

		response, err := adapter.HandleRequest(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, otherResult, response.Result)
		mockHandler.AssertExpectations(t)
	})
}

func TestRequestHandlerAdapter_HandleRequest_Error(t *testing.T) {
	mockHandler := &MockInternalRequestHandler{}
	adapter := &requestHandlerAdapter{handler: mockHandler}

	ctx := context.Background()
	req := &mcptypes.MCPRequest{
		Method: "test_method",
		Params: map[string]interface{}{"key": "value"},
	}

	expectedError := errors.New("handler error")
	mockHandler.On("HandleRequest", ctx, req).Return(nil, expectedError)

	response, err := adapter.HandleRequest(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, expectedError, err)
	mockHandler.AssertExpectations(t)
}

func TestRequestHandlerAdapter_HandleRequest_NilResult(t *testing.T) {
	mockHandler := &MockInternalRequestHandler{}
	adapter := &requestHandlerAdapter{handler: mockHandler}

	ctx := context.Background()
	req := &mcptypes.MCPRequest{
		Method: "test_method",
		Params: map[string]interface{}{"key": "value"},
	}

	// Test when handler returns nil result
	mockHandler.On("HandleRequest", ctx, req).Return(nil, nil)

	response, err := adapter.HandleRequest(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Nil(t, response.Result)
	mockHandler.AssertExpectations(t)
}

func TestTransportAdapter_IntegrationWithMocks(t *testing.T) {
	// Test the full flow of creating an adapter and using it
	mockTransport := &MockInternalTransport{}

	// Create adapter
	adapter := NewTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	ctx := context.Background()

	// Test Name (cast to TransportAdapter to access Name method)
	mockTransport.On("Name").Return("integration-test-transport")
	transportAdapter := adapter.(*TransportAdapter)
	name := transportAdapter.Name()
	assert.Equal(t, "integration-test-transport", name)

	// Test SetHandler
	mockHandler := &mockLocalRequestHandler{}
	mockTransport.On("SetHandler", mock.Anything).Once()
	transportAdapter.SetHandler(mockHandler)

	// Test Serve
	mockTransport.On("Serve", ctx).Return(nil)
	err := adapter.Serve(ctx)
	assert.NoError(t, err)

	// Test Stop
	mockTransport.On("Stop", ctx).Return(nil)
	err = adapter.Stop(ctx)
	assert.NoError(t, err)

	mockTransport.AssertExpectations(t)
}

func TestTransportAdapter_TypeAssertion(t *testing.T) {
	// Test that the adapter properly implements InternalTransport
	mockTransport := &MockInternalTransport{}
	adapter := NewTransportAdapter(mockTransport)

	// This should compile and not panic
	var _ InternalTransport = adapter

	assert.NotNil(t, adapter)
}
