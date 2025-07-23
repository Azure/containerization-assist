package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
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
	samplingClient := sampling.NewSpecializedClient(logger)
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
	// Start with original analysis as the foundation
	result := &AnalyzeResult{
		Language:  original.Language,
		Framework: original.Framework,
		Port:      original.Port,
		Analysis:  make(map[string]interface{}),
		RepoPath:  original.RepoPath,
		SessionID: original.SessionID,
	}

	// Copy all original analysis data
	for k, v := range original.Analysis {
		result.Analysis[k] = v
	}

	// Use domain-specific merge strategies optimized for each field type
	result.Language = mergeLanguage(original.Language, enhancedAnalysis.Language, logger)
	result.Framework = mergeFramework(original.Framework, enhancedAnalysis.Framework, logger)
	result.Port = mergePort(original.Port, enhancedAnalysis.SuggestedPorts, logger)

	// Add AI insights as supplementary data
	result.Analysis["ai_enhanced"] = true
	result.Analysis["ai_build_tools"] = enhancedAnalysis.BuildTools
	result.Analysis["ai_services"] = enhancedAnalysis.Services
	result.Analysis["ai_environment_vars"] = enhancedAnalysis.EnvironmentVars
	result.Analysis["ai_suggested_ports"] = enhancedAnalysis.SuggestedPorts
	result.Analysis["ai_confidence"] = enhancedAnalysis.Confidence

	// Preserve both static and AI results for comparison/debugging
	result.Analysis["static_language"] = original.Language
	result.Analysis["static_framework"] = original.Framework
	result.Analysis["static_port"] = original.Port
	result.Analysis["ai_language"] = enhancedAnalysis.Language
	result.Analysis["ai_framework"] = enhancedAnalysis.Framework
	if len(enhancedAnalysis.SuggestedPorts) > 0 {
		result.Analysis["ai_port"] = enhancedAnalysis.SuggestedPorts[0]
	}

	// Record merge strategy for transparency
	result.Analysis["merge_strategy"] = "hybrid_domain_specific"

	// Keep original static analysis data intact
	if deps, ok := original.Analysis["dependencies"]; ok {
		result.Analysis["static_dependencies_count"] = deps
	}
	if entryPoints, ok := original.Analysis["entry_points"]; ok {
		result.Analysis["static_entry_points"] = entryPoints
	}

	logger.Info("Successfully merged enhanced analysis",
		"final_language", result.Language,
		"final_framework", result.Framework,
		"final_port", result.Port,
		"ai_confidence", enhancedAnalysis.Confidence,
		"has_static_analysis", original.Analysis != nil)

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

// mergeLanguage uses static-first strategy since file extensions are highly reliable
func mergeLanguage(staticLang, aiLang string, logger *slog.Logger) string {
	// Fill gaps: AI provides missing language
	if staticLang == "" && aiLang != "" {
		logger.Info("AI provided missing language", "ai_language", aiLang)
		return aiLang
	}

	// Static wins: File extensions are authoritative
	if staticLang != "" {
		if aiLang != "" && aiLang != staticLang {
			logger.Debug("Static language detection preferred over AI",
				"static_language", staticLang,
				"ai_language", aiLang,
				"reason", "file extensions are authoritative")
		}
		return staticLang
	}

	return staticLang // Empty if both are empty
}

// mergeFramework uses AI-first strategy since AI can read README and infer better
func mergeFramework(staticFramework, aiFramework string, logger *slog.Logger) string {
	// Fill gaps: AI provides missing framework
	if staticFramework == "" && aiFramework != "" {
		logger.Info("AI provided missing framework", "ai_framework", aiFramework)
		return aiFramework
	}

	// AI wins: AI is better at framework inference from README/context
	if aiFramework != "" && aiFramework != staticFramework {
		logger.Info("AI framework detection preferred over static",
			"static_framework", staticFramework,
			"ai_framework", aiFramework,
			"reason", "AI better at README/context analysis")
		return aiFramework
	}

	// Use static if AI provided nothing different
	if staticFramework != "" {
		return staticFramework
	}

	return aiFramework // Use AI even if empty, if it's all we have
}

// mergePort uses static-first with AI fallback strategy
func mergePort(staticPort int, aiPorts []int, logger *slog.Logger) int {
	aiPort := 0
	if len(aiPorts) > 0 {
		aiPort = aiPorts[0]
	}

	// Fill gaps: AI provides missing port
	if staticPort == 0 && aiPort > 0 {
		logger.Info("AI provided missing port", "ai_port", aiPort)
		return aiPort
	}

	// Static wins: Actual config files beat framework defaults
	if staticPort > 0 {
		if aiPort > 0 && aiPort != staticPort {
			logger.Debug("Static port detection preferred over AI",
				"static_port", staticPort,
				"ai_port", aiPort,
				"reason", "actual config beats framework defaults")
		}
		return staticPort
	}

	return aiPort // Use AI suggestion if static found nothing
}
