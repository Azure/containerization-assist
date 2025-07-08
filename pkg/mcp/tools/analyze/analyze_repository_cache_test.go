package analyze

import (
	"io"
	"testing"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/stretchr/testify/assert"
)

func TestCacheManager_NewCacheManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	assert.NotNil(t, cacheManager)
	// Logger comparison removed - slog loggers with context are different instances
}

func TestCacheManager_PrepareForCachedResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	result := &AtomicAnalysisResult{}

	preparedResult := cacheManager.prepareForCachedResult(result)

	assert.NotNil(t, preparedResult.AnalysisContext)
	assert.NotNil(t, preparedResult.Analysis)
}

func TestCacheManager_IsAnalysisCacheValid(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	tests := []struct {
		name        string
		scanSummary map[string]interface{}
		result      *AtomicAnalysisResult
		sessionID   string
		expected    bool
	}{
		{
			name: "valid cache with matching repo path",
			scanSummary: map[string]interface{}{
				"repo_path": "/test/repo",
			},
			result: &AtomicAnalysisResult{
				CloneDir: "/test/repo",
			},
			sessionID: "test-session",
			expected:  true,
		},
		{
			name: "invalid cache with mismatched repo path",
			scanSummary: map[string]interface{}{
				"repo_path": "/different/repo",
			},
			result: &AtomicAnalysisResult{
				CloneDir: "/test/repo",
			},
			sessionID: "test-session",
			expected:  false,
		},
		{
			name:        "invalid cache with missing repo path",
			scanSummary: map[string]interface{}{},
			result: &AtomicAnalysisResult{
				CloneDir: "/test/repo",
			},
			sessionID: "test-session",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cacheManager.isAnalysisCacheValid(tt.scanSummary, tt.result, tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCacheManager_IsCacheTimeValid(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	tests := []struct {
		name        string
		scanSummary map[string]interface{}
		sessionID   string
		expected    bool
	}{
		{
			name: "valid cache within time window",
			scanSummary: map[string]interface{}{
				"cached_at": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
			},
			sessionID: "test-session",
			expected:  true,
		},
		{
			name: "expired cache outside time window",
			scanSummary: map[string]interface{}{
				"cached_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			},
			sessionID: "test-session",
			expected:  false,
		},
		{
			name:        "invalid cache with missing timestamp",
			scanSummary: map[string]interface{}{},
			sessionID:   "test-session",
			expected:    false,
		},
		{
			name: "invalid cache with malformed timestamp",
			scanSummary: map[string]interface{}{
				"cached_at": "invalid-timestamp",
			},
			sessionID: "test-session",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cacheManager.isCacheTimeValid(tt.scanSummary, tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCacheManager_ValidateRequiredCachedFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	tests := []struct {
		name        string
		scanSummary map[string]interface{}
		sessionID   string
		expected    bool
	}{
		{
			name: "valid cache with all required fields",
			scanSummary: map[string]interface{}{
				"language":  "javascript",
				"framework": "express",
				"port":      3000.0,
			},
			sessionID: "test-session",
			expected:  true,
		},
		{
			name: "invalid cache missing language",
			scanSummary: map[string]interface{}{
				"framework": "express",
				"port":      3000.0,
			},
			sessionID: "test-session",
			expected:  false,
		},
		{
			name: "invalid cache missing framework",
			scanSummary: map[string]interface{}{
				"language": "javascript",
				"port":     3000.0,
			},
			sessionID: "test-session",
			expected:  false,
		},
		{
			name: "invalid cache missing port",
			scanSummary: map[string]interface{}{
				"language":  "javascript",
				"framework": "express",
			},
			sessionID: "test-session",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cacheManager.validateRequiredCachedFields(tt.scanSummary, tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCacheManager_ExtractCachedDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	tests := []struct {
		name         string
		scanSummary  map[string]interface{}
		sessionID    string
		expectedDeps int
	}{
		{
			name: "valid dependencies array",
			scanSummary: map[string]interface{}{
				"dependencies": []interface{}{"express", "lodash", "moment"},
			},
			sessionID:    "test-session",
			expectedDeps: 3,
		},
		{
			name:         "missing dependencies",
			scanSummary:  map[string]interface{}{},
			sessionID:    "test-session",
			expectedDeps: 0,
		},
		{
			name: "invalid dependencies type",
			scanSummary: map[string]interface{}{
				"dependencies": "not-an-array",
			},
			sessionID:    "test-session",
			expectedDeps: 0,
		},
		{
			name: "mixed types in dependencies array",
			scanSummary: map[string]interface{}{
				"dependencies": []interface{}{"express", 123, "lodash"},
			},
			sessionID:    "test-session",
			expectedDeps: 2, // Only valid string entries
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := cacheManager.extractCachedDependencies(tt.scanSummary, tt.sessionID)
			assert.Len(t, deps, tt.expectedDeps)

			// Verify all returned dependencies have names
			for _, dep := range deps {
				assert.NotEmpty(t, dep.Name)
			}
		})
	}
}

func TestGetHelperFunctions(t *testing.T) {
	summary := map[string]interface{}{
		"int_field":    42.0,
		"int64_field":  1000.0,
		"float_field":  3.14,
		"bool_field":   true,
		"string_slice": []interface{}{"item1", "item2", "item3"},
	}

	// Test getIntFromSummary
	assert.Equal(t, 42, getIntFromSummary(summary, "int_field"))
	assert.Equal(t, 0, getIntFromSummary(summary, "missing_field"))

	// Test getInt64FromSummary
	assert.Equal(t, int64(1000), getInt64FromSummary(summary, "int64_field"))
	assert.Equal(t, int64(0), getInt64FromSummary(summary, "missing_field"))

	// Test getFloat64FromSummary
	assert.Equal(t, 3.14, getFloat64FromSummary(summary, "float_field"))
	assert.Equal(t, 0.0, getFloat64FromSummary(summary, "missing_field"))

	// Test getBoolFromSummary
	assert.True(t, getBoolFromSummary(summary, "bool_field"))
	assert.False(t, getBoolFromSummary(summary, "missing_field"))

	// Test getStringSliceFromSummary
	slice := getStringSliceFromSummary(summary, "string_slice")
	assert.Len(t, slice, 3)
	assert.Equal(t, []string{"item1", "item2", "item3"}, slice)
	assert.Empty(t, getStringSliceFromSummary(summary, "missing_field"))
}

func TestAtomicAnalyzeRepositoryTool_CheckCacheIntegration(t *testing.T) {
	// This test verifies that the checkCache method is properly integrated
	// with the AtomicAnalyzeRepositoryTool

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tool := &AtomicAnalyzeRepositoryTool{
		logger: logger,
	}

	// Test with nil metadata (no cache)
	session := &core.SessionState{
		SessionID: "test-session",
		Metadata:  nil,
	}

	result := &AtomicAnalysisResult{
		CloneDir: "/test/repo",
	}

	startTime := time.Now()
	cachedResult := tool.checkCache(session, result, startTime)

	// Should return nil when no cache exists
	assert.Nil(t, cachedResult)
}
