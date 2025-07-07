package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
)

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// ErrorObject represents a JSON-RPC 2.0 error
type ErrorObject struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Client provides bidirectional JSON-RPC communication over stdio
type Client struct {
	reader      io.Reader
	writer      io.Writer
	scanner     *bufio.Scanner
	requestID   atomic.Uint64
	pendingReqs map[uint64]chan *Response
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewClient creates a new JSON-RPC client for stdio communication
func NewClient(reader io.Reader, writer io.Writer) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		reader:      reader,
		writer:      writer,
		scanner:     bufio.NewScanner(reader),
		pendingReqs: make(map[uint64]chan *Response),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start reading responses
	go client.readLoop()

	return client
}

// Call sends a JSON-RPC request and waits for a response
func (c *Client) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Generate request ID
	id := c.requestID.Add(1)

	// Create response channel
	respChan := make(chan *Response, 1)
	c.mu.Lock()
	c.pendingReqs[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pendingReqs, id)
		c.mu.Unlock()
	}()

	// Create and send request
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		networkErr := errors.NetworkError(
			codes.NETWORK_ERROR,
			"Failed to marshal request",
			err,
		)
		networkErr.Context["component"] = "transport_client"
		return nil, networkErr
	}

	// Write request with newline
	if _, err := fmt.Fprintf(c.writer, "%s\n", reqBytes); err != nil {
		networkErr := errors.NetworkError(
			codes.NETWORK_ERROR,
			"Failed to write request",
			err,
		)
		networkErr.Context["component"] = "transport_client"
		return nil, networkErr
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		systemErr := errors.SystemError(
			codes.SYSTEM_UNAVAILABLE,
			"Client closed",
			nil,
		)
		systemErr.Context["component"] = "transport_client"
		return nil, systemErr
	case resp := <-respChan:
		if resp.Error != nil {
			networkErr := errors.NetworkError(
				codes.NETWORK_ERROR,
				fmt.Sprintf("RPC error %d: %s", resp.Error.Code, resp.Error.Message),
				nil,
			)
			networkErr.Context["rpc_code"] = resp.Error.Code
			networkErr.Context["component"] = "transport_client"
			return nil, networkErr
		}
		return resp.Result, nil
	}
}

// readLoop continuously reads responses from the reader
func (c *Client) readLoop() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Skip invalid JSON
			continue
		}

		// Match response to pending request
		if resp.ID != nil {
			c.mu.RLock()
			if ch, ok := c.pendingReqs[uint64(resp.ID.(float64))]; ok {
				c.mu.RUnlock()
				select {
				case ch <- &resp:
				default:
				}
			} else {
				c.mu.RUnlock()
			}
		}
	}
}

// Close shuts down the client
func (c *Client) Close() error {
	c.cancel()
	return nil
}
