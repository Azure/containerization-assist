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

	// Parse enhanced analysis to update fields
	enhanced := parseEnhancedAnalysis(enhancedAnalysis, analyzeResult, logger)

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
func parseEnhancedAnalysis(enhancedText string, original *AnalyzeResult, logger *slog.Logger) *AnalyzeResult {
	// Create a copy of the original
	enhanced := &AnalyzeResult{
		Language:  original.Language,
		Framework: original.Framework,
		Port:      original.Port,
		Analysis:  make(map[string]interface{}),
		RepoPath:  original.RepoPath,
		SessionID: original.SessionID,
	}

	// Copy original analysis
	for k, v := range original.Analysis {
		enhanced.Analysis[k] = v
	}

	// Add AI enhancement to analysis
	enhanced.Analysis["ai_enhanced"] = true
	enhanced.Analysis["ai_analysis"] = enhancedText

	// Parse enhanced text for improvements
	lines := strings.Split(enhancedText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for framework improvements
		if strings.Contains(strings.ToLower(line), "framework:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				framework := strings.TrimSpace(parts[1])
				if framework != "" && framework != enhanced.Framework {
					logger.Info("AI detected better framework", "original", enhanced.Framework, "improved", framework)
					enhanced.Framework = framework
				}
			}
		}

		// Look for port suggestions
		if strings.Contains(strings.ToLower(line), "port:") || strings.Contains(strings.ToLower(line), "suggested port:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				portStr := strings.TrimSpace(parts[1])
				// Extract just the number
				for _, part := range strings.Fields(portStr) {
					if port := parsePort(part); port > 0 {
						if port != enhanced.Port {
							logger.Info("AI suggested different port", "original", enhanced.Port, "suggested", port)
							enhanced.Port = port
						}
						break
					}
				}
			}
		}

		// Look for build tool detection
		if strings.Contains(strings.ToLower(line), "build tool:") || strings.Contains(strings.ToLower(line), "package manager:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				buildTool := strings.TrimSpace(parts[1])
				enhanced.Analysis["build_tool"] = buildTool
			}
		}
	}

	return enhanced
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
