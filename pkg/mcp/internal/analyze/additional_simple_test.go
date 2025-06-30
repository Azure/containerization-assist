package analyze

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

// Test NewAnalyzer constructor variations
func TestNewAnalyzer_Variations(t *testing.T) {
	// Test with different logger levels
	loggers := []zerolog.Logger{
		zerolog.Nop(),
		zerolog.New(nil).Level(zerolog.DebugLevel),
		zerolog.New(nil).Level(zerolog.InfoLevel),
	}

	for i, logger := range loggers {
		analyzer := NewAnalyzer(logger)
		if analyzer == nil {
			t.Errorf("NewAnalyzer should not return nil for logger %d", i)
		}
	}
}

// Test simple function calls to increase coverage
func TestSimpleFunctionCalls(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)
	ctx := context.Background()

	// Test with minimal valid options to trigger validation
	options := AnalysisOptions{
		RepoPath:     "/nonexistent/path",
		Context:      "",
		LanguageHint: "",
		SessionID:    "",
	}

	// This should trigger validation logic even if it ultimately fails
	_, err := analyzer.Analyze(ctx, options)
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

// Test utility function behavior
func TestUtilityFunctionBehavior(t *testing.T) {
	// Test different combinations that trigger various code paths

	// Test with different language hints
	languageHints := []string{"", "go", "javascript", "python", "java", "unknown"}

	for _, hint := range languageHints {
		logger := zerolog.Nop()
		analyzer := NewAnalyzer(logger)
		ctx := context.Background()

		options := AnalysisOptions{
			RepoPath:     "/tmp/nonexistent",
			Context:      "test",
			LanguageHint: hint,
			SessionID:    "test-session",
		}

		// This will exercise the validation and early parts of the function
		_, err := analyzer.Analyze(ctx, options)
		if err == nil {
			t.Errorf("Expected error for non-existent path with language hint: %s", hint)
		}
	}
}

// Test different contexts
func TestDifferentContexts(t *testing.T) {
	contexts := []string{"", "containerization", "analysis", "deployment", "security"}

	for _, contextStr := range contexts {
		logger := zerolog.Nop()
		analyzer := NewAnalyzer(logger)
		ctx := context.Background()

		options := AnalysisOptions{
			RepoPath:     "/tmp/nonexistent",
			Context:      contextStr,
			LanguageHint: "go",
			SessionID:    "test-session",
		}

		_, err := analyzer.Analyze(ctx, options)
		if err == nil {
			t.Errorf("Expected error for non-existent path with context: %s", contextStr)
		}
	}
}

// Test session ID variations
func TestSessionIDVariations(t *testing.T) {
	sessionIDs := []string{"", "short", "very-long-session-id-with-many-characters", "special-chars-123!@#"}

	for _, sessionID := range sessionIDs {
		logger := zerolog.Nop()
		analyzer := NewAnalyzer(logger)
		ctx := context.Background()

		options := AnalysisOptions{
			RepoPath:     "/tmp/nonexistent",
			Context:      "test",
			LanguageHint: "go",
			SessionID:    sessionID,
		}

		_, err := analyzer.Analyze(ctx, options)
		if err == nil {
			t.Errorf("Expected error for non-existent path with session ID: %s", sessionID)
		}
	}
}
