package analyze

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// LLMTransport interface for local use (to avoid import cycles)
type LLMTransport interface {
	SendPrompt(prompt string) (string, error)
}

// CallerAnalyzer forwards prompts to the hosting LLM via LLMTransport
// This allows MCP tools to get AI reasoning without external dependencies
type CallerAnalyzer struct {
	transport      LLMTransport
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
var _ mcptypes.AIAnalyzer = (*CallerAnalyzer)(nil)
var _ mcptypes.AIAnalyzer = (*StubAnalyzer)(nil)

// NewCallerAnalyzer creates an analyzer that sends prompts back to the hosting LLM
func NewCallerAnalyzer(transport LLMTransport, opts CallerAnalyzerOpts) *CallerAnalyzer {
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

	response, err := c.transport.SendPrompt(fullPrompt)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to send prompt to hosting LLM")
		return "", fmt.Errorf("failed to analyze via hosting LLM: %w", err)
	}

	if response == "" {
		return "", fmt.Errorf("received empty response from hosting LLM")
	}

	result := strings.TrimSpace(response)
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

// GetTokenUsage implements AIAnalyzer interface
// For MCP, we don't track token usage as the hosting LLM handles this
func (c *CallerAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return mcptypes.TokenUsage{} // Always empty for MCP
}

// ResetTokenUsage implements AIAnalyzer interface
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

// Analyze implements AIAnalyzer interface with stub behavior
func (s *StubAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI analysis not available in MCP mode")
}

// AnalyzeWithFileTools implements AIAnalyzer interface with stub behavior
func (s *StubAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI file analysis not available in MCP mode")
}

// AnalyzeWithFormat implements AIAnalyzer interface with stub behavior
func (s *StubAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return "", fmt.Errorf("stub analyzer: AI analysis not available in MCP mode")
}

// GetTokenUsage implements AIAnalyzer interface
func (s *StubAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return mcptypes.TokenUsage{}
}

// ResetTokenUsage implements AIAnalyzer interface
func (s *StubAnalyzer) ResetTokenUsage() {
	// No-op for stub
}

// AnalyzerFactory creates the appropriate analyzer based on configuration
type AnalyzerFactory struct {
	logger       zerolog.Logger
	enableAI     bool
	transport    LLMTransport
	analyzerOpts CallerAnalyzerOpts
}

// NewAnalyzerFactory creates a new analyzer factory
func NewAnalyzerFactory(logger zerolog.Logger, enableAI bool, transport LLMTransport) *AnalyzerFactory {
	return &AnalyzerFactory{
		logger:    logger,
		enableAI:  enableAI,
		transport: transport,
		analyzerOpts: CallerAnalyzerOpts{
			ToolName:       "chat",
			SystemPrompt:   "You are an AI assistant helping with container analysis and deployment.",
			PerCallTimeout: 60 * time.Second,
		},
	}
}

// SetAnalyzerOptions configures the CallerAnalyzer options
func (f *AnalyzerFactory) SetAnalyzerOptions(opts CallerAnalyzerOpts) {
	f.analyzerOpts = opts
}

// CreateAnalyzer creates the appropriate analyzer based on configuration
func (f *AnalyzerFactory) CreateAnalyzer() mcptypes.AIAnalyzer {
	if f.enableAI && f.transport != nil {
		f.logger.Info().Msg("Creating CallerAnalyzer for AI-enabled mode")
		return NewCallerAnalyzer(f.transport, f.analyzerOpts)
	}

	f.logger.Info().Msg("Creating StubAnalyzer (AI disabled or no transport)")
	return NewStubAnalyzer()
}

// CreateAnalyzerFromEnv creates an analyzer based on environment configuration
// Note: This returns a stub analyzer since we don't have transport available here
func CreateAnalyzerFromEnv(logger zerolog.Logger) mcptypes.AIAnalyzer {
	// Use centralized configuration logic
	config := DefaultAnalyzerConfig()
	config.LoadFromEnv()

	// Delegate to the config-based creator
	return CreateAnalyzerFromConfig(config, logger)
}
