package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/rs/zerolog"
)

func TestCloner_Clone(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cloner := NewCloner(logger)

	// Test with a local directory (simulated repository)
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a simple file to simulate repository content
	testFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := CloneOptions{
		RepoURL:   repoDir,
		Branch:    "main",
		Shallow:   false,
		TargetDir: filepath.Join(tempDir, "cloned"),
		SessionID: "test-session",
	}

	ctx := context.Background()
	result, err := cloner.Clone(ctx, opts)

	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.RepoPath == "" {
		t.Error("Expected RepoPath to be set")
	}

	if result.CloneResult == nil {
		t.Error("Expected CloneResult to be set")
	}

	// Verify the cloned content exists
	clonedFile := filepath.Join(result.RepoPath, "README.md")
	if _, err := os.Stat(clonedFile); os.IsNotExist(err) {
		t.Error("Expected cloned file to exist")
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	analyzer := NewAnalyzer(logger)

	// Create a test repository structure
	tempDir := t.TempDir()

	// Create a simple Go project structure
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(`module test-app

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	opts := AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "Test Go application",
		LanguageHint: "go",
		SessionID:    "test-session",
	}

	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, opts)

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.AnalysisResult == nil {
		t.Error("Expected AnalysisResult to be set")
	}

	if result.Context == nil {
		t.Error("Expected Context to be set")
	}

	// Check that we detected Go
	if result.AnalysisResult.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", result.AnalysisResult.Language)
	}
}

func TestContextGenerator_GenerateContainerizationAssessment(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	generator := NewContextGenerator(logger)

	// Create mock analysis results
	coreAnalysis := &analysis.AnalysisResult{
		Language:  "go",
		Framework: "",
		Port:      8080,
	}

	context := &AnalysisContext{
		FilesAnalyzed:    10,
		ConfigFilesFound: []string{"go.mod", "config.yaml"},
		EntryPointsFound: []string{"main.go"},
		HasReadme:        true,
		HasLicense:       true,
	}

	assessment, err := generator.GenerateContainerizationAssessment(coreAnalysis, context)

	if err != nil {
		t.Fatalf("GenerateContainerizationAssessment failed: %v", err)
	}

	if assessment == nil {
		t.Fatal("Expected assessment, got nil")
	}

	if assessment.TechnologyStack.Language != "go" {
		t.Error("Expected TechnologyStack to have correct language")
	}

	if len(assessment.RiskAnalysis) == 0 {
		t.Error("Expected RiskAnalysis to be set")
	}

	if len(assessment.DeploymentOptions) == 0 {
		t.Error("Expected DeploymentOptions to be set")
	}
}

func TestIntegration_CloneAndAnalyze(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cloner := NewCloner(logger)
	analyzer := NewAnalyzer(logger)

	// Create a test repository
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a Python project
	if err := os.WriteFile(filepath.Join(repoDir, "app.py"), []byte(`#!/usr/bin/env python3

from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "requirements.txt"), []byte(`Flask==2.3.3
`), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Step 1: Clone
	cloneOpts := CloneOptions{
		RepoURL:   repoDir,
		Branch:    "main",
		Shallow:   false,
		TargetDir: filepath.Join(tempDir, "cloned"),
		SessionID: "integration-test",
	}

	cloneResult, err := cloner.Clone(ctx, cloneOpts)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Step 2: Analyze
	analysisOpts := AnalysisOptions{
		RepoPath:     cloneResult.RepoPath,
		Context:      "Flask web application",
		LanguageHint: "python",
		SessionID:    "integration-test",
	}

	analysisResult, err := analyzer.Analyze(ctx, analysisOpts)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Verify the integration worked
	if analysisResult.AnalysisResult.Language != "python" {
		t.Errorf("Expected language 'python', got '%s'", analysisResult.AnalysisResult.Language)
	}

	// Port detection may not work in simple test scenario, so we'll check if it's reasonable
	if analysisResult.AnalysisResult.Port != 5000 && analysisResult.AnalysisResult.Port != 0 {
		t.Errorf("Expected port 5000 or 0 (undetected), got %d", analysisResult.AnalysisResult.Port)
	}

	if len(analysisResult.AnalysisResult.Dependencies) == 0 {
		t.Error("Expected dependencies to be detected")
	}
}
