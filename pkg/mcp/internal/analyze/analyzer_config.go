package analyze

import (
	"os"
	"strconv"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/config"
	"github.com/rs/zerolog"
)

// AnalyzerConfig holds configuration for the analyzer factory
// Deprecated: Use config.AnalyzerConfig instead
type AnalyzerConfig struct {
	// EnableAI determines whether to use CallerAnalyzer (true) or StubAnalyzer (false)
	EnableAI bool

	// LogLevel for analyzer operations
	LogLevel string

	// MaxPromptLength limits the size of prompts sent to the analyzer
	MaxPromptLength int

	// CacheEnabled determines if analyzer responses should be cached
	CacheEnabled bool

	// CacheTTLSeconds is the cache time-to-live in seconds
	CacheTTLSeconds int
}

// DefaultAnalyzerConfig returns the default configuration
// Deprecated: Use config.GetAnalyzer() instead
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		EnableAI:        false, // Default to stub for safety
		LogLevel:        "info",
		MaxPromptLength: 4096,
		CacheEnabled:    true,
		CacheTTLSeconds: 300, // 5 minutes
	}
}

// FromCentralConfig creates an AnalyzerConfig from the centralized config
func FromCentralConfig() *AnalyzerConfig {
	centralCfg, err := config.GetAnalyzer()
	if err != nil {
		// Fallback to default if centralized config not available
		return DefaultAnalyzerConfig()
	}

	return &AnalyzerConfig{
		EnableAI:        centralCfg.EnableAI,
		LogLevel:        centralCfg.AIAnalyzerLogLevel,
		MaxPromptLength: 4096, // TODO: Add to central config
		CacheEnabled:    centralCfg.CacheResults,
		CacheTTLSeconds: int(centralCfg.CacheTTL.Seconds()),
	}
}

// LoadFromEnv loads configuration from environment variables
// Deprecated: Environment variables are now handled by the centralized config system
func (c *AnalyzerConfig) LoadFromEnv() {
	logger := zerolog.New(os.Stderr).With().Str("component", "analyzer_config").Logger()
	logger.Warn().Msg("LoadFromEnv is deprecated - use centralized config system instead")

	// Try to load from centralized config first
	if centralCfg, err := config.GetAnalyzer(); err == nil {
		c.EnableAI = centralCfg.EnableAI
		c.LogLevel = centralCfg.AIAnalyzerLogLevel
		c.CacheEnabled = centralCfg.CacheResults
		c.CacheTTLSeconds = int(centralCfg.CacheTTL.Seconds())
		return
	}

	// Fallback to direct env var reading (deprecated path)
	if val := os.Getenv("MCP_ENABLE_AI_ANALYZER"); val != "" {
		c.EnableAI = val == "true"
	}

	if val := os.Getenv("MCP_ANALYZER_LOG_LEVEL"); val != "" {
		c.LogLevel = val
	}

	if val := os.Getenv("MCP_ANALYZER_MAX_PROMPT_LENGTH"); val != "" {
		if maxLen, err := strconv.Atoi(val); err == nil {
			c.MaxPromptLength = maxLen
		} else {
			logger.Warn().
				Err(err).
				Str("env_var", "MCP_ANALYZER_MAX_PROMPT_LENGTH").
				Str("invalid_value", val).
				Msg("Failed to parse MCP_ANALYZER_MAX_PROMPT_LENGTH, using default value")
		}
	}

	if val := os.Getenv("MCP_ANALYZER_CACHE_ENABLED"); val != "" {
		c.CacheEnabled = val == "true"
	}

	if val := os.Getenv("MCP_ANALYZER_CACHE_TTL"); val != "" {
		if ttl, err := strconv.Atoi(val); err == nil {
			c.CacheTTLSeconds = ttl
		} else {
			logger.Warn().
				Err(err).
				Str("env_var", "MCP_ANALYZER_CACHE_TTL").
				Str("invalid_value", val).
				Msg("Failed to parse MCP_ANALYZER_CACHE_TTL, using default value")
		}
	}
}

// CreateAnalyzerFromConfig creates an analyzer based on the provided configuration
// Note: For CallerAnalyzer, you need to use AnalyzerFactory with a transport
func CreateAnalyzerFromConfig(config *AnalyzerConfig, logger zerolog.Logger) mcp.AIAnalyzer {
	if config.EnableAI {
		logger.Warn().
			Bool("ai_enabled", true).
			Msg("AI analyzer requested but no transport provided - use AnalyzerFactory instead")
	}

	logger.Info().
		Bool("ai_enabled", false).
		Msg("Creating StubAnalyzer")
	return NewStubAnalyzer()
}
