// Package analyze contains business rules for repository analysis
package analyze

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ValidationError represents an analysis validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("analysis validation error: %s - %s", e.Field, e.Message)
}

// Validate performs domain-level validation on an analysis result
func (ar *AnalysisResult) Validate() []ValidationError {
	var errors []ValidationError

	// Repository validation
	if ar.Repository.Path == "" {
		errors = append(errors, ValidationError{
			Field:   "repository.path",
			Message: "repository path is required",
			Code:    "MISSING_REPOSITORY_PATH",
		})
	}

	// Language validation
	if ar.Language.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "language.name",
			Message: "detected language name is required",
			Code:    "MISSING_LANGUAGE",
		})
	}

	if ar.Language.Confidence < 0 || ar.Language.Confidence > 1 {
		errors = append(errors, ValidationError{
			Field:   "language.confidence",
			Message: "language confidence must be between 0 and 1",
			Code:    "INVALID_CONFIDENCE",
		})
	}

	// Dependencies validation
	for i, dep := range ar.Dependencies {
		if dep.Name == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("dependencies[%d].name", i),
				Message: "dependency name is required",
				Code:    "MISSING_DEPENDENCY_NAME",
			})
		}
	}

	// Security issues validation
	for i, issue := range ar.SecurityIssues {
		if issue.ID == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("security_issues[%d].id", i),
				Message: "security issue ID is required",
				Code:    "MISSING_SECURITY_ISSUE_ID",
			})
		}
		if issue.Severity == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("security_issues[%d].severity", i),
				Message: "security issue severity is required",
				Code:    "MISSING_SECURITY_SEVERITY",
			})
		}
	}

	return errors
}

// Business Rules for Repository Analysis

// IsValidRepository checks if a repository meets basic requirements for analysis
func (r *Repository) IsValidRepository() bool {
	// Must have a valid path
	if r.Path == "" {
		return false
	}

	// Must have at least one file
	if len(r.Files) == 0 {
		return false
	}

	// Must have detected languages
	if len(r.Languages) == 0 {
		return false
	}

	return true
}

// GetPrimaryLanguage returns the primary programming language based on file percentage
func (r *Repository) GetPrimaryLanguage() (string, float64) {
	var primaryLang string
	var maxPercentage float64

	for lang, percentage := range r.Languages {
		if percentage > maxPercentage {
			maxPercentage = percentage
			primaryLang = lang
		}
	}

	return primaryLang, maxPercentage
}

// HasConfiguration checks if the repository has configuration files
func (r *Repository) HasConfiguration() bool {
	for _, file := range r.Files {
		if file.Type == FileTypeConfiguration {
			return true
		}
	}
	return false
}

// HasTests checks if the repository has test files
func (r *Repository) HasTests() bool {
	for _, file := range r.Files {
		if file.Type == FileTypeTest {
			return true
		}
	}
	return false
}

// GetFilesByType returns files of a specific type
func (r *Repository) GetFilesByType(fileType FileType) []File {
	var files []File
	for _, file := range r.Files {
		if file.Type == fileType {
			files = append(files, file)
		}
	}
	return files
}

// Business Rules for Analysis Quality

// IsHighQualityAnalysis determines if an analysis result meets quality standards
func (ar *AnalysisResult) IsHighQualityAnalysis() bool {
	// Language detection must be confident
	if ar.Language.Confidence < 0.7 {
		return false
	}

	// Must have detected dependencies for non-trivial projects
	if len(ar.Repository.Files) > 10 && len(ar.Dependencies) == 0 {
		return false
	}

	// Must have reasonable analysis duration (not too fast, suggesting incomplete analysis)
	if ar.AnalysisMetadata.Duration < 100*time.Millisecond {
		return false
	}

	return true
}

// GetCriticalSecurityIssues returns security issues with critical severity
func (ar *AnalysisResult) GetCriticalSecurityIssues() []SecurityIssue {
	var critical []SecurityIssue
	for _, issue := range ar.SecurityIssues {
		if issue.Severity == SeverityCritical {
			critical = append(critical, issue)
		}
	}
	return critical
}

// GetHighPriorityRecommendations returns high priority recommendations
func (ar *AnalysisResult) GetHighPriorityRecommendations() []Recommendation {
	var highPriority []Recommendation
	for _, rec := range ar.Recommendations {
		if rec.Priority == PriorityHigh {
			highPriority = append(highPriority, rec)
		}
	}
	return highPriority
}

// HasFramework checks if a specific framework type was detected
func (ar *AnalysisResult) HasFramework(frameworkType FrameworkType) bool {
	return ar.Framework.Type == frameworkType
}

// HasDatabase checks if any database was detected
func (ar *AnalysisResult) HasDatabase() bool {
	return len(ar.Databases) > 0
}

// HasDatabaseType checks if a specific database type was detected
func (ar *AnalysisResult) HasDatabaseType(dbType DatabaseType) bool {
	for _, db := range ar.Databases {
		if db.Type == dbType {
			return true
		}
	}
	return false
}

// Business Rules for File Classification

// ClassifyFileType determines the type of a file based on its path and content
func ClassifyFileType(filePath string) FileType {
	fileName := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))
	dirName := strings.ToLower(filepath.Dir(filePath))

	// Test files
	if strings.Contains(dirName, "test") || strings.Contains(fileName, "test") ||
		strings.Contains(fileName, "spec") || strings.HasSuffix(fileName, "_test.go") ||
		strings.HasSuffix(fileName, ".test.js") {
		return FileTypeTest
	}

	// Build files (check first, before configuration)
	buildNames := []string{"makefile", "build.sh", "deploy.sh"}
	for _, buildName := range buildNames {
		if strings.ToLower(fileName) == buildName {
			return FileTypeBuild
		}
	}

	// Configuration files (including Dockerfile which is a configuration for container builds)
	configExtensions := []string{".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".config"}
	configNames := []string{"dockerfile", "package.json", "pom.xml", "build.gradle", "cargo.toml", "go.mod"}

	for _, configExt := range configExtensions {
		if ext == configExt {
			return FileTypeConfiguration
		}
	}

	for _, configName := range configNames {
		if strings.ToLower(fileName) == configName {
			return FileTypeConfiguration
		}
	}

	// Documentation files
	docExtensions := []string{".md", ".txt", ".rst", ".adoc"}
	for _, docExt := range docExtensions {
		if ext == docExt {
			return FileTypeDocumentation
		}
	}

	// Source files
	sourceExtensions := []string{".go", ".js", ".ts", ".py", ".java", ".cpp", ".c", ".cs", ".rb", ".php", ".rs", ".kt", ".scala"}
	for _, sourceExt := range sourceExtensions {
		if ext == sourceExt {
			return FileTypeSource
		}
	}

	// Data files
	dataExtensions := []string{".sql", ".csv", ".xml", ".dat", ".db"}
	for _, dataExt := range dataExtensions {
		if ext == dataExt {
			return FileTypeData
		}
	}

	return FileTypeUnknown
}

// Business Rules for Confidence Calculation

// CalculateOverallConfidence calculates the overall confidence of the analysis
func (ar *AnalysisResult) CalculateOverallConfidence() ConfidenceLevel {
	confidenceScores := []float64{ar.Language.Confidence}

	// Add framework confidence if detected
	if ar.Framework.Name != "" {
		switch ar.Framework.Confidence {
		case ConfidenceHigh:
			confidenceScores = append(confidenceScores, 0.9)
		case ConfidenceMedium:
			confidenceScores = append(confidenceScores, 0.6)
		case ConfidenceLow:
			confidenceScores = append(confidenceScores, 0.3)
		}
	}

	// Calculate average confidence
	var total float64
	for _, score := range confidenceScores {
		total += score
	}
	avgConfidence := total / float64(len(confidenceScores))

	// Convert to confidence level
	if avgConfidence >= 0.8 {
		return ConfidenceHigh
	} else if avgConfidence >= 0.5 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

// ShouldRecommendDockerization determines if the project should be containerized
func (ar *AnalysisResult) ShouldRecommendDockerization() bool {
	// Has web framework
	if ar.Framework.Type == FrameworkTypeWeb || ar.Framework.Type == FrameworkTypeAPI {
		return true
	}

	// Has database dependencies
	if len(ar.Databases) > 0 {
		return true
	}

	// Has multiple dependencies suggesting complex deployment
	if len(ar.Dependencies) > 5 {
		return true
	}

	return false
}

// NeedsSecurityReview determines if the project needs a security review
func (ar *AnalysisResult) NeedsSecurityReview() bool {
	// Has critical security issues
	if len(ar.GetCriticalSecurityIssues()) > 0 {
		return true
	}

	// Has database without proper configuration
	if ar.HasDatabase() && !ar.Repository.HasConfiguration() {
		return true
	}

	// Has web framework without security recommendations
	if ar.Framework.Type == FrameworkTypeWeb {
		hasSecurityRec := false
		for _, rec := range ar.Recommendations {
			if rec.Type == RecommendationTypeSecurity {
				hasSecurityRec = true
				break
			}
		}
		if !hasSecurityRec {
			return true
		}
	}

	return false
}
