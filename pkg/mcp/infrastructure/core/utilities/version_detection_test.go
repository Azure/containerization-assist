package utilities

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/version"
)

func TestVersionDetection(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "version-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	analyzer := NewRepositoryAnalyzer(logger)

	tests := []struct {
		name              string
		files             map[string]string
		expectedLanguage  string
		expectedLangVer   string
		expectedFramework string
		expectedFrameVer  string
	}{
		{
			name: "Node.js with package.json engines",
			files: map[string]string{
				"package.json": `{
					"name": "test-app",
					"engines": {
						"node": "18.17.0"
					},
					"dependencies": {
						"express": "^4.18.0"
					}
				}`,
			},
			expectedLanguage:  "javascript",
			expectedLangVer:   "18.17.0",
			expectedFramework: "express",
			expectedFrameVer:  "^4.18.0",
		},
		{
			name: "Python with requirements.txt",
			files: map[string]string{
				"requirements.txt": "flask==2.3.0\ndjango>=4.2.0",
				".python-version":  "3.11.5",
			},
			expectedLanguage: "python",
			expectedLangVer:  "3.11.5",
		},
		{
			name: "Go with go.mod",
			files: map[string]string{
				"go.mod": `module test-app

go 1.21.1

require (
	github.com/gin-gonic/gin v1.9.1
)`,
			},
			expectedLanguage:  "go",
			expectedLangVer:   "1.21.1",
			expectedFramework: "gin",
			expectedFrameVer:  "1.9.1",
		},
		{
			name: "Java Maven with version",
			files: map[string]string{
				"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
	<properties>
		<maven.compiler.source>17</maven.compiler.source>
		<maven.compiler.target>17</maven.compiler.target>
	</properties>
	<parent>
		<groupId>org.springframework.boot</groupId>
		<artifactId>spring-boot-starter-parent</artifactId>
		<version>3.1.0</version>
	</parent>
</project>`,
			},
			expectedLanguage:  "java",
			expectedLangVer:   "17",
			expectedFramework: "maven",
		},
		{
			name: "Rust with Cargo.toml",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "test-app"
version = "0.1.0"
rust-version = "1.70.0"

[dependencies]
serde = "1.0"`,
			},
			expectedLanguage: "rust",
			expectedLangVer:  "1.70.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			// Create test files
			for filename, content := range tt.files {
				filePath := filepath.Join(testDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			// Run analysis
			result, err := analyzer.AnalyzeRepository(testDir)
			if err != nil {
				t.Fatalf("Analysis failed: %v", err)
			}

			if !result.Success {
				t.Fatalf("Analysis was not successful")
			}

			// Check language detection
			if result.Language != tt.expectedLanguage {
				t.Errorf("Expected language %s, got %s", tt.expectedLanguage, result.Language)
			}

			// Check language version detection
			if tt.expectedLangVer != "" && result.LanguageVersion != tt.expectedLangVer {
				t.Errorf("Expected language version %s, got %s", tt.expectedLangVer, result.LanguageVersion)
			}

			// Check framework detection
			if tt.expectedFramework != "" && result.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, result.Framework)
			}

			// Check framework version detection
			if tt.expectedFrameVer != "" && result.FrameworkVersion != tt.expectedFrameVer {
				t.Errorf("Expected framework version %s, got %s", tt.expectedFrameVer, result.FrameworkVersion)
			}

			t.Logf("Analysis result: Language=%s (%s), Framework=%s (%s)",
				result.Language, result.LanguageVersion,
				result.Framework, result.FrameworkVersion)
		})
	}
}

func TestDetectNodeVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "node-version-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	versionDetector := version.NewDetector(logger)

	tests := []struct {
		name        string
		files       map[string]string
		expectedVer string
	}{
		{
			name: "nvmrc file",
			files: map[string]string{
				".nvmrc": "18.17.0",
			},
			expectedVer: "18.17.0",
		},
		{
			name: "package.json engines",
			files: map[string]string{
				"package.json": `{"engines": {"node": ">=16.0.0"}}`,
			},
			expectedVer: ">=16.0.0",
		},
		{
			name: "Dockerfile",
			files: map[string]string{
				"Dockerfile": "FROM node:18.17-alpine\nRUN npm install",
			},
			expectedVer: "18.17-alpine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			for filename, content := range tt.files {
				filePath := filepath.Join(testDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			version := versionDetector.DetectLanguageVersion(testDir, "javascript")
			if version != tt.expectedVer {
				t.Errorf("Expected version %s, got %s", tt.expectedVer, version)
			}
		})
	}
}

func TestDetectPythonVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "python-version-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	versionDetector := version.NewDetector(logger)

	tests := []struct {
		name        string
		files       map[string]string
		expectedVer string
	}{
		{
			name: "python-version file",
			files: map[string]string{
				".python-version": "3.11.5",
			},
			expectedVer: "3.11.5",
		},
		{
			name: "pyproject.toml",
			files: map[string]string{
				"pyproject.toml": `[tool.poetry.dependencies]
python = "^3.11"`,
			},
			expectedVer: "^3.11",
		},
		{
			name: "runtime.txt",
			files: map[string]string{
				"runtime.txt": "python-3.11.5",
			},
			expectedVer: "3.11.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			for filename, content := range tt.files {
				filePath := filepath.Join(testDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			version := versionDetector.DetectLanguageVersion(testDir, "python")
			if version != tt.expectedVer {
				t.Errorf("Expected version %s, got %s", tt.expectedVer, version)
			}
		})
	}
}
