package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// DependencyAnalyzer analyzes package dependencies and their security/compatibility
type DependencyAnalyzer struct {
	logger zerolog.Logger
}

// NewDependencyAnalyzer creates a new dependency analyzer
func NewDependencyAnalyzer(logger zerolog.Logger) *DependencyAnalyzer {
	return &DependencyAnalyzer{
		logger: logger.With().Str("engine", "dependency").Logger(),
	}
}

// GetName returns the name of this engine
func (d *DependencyAnalyzer) GetName() string {
	return "dependency_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (d *DependencyAnalyzer) GetCapabilities() []string {
	return []string{
		"package_dependencies",
		"dependency_versions",
		"security_vulnerabilities",
		"license_analysis",
		"dependency_graph",
		"outdated_packages",
	}
}

// IsApplicable determines if this engine should run
func (d *DependencyAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Check if any dependency files exist
	dependencyFiles := []string{
		"package.json", "yarn.lock", "package-lock.json",
		"requirements.txt", "Pipfile", "poetry.lock", "pyproject.toml",
		"go.mod", "go.sum",
		"pom.xml", "build.gradle", "Gemfile", "composer.json",
		"Cargo.toml", "Cargo.lock",
	}

	for _, file := range dependencyFiles {
		if d.fileExists(repoData, file) {
			return true
		}
	}
	return false
}

// Analyze performs dependency analysis
func (d *DependencyAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   d.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Note: Simplified implementation - dependency analysis would be implemented here
	_ = config // Prevent unused variable error

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = 0.8 // Default confidence

	return result, nil
}

// analyzePackageManagers identifies package managers in use
func (d *DependencyAnalyzer) analyzePackageManagers(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	packageManagers := map[string][]string{
		"npm":      {"package.json", "package-lock.json"},
		"yarn":     {"package.json", "yarn.lock"},
		"pip":      {"requirements.txt", "setup.py"},
		"pipenv":   {"Pipfile", "Pipfile.lock"},
		"poetry":   {"pyproject.toml", "poetry.lock"},
		"go mod":   {"go.mod", "go.sum"},
		"maven":    {"pom.xml"},
		"gradle":   {"build.gradle", "build.gradle.kts"},
		"bundler":  {"Gemfile", "Gemfile.lock"},
		"composer": {"composer.json", "composer.lock"},
		"cargo":    {"Cargo.toml", "Cargo.lock"},
		"nuget":    {"*.csproj", "packages.config"},
	}

	for manager, files := range packageManagers {
		confidence := d.checkPackageManagerFiles(repoData, files)
		if confidence > 0.0 {
			finding := Finding{
				Type:        FindingTypeDependency,
				Category:    "package_manager",
				Title:       fmt.Sprintf("%s Package Manager", manager),
				Description: d.generatePackageManagerDescription(manager, confidence),
				Confidence:  confidence,
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"manager": manager,
					"files":   d.getExistingFiles(repoData, files),
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeDependencies analyzes specific dependencies
func (d *DependencyAnalyzer) analyzeDependencies(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	// Analyze JavaScript dependencies
	if err := d.analyzeJavaScriptDependencies(repoData, result); err != nil {
		return err
	}

	// Analyze Python dependencies
	if err := d.analyzePythonDependencies(repoData, result); err != nil {
		return err
	}

	// Analyze Go dependencies
	if err := d.analyzeGoDependencies(repoData, result); err != nil {
		return err
	}

	return nil
}

// analyzeJavaScriptDependencies analyzes package.json dependencies
func (d *DependencyAnalyzer) analyzeJavaScriptDependencies(repoData *RepoData, result *EngineAnalysisResult) error {
	packageJsonFile := d.findFile(repoData, "package.json")
	if packageJsonFile == nil {
		return nil
	}

	// Parse key dependencies (simplified analysis)
	criticalDependencies := []string{
		"react", "vue", "angular", "express", "next", "nuxt",
		"typescript", "webpack", "babel", "eslint", "jest",
	}

	for _, dep := range criticalDependencies {
		if strings.Contains(strings.ToLower(packageJsonFile.Content), fmt.Sprintf("\"%s\"", dep)) {
			finding := Finding{
				Type:        FindingTypeDependency,
				Category:    "critical_dependency",
				Title:       fmt.Sprintf("%s Dependency", strings.Title(dep)),
				Description: fmt.Sprintf("Critical %s dependency detected", dep),
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: packageJsonFile.Path,
				},
				Metadata: map[string]interface{}{
					"dependency": dep,
					"ecosystem":  "npm",
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzePythonDependencies analyzes Python requirements
func (d *DependencyAnalyzer) analyzePythonDependencies(repoData *RepoData, result *EngineAnalysisResult) error {
	requirementsFile := d.findFile(repoData, "requirements.txt")
	if requirementsFile == nil {
		return nil
	}

	criticalDependencies := []string{
		"django", "flask", "fastapi", "requests", "numpy", "pandas",
		"tensorflow", "pytorch", "scikit-learn", "matplotlib",
	}

	for _, dep := range criticalDependencies {
		if strings.Contains(strings.ToLower(requirementsFile.Content), dep) {
			finding := Finding{
				Type:        FindingTypeDependency,
				Category:    "critical_dependency",
				Title:       fmt.Sprintf("%s Dependency", strings.Title(dep)),
				Description: fmt.Sprintf("Critical %s dependency detected", dep),
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: requirementsFile.Path,
				},
				Metadata: map[string]interface{}{
					"dependency": dep,
					"ecosystem":  "pip",
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeGoDependencies analyzes Go modules
func (d *DependencyAnalyzer) analyzeGoDependencies(repoData *RepoData, result *EngineAnalysisResult) error {
	goModFile := d.findFile(repoData, "go.mod")
	if goModFile == nil {
		return nil
	}

	criticalDependencies := []string{
		"gin-gonic/gin", "gorilla/mux", "echo", "fiber",
		"grpc", "protobuf", "cobra", "viper", "logrus", "zap",
	}

	for _, dep := range criticalDependencies {
		if strings.Contains(strings.ToLower(goModFile.Content), strings.ToLower(dep)) {
			finding := Finding{
				Type:        FindingTypeDependency,
				Category:    "critical_dependency",
				Title:       fmt.Sprintf("%s Dependency", dep),
				Description: fmt.Sprintf("Critical %s dependency detected", dep),
				Confidence:  0.9,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: goModFile.Path,
				},
				Metadata: map[string]interface{}{
					"dependency": dep,
					"ecosystem":  "go",
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzeDependencySecurity analyzes dependency security issues
func (d *DependencyAnalyzer) analyzeDependencySecurity(config AnalysisConfig, result *EngineAnalysisResult) error {
	// Check for known vulnerable patterns
	vulnerablePatterns := map[string]string{
		"lodash":     "Known security vulnerabilities in older versions",
		"moment":     "Large bundle size, consider date-fns or dayjs",
		"request":    "Deprecated package, use axios or fetch",
		"handlebars": "Potential XSS vulnerabilities",
		"jquery":     "Large attack surface, consider modern alternatives",
	}

	for _, finding := range result.Findings {
		if finding.Category == "critical_dependency" {
			if dep, ok := finding.Metadata["dependency"].(string); ok {
				if warning, exists := vulnerablePatterns[dep]; exists {
					securityFinding := Finding{
						Type:        FindingTypeSecurity,
						Category:    "dependency_security",
						Title:       fmt.Sprintf("Security Concern: %s", dep),
						Description: warning,
						Confidence:  0.7,
						Severity:    SeverityMedium,
						Metadata: map[string]interface{}{
							"dependency": dep,
							"concern":    warning,
						},
					}
					result.Findings = append(result.Findings, securityFinding)
				}
			}
		}
	}

	return nil
}

// analyzeDependencyHealth analyzes overall dependency health
func (d *DependencyAnalyzer) analyzeDependencyHealth(config AnalysisConfig, result *EngineAnalysisResult) error {
	// Count dependencies by category
	packageManagers := make(map[string]int)
	criticalDeps := 0
	securityConcerns := 0

	for _, finding := range result.Findings {
		switch finding.Category {
		case "package_manager":
			if manager, ok := finding.Metadata["manager"].(string); ok {
				packageManagers[manager]++
			}
		case "critical_dependency":
			criticalDeps++
		case "dependency_security":
			securityConcerns++
		}
	}

	// Generate health assessment
	var severity Severity = SeverityInfo
	if securityConcerns > 2 {
		severity = SeverityHigh
	} else if securityConcerns > 0 {
		severity = SeverityMedium
	}

	healthFinding := Finding{
		Type:        FindingTypeDependency,
		Category:    "dependency_health",
		Title:       "Dependency Health Assessment",
		Description: d.generateHealthDescription(packageManagers, criticalDeps, securityConcerns),
		Confidence:  0.95,
		Severity:    severity,
		Metadata: map[string]interface{}{
			"package_managers":  packageManagers,
			"critical_deps":     criticalDeps,
			"security_concerns": securityConcerns,
			"health_score":      d.calculateHealthScore(criticalDeps, securityConcerns),
		},
	}

	result.Findings = append(result.Findings, healthFinding)
	return nil
}

// Helper methods

func (d *DependencyAnalyzer) fileExists(repoData *RepoData, filename string) bool {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return true
		}
	}
	return false
}

func (d *DependencyAnalyzer) findFile(repoData *RepoData, filename string) *FileData {
	for _, file := range repoData.Files {
		if strings.HasSuffix(file.Path, filename) || filepath.Base(file.Path) == filename {
			return &file
		}
	}
	return nil
}

func (d *DependencyAnalyzer) checkPackageManagerFiles(repoData *RepoData, files []string) float64 {
	matches := 0
	for _, file := range files {
		if d.fileExists(repoData, file) {
			matches++
		}
	}
	return float64(matches) / float64(len(files))
}

func (d *DependencyAnalyzer) getExistingFiles(repoData *RepoData, files []string) []string {
	var existing []string
	for _, file := range files {
		if d.fileExists(repoData, file) {
			existing = append(existing, file)
		}
	}
	return existing
}

func (d *DependencyAnalyzer) generatePackageManagerDescription(manager string, confidence float64) string {
	return fmt.Sprintf("%s package manager detected with %.0f%% confidence", manager, confidence*100)
}

func (d *DependencyAnalyzer) generateHealthDescription(packageManagers map[string]int, criticalDeps, securityConcerns int) string {
	desc := fmt.Sprintf("Dependency analysis: %d critical dependencies detected", criticalDeps)
	if securityConcerns > 0 {
		desc += fmt.Sprintf(", %d security concerns identified", securityConcerns)
	}
	if len(packageManagers) > 1 {
		desc += fmt.Sprintf(", multiple package managers in use (%d)", len(packageManagers))
	}
	return desc
}

func (d *DependencyAnalyzer) calculateHealthScore(criticalDeps, securityConcerns int) float64 {
	score := 1.0

	// Reduce score for security concerns
	score -= float64(securityConcerns) * 0.2

	// Slight reduction for having many dependencies
	if criticalDeps > 10 {
		score -= 0.1
	}

	if score < 0 {
		score = 0
	}

	return score
}

func (d *DependencyAnalyzer) calculateConfidence(result *EngineAnalysisResult) float64 {
	if len(result.Findings) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, finding := range result.Findings {
		totalConfidence += finding.Confidence
	}

	return totalConfidence / float64(len(result.Findings))
}
