package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// CallerAnalyzer forwards prompts to the hosting LLM via LLMTransport
// This allows MCP tools to get AI reasoning without external dependencies
type CallerAnalyzer struct {
	transport      contract.LLMTransport
	toolName       string
	systemPreamble string
	timeout        time.Duration
	logger         zerolog.Logger
}

// CallerAnalyzerOpts configures the CallerAnalyzer
type CallerAnalyzerOpts struct {
	ToolName       string        // tool name to invoke (default: "chat")
	SystemPrompt   string        // system prompt prefix
	PerCallTimeout time.Duration // timeout per call (default: 60s)
}

// Ensure interface compliance at compile time.
var _ Analyzer = (*CallerAnalyzer)(nil)

// NewCallerAnalyzer creates an analyzer that sends prompts back to the hosting LLM
func NewCallerAnalyzer(transport contract.LLMTransport, opts CallerAnalyzerOpts) *CallerAnalyzer {
	if opts.ToolName == "" {
		opts.ToolName = "chat"
	}
	if opts.PerCallTimeout == 0 {
		opts.PerCallTimeout = 60 * time.Second
	}

	return &CallerAnalyzer{
		transport:      transport,
		toolName:       opts.ToolName,
		systemPreamble: opts.SystemPrompt,
		timeout:        opts.PerCallTimeout,
		logger:         zerolog.New(nil).With().Str("component", "caller_analyzer").Logger(),
	}
}

// Analyze implements ai.Analyzer interface by sending prompt back to hosting LLM
func (c *CallerAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Hash prompt for privacy-safe logging
	promptHash := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt)))
	c.logger.Debug().
		Str("prompt_hash", promptHash[:8]).
		Str("tool", c.toolName).
		Msg("Sending analysis request to hosting LLM")

	// Build the payload
	fullPrompt := prompt
	if c.systemPreamble != "" {
		fullPrompt = c.systemPreamble + "\n\n" + prompt
	}

	args := map[string]any{
		"session_id": uuid.NewString(),
		"message":    fullPrompt,
		"stream":     false,
	}

	ch, err := c.transport.InvokeTool(ctx, c.toolName, args, false)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to invoke tool on hosting LLM")
		return "", fmt.Errorf("failed to analyze via hosting LLM: %w", err)
	}

	var reply json.RawMessage
	select {
	case reply = <-ch:
		if reply == nil {
			return "", fmt.Errorf("received nil response from hosting LLM")
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	var out contract.ToolInvocationResponse
	if err := json.Unmarshal(reply, &out); err != nil {
		c.logger.Error().Err(err).Msg("Failed to unmarshal LLM response")
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if out.Error != "" {
		c.logger.Error().Str("llm_error", out.Error).Msg("LLM returned error")
		return "", fmt.Errorf("LLM error: %s", out.Error)
	}

	result := strings.TrimSpace(out.Content)
	c.logger.Debug().
		Str("response_hash", fmt.Sprintf("%x", sha256.Sum256([]byte(result)))[:8]).
		Int("response_len", len(result)).
		Msg("Received analysis from hosting LLM")

	return result, nil
}

// AnalyzeWithFileTools implements ai.Analyzer interface
// For MCP, we send file context along with the prompt to the hosting LLM
func (c *CallerAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	// Create enhanced prompt with file context information
	enhancedPrompt := fmt.Sprintf("%s\n\nBase directory: %s\nNote: Use file reading tools to examine the codebase as needed.", prompt, baseDir)

	c.logger.Debug().
		Str("base_dir", baseDir).
		Msg("Sending file-based analysis request to hosting LLM")

	// Delegate to the main Analyze method with enhanced prompt
	return c.Analyze(ctx, enhancedPrompt)
}

// AnalyzeWithFormat implements ai.Analyzer interface
func (c *CallerAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	formattedPrompt := fmt.Sprintf(promptTemplate, args...)
	return c.Analyze(ctx, formattedPrompt)
}

// GetTokenUsage implements Analyzer interface
// For MCP, we don't track token usage as the hosting LLM handles this
func (c *CallerAnalyzer) GetTokenUsage() TokenUsage {
	return TokenUsage{} // Always empty for MCP
}

// ResetTokenUsage implements Analyzer interface
// No-op for MCP as we don't track token usage
func (c *CallerAnalyzer) ResetTokenUsage() {
	// No-op for MCP
}

// StubAnalyzer provides a no-op implementation for testing or when AI is disabled
type StubAnalyzer struct{}

// NewStubAnalyzer creates a stub analyzer that returns empty responses
func NewStubAnalyzer() *StubAnalyzer {
	return &StubAnalyzer{}
}

// Analyze implements Analyzer interface with stub behavior
func (s *StubAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI analysis not available in MCP mode")
}

// AnalyzeWithFileTools implements Analyzer interface with stub behavior
func (s *StubAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI file analysis not available in MCP mode")
}

// AnalyzeWithFormat implements Analyzer interface with stub behavior
func (s *StubAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI analysis not available in MCP mode")
}

// GetTokenUsage implements Analyzer interface
func (s *StubAnalyzer) GetTokenUsage() TokenUsage {
	return TokenUsage{}
}

// ResetTokenUsage implements Analyzer interface
func (s *StubAnalyzer) ResetTokenUsage() {
	// No-op for stub
}
