package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
)

func main() {
	// Create a test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create test analysis results
	testCases := []struct {
		name             string
		language         string
		framework        string
		port             int
		langVersion      string
		frameworkVersion string
	}{
		{"Go", "go", "", 8080, "1.21", ""},
		{"Java Standard", "java", "spring", 8080, "17", "3.1.0"},
		{"Java Servlet", "java", "servlet", 8080, "11", ""},
		{"Node.js", "javascript", "express", 3000, "18.0.0", "4.18.0"},
		{"Python Django", "python", "django", 8000, "3.11", "4.2"},
		{"Python FastAPI", "python", "fastapi", 8000, "3.9", "0.104.0"},
		{"Rust", "rust", "", 8080, "1.70", ""},
		{"PHP", "php", "laravel", 80, "8.2", "10.0"},
		{"Unknown", "cobol", "", 8080, "", ""},
	}

	for _, tc := range testCases {
		fmt.Printf("\n=== Testing %s ===\n", tc.name)

		analyzeResult := &steps.AnalyzeResult{
			Language:  tc.language,
			Framework: tc.framework,
			Port:      tc.port,
			Analysis: map[string]interface{}{
				"language_version":  tc.langVersion,
				"framework_version": tc.frameworkVersion,
			},
		}

		result, err := steps.GenerateDockerfile(analyzeResult, logger)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Language: %s, Framework: %s, Port: %d\n", tc.language, tc.framework, tc.port)
		fmt.Printf("Base Image: %s\n", result.BaseImage)
		fmt.Printf("Language Version: %s\n", result.LanguageVersion)
		fmt.Printf("Framework Version: %s\n", result.FrameworkVersion)
		fmt.Printf("Dockerfile (first 10 lines):\n")

		lines := strings.Split(result.Content, "\n")
		for i, line := range lines {
			if i >= 10 {
				fmt.Printf("... (truncated)\n")
				break
			}
			fmt.Printf("%s\n", line)
		}
	}
}
