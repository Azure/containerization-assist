package analyze

import (
	"testing"
	"time"
)

func TestCloneOptions_Defaults(t *testing.T) {
	opts := &CloneOptions{
		RepoURL:   "https://github.com/example/repo",
		Branch:    "main",
		Shallow:   true,
		TargetDir: "/tmp/repo",
		SessionID: "test-session",
	}

	if opts.RepoURL != "https://github.com/example/repo" {
		t.Errorf("RepoURL = %s, want https://github.com/example/repo", opts.RepoURL)
	}

	if opts.Branch != "main" {
		t.Errorf("Branch = %s, want main", opts.Branch)
	}

	if !opts.Shallow {
		t.Error("Expected shallow clone to be true")
	}
}

func TestCloneResult_Fields(t *testing.T) {
	result := &CloneResult{
		Duration: 5 * time.Second,
	}

	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", result.Duration)
	}
}

func TestAnalysisOptions_Fields(t *testing.T) {
	opts := &AnalysisOptions{
		RepoPath:     "/tmp/repo",
		Context:      "container deployment",
		LanguageHint: "Go",
		SessionID:    "test-session",
	}

	if opts.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath = %s, want /tmp/repo", opts.RepoPath)
	}

	if opts.Context != "container deployment" {
		t.Errorf("Context = %s, want container deployment", opts.Context)
	}
}

func TestAnalysisResult_Fields(t *testing.T) {
	result := &AnalysisResult{
		Duration: 10 * time.Second,
		Context: &AnalysisContext{
			FilesAnalyzed: 42,
		},
	}

	if result.Duration != 10*time.Second {
		t.Errorf("Duration = %v, want 10s", result.Duration)
	}

	if result.Context == nil {
		t.Fatal("Context should not be nil")
	}

	if result.Context.FilesAnalyzed != 42 {
		t.Errorf("FilesAnalyzed = %d, want 42", result.Context.FilesAnalyzed)
	}
}

func TestAnalysisContext_Fields(t *testing.T) {
	ctx := &AnalysisContext{
		FilesAnalyzed:    100,
		ConfigFilesFound: []string{"config.yaml", ".env"},
		EntryPointsFound: []string{"main.go", "cmd/server/main.go"},
		TestFilesFound:   []string{"main_test.go"},
		BuildFilesFound:  []string{"Dockerfile", "Makefile"},
		PackageManagers:  []string{"go modules"},
	}

	if ctx.FilesAnalyzed != 100 {
		t.Errorf("FilesAnalyzed = %d, want 100", ctx.FilesAnalyzed)
	}

	if len(ctx.ConfigFilesFound) != 2 {
		t.Errorf("ConfigFilesFound length = %d, want 2", len(ctx.ConfigFilesFound))
	}

	if len(ctx.EntryPointsFound) != 2 {
		t.Errorf("EntryPointsFound length = %d, want 2", len(ctx.EntryPointsFound))
	}
}

func TestRepoData_Structure(t *testing.T) {
	data := &RepoData{
		Path: "/tmp/repo",
		Files: []FileData{
			{Path: "main.go", Content: "package main", Size: 12},
			{Path: "go.mod", Content: "module test", Size: 11},
		},
		Languages: map[string]float64{
			"Go":       80.0,
			"Makefile": 20.0,
		},
		Structure: map[string]interface{}{
			"dirs":  []string{"cmd", "pkg"},
			"files": 10,
		},
	}

	if data.Path != "/tmp/repo" {
		t.Errorf("Path = %s, want /tmp/repo", data.Path)
	}

	if len(data.Files) != 2 {
		t.Errorf("Files length = %d, want 2", len(data.Files))
	}

	if data.Languages["Go"] != 80.0 {
		t.Errorf("Go language percentage = %f, want 80.0", data.Languages["Go"])
	}
}

func TestFileData_Structure(t *testing.T) {
	file := &FileData{
		Path:    "test.go",
		Content: "package test",
		Size:    12,
	}

	if file.Path != "test.go" {
		t.Errorf("Path = %s, want test.go", file.Path)
	}

	if file.Size != 12 {
		t.Errorf("Size = %d, want 12", file.Size)
	}
}

func TestAnalysisConfig_Structure(t *testing.T) {
	config := &AnalysisConfig{
		RepositoryPath: "/tmp/repo",
		RepoData: &RepoData{
			Path: "/tmp/repo",
		},
	}

	if config.RepositoryPath != "/tmp/repo" {
		t.Errorf("RepositoryPath = %s, want /tmp/repo", config.RepositoryPath)
	}

	if config.RepoData == nil {
		t.Fatal("RepoData should not be nil")
	}
}

func TestEngineAnalysisOptions_Defaults(t *testing.T) {
	opts := &EngineAnalysisOptions{
		IncludeFrameworks:    true,
		IncludeDependencies:  true,
		IncludeConfiguration: true,
		IncludeDatabase:      false,
		IncludeBuild:         true,
		DeepAnalysis:         false,
		MaxDepth:             3,
	}

	if !opts.IncludeFrameworks {
		t.Error("IncludeFrameworks should be true")
	}

	if opts.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", opts.MaxDepth)
	}
}
