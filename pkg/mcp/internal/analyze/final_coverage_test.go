package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// Test to push coverage over 15%
func TestExtraSimpleCoverage(t *testing.T) {
	logger := zerolog.Nop()

	// Test NewAnalyzer multiple times with different scenarios
	analyzers := []*Analyzer{
		NewAnalyzer(logger.With().Str("test", "1").Logger()),
		NewAnalyzer(logger.With().Str("test", "2").Logger()),
		NewAnalyzer(logger.With().Str("test", "3").Logger()),
	}

	for i, analyzer := range analyzers {
		if analyzer == nil {
			t.Errorf("Analyzer %d should not be nil", i)
		}
	}

	// Test with a real directory that has varied content
	tempDir := t.TempDir()

	// Create a comprehensive test repository structure
	testStructure := map[string]string{
		"main.go":                  "package main\n\nfunc main() { println(\"hello\") }\n",
		"go.mod":                   "module test\n\ngo 1.21\n",
		"go.sum":                   "example.com/pkg v1.0.0 h1:abc123\n",
		"package.json":             `{"name": "test", "version": "1.0.0"}`,
		"yarn.lock":                "# yarn lockfile v1\n",
		"Cargo.toml":               "[package]\nname = \"test\"\nversion = \"0.1.0\"\n",
		"requirements.txt":         "flask==2.0.1\nrequests==2.26.0\n",
		"pom.xml":                  "<project><artifactId>test</artifactId></project>\n",
		"Dockerfile":               "FROM alpine:latest\nRUN apk add --no-cache ca-certificates\n",
		"docker-compose.yml":       "version: '3.8'\nservices:\n  app:\n    build: .\n",
		"docker-compose.yaml":      "version: '3.8'\nservices:\n  web:\n    image: nginx\n",
		"kubernetes.yml":           "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n",
		"deployment.yaml":          "apiVersion: apps/v1\nkind: Deployment\n",
		"service.yaml":             "apiVersion: v1\nkind: Service\n",
		"ingress.yml":              "apiVersion: networking.k8s.io/v1\nkind: Ingress\n",
		"configmap.yaml":           "apiVersion: v1\nkind: ConfigMap\n",
		"main_test.go":             "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n",
		"app_test.py":              "import unittest\n\nclass TestApp(unittest.TestCase):\n    pass\n",
		"test_component.js":        "describe('component', () => {\n  it('works', () => {});\n});\n",
		"Makefile":                 "all:\n\tgo build -o app .\n\ntest:\n\tgo test ./...\n",
		"build.sh":                 "#!/bin/bash\nset -e\ngo build -o app .\n",
		"build.gradle":             "plugins {\n    id 'java'\n}\n",
		"CMakeLists.txt":           "cmake_minimum_required(VERSION 3.10)\nproject(test)\n",
		"schema.sql":               "CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(255));\n",
		"migrations/001_init.sql":  "CREATE DATABASE test;\n",
		"data.db":                  "SQLite format 3\x00",
		"config.toml":              "[database]\nurl = \"postgres://localhost/test\"\n",
		"settings.ini":             "[section]\nkey=value\n",
		"app.properties":           "spring.datasource.url=jdbc:h2:mem:testdb\n",
		".env":                     "DATABASE_URL=postgres://localhost/test\n",
		".gitignore":               "*.log\n*.tmp\nnode_modules/\n",
		".dockerignore":            "*.log\n*.tmp\n.git/\n",
		"README.md":                "# Test Project\n\nThis is a test project for coverage.\n",
		"LICENSE":                  "MIT License\n\nCopyright (c) 2023\n",
		"CHANGELOG.md":             "# Changelog\n\n## v1.0.0\n- Initial release\n",
		".github/workflows/ci.yml": "name: CI\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
		".gitlab-ci.yml":           "stages:\n  - test\n\ntest:\n  script:\n    - go test\n",
		"Jenkinsfile":              "pipeline {\n  agent any\n  stages {\n    stage('Test') {\n      steps {\n        sh 'go test'\n      }\n    }\n  }\n}",
	}

	// Create all the test files
	for filePath, content := range testStructure {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)

		// Create directory if needed
		if dir != tempDir {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	// Test analysis with the comprehensive structure
	analyzer := NewAnalyzer(logger)
	ctx := context.Background()

	options := AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "comprehensive",
		LanguageHint: "multi",
		SessionID:    "comprehensive-test",
	}

	result, err := analyzer.Analyze(ctx, options)
	if err != nil {
		t.Errorf("Comprehensive analysis failed: %v", err)
	}

	if result == nil {
		t.Error("Result should not be nil")
		return
	}

	if result.Context == nil {
		t.Error("Result context should not be nil")
		return
	}

	// Verify that many files were analyzed
	if result.Context.FilesAnalyzed < 10 {
		t.Errorf("Expected to analyze many files, got %d", result.Context.FilesAnalyzed)
	}

	// Test that various file types were detected
	totalDetected := len(result.Context.ConfigFilesFound) +
		len(result.Context.EntryPointsFound) +
		len(result.Context.TestFilesFound) +
		len(result.Context.BuildFilesFound) +
		len(result.Context.PackageManagers) +
		len(result.Context.DatabaseFiles) +
		len(result.Context.DockerFiles) +
		len(result.Context.K8sFiles)

	if totalDetected == 0 {
		t.Error("Should have detected various file types")
	}

	// Verify repository insights
	if !result.Context.HasGitIgnore {
		t.Error("Should have detected .gitignore")
	}
	if !result.Context.HasReadme {
		t.Error("Should have detected README.md")
	}
	if !result.Context.HasLicense {
		t.Error("Should have detected LICENSE")
	}
	if !result.Context.HasCI {
		t.Error("Should have detected CI configuration")
	}

	// Verify suggestions were generated
	if len(result.Context.ContainerizationSuggestions) == 0 {
		t.Error("Should have generated containerization suggestions")
	}
	if len(result.Context.NextStepSuggestions) == 0 {
		t.Error("Should have generated next step suggestions")
	}
}
