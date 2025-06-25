package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// HTTPLLMTransport implements types.LLMTransport for HTTP transport
// It can invoke tools back to the hosting LLM via HTTP requests
type HTTPLLMTransport struct {
	client  *http.Client
	baseURL string
	apiKey  string
	logger  zerolog.Logger
}

// HTTPLLMTransportConfig configures the HTTP LLM transport
type HTTPLLMTransportConfig struct {
	BaseURL string        // Base URL for the hosting LLM API
	APIKey  string        // API key for authentication
	Timeout time.Duration // HTTP timeout (default: 30s)
}

// NewHTTPLLMTransport creates a new HTTP LLM transport
func NewHTTPLLMTransport(config HTTPLLMTransportConfig, logger zerolog.Logger) *HTTPLLMTransport {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &HTTPLLMTransport{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		logger:  logger.With().Str("component", "http_llm_transport").Logger(),
	}
}

// InvokeTool implements types.LLMTransport
// For HTTP, this means making an HTTP request to the hosting LLM
func (h *HTTPLLMTransport) InvokeTool(ctx context.Context, name string, payload map[string]any, stream bool) (<-chan json.RawMessage, error) {
	h.logger.Debug().
		Str("tool_name", name).
		Bool("stream", stream).
		Str("base_url", h.baseURL).
		Msg("Invoking tool on hosting LLM via HTTP")

	// Create a response channel
	responseCh := make(chan json.RawMessage, 1)

	go func() {
		defer close(responseCh)

		// For HTTP transport, we need to know the LLM's API endpoint
		// This is environment/deployment specific
		if h.baseURL == "" {
			h.logger.Error().Msg("Base URL not configured for HTTP LLM transport")

			errorResponse := types.ToolInvocationResponse{
				Content: "",
				Error:   "HTTP LLM transport not configured (missing base URL)",
			}

			if responseBytes, err := json.Marshal(errorResponse); err == nil {
				select {
				case responseCh <- json.RawMessage(responseBytes):
				case <-ctx.Done():
				}
			}
			return
		}

		// Build the request
		requestPayload := map[string]interface{}{
			"tool":    name,
			"payload": payload,
			"stream":  stream,
		}

		requestBytes, err := json.Marshal(requestPayload)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal request payload")
			return
		}

		// Create HTTP request
		url := fmt.Sprintf("%s/tools/invoke", h.baseURL)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBytes))
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to create HTTP request")
			return
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		if h.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+h.apiKey)
		}

		// Make the request
		resp, err := h.client.Do(req)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to make HTTP request to hosting LLM")

			errorResponse := types.ToolInvocationResponse{
				Content: "",
				Error:   fmt.Sprintf("HTTP request failed: %v", err),
			}

			if responseBytes, err := json.Marshal(errorResponse); err == nil {
				select {
				case responseCh <- json.RawMessage(responseBytes):
				case <-ctx.Done():
				}
			}
			return
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				// Log but don't fail - response already processed
				h.logger.Warn().Err(err).Msg("Failed to close response body")
			}
		}()

		// Read response
		responseBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to read HTTP response")
			return
		}

		// Check HTTP status
		if resp.StatusCode != http.StatusOK {
			h.logger.Error().
				Int("status_code", resp.StatusCode).
				Str("response", string(responseBytes)).
				Msg("HTTP request returned error status")

			errorResponse := types.ToolInvocationResponse{
				Content: "",
				Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(responseBytes)),
			}

			if errorBytes, err := json.Marshal(errorResponse); err == nil {
				select {
				case responseCh <- json.RawMessage(errorBytes):
				case <-ctx.Done():
				}
			}
			return
		}

		h.logger.Debug().
			Int("response_size", len(responseBytes)).
			Msg("Received HTTP response from hosting LLM")

		// Send response
		select {
		case responseCh <- json.RawMessage(responseBytes):
		case <-ctx.Done():
			h.logger.Debug().Msg("Context cancelled while sending HTTP response")
		}
	}()

	return responseCh, nil
}

// Ensure interface compliance
var _ types.LLMTransport = (*HTTPLLMTransport)(nil)
