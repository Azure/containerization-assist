// Package analyze provides caching functionality for repository analysis operations.
//
// This module handles cache validation, retrieval, and result construction for
// repository analysis operations. It manages cache expiration, validates cached
// data integrity, and constructs analysis results from cached metadata.
package analyze

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
)

// CacheManager handles cache operations for repository analysis results.
//
// This manager provides a clean interface for cache operations and ensures
// consistent cache behavior across different analysis operations.
type CacheManager struct {
	logger *slog.Logger
}

// NewCacheManager creates a new cache manager with the provided logger.
//
// Parameters:
//   - logger: The logger instance for cache operations
//
// Returns a configured CacheManager instance.
func NewCacheManager(logger *slog.Logger) *CacheManager {
	return &CacheManager{
		logger: logger.With("component", "cache-manager"),
	}
}

// checkAndUseCachedAnalysis checks for and returns cached analysis results if valid.
//
// This method performs comprehensive cache validation including:
// - Checking for cached metadata presence
// - Validating cache age (expires after 1 hour)
// - Verifying repository path matches current analysis
// - Validating required cached fields for data integrity
//
// Parameters:
//   - tool: The atomic analysis tool instance
//   - session: The current session state
//   - result: The analysis result to populate if cache is valid
//   - startTime: The start time for duration calculation
//
// Returns:
//   - *AtomicAnalysisResult: The cached result if valid, nil otherwise
func (c *CacheManager) checkAndUseCachedAnalysis(tool *AtomicAnalyzeRepositoryTool, session *core.SessionState, result *AtomicAnalysisResult, startTime time.Time) *AtomicAnalysisResult {
	// Check if session has metadata
	if session.Metadata == nil {
		c.logger.Debug("No session metadata found", "session_id", session.SessionID)
		return nil
	}

	// Look for cached scan summary
	scanSummaryData, exists := session.Metadata["scan_summary"]
	if !exists {
		c.logger.Debug("No scan summary found in session metadata", "session_id", session.SessionID)
		return nil
	}

	// Validate scan summary type
	scanSummary, ok := scanSummaryData.(map[string]interface{})
	if !ok {
		c.logger.Warn("scan_summary has invalid type, skipping cache check", "session_id", session.SessionID)
		return nil
	}

	// Validate repository path matches current analysis
	if !c.isAnalysisCacheValid(scanSummary, result, session.SessionID) {
		return nil
	}

	// Check if cached results are still valid (not expired)
	if !c.isCacheTimeValid(scanSummary, session.SessionID) {
		return nil
	}

	// Validate required cached fields
	if !c.validateRequiredCachedFields(scanSummary, session.SessionID) {
		return nil
	}

	// Construct and return cached result
	return c.constructCachedResult(tool, session, scanSummary, result, startTime)
}

// prepareForCachedResult prepares the analysis result for cached data population.
//
// This helper method ensures the result structure is ready to receive cached
// data and initializes any necessary fields.
//
// Parameters:
//   - result: The analysis result to prepare
//
// Returns the prepared result for chaining.
func (c *CacheManager) prepareForCachedResult(result *AtomicAnalysisResult) *AtomicAnalysisResult {
	// Ensure analysis context is initialized
	if result.AnalysisContext == nil {
		result.AnalysisContext = &AnalysisContext{}
	}

	// Initialize other required fields if needed
	if result.Analysis == nil {
		result.Analysis = &analysis.AnalysisResult{}
	}

	return result
}

// isAnalysisCacheValid validates that cached analysis is for the current repository.
//
// This method checks if the cached repository path matches the current analysis
// target to ensure cache consistency.
//
// Parameters:
//   - scanSummary: The cached scan summary data
//   - result: The current analysis result
//   - sessionID: The session ID for logging
//
// Returns:
//   - bool: True if cache is valid for current repository, false otherwise
func (c *CacheManager) isAnalysisCacheValid(scanSummary map[string]interface{}, result *AtomicAnalysisResult, sessionID string) bool {
	repoPath, ok := scanSummary["repo_path"].(string)
	if !ok {
		c.logger.Debug("No repo_path found in cached scan summary", "session_id", sessionID)
		return false
	}

	if repoPath != result.CloneDir {
		c.logger.Debug("Repository path mismatch, cache invalid",
			"session_id", sessionID,
			"cached_repo_path", repoPath,
			"current_repo_path", result.CloneDir)
		return false
	}

	return true
}

// isCacheTimeValid checks if the cached results are still within the valid time window.
//
// Cache entries expire after 1 hour to ensure analysis results remain current.
//
// Parameters:
//   - scanSummary: The cached scan summary data
//   - sessionID: The session ID for logging
//
// Returns:
//   - bool: True if cache is still valid, false if expired
func (c *CacheManager) isCacheTimeValid(scanSummary map[string]interface{}, sessionID string) bool {
	cachedAtStr, ok := scanSummary["cached_at"].(string)
	if !ok {
		c.logger.Debug("No cached_at timestamp found", "session_id", sessionID)
		return false
	}

	cachedAt, err := time.Parse(time.RFC3339, cachedAtStr)
	if err != nil {
		c.logger.Warn("Failed to parse cached_at timestamp",
			"session_id", sessionID,
			"cached_at", cachedAtStr,
			"error", err)
		return false
	}

	// Check if cache is expired (1 hour TTL)
	if time.Since(cachedAt) >= time.Hour {
		c.logger.Info("Cached analysis results are stale, performing fresh analysis",
			"session_id", sessionID,
			"cached_at", cachedAt,
			"cache_age", time.Since(cachedAt))
		return false
	}

	return true
}

// validateRequiredCachedFields ensures all required fields are present in cached data.
//
// This method validates that essential analysis fields are available in the
// cached data to ensure result integrity.
//
// Parameters:
//   - scanSummary: The cached scan summary data
//   - sessionID: The session ID for logging
//
// Returns:
//   - bool: True if all required fields are present, false otherwise
func (c *CacheManager) validateRequiredCachedFields(scanSummary map[string]interface{}, sessionID string) bool {
	// Validate required cached fields
	_, langOK := scanSummary["language"].(string)
	_, frameworkOK := scanSummary["framework"].(string)
	_, portOK := scanSummary["port"].(float64)

	if !langOK || !frameworkOK || !portOK {
		c.logger.Error("Invalid cached scan summary format - falling back to fresh analysis",
			"session_id", sessionID,
			"scan_summary", scanSummary)
		return false
	}

	return true
}

// constructCachedResult builds the analysis result from cached data.
//
// This method constructs a complete analysis result from cached metadata,
// including core analysis data, context information, and timing details.
//
// Parameters:
//   - tool: The atomic analysis tool instance
//   - session: The current session state
//   - scanSummary: The cached scan summary data
//   - result: The analysis result to populate
//   - startTime: The start time for duration calculation
//
// Returns:
//   - *AtomicAnalysisResult: The constructed cached result
func (c *CacheManager) constructCachedResult(_ *AtomicAnalyzeRepositoryTool, session *core.SessionState, scanSummary map[string]interface{}, result *AtomicAnalysisResult, startTime time.Time) *AtomicAnalysisResult {
	// Extract core analysis data
	lang := scanSummary["language"].(string)
	framework := scanSummary["framework"].(string)
	portFloat := scanSummary["port"].(float64)

	c.logger.Info("Using cached repository analysis results",
		"session_id", session.SessionID,
		"repo_path", result.CloneDir,
		"cached_at", func() time.Time {
			if cachedAtStr, ok := scanSummary["cached_at"].(string); ok {
				if cachedAt, err := time.Parse(time.RFC3339, cachedAtStr); err == nil {
					return cachedAt
				}
			}
			return time.Time{}
		}())

	// Build cached analysis result
	result.Analysis = &analysis.AnalysisResult{
		Language:     lang,
		Framework:    framework,
		Port:         int(portFloat),
		Dependencies: c.extractCachedDependencies(scanSummary, session.SessionID),
	}

	// Extract cached context
	result.AnalysisContext = c.extractCachedContext(scanSummary)

	// Set timing information
	result.AnalysisDuration = time.Duration(getFloat64FromSummary(scanSummary, "analysis_duration") * float64(time.Second))
	result.TotalDuration = time.Since(startTime)

	// Mark as successful
	result.Success = true
	result.IsSuccessful = true
	result.Duration = result.TotalDuration

	c.logger.Info("Repository analysis completed using cached results",
		"session_id", session.SessionID,
		"language", result.Analysis.Language,
		"framework", result.Analysis.Framework,
		"cached_analysis_duration", result.AnalysisDuration,
		"total_duration", result.TotalDuration)

	return result
}

// extractCachedDependencies safely extracts dependencies from cached data.
//
// This method handles the conversion of cached dependency data into the
// expected dependency structure, with proper error handling for invalid data.
//
// Parameters:
//   - scanSummary: The cached scan summary data
//   - sessionID: The session ID for logging
//
// Returns:
//   - []analysis.Dependency: The extracted dependencies
func (c *CacheManager) extractCachedDependencies(scanSummary map[string]interface{}, sessionID string) []analysis.Dependency {
	var dependencies []analysis.Dependency

	deps, ok := scanSummary["dependencies"].([]interface{})
	if !ok {
		c.logger.Debug("No dependencies found in cached data", "session_id", sessionID)
		return dependencies
	}

	for _, dep := range deps {
		if depName, ok := dep.(string); ok {
			dependencies = append(dependencies, analysis.Dependency{Name: depName})
		} else {
			c.logger.Warn("Skipping invalid dependency: expected string",
				"session_id", sessionID,
				"actual_type", fmt.Sprintf("%T", dep))
		}
	}

	c.logger.Debug("Extracted dependencies from cache",
		"session_id", sessionID,
		"dependency_count", len(dependencies))

	return dependencies
}

// extractCachedContext safely extracts analysis context from cached data.
//
// This method reconstructs the analysis context from cached metadata,
// including file structure insights, ecosystem information, and suggestions.
//
// Parameters:
//   - scanSummary: The cached scan summary data
//
// Returns:
//   - *AnalysisContext: The reconstructed analysis context
func (c *CacheManager) extractCachedContext(scanSummary map[string]interface{}) *AnalysisContext {
	return &AnalysisContext{
		// File structure insights
		FilesAnalyzed:    getIntFromSummary(scanSummary, "files_analyzed"),
		ConfigFilesFound: getStringSliceFromSummary(scanSummary, "config_files_found"),
		EntryPointsFound: getStringSliceFromSummary(scanSummary, "entry_points_found"),
		TestFilesFound:   getStringSliceFromSummary(scanSummary, "test_files_found"),
		BuildFilesFound:  getStringSliceFromSummary(scanSummary, "build_files_found"),

		// Language ecosystem insights
		PackageManagers: getStringSliceFromSummary(scanSummary, "package_managers"),
		DatabaseFiles:   getStringSliceFromSummary(scanSummary, "database_files"),
		DockerFiles:     getStringSliceFromSummary(scanSummary, "docker_files"),
		K8sFiles:        getStringSliceFromSummary(scanSummary, "k8s_files"),

		// Repository insights
		HasGitIgnore:   getBoolFromSummary(scanSummary, "has_git_ignore"),
		HasReadme:      getBoolFromSummary(scanSummary, "has_readme"),
		HasLicense:     getBoolFromSummary(scanSummary, "has_license"),
		HasCI:          getBoolFromSummary(scanSummary, "has_ci"),
		RepositorySize: getInt64FromSummary(scanSummary, "repository_size"),

		// Suggestions for containerization
		ContainerizationSuggestions: getStringSliceFromSummary(scanSummary, "containerization_suggestions"),
		NextStepSuggestions:         getStringSliceFromSummary(scanSummary, "next_step_suggestions"),
	}
}

// Integration methods for AtomicAnalyzeRepositoryTool

// checkCache is the main entry point for cache checking in the atomic analysis tool.
//
// This method integrates the cache manager with the atomic analysis tool,
// providing a clean interface for cache operations.
//
// Parameters:
//   - session: The current session state
//   - result: The analysis result to populate if cache is valid
//   - startTime: The start time for duration calculation
//
// Returns:
//   - *AtomicAnalysisResult: The cached result if valid, nil otherwise
func (t *AtomicAnalyzeRepositoryTool) checkCache(session *core.SessionState, result *AtomicAnalysisResult, startTime time.Time) *AtomicAnalysisResult {
	cacheManager := NewCacheManager(t.logger)
	return cacheManager.checkAndUseCachedAnalysis(t, session, result, startTime)
}

// Helper functions for extracting values from scan summary metadata

// getIntFromSummary safely extracts an integer value from scan summary metadata.
//
// Parameters:
//   - summary: The scan summary metadata map
//   - key: The key to extract
//
// Returns:
//   - int: The extracted integer value, or 0 if not found/invalid
func getIntFromSummary(summary map[string]interface{}, key string) int {
	if val, ok := summary[key]; ok {
		if intVal, ok := val.(float64); ok {
			return int(intVal)
		}
	}
	return 0
}

// getInt64FromSummary safely extracts an int64 value from scan summary metadata.
//
// Parameters:
//   - summary: The scan summary metadata map
//   - key: The key to extract
//
// Returns:
//   - int64: The extracted int64 value, or 0 if not found/invalid
func getInt64FromSummary(summary map[string]interface{}, key string) int64 {
	if val, ok := summary[key]; ok {
		if intVal, ok := val.(float64); ok {
			return int64(intVal)
		}
	}
	return 0
}

// getFloat64FromSummary safely extracts a float64 value from scan summary metadata.
//
// Parameters:
//   - summary: The scan summary metadata map
//   - key: The key to extract
//
// Returns:
//   - float64: The extracted float64 value, or 0.0 if not found/invalid
func getFloat64FromSummary(summary map[string]interface{}, key string) float64 {
	if val, ok := summary[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return 0.0
}

// getBoolFromSummary safely extracts a boolean value from scan summary metadata.
//
// Parameters:
//   - summary: The scan summary metadata map
//   - key: The key to extract
//
// Returns:
//   - bool: The extracted boolean value, or false if not found/invalid
func getBoolFromSummary(summary map[string]interface{}, key string) bool {
	if val, ok := summary[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// getStringSliceFromSummary safely extracts a string slice from scan summary metadata.
//
// Parameters:
//   - summary: The scan summary metadata map
//   - key: The key to extract
//
// Returns:
//   - []string: The extracted string slice, or empty slice if not found/invalid
func getStringSliceFromSummary(summary map[string]interface{}, key string) []string {
	if val, ok := summary[key]; ok {
		if slice, ok := val.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, item := range slice {
				if str, ok := item.(string); ok {
					result[i] = str
				}
			}
			return result
		}
	}
	return []string{}
}
