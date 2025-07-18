package version

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguageVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "version-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	detector := NewDetector(logger)

	tests := []struct {
		name        string
		language    string
		files       map[string]string
		expectedVer string
	}{
		{
			name:     "Node.js with .nvmrc",
			language: "javascript",
			files: map[string]string{
				".nvmrc": "18.17.0",
			},
			expectedVer: "18.17.0",
		},
		{
			name:     "Node.js with package.json engines",
			language: "typescript",
			files: map[string]string{
				"package.json": `{"engines": {"node": ">=16.0.0"}}`,
			},
			expectedVer: ">=16.0.0",
		},
		{
			name:     "Python with .python-version",
			language: "python",
			files: map[string]string{
				".python-version": "3.11.5",
			},
			expectedVer: "3.11.5",
		},
		{
			name:     "Python with pyproject.toml",
			language: "python",
			files: map[string]string{
				"pyproject.toml": `[tool.poetry.dependencies]
python = "^3.11"`,
			},
			expectedVer: "^3.11",
		},
		{
			name:     "Go with go.mod",
			language: "go",
			files: map[string]string{
				"go.mod": `module test
go 1.21`,
			},
			expectedVer: "1.21",
		},
		{
			name:     "Java with pom.xml",
			language: "java",
			files: map[string]string{
				"pom.xml": `<project>
	<properties>
		<java.version>17</java.version>
	</properties>
</project>`,
			},
			expectedVer: "17",
		},
		{
			name:     "Rust with Cargo.toml",
			language: "rust",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "test"
rust-version = "1.70"`,
			},
			expectedVer: "1.70",
		},
		{
			name:     "PHP with composer.json",
			language: "php",
			files: map[string]string{
				"composer.json": `{"require": {"php": "^8.1"}}`,
			},
			expectedVer: "^8.1",
		},
		{
			name:     "Ruby with .ruby-version",
			language: "ruby",
			files: map[string]string{
				".ruby-version": "3.2.0",
			},
			expectedVer: "3.2.0",
		},
		{
			name:     "C# with .csproj",
			language: "csharp",
			files: map[string]string{
				"test.csproj": `<Project Sdk="Microsoft.NET.Sdk">
	<PropertyGroup>
		<TargetFramework>net8.0</TargetFramework>
	</PropertyGroup>
</Project>`,
			},
			expectedVer: "net8.0",
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

			version := detector.DetectLanguageVersion(testDir, tt.language)
			if version != tt.expectedVer {
				t.Errorf("Expected version %s, got %s", tt.expectedVer, version)
			}
		})
	}
}

func TestDetectFrameworkVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "framework-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	detector := NewDetector(logger)

	tests := []struct {
		name        string
		framework   string
		files       map[string]string
		expectedVer string
	}{
		{
			name:      "Express framework",
			framework: "express",
			files: map[string]string{
				"package.json": `{
					"dependencies": {
						"express": "^4.18.0"
					}
				}`,
			},
			expectedVer: "^4.18.0",
		},
		{
			name:      "React framework",
			framework: "react",
			files: map[string]string{
				"package.json": `{
					"dependencies": {
						"react": "^18.2.0"
					}
				}`,
			},
			expectedVer: "^18.2.0",
		},
		{
			name:      "Django framework",
			framework: "django",
			files: map[string]string{
				"requirements.txt": "django==4.2.0",
			},
			expectedVer: "4.2.0",
		},
		{
			name:      "Flask framework",
			framework: "flask",
			files: map[string]string{
				"requirements.txt": "flask>=2.3.0",
			},
			expectedVer: "2.3.0",
		},
		{
			name:      "Gin framework",
			framework: "gin",
			files: map[string]string{
				"go.mod": `module test
go 1.21

require github.com/gin-gonic/gin v1.9.1`,
			},
			expectedVer: "1.9.1",
		},
		{
			name:      "Spring Boot framework",
			framework: "spring-boot",
			files: map[string]string{
				"pom.xml": `<project>
	<parent>
		<groupId>org.springframework.boot</groupId>
		<artifactId>spring-boot-starter-parent</artifactId>
		<version>3.1.0</version>
	</parent>
</project>`,
			},
			expectedVer: "3.1.0",
		},
		{
			name:      "Maven framework",
			framework: "maven",
			files: map[string]string{
				".mvn/wrapper/maven-wrapper.properties": "distributionUrl=https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven-3.9.4-bin.zip",
			},
			expectedVer: "3.9.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			for filename, content := range tt.files {
				filePath := filepath.Join(testDir, filename)
				dir := filepath.Dir(filePath)
				err := os.MkdirAll(dir, 0755)
				if err != nil {
					t.Fatalf("Failed to create directory %s: %v", dir, err)
				}
				err = os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			version := detector.DetectFrameworkVersion(testDir, tt.framework)
			if version != tt.expectedVer {
				t.Errorf("Expected version %s, got %s", tt.expectedVer, version)
			}
		})
	}
}

func TestDetectGoFrameworkFromMod(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-framework-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	detector := NewDetector(logger)

	tests := []struct {
		name              string
		goModContent      string
		expectedFramework string
	}{
		{
			name: "Gin framework",
			goModContent: `module test
go 1.21

require github.com/gin-gonic/gin v1.9.1`,
			expectedFramework: "gin",
		},
		{
			name: "Echo framework",
			goModContent: `module test
go 1.21

require github.com/labstack/echo v4.11.1`,
			expectedFramework: "echo",
		},
		{
			name: "Fiber framework",
			goModContent: `module test
go 1.21

require github.com/gofiber/fiber v2.48.0`,
			expectedFramework: "fiber",
		},
		{
			name: "No framework",
			goModContent: `module test
go 1.21

require github.com/stretchr/testify v1.8.4`,
			expectedFramework: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			goModPath := filepath.Join(testDir, "go.mod")
			err = os.WriteFile(goModPath, []byte(tt.goModContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create go.mod: %v", err)
			}

			framework := detector.DetectGoFrameworkFromMod(testDir)
			if framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, framework)
			}
		})
	}
}
