package scan

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// ToolAnalyzer interface defines the contract for scan tool analyzers
type ToolAnalyzer interface {
	AnalyzeScanFailure(imageRef, sessionID string) error
	AnalyzeSecretsFailure(path, sessionID string) error
}

// DefaultToolAnalyzer provides default implementation for scan tool analysis
type DefaultToolAnalyzer struct {
	toolName string
	logger   zerolog.Logger
}

// NewDefaultToolAnalyzer creates a new default scan tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	return &DefaultToolAnalyzer{
		toolName: toolName,
		logger:   zerolog.Nop(),
	}
}

// SetLogger sets the logger for the analyzer
func (a *DefaultToolAnalyzer) SetLogger(logger zerolog.Logger) {
	a.logger = logger.With().Str("component", "scan_analyzer").Str("tool", a.toolName).Logger()
}

// AnalyzeScanFailure analyzes security scan failures
func (a *DefaultToolAnalyzer) AnalyzeScanFailure(imageRef, sessionID string) error {
	a.logger.Info().
		Str("image_ref", imageRef).
		Str("session_id", sessionID).
		Msg("Analyzing security scan failure")

	// Basic analysis logic
	if imageRef == "" {
		return fmt.Errorf("image reference is required for scan analysis")
	}

	// Check common scan failure patterns
	commonPatterns := []struct {
		pattern string
		advice  string
	}{
		{"not found", "Ensure the image exists and is accessible"},
		{"timeout", "Increase timeout or check network connectivity"},
		{"authentication", "Check registry credentials"},
		{"rate limit", "Wait and retry or upgrade plan"},
	}

	// Log analysis insights
	for _, p := range commonPatterns {
		if strings.Contains(strings.ToLower(imageRef), p.pattern) {
			a.logger.Info().
				Str("pattern", p.pattern).
				Str("advice", p.advice).
				Msg("Detected known failure pattern")
		}
	}

	return nil
}

// AnalyzeSecretsFailure analyzes secrets scan failures
func (a *DefaultToolAnalyzer) AnalyzeSecretsFailure(path, sessionID string) error {
	a.logger.Info().
		Str("path", path).
		Str("session_id", sessionID).
		Msg("Analyzing secrets scan failure")

	// Basic validation
	if path == "" {
		return fmt.Errorf("path is required for secrets scan analysis")
	}

	// Check common secrets scan failure patterns
	commonPatterns := []struct {
		pattern string
		advice  string
	}{
		{"permission denied", "Check file permissions"},
		{"no such file", "Verify the path exists"},
		{"too many files", "Consider scanning smaller directories"},
		{"memory", "Increase memory limits or scan in batches"},
	}

	// Log analysis insights
	for _, p := range commonPatterns {
		if strings.Contains(strings.ToLower(path), p.pattern) {
			a.logger.Info().
				Str("pattern", p.pattern).
				Str("advice", p.advice).
				Msg("Detected known failure pattern")
		}
	}

	return nil
}
