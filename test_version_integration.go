package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test with Go project
	analyzeResult := &steps.AnalyzeResult{
		Language:  "go",
		Framework: "gin",
		Port:      8080,
		Analysis: map[string]interface{}{
			"language_version":  "1.21.0",
			"framework_version": "v1.9.1",
		},
		RepoPath: "/test/repo",
	}

	result, err := steps.GenerateDockerfile(analyzeResult, logger)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Generated Dockerfile for Go 1.21.0:\n")
	fmt.Printf("Base Image: %s\n", result.BaseImage)
	fmt.Printf("Language Version: %s\n", result.LanguageVersion)
	fmt.Printf("Framework Version: %s\n", result.FrameworkVersion)
	fmt.Printf("Dockerfile Content:\n%s\n", result.Content)

	// Verify the Dockerfile uses the correct Go version
	if !strings.Contains(result.Content, "golang:1.21.0-alpine") {
		fmt.Printf("❌ ERROR: Dockerfile should contain golang:1.21.0-alpine\n")
	} else {
		fmt.Printf("✅ SUCCESS: Dockerfile correctly uses Go 1.21.0\n")
	}

	// Test with Node.js project
	analyzeResult = &steps.AnalyzeResult{
		Language:  "javascript",
		Framework: "nextjs",
		Port:      3000,
		Analysis: map[string]interface{}{
			"language_version":  "18.16.0",
			"framework_version": "13.4.0",
		},
		RepoPath: "/test/repo",
	}

	result, err = steps.GenerateDockerfile(analyzeResult, logger)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nGenerated Dockerfile for Node.js 18.16.0:\n")
	fmt.Printf("Base Image: %s\n", result.BaseImage)
	fmt.Printf("Language Version: %s\n", result.LanguageVersion)
	fmt.Printf("Framework Version: %s\n", result.FrameworkVersion)

	// Verify the Dockerfile uses the correct Node.js version
	if !strings.Contains(result.Content, "node:18-alpine") {
		fmt.Printf("❌ ERROR: Dockerfile should contain node:18-alpine\n")
	} else {
		fmt.Printf("✅ SUCCESS: Dockerfile correctly uses Node.js 18\n")
	}
}
