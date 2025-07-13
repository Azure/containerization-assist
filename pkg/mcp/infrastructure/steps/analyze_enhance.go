package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// EnhanceRepositoryAnalysis uses AI to improve the initial repository analysis
func EnhanceRepositoryAnalysis(ctx context.Context, analyzeResult *AnalyzeResult, logger *slog.Logger) (*AnalyzeResult, error) {
	logger.Info("Enhancing repository analysis with AI",
		"initial_language", analyzeResult.Language,
		"initial_framework", analyzeResult.Framework)

	// Read README if available
	readmeContent := ""
	readmePaths := []string{"README.md", "readme.md", "README.rst", "README.txt"}
	for _, readmePath := range readmePaths {
		fullPath := filepath.Join(analyzeResult.RepoPath, readmePath)
		if content, err := os.ReadFile(fullPath); err == nil {
			readmeContent = string(content)
			logger.Info("Found README file", "path", readmePath)
			break
		}
	}

	// Get file tree
	fileTree := getFileTree(analyzeResult.RepoPath, logger)

	// Create initial analysis summary
	initialAnalysis := fmt.Sprintf(`Language: %s
Framework: %s
Port: %d
Build Files: %v
Entry Points: %v
Database: %v
Database Types: %v`,
		analyzeResult.Language,
		analyzeResult.Framework,
		analyzeResult.Port,
		analyzeResult.Analysis["build_files"],
		analyzeResult.Analysis["entry_points"],
		analyzeResult.Analysis["database_detected"],
		analyzeResult.Analysis["database_types"])

	// Use AI to enhance the analysis
	samplingClient := sampling.NewClient(logger)
	enhancedAnalysis, err := samplingClient.ImproveRepositoryAnalysis(
		ctx,
		initialAnalysis,
		fileTree,
		readmeContent,
	)
	if err != nil {
		logger.Warn("Failed to enhance repository analysis with AI", "error", err)
		// Return original analysis if AI enhancement fails
		return analyzeResult, nil
	}

	// Use enhanced analysis to update fields
	enhanced := mergeEnhancedAnalysis(enhancedAnalysis, analyzeResult, logger)

	logger.Info("Repository analysis enhanced",
		"improved_language", enhanced.Language,
		"improved_framework", enhanced.Framework,
		"improved_port", enhanced.Port)

	return enhanced, nil
}

// getFileTree generates a simple file tree representation
func getFileTree(repoPath string, logger *slog.Logger) string {
	var result strings.Builder

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip .git directory
		if strings.Contains(path, ".git/") {
			return filepath.SkipDir
		}

		// Get relative path
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}

		// Skip if too deep (more than 3 levels)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > 3 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Add to result with indentation
		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			result.WriteString(fmt.Sprintf("%s%s/\n", indent, info.Name()))
		} else {
			result.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
		}

		return nil
	})

	if err != nil {
		logger.Warn("Error generating file tree", "error", err)
	}

	return result.String()
}

// parseEnhancedAnalysis extracts improvements from AI analysis
func mergeEnhancedAnalysis(enhancedAnalysis *sampling.RepositoryAnalysis, original *AnalyzeResult, logger *slog.Logger) *AnalyzeResult {
	// Create a copy of the original
	result := &AnalyzeResult{
		Language:  original.Language,
		Framework: original.Framework,
		Port:      original.Port,
		Analysis:  make(map[string]interface{}),
		RepoPath:  original.RepoPath,
		SessionID: original.SessionID,
	}

	// Copy original analysis
	for k, v := range original.Analysis {
		result.Analysis[k] = v
	}

	// Merge enhanced analysis data
	if enhancedAnalysis.Language != "" {
		result.Language = enhancedAnalysis.Language
	}
	if enhancedAnalysis.Framework != "" {
		result.Framework = enhancedAnalysis.Framework
	}
	if len(enhancedAnalysis.SuggestedPorts) > 0 {
		result.Port = enhancedAnalysis.SuggestedPorts[0]
	}

	// Add enhanced data to analysis
	result.Analysis["ai_enhanced"] = true
	result.Analysis["build_tools"] = enhancedAnalysis.BuildTools
	result.Analysis["dependencies"] = enhancedAnalysis.Dependencies
	result.Analysis["services"] = enhancedAnalysis.Services
	result.Analysis["entry_points"] = enhancedAnalysis.EntryPoints
	result.Analysis["environment_vars"] = enhancedAnalysis.EnvironmentVars
	result.Analysis["suggested_ports"] = enhancedAnalysis.SuggestedPorts
	result.Analysis["confidence"] = enhancedAnalysis.Confidence

	logger.Info("Successfully merged enhanced analysis",
		"language", result.Language,
		"framework", result.Framework,
		"port", result.Port,
		"confidence", enhancedAnalysis.Confidence)

	return result
}

// parsePort extracts a port number from a string
func parsePort(s string) int {
	// Remove non-numeric characters
	cleaned := strings.TrimFunc(s, func(r rune) bool {
		return r < '0' || r > '9'
	})

	var port int
	fmt.Sscanf(cleaned, "%d", &port)

	// Validate port range
	if port > 0 && port < 65536 {
		return port
	}
	return 0
}
