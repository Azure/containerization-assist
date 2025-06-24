package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReadWriter implements io.Reader and io.Writer for testing
type mockReadWriter struct {
	readData  string
	writeData bytes.Buffer
	readPos   int
	closed    bool
	mu        sync.Mutex
}

func newMockReadWriter(readData string) *mockReadWriter {
	return &mockReadWriter{
		readData: readData,
	}
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, io.EOF
	}
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}

	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, fmt.Errorf("write to closed writer")
	}
	return m.writeData.Write(p)
}

func (m *mockReadWriter) GetWritten() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeData.String()
}

func (m *mockReadWriter) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

func TestNewClient(t *testing.T) {
	mockRW := newMockReadWriter("")
	client := NewClient(mockRW, mockRW)

	assert.NotNil(t, client)
	assert.NotNil(t, client.reader)
	assert.NotNil(t, client.writer)
	assert.NotNil(t, client.scanner)
	assert.NotNil(t, client.pendingReqs)
	assert.NotNil(t, client.ctx)
	assert.NotNil(t, client.cancel)

	// Clean up
	client.Close()
}

func TestClient_Call_Success(t *testing.T) {
	// Use pipes for reliable communication
	pr, pw := io.Pipe()
	responseWriter := &bytes.Buffer{}

	client := NewClient(pr, responseWriter)
	defer client.Close()

	// Start response writer in goroutine
	go func() {
		defer pw.Close()
		time.Sleep(10 * time.Millisecond) // Let readLoop start
		responseJSON := `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`
		pw.Write([]byte(responseJSON + "\n"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := client.Call(ctx, "test_method", map[string]string{"param": "value"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ok"}`, string(result))

	// Verify request format
	written := responseWriter.String()
	assert.Contains(t, written, `"method":"test_method"`)
	assert.Contains(t, written, `"jsonrpc":"2.0"`)
}

func TestClient_Call_Error_Response(t *testing.T) {
	pr, pw := io.Pipe()
	responseWriter := &bytes.Buffer{}

	client := NewClient(pr, responseWriter)
	defer client.Close()

	go func() {
		defer pw.Close()
		time.Sleep(10 * time.Millisecond)
		errorResponse := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`
		pw.Write([]byte(errorResponse + "\n"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := client.Call(ctx, "nonexistent_method", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "RPC error -32601: Method not found")
}

func TestClient_Call_Context_Timeout(t *testing.T) {
	// No response will be sent - just empty reader
	mockRW := newMockReadWriter("")

	client := NewClient(mockRW, mockRW)
	defer client.Close()

	// Very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result, err := client.Call(ctx, "test_method", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestClient_Call_Client_Closed(t *testing.T) {
	mockRW := newMockReadWriter("")

	client := NewClient(mockRW, mockRW)
	client.Close() // Close immediately

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := client.Call(ctx, "test_method", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "client closed")
}

func TestClient_Call_Invalid_Params(t *testing.T) {
	mockRW := newMockReadWriter("")

	client := NewClient(mockRW, mockRW)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use a type that can't be marshaled to JSON
	invalidParams := make(chan int)

	result, err := client.Call(ctx, "test_method", invalidParams)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to marshal request")
}

func TestClient_Call_Write_Error(t *testing.T) {
	// Create a mock that fails on write
	mockR := strings.NewReader("")
	mockW := &failingWriter{}

	client := NewClient(mockR, mockW)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := client.Call(ctx, "test_method", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to write request")
}

func TestClient_readLoop_Invalid_JSON(t *testing.T) {
	// Test that invalid JSON is skipped gracefully
	pr, pw := io.Pipe()
	responseWriter := &bytes.Buffer{}

	client := NewClient(pr, responseWriter)
	defer client.Close()

	go func() {
		defer pw.Close()
		time.Sleep(10 * time.Millisecond)
		// Send invalid JSON followed by valid response
		pw.Write([]byte("invalid json\n"))
		pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}` + "\n"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// The call should still succeed despite the invalid JSON
	result, err := client.Call(ctx, "test_method", nil)

	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ok"}`, string(result))
}

func TestClient_readLoop_No_ID(t *testing.T) {
	// Send response without ID (notification-style) - should not cause issues
	responseData := `{"jsonrpc":"2.0","result":{"status":"ok"}}`
	mockRW := newMockReadWriter(responseData + "\n")

	client := NewClient(mockRW, mockRW)
	defer client.Close()

	// Give time for readLoop to process
	time.Sleep(20 * time.Millisecond)

	// Should not panic or cause issues
	assert.NotNil(t, client)
}

func TestClient_Close(t *testing.T) {
	mockRW := newMockReadWriter("")
	client := NewClient(mockRW, mockRW)

	err := client.Close()
	assert.NoError(t, err)

	// Verify context is cancelled
	select {
	case <-client.ctx.Done():
		// Expected
	case <-time.After(10 * time.Millisecond):
		t.Error("Context should be cancelled after Close()")
	}
}

func TestRequest_JSONSerialization(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      123,
		Method:  "test_method",
		Params:  map[string]string{"key": "value"},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var unmarshaled Request
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.JSONRPC, unmarshaled.JSONRPC)
	assert.Equal(t, float64(123), unmarshaled.ID) // JSON numbers become float64
	assert.Equal(t, req.Method, unmarshaled.Method)
}

func TestResponse_JSONSerialization(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      456,
		Result:  json.RawMessage(`{"success":true}`),
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled Response
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.JSONRPC, unmarshaled.JSONRPC)
	assert.Equal(t, float64(456), unmarshaled.ID)
	assert.JSONEq(t, string(resp.Result), string(unmarshaled.Result))
}

func TestErrorObject_JSONSerialization(t *testing.T) {
	errObj := ErrorObject{
		Code:    -32601,
		Message: "Method not found",
		Data:    map[string]string{"detail": "Unknown method"},
	}

	data, err := json.Marshal(errObj)
	require.NoError(t, err)

	var unmarshaled ErrorObject
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, errObj.Code, unmarshaled.Code)
	assert.Equal(t, errObj.Message, unmarshaled.Message)
}

func TestRequestID_Increment(t *testing.T) {
	mockRW := newMockReadWriter("")
	client := NewClient(mockRW, mockRW)
	defer client.Close()

	// Check that request IDs increment
	initialID := client.requestID.Load()
	client.requestID.Add(1)
	nextID := client.requestID.Load()

	assert.Equal(t, initialID+1, nextID)
}

func TestClient_PendingRequests_Management(t *testing.T) {
	mockRW := newMockReadWriter("")
	client := NewClient(mockRW, mockRW)
	defer client.Close()

	// Test that pending requests map starts empty
	client.mu.RLock()
	assert.Empty(t, client.pendingReqs)
	client.mu.RUnlock()

	// Test adding and removing entries
	testChan := make(chan *Response, 1)
	client.mu.Lock()
	client.pendingReqs[123] = testChan
	client.mu.Unlock()

	client.mu.RLock()
	assert.Len(t, client.pendingReqs, 1)
	assert.Equal(t, testChan, client.pendingReqs[123])
	client.mu.RUnlock()

	client.mu.Lock()
	delete(client.pendingReqs, 123)
	client.mu.Unlock()

	client.mu.RLock()
	assert.Empty(t, client.pendingReqs)
	client.mu.RUnlock()
}

// failingWriter is a mock writer that always fails
type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write failed")
}

// Benchmark tests
func BenchmarkJSON_Marshal_Request(b *testing.B) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_method",
		Params:  map[string]interface{}{"key": "value", "number": 42},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(req)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Unmarshal_Response(b *testing.B) {
	responseData := []byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"ok","data":[1,2,3,4,5]}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp Response
		err := json.Unmarshal(responseData, &resp)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}
