package tools

import (
	"context"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/analysis"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// AnalysisAdapter integrates the new modular analysis engines with the existing atomic tool
type AnalysisAdapter struct {
	orchestrator *analysis.AnalysisOrchestrator
	logger       zerolog.Logger
}

// NewAnalysisAdapter creates a new analysis adapter
func NewAnalysisAdapter(logger zerolog.Logger) *AnalysisAdapter {
	orchestrator := analysis.NewAnalysisOrchestrator(logger)

	// Register all analysis engines
	orchestrator.RegisterEngine(analysis.NewLanguageAnalyzer(logger))
	orchestrator.RegisterEngine(analysis.NewDependencyAnalyzer(logger))
	orchestrator.RegisterEngine(analysis.NewConfigurationAnalyzer(logger))
	orchestrator.RegisterEngine(analysis.NewBuildAnalyzer(logger))

	return &AnalysisAdapter{
		orchestrator: orchestrator,
		logger:       logger.With().Str("component", "analysis_adapter").Logger(),
	}
}

// AnalyzeWithModules performs repository analysis using the modular engines
func (a *AnalysisAdapter) AnalyzeWithModules(
	ctx context.Context,
	args interface{},
	repositoryPath string,
	repoData *analysis.RepoData,
) (*AnalysisResponse, error) {

	// Convert args to analysis options
	options := a.convertToAnalysisOptions(args)

	// Create analysis configuration
	config := analysis.AnalysisConfig{
		RepositoryPath: repositoryPath,
		RepoData:       repoData,
		Options:        options,
		Logger:         a.logger,
	}

	// Run analysis through orchestrator
	result, err := a.orchestrator.Analyze(ctx, config)
	if err != nil {
		return nil, types.NewRichError("ANALYSIS_FAILED", "modular analysis failed: "+err.Error(), "analysis_error")
	}

	// Convert to response format
	response := a.convertToAnalysisResponse(result, repoData)

	a.logger.Info().
		Int("engines", len(result.EngineResults)).
		Int("findings", len(result.AllFindings)).
		Dur("duration", result.Duration).
		Msg("Modular analysis completed")

	return response, nil
}

// convertToAnalysisOptions converts atomic tool args to analysis options
func (a *AnalysisAdapter) convertToAnalysisOptions(args interface{}) analysis.AnalysisOptions {
	// Default options - could be enhanced to parse specific args
	return analysis.AnalysisOptions{
		IncludeFrameworks:    true,
		IncludeDependencies:  true,
		IncludeConfiguration: true,
		IncludeDatabase:      true,
		IncludeBuild:         true,
		DeepAnalysis:         true,
		MaxDepth:             10,
	}
}

// convertToAnalysisResponse converts modular analysis results to atomic tool response
func (a *AnalysisAdapter) convertToAnalysisResponse(
	result *analysis.CombinedAnalysisResult,
	repoData *analysis.RepoData,
) *AnalysisResponse {

	response := &AnalysisResponse{
		Success:   true,
		Timestamp: time.Now(),
		Duration:  result.Duration,
		Analysis:  a.buildAnalysisContext(result, repoData),
		Metadata: map[string]interface{}{
			"engines_used":   result.EngineResults,
			"total_findings": len(result.AllFindings),
			"summary":        result.Summary,
			"analysis_type":  "modular",
		},
	}

	return response
}

// buildAnalysisContext creates the analysis context from modular results
func (a *AnalysisAdapter) buildAnalysisContext(
	result *analysis.CombinedAnalysisResult,
	repoData *analysis.RepoData,
) AnalysisContext {

	context := AnalysisContext{
		Languages:        a.extractLanguages(result),
		Frameworks:       a.extractFrameworks(result),
		Dependencies:     a.extractDependencies(result),
		Configuration:    a.extractConfiguration(result),
		BuildSystem:      a.extractBuildSystem(result),
		EntryPoints:      a.extractEntryPoints(result),
		Ports:            a.extractPorts(result),
		Environment:      a.extractEnvironment(result),
		Security:         a.extractSecurity(result),
		Containerization: a.extractContainerization(result),
		Recommendations:  a.generateRecommendations(result),
	}

	return context
}

// extractLanguages extracts language information from findings
func (a *AnalysisAdapter) extractLanguages(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	languages := make(map[string]interface{})

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeLanguage {
			switch finding.Category {
			case "primary_language":
				languages["primary"] = finding.Metadata
			case "secondary_language":
				if languages["secondary"] == nil {
					languages["secondary"] = make([]interface{}, 0)
				}
				languages["secondary"] = append(languages["secondary"].([]interface{}), finding.Metadata)
			case "technology_stack":
				languages["stack"] = finding.Metadata
			}
		}
	}

	return languages
}

// extractFrameworks extracts framework information
func (a *AnalysisAdapter) extractFrameworks(result *analysis.CombinedAnalysisResult) []interface{} {
	var frameworks []interface{}

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeFramework {
			frameworks = append(frameworks, map[string]interface{}{
				"name":        finding.Metadata["framework"],
				"confidence":  finding.Confidence,
				"description": finding.Description,
			})
		}
	}

	return frameworks
}

// extractDependencies extracts dependency information
func (a *AnalysisAdapter) extractDependencies(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	dependencies := make(map[string]interface{})

	packageManagers := make([]interface{}, 0)
	criticalDeps := make([]interface{}, 0)
	securityConcerns := make([]interface{}, 0)

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeDependency {
			switch finding.Category {
			case "package_manager":
				packageManagers = append(packageManagers, finding.Metadata)
			case "critical_dependency":
				criticalDeps = append(criticalDeps, finding.Metadata)
			case "dependency_health":
				dependencies["health"] = finding.Metadata
			}
		} else if finding.Type == analysis.FindingTypeSecurity && finding.Category == "dependency_security" {
			securityConcerns = append(securityConcerns, finding.Metadata)
		}
	}

	dependencies["package_managers"] = packageManagers
	dependencies["critical"] = criticalDeps
	dependencies["security_concerns"] = securityConcerns

	return dependencies
}

// extractConfiguration extracts configuration information
func (a *AnalysisAdapter) extractConfiguration(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	config := make(map[string]interface{})

	configFiles := make([]interface{}, 0)
	logging := make([]interface{}, 0)
	security := make([]interface{}, 0)

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeConfiguration {
			switch finding.Category {
			case "config_file":
				configFiles = append(configFiles, finding.Metadata)
			case "logging_configuration":
				logging = append(logging, finding.Metadata)
			case "security_configuration":
				security = append(security, finding.Metadata)
			}
		}
	}

	config["files"] = configFiles
	config["logging"] = logging
	config["security"] = security

	return config
}

// extractBuildSystem extracts build system information
func (a *AnalysisAdapter) extractBuildSystem(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	buildSystem := make(map[string]interface{})

	systems := make([]interface{}, 0)
	scripts := make([]interface{}, 0)
	cicd := make([]interface{}, 0)

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeBuild {
			switch finding.Category {
			case "build_system":
				systems = append(systems, finding.Metadata)
			case "build_script":
				scripts = append(scripts, finding.Metadata)
			case "cicd_system":
				cicd = append(cicd, finding.Metadata)
			case "containerization_readiness":
				buildSystem["containerization"] = finding.Metadata
			}
		}
	}

	buildSystem["systems"] = systems
	buildSystem["scripts"] = scripts
	buildSystem["cicd"] = cicd

	return buildSystem
}

// extractEntryPoints extracts entry point information
func (a *AnalysisAdapter) extractEntryPoints(result *analysis.CombinedAnalysisResult) []interface{} {
	var entryPoints []interface{}

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeEntrypoint {
			entryPoints = append(entryPoints, map[string]interface{}{
				"language": finding.Metadata["language"],
				"path":     finding.Metadata["entry_point"],
				"source":   finding.Category,
			})
		}
	}

	return entryPoints
}

// extractPorts extracts port information
func (a *AnalysisAdapter) extractPorts(result *analysis.CombinedAnalysisResult) []interface{} {
	var ports []interface{}

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypePort {
			ports = append(ports, map[string]interface{}{
				"port":     finding.Metadata["port"],
				"type":     finding.Metadata["port_type"],
				"files":    finding.Metadata["files"],
				"severity": string(finding.Severity),
			})
		}
	}

	return ports
}

// extractEnvironment extracts environment variable information
func (a *AnalysisAdapter) extractEnvironment(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	environment := make(map[string]interface{})

	variables := make([]interface{}, 0)
	usage := make([]interface{}, 0)

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeEnvironment {
			switch finding.Category {
			case "environment_variable":
				variables = append(variables, finding.Metadata)
			case "environment_usage":
				usage = append(usage, finding.Metadata)
			}
		}
	}

	environment["variables"] = variables
	environment["usage"] = usage

	return environment
}

// extractSecurity extracts security-related information
func (a *AnalysisAdapter) extractSecurity(result *analysis.CombinedAnalysisResult) []interface{} {
	var security []interface{}

	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeSecurity {
			security = append(security, map[string]interface{}{
				"category":    finding.Category,
				"title":       finding.Title,
				"description": finding.Description,
				"severity":    string(finding.Severity),
				"metadata":    finding.Metadata,
			})
		}
	}

	return security
}

// extractContainerization extracts containerization readiness
func (a *AnalysisAdapter) extractContainerization(result *analysis.CombinedAnalysisResult) map[string]interface{} {
	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeBuild && finding.Category == "containerization_readiness" {
			return finding.Metadata
		}
	}
	return map[string]interface{}{}
}

// generateRecommendations generates actionable recommendations
func (a *AnalysisAdapter) generateRecommendations(result *analysis.CombinedAnalysisResult) []string {
	var recommendations []string

	// Extract recommendations from containerization readiness
	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeBuild && finding.Category == "containerization_readiness" {
			if recs, ok := finding.Metadata["recommendations"].([]string); ok {
				recommendations = append(recommendations, recs...)
			}
		}
	}

	// Add security recommendations
	hasSecurityConcerns := false
	for _, finding := range result.AllFindings {
		if finding.Type == analysis.FindingTypeSecurity && finding.Severity != analysis.SeverityInfo {
			hasSecurityConcerns = true
			break
		}
	}

	if hasSecurityConcerns {
		recommendations = append(recommendations, "Review and address security concerns in dependencies")
	}

	// Add default recommendations if none found
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Repository analysis completed successfully")
	}

	return recommendations
}

// Response types to match existing atomic tool interface

// AnalysisResponse represents the response from repository analysis
type AnalysisResponse struct {
	Success   bool                   `json:"success"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Analysis  AnalysisContext        `json:"analysis"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// AnalysisContext contains the analyzed repository information
type AnalysisContext struct {
	Languages        map[string]interface{} `json:"languages"`
	Frameworks       []interface{}          `json:"frameworks"`
	Dependencies     map[string]interface{} `json:"dependencies"`
	Configuration    map[string]interface{} `json:"configuration"`
	BuildSystem      map[string]interface{} `json:"build_system"`
	EntryPoints      []interface{}          `json:"entry_points"`
	Ports            []interface{}          `json:"ports"`
	Environment      map[string]interface{} `json:"environment"`
	Security         []interface{}          `json:"security"`
	Containerization map[string]interface{} `json:"containerization"`
	Recommendations  []string               `json:"recommendations"`
}
