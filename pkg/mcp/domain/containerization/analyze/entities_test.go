package analyze

import (
	"testing"
	"time"
)

func TestRepository_IsValidRepository(t *testing.T) {
	validRepo := &Repository{
		Path: "/test/repo",
		Files: []File{
			{Path: "main.go", Type: FileTypeSource},
		},
		Languages: map[string]float64{
			"Go": 100.0,
		},
	}

	if !validRepo.IsValidRepository() {
		t.Error("expected valid repository to be valid")
	}

	// Test invalid repository (no files)
	invalidRepo := &Repository{
		Path:      "/test/repo",
		Files:     []File{},
		Languages: map[string]float64{"Go": 100.0},
	}

	if invalidRepo.IsValidRepository() {
		t.Error("expected repository with no files to be invalid")
	}
}

func TestRepository_GetPrimaryLanguage(t *testing.T) {
	repo := &Repository{
		Languages: map[string]float64{
			"Go":         70.0,
			"JavaScript": 20.0,
			"HTML":       10.0,
		},
	}

	lang, percentage := repo.GetPrimaryLanguage()
	if lang != "Go" {
		t.Errorf("expected primary language 'Go', got '%s'", lang)
	}
	if percentage != 70.0 {
		t.Errorf("expected percentage 70.0, got %f", percentage)
	}
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		filePath     string
		expectedType FileType
	}{
		{"main.go", FileTypeSource},
		{"main_test.go", FileTypeTest},
		{"package.json", FileTypeConfiguration},
		{"README.md", FileTypeDocumentation},
		{"Dockerfile", FileTypeConfiguration},
		{"Makefile", FileTypeBuild},
		{"data.sql", FileTypeData},
		{"unknown.xyz", FileTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := ClassifyFileType(tt.filePath)
			if result != tt.expectedType {
				t.Errorf("expected %s, got %s", tt.expectedType, result)
			}
		})
	}
}

func TestAnalysisResult_GetCriticalSecurityIssues(t *testing.T) {
	result := &AnalysisResult{
		SecurityIssues: []SecurityIssue{
			{ID: "1", Severity: SeverityCritical},
			{ID: "2", Severity: SeverityHigh},
			{ID: "3", Severity: SeverityCritical},
			{ID: "4", Severity: SeverityMedium},
		},
	}

	critical := result.GetCriticalSecurityIssues()
	if len(critical) != 2 {
		t.Errorf("expected 2 critical issues, got %d", len(critical))
	}

	for _, issue := range critical {
		if issue.Severity != SeverityCritical {
			t.Error("expected only critical severity issues")
		}
	}
}

func TestAnalysisResult_HasDatabaseType(t *testing.T) {
	result := &AnalysisResult{
		Databases: []Database{
			{Type: DatabaseTypePostgreSQL, Name: "main_db"},
			{Type: DatabaseTypeRedis, Name: "cache_db"},
		},
	}

	if !result.HasDatabaseType(DatabaseTypePostgreSQL) {
		t.Error("expected to find PostgreSQL database")
	}

	if !result.HasDatabaseType(DatabaseTypeRedis) {
		t.Error("expected to find Redis database")
	}

	if result.HasDatabaseType(DatabaseTypeMySQL) {
		t.Error("expected not to find MySQL database")
	}
}

func TestAnalysisResult_ShouldRecommendDockerization(t *testing.T) {
	// Test with web framework
	webResult := &AnalysisResult{
		Framework: Framework{Type: FrameworkTypeWeb},
	}
	if !webResult.ShouldRecommendDockerization() {
		t.Error("expected to recommend dockerization for web framework")
	}

	// Test with database
	dbResult := &AnalysisResult{
		Databases: []Database{
			{Type: DatabaseTypePostgreSQL},
		},
	}
	if !dbResult.ShouldRecommendDockerization() {
		t.Error("expected to recommend dockerization for database usage")
	}

	// Test with many dependencies
	depsResult := &AnalysisResult{
		Dependencies: make([]Dependency, 10),
	}
	if !depsResult.ShouldRecommendDockerization() {
		t.Error("expected to recommend dockerization for many dependencies")
	}

	// Test simple project
	simpleResult := &AnalysisResult{
		Framework:    Framework{Type: FrameworkTypeLibrary},
		Dependencies: make([]Dependency, 2),
	}
	if simpleResult.ShouldRecommendDockerization() {
		t.Error("expected not to recommend dockerization for simple library")
	}
}

func TestAnalysisResult_CalculateOverallConfidence(t *testing.T) {
	// High confidence result
	highResult := &AnalysisResult{
		Language: Language{Confidence: 0.9},
		Framework: Framework{Confidence: ConfidenceHigh},
	}
	if highResult.CalculateOverallConfidence() != ConfidenceHigh {
		t.Error("expected high confidence")
	}

	// Medium confidence result
	mediumResult := &AnalysisResult{
		Language: Language{Confidence: 0.6},
		Framework: Framework{Confidence: ConfidenceMedium},
	}
	if mediumResult.CalculateOverallConfidence() != ConfidenceMedium {
		t.Error("expected medium confidence")
	}

	// Low confidence result
	lowResult := &AnalysisResult{
		Language: Language{Confidence: 0.3},
		Framework: Framework{Confidence: ConfidenceLow},
	}
	if lowResult.CalculateOverallConfidence() != ConfidenceLow {
		t.Error("expected low confidence")
	}
}

func TestAnalysisResult_IsHighQualityAnalysis(t *testing.T) {
	highQualityResult := &AnalysisResult{
		Language: Language{Confidence: 0.8},
		Repository: Repository{
			Files: make([]File, 15),
		},
		Dependencies: []Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		},
		AnalysisMetadata: AnalysisMetadata{
			Duration: time.Second,
		},
	}

	if !highQualityResult.IsHighQualityAnalysis() {
		t.Error("expected high quality analysis")
	}

	// Low confidence
	lowConfidenceResult := &AnalysisResult{
		Language: Language{Confidence: 0.5},
		Repository: Repository{
			Files: make([]File, 15),
		},
		Dependencies: []Dependency{{Name: "dep1"}},
		AnalysisMetadata: AnalysisMetadata{
			Duration: time.Second,
		},
	}

	if lowConfidenceResult.IsHighQualityAnalysis() {
		t.Error("expected low quality analysis due to low confidence")
	}
}

func TestAnalysisResult_Validate(t *testing.T) {
	validResult := &AnalysisResult{
		Repository: Repository{Path: "/test/repo"},
		Language:   Language{Name: "Go", Confidence: 0.8},
		Dependencies: []Dependency{
			{Name: "dep1"},
		},
		SecurityIssues: []SecurityIssue{
			{ID: "issue1", Severity: SeverityHigh},
		},
	}

	errors := validResult.Validate()
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errors), errors)
	}

	// Test with missing required fields
	invalidResult := &AnalysisResult{
		Repository: Repository{Path: ""},
		Language:   Language{Name: "", Confidence: 1.5},
		Dependencies: []Dependency{
			{Name: ""},
		},
		SecurityIssues: []SecurityIssue{
			{ID: "", Severity: ""},
		},
	}

	errors = invalidResult.Validate()
	if len(errors) == 0 {
		t.Error("expected validation errors for invalid result")
	}

	// Check for specific error codes
	errorCodes := make(map[string]bool)
	for _, err := range errors {
		errorCodes[err.Code] = true
	}

	expectedCodes := []string{"MISSING_REPOSITORY_PATH", "MISSING_LANGUAGE", "INVALID_CONFIDENCE", "MISSING_DEPENDENCY_NAME", "MISSING_SECURITY_ISSUE_ID", "MISSING_SECURITY_SEVERITY"}
	for _, code := range expectedCodes {
		if !errorCodes[code] {
			t.Errorf("expected error code %s", code)
		}
	}
}