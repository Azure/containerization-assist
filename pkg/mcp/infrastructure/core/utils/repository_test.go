package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"log/slog"
)

func TestNewRepositoryAnalyzer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}

	// Test that logger is properly set (the logger should be configured with component field)
	// We can't easily test the internal state, but we can verify the analyzer was created
	if analyzer == nil {
		t.Errorf("Expected analyzer to be properly initialized")
	}
}

func TestAnalysisResult_JSONMarshaling(t *testing.T) {
	result := &AnalysisResult{
		Success:      true,
		Language:     "go",
		Framework:    "gin",
		Dependencies: []Dependency{{Name: "gin", Version: "v1.9.0", Type: "runtime", Manager: "go"}},
		ConfigFiles:  []ConfigFile{{Path: "go.mod", Type: "package", Relevant: true}},
		Structure:    map[string]interface{}{"test": "value"},
		EntryPoints:  []string{"main.go"},
		BuildFiles:   []string{"Makefile"},
		Port:         8080,
		DatabaseInfo: &DatabaseInfo{Detected: true, Types: []string{"postgres"}},
		Suggestions:  []string{"Great project!"},
		Context:      map[string]interface{}{"files": 10},
	}

	// Test marshaling to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Test unmarshaling from JSON
	var unmarshaled AnalysisResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal from JSON: %v", err)
	}

	// Verify key fields
	if unmarshaled.Success != result.Success {
		t.Errorf("Expected Success=%v, got %v", result.Success, unmarshaled.Success)
	}
	if unmarshaled.Language != result.Language {
		t.Errorf("Expected Language=%s, got %s", result.Language, unmarshaled.Language)
	}
	if len(unmarshaled.Dependencies) != len(result.Dependencies) {
		t.Errorf("Expected %d dependencies, got %d", len(result.Dependencies), len(unmarshaled.Dependencies))
	}
}

func TestDependency_Structure(t *testing.T) {
	dep := Dependency{
		Name:    "express",
		Version: "^4.18.0",
		Type:    "runtime",
		Manager: "npm",
	}

	if dep.Name != "express" {
		t.Errorf("Expected Name=express, got %s", dep.Name)
	}
	if dep.Version != "^4.18.0" {
		t.Errorf("Expected Version=^4.18.0, got %s", dep.Version)
	}
	if dep.Type != "runtime" {
		t.Errorf("Expected Type=runtime, got %s", dep.Type)
	}
	if dep.Manager != "npm" {
		t.Errorf("Expected Manager=npm, got %s", dep.Manager)
	}
}

func TestConfigFile_Structure(t *testing.T) {
	configFile := ConfigFile{
		Path:     "package.json",
		Type:     "package",
		Content:  map[string]interface{}{"name": "test"},
		Relevant: true,
	}

	if configFile.Path != "package.json" {
		t.Errorf("Expected Path=package.json, got %s", configFile.Path)
	}
	if configFile.Type != "package" {
		t.Errorf("Expected Type=package, got %s", configFile.Type)
	}
	if !configFile.Relevant {
		t.Errorf("Expected Relevant=true, got %v", configFile.Relevant)
	}
	if configFile.Content["name"] != "test" {
		t.Errorf("Expected Content name=test, got %v", configFile.Content["name"])
	}
}

func TestDatabaseInfo_Structure(t *testing.T) {
	dbInfo := &DatabaseInfo{
		Detected:    true,
		Types:       []string{"mysql", "redis"},
		Libraries:   []string{"mysql2", "redis"},
		ConfigFiles: []string{".env", "config.json"},
	}

	if !dbInfo.Detected {
		t.Errorf("Expected Detected=true, got %v", dbInfo.Detected)
	}
	if len(dbInfo.Types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(dbInfo.Types))
	}
	if len(dbInfo.Libraries) != 2 {
		t.Errorf("Expected 2 libraries, got %d", len(dbInfo.Libraries))
	}
	if len(dbInfo.ConfigFiles) != 2 {
		t.Errorf("Expected 2 config files, got %d", len(dbInfo.ConfigFiles))
	}
}

func TestAnalysisError_Structure(t *testing.T) {
	analysisError := &AnalysisError{
		Type:    "validation_error",
		Message: "Invalid input",
		Path:    "/test/path",
		Context: map[string]interface{}{"detail": "error"},
	}

	if analysisError.Type != "validation_error" {
		t.Errorf("Expected Type=validation_error, got %s", analysisError.Type)
	}
	if analysisError.Message != "Invalid input" {
		t.Errorf("Expected Message=Invalid input, got %s", analysisError.Message)
	}
	if analysisError.Path != "/test/path" {
		t.Errorf("Expected Path=/test/path, got %s", analysisError.Path)
	}
	if analysisError.Context["detail"] != "error" {
		t.Errorf("Expected Context detail=error, got %v", analysisError.Context["detail"])
	}
}

func TestValidateInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "non-existent path",
			path:        "/non/existent/path",
			expectError: true,
		},
		{
			name:        "current directory",
			path:        ".",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := analyzer.validateInput(tt.path)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for path %s", tt.path)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for path %s, got %v", tt.path, err)
			}
		})
	}
}

func TestDetectLanguageByExtensions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "lang_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test files for different languages
	testFiles := map[string]string{
		"main.go":   "package main",
		"app.py":    "print('hello')",
		"index.js":  "console.log('hello')",
		"script.ts": "const x: string = 'hello'",
		"README.md": "# Test Project",
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test language detection (will depend on which has most files)
	language := analyzer.detectLanguageByExtensions(tempDir)
	if language == "unknown" {
		t.Errorf("Expected to detect a language, got unknown")
	}
}

func TestParseJSONFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	// Create a temporary JSON file
	tempDir, err := os.MkdirTemp("", "json_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	jsonContent := `{"name": "test-project", "version": "1.0.0", "dependencies": {"express": "^4.18.0"}}`
	jsonPath := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(jsonPath, []byte(jsonContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create JSON file: %v", err)
	}

	result, err := analyzer.parseJSONFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to parse JSON file: %v", err)
	}

	if result["name"] != "test-project" {
		t.Errorf("Expected name=test-project, got %v", result["name"])
	}
	if result["version"] != "1.0.0" {
		t.Errorf("Expected version=1.0.0, got %v", result["version"])
	}
}

func TestExtractJSONDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	packageJSON := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"express": "^4.18.0",
			"lodash":  "^4.17.21",
		},
		"devDependencies": map[string]interface{}{
			"jest":       "^29.0.0",
			"typescript": "^4.8.0",
		},
	}

	// Test extracting runtime dependencies
	deps := analyzer.extractJSONDependencies(packageJSON, "dependencies")
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	// Verify dependency details
	for _, dep := range deps {
		if dep.Manager != "npm" {
			t.Errorf("Expected manager=npm, got %s", dep.Manager)
		}
		if dep.Type != "dependencies" {
			t.Errorf("Expected type=dependencies, got %s", dep.Type)
		}
	}

	// Test extracting dev dependencies
	devDeps := analyzer.extractJSONDependencies(packageJSON, "devDependencies")
	if len(devDeps) != 2 {
		t.Errorf("Expected 2 dev dependencies, got %d", len(devDeps))
	}
}

func TestExtractNpmDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	// Create a temporary package.json
	tempDir, err := os.MkdirTemp("", "npm_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	packageJSON := `{
		"name": "test-project",
		"dependencies": {
			"express": "^4.18.0",
			"lodash": "^4.17.21"
		},
		"devDependencies": {
			"jest": "^29.0.0"
		}
	}`

	packagePath := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(packagePath, []byte(packageJSON), 0600)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	deps := analyzer.extractNpmDependencies(packagePath)
	if len(deps) != 3 { // 2 dependencies + 1 devDependency
		t.Errorf("Expected 3 total dependencies, got %d", len(deps))
	}

	// Check that both runtime and dev dependencies are included
	hasExpress := false
	hasJest := false
	for _, dep := range deps {
		if dep.Name == "express" {
			hasExpress = true
			if dep.Type != "dependencies" {
				t.Errorf("Expected express to be runtime dependency")
			}
		}
		if dep.Name == "jest" {
			hasJest = true
			if dep.Type != "devDependencies" {
				t.Errorf("Expected jest to be dev dependency")
			}
		}
	}

	if !hasExpress {
		t.Errorf("Expected to find express dependency")
	}
	if !hasJest {
		t.Errorf("Expected to find jest dev dependency")
	}
}

func TestExtractPipDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	// Create a temporary requirements.txt
	tempDir, err := os.MkdirTemp("", "pip_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	requirements := `# This is a comment
flask>=2.0.0
requests==2.28.1
django~=4.1.0
# Another comment
pytest
`

	requirementsPath := filepath.Join(tempDir, "requirements.txt")
	err = os.WriteFile(requirementsPath, []byte(requirements), 0600)
	if err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	deps := analyzer.extractPipDependencies(requirementsPath)
	if len(deps) != 4 { // flask, requests, django, pytest
		t.Errorf("Expected 4 dependencies, got %d", len(deps))
	}

	// Check specific dependencies
	depNames := make(map[string]bool)
	for _, dep := range deps {
		depNames[dep.Name] = true
		if dep.Manager != "pip" {
			t.Errorf("Expected manager=pip, got %s", dep.Manager)
		}
		if dep.Type != "runtime" {
			t.Errorf("Expected type=runtime, got %s", dep.Type)
		}
	}

	expected := []string{"flask", "requests", "django", "pytest"}
	for _, name := range expected {
		if !depNames[name] {
			t.Errorf("Expected to find dependency %s", name)
		}
	}
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !slices.Contains(slice, "banana") {
		t.Errorf("Expected to find 'banana' in slice")
	}

	if slices.Contains(slice, "grape") {
		t.Errorf("Expected not to find 'grape' in slice")
	}

	if slices.Contains([]string{}, "anything") {
		t.Errorf("Expected not to find anything in empty slice")
	}
}

func TestExtractPortFromFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	tests := []struct {
		name         string
		content      string
		expectedPort int
	}{
		{
			name:         "explicit port variable",
			content:      "PORT=3000",
			expectedPort: 3000,
		},
		{
			name:         "port in environment",
			content:      "process.env.PORT || 8080",
			expectedPort: 8080,
		},
		{
			name:         "listen call",
			content:      "app.listen(port: 5000)",
			expectedPort: 5000,
		},
		{
			name:         "no port found",
			content:      "console.log('hello world')",
			expectedPort: 0,
		},
		{
			name:         "invalid port too high",
			content:      "PORT=99999",
			expectedPort: 0,
		},
		{
			name:         "case insensitive",
			content:      "port: 4200",
			expectedPort: 4200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempDir, err := os.MkdirTemp("", "port_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			testFile := filepath.Join(tempDir, "test.txt")
			err = os.WriteFile(testFile, []byte(tt.content), 0600)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			port := analyzer.extractPortFromFile(testFile)
			if port != tt.expectedPort {
				t.Errorf("Expected port %d, got %d", tt.expectedPort, port)
			}
		})
	}
}

func TestContainsDatabaseConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "contains database",
			content:  "DATABASE_URL=postgres://localhost:5432/mydb",
			expected: true,
		},
		{
			name:     "contains mongodb",
			content:  "MONGODB_URI=mongodb://localhost:27017",
			expected: true,
		},
		{
			name:     "contains mysql",
			content:  "MYSQL_HOST=localhost",
			expected: true,
		},
		{
			name:     "contains postgres",
			content:  "POSTGRES_DB=myapp",
			expected: true,
		},
		{
			name:     "contains redis",
			content:  "REDIS_URL=redis://localhost:6379",
			expected: true,
		},
		{
			name:     "contains connection",
			content:  "DB_CONNECTION=mysql",
			expected: true,
		},
		{
			name:     "contains db underscore",
			content:  "DB_PASSWORD=secret",
			expected: true,
		},
		{
			name:     "no database keywords",
			content:  "API_KEY=12345\nSECRET=abcdef",
			expected: false,
		},
		{
			name:     "case insensitive",
			content:  "Database_Host=localhost",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempDir, err := os.MkdirTemp("", "db_config_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			testFile := filepath.Join(tempDir, "config.txt")
			err = os.WriteFile(testFile, []byte(tt.content), 0600)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := analyzer.containsDatabaseConfig(testFile)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateSuggestions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	analyzer := NewRepositoryAnalyzer(logger)

	result := &AnalysisResult{
		Language:     "python",
		Framework:    "flask",
		Dependencies: []Dependency{{Name: "flask"}},
		Port:         5000,
		DatabaseInfo: &DatabaseInfo{Detected: true},
	}

	suggestions := analyzer.generateSuggestions(result)

	if len(suggestions) == 0 {
		t.Errorf("Expected suggestions to be generated")
	}

	// Check for expected suggestion patterns
	hasLanguage := false
	hasFramework := false
	hasDependencies := false
	hasDatabase := false
	hasPort := false

	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, "python project") {
			hasLanguage = true
		}
		if strings.Contains(suggestion, "flask framework") {
			hasFramework = true
		}
		if strings.Contains(suggestion, "1 dependencies") {
			hasDependencies = true
		}
		if strings.Contains(suggestion, "Database usage detected") {
			hasDatabase = true
		}
		if strings.Contains(suggestion, "port 5000") {
			hasPort = true
		}
	}

	if !hasLanguage {
		t.Errorf("Expected language suggestion")
	}
	if !hasFramework {
		t.Errorf("Expected framework suggestion")
	}
	if !hasDependencies {
		t.Errorf("Expected dependencies suggestion")
	}
	if !hasDatabase {
		t.Errorf("Expected database suggestion")
	}
	if !hasPort {
		t.Errorf("Expected port suggestion")
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"3000", 3000},
		{"abc", 0},      // Invalid input should return 0
		{"", 0},         // Empty input should return 0
		{"123abc", 123}, // Should parse the numeric part
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}
