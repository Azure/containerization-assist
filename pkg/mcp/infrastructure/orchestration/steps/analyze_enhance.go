package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	aisample "github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
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

	// Use the file tree that was already generated in repository analysis
	fileTree := extractFileTree(analyzeResult.Analysis, logger)

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
	samplingClient := aisample.CreateDomainClient(logger)

	// Create enhanced prompt with structured JSON request
	enhancedPrompt := fmt.Sprintf(`Analyze this repository and provide enhanced analysis in JSON format.

Initial Analysis:
%s

File Tree:
%s

README Content:
%s

Please respond with ONLY a valid JSON object in this exact format:
{
  "language": "detected_language",
  "framework": "detected_framework", 
  "suggested_ports": [8080, 3000],
  "build_tools": ["npm", "webpack"],
  "services": ["web", "api"],
  "environment_vars": ["PORT", "NODE_ENV"],
  "dockerfile_suggestions": ["use multi-stage build", "use alpine base"],
  "security_considerations": ["avoid running as root", "scan for vulnerabilities"],
  "confidence": 0.85
}

Focus on containerization-relevant insights and provide actionable recommendations.`, initialAnalysis, fileTree, readmeContent)

	req := sampling.Request{
		Prompt:      enhancedPrompt,
		MaxTokens:   2048,
		Temperature: 0.3, // Lower temperature for more consistent JSON output
	}

	response, err := samplingClient.Sample(ctx, req) //NOTE: Sampling is currently limited as the LLM can only work off the data we provide to it. It does not search through and read the repository.
	if err != nil {
		logger.Warn("Failed to enhance repository analysis with AI", "error", err)
		// Return original analysis if AI enhancement fails
		return analyzeResult, nil
	}

	// Parse the JSON response
	enhancedAnalysis, err := parseAIResponse(response.Content, logger)
	if err != nil {
		logger.Warn("Failed to parse AI response", "error", err, "response", response.Content)
		// Return original analysis if parsing fails
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

// extractFileTree extracts and marshals the file tree structure from analysis data
func extractFileTree(analysis map[string]interface{}, logger *slog.Logger) string {
	structure, ok := analysis["structure"]
	if !ok {
		logger.Warn("No file tree structure found in analysis result")
		return "{}"
	}

	fileTreeBytes, err := json.MarshalIndent(structure, "", "  ")
	if err != nil {
		logger.Warn("Failed to marshal existing file tree structure", "error", err)
		return "{}"
	}

	return string(fileTreeBytes)
}

// extractJSON extracts JSON content from a potentially messy LLM response
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	startIdx := strings.Index(content, "{")
	if startIdx == -1 {
		return ""
	}

	endIdx := strings.LastIndex(content, "}")
	if endIdx == -1 || endIdx <= startIdx {
		return ""
	}

	return content[startIdx : endIdx+1]
}

// parseAIResponse parses the AI response JSON into a RepositoryAnalysis struct
func parseAIResponse(content string, logger *slog.Logger) (*aisample.RepositoryAnalysis, error) {
	cleaned := extractJSON(content)
	if cleaned == "" {
		return nil, fmt.Errorf("no valid JSON object found in response")
	}

	// Parse the JSON into a temporary struct
	var aiResponse struct {
		Language               string   `json:"language"`
		Framework              string   `json:"framework"`
		SuggestedPorts         []int    `json:"suggested_ports"`
		BuildTools             []string `json:"build_tools"`
		Services               []string `json:"services"`
		EnvironmentVars        []string `json:"environment_vars"`
		DockerfileSuggestions  []string `json:"dockerfile_suggestions"`
		SecurityConsiderations []string `json:"security_considerations"`
		Confidence             float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(cleaned), &aiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Convert to RepositoryAnalysis struct
	analysis := &aisample.RepositoryAnalysis{
		Language:        aiResponse.Language,
		Framework:       aiResponse.Framework,
		SuggestedPorts:  aiResponse.SuggestedPorts,
		BuildTools:      aiResponse.BuildTools,
		Services:        convertToServices(aiResponse.Services),
		EnvironmentVars: convertToEnvVars(aiResponse.EnvironmentVars),
		Confidence:      aiResponse.Confidence,
	}

	logger.Info("Successfully parsed AI response",
		"language", analysis.Language,
		"framework", analysis.Framework,
		"confidence", analysis.Confidence,
		"ports", analysis.SuggestedPorts)

	return analysis, nil
}

// convertToServices converts string slice to Service slice
func convertToServices(serviceNames []string) []aisample.Service {
	services := make([]aisample.Service, len(serviceNames))
	for i, name := range serviceNames {
		services[i] = aisample.Service{
			Name:     name,
			Type:     "unknown", // AI would need to specify this in a more structured response
			Required: true,
		}
	}
	return services
}

// convertToEnvVars converts string slice to EnvVar slice
func convertToEnvVars(varNames []string) []aisample.EnvVar {
	envVars := make([]aisample.EnvVar, len(varNames))
	for i, name := range varNames {
		envVars[i] = aisample.EnvVar{
			Name:        name,
			Description: fmt.Sprintf("Environment variable: %s", name),
			Required:    true,
			Type:        "string",
		}
	}
	return envVars
}

// parseEnhancedAnalysis extracts improvements from AI analysis
func mergeEnhancedAnalysis(enhancedAnalysis *aisample.RepositoryAnalysis, original *AnalyzeResult, logger *slog.Logger) *AnalyzeResult {
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
