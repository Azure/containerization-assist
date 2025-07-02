package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write request with newline
	if _, err := fmt.Fprintf(c.writer, "%s\n", reqBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, fmt.Errorf("client closed")
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
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
