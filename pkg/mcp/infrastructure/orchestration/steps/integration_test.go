//go:build integration

package steps

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowIntegration_AnalyzeStep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary test project
	tempDir := t.TempDir()

	// Create a simple Go project structure
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(`
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(`
module test-app

go 1.21
`), 0644)
	require.NoError(t, err)

	// Create analyze step
	analyzeStep := NewAnalyzeStep()

	// Create workflow state
	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
	}

	// Execute analyze step
	result, err := analyzeStep.Execute(ctx, state)
	require.NoError(t, err, "Analyze step should succeed")
	require.NotNil(t, result, "Result should not be nil")

	// Verify analysis results
	analyzeResult, ok := result.Data.(*workflow.AnalyzeResult)
	require.True(t, ok, "Result should be AnalyzeResult")
	assert.NotEmpty(t, analyzeResult.Language, "Language should be detected")
	assert.NotEmpty(t, analyzeResult.Framework, "Framework should be detected")
	assert.NotEmpty(t, analyzeResult.BuildSystem, "Build system should be detected")

	// For Go projects, we expect specific values
	assert.Equal(t, "go", analyzeResult.Language, "Should detect Go language")
	assert.Contains(t, []string{"standard", "go"}, analyzeResult.Framework, "Should detect Go framework")
}

func TestWorkflowIntegration_DockerfileStep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary test project
	tempDir := t.TempDir()

	// Create a simple Node.js project
	err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{
  "name": "test-app",
  "version": "1.0.0",
  "main": "index.js",
  "scripts": {
    "start": "node index.js"
  },
  "dependencies": {
    "express": "^4.18.0"
  }
}`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(`
const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.listen(port, () => {
  console.log('Server running on port', port);
});
`), 0644)
	require.NoError(t, err)

	// Create workflow state with analysis result
	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
		Results: map[string]*workflow.StepResult{
			"analyze": {
				Success: true,
				Data: &workflow.AnalyzeResult{
					Language:    "javascript",
					Framework:   "node",
					BuildSystem: "npm",
					Port:        3000,
					Files: []workflow.FileInfo{
						{Path: "package.json", Type: "config"},
						{Path: "index.js", Type: "source"},
					},
				},
			},
		},
	}

	// Create dockerfile step
	dockerfileStep := NewDockerfileStep()

	// Execute dockerfile step
	result, err := dockerfileStep.Execute(ctx, state)
	require.NoError(t, err, "Dockerfile step should succeed")
	require.NotNil(t, result, "Result should not be nil")

	// Verify dockerfile generation
	dockerfileResult, ok := result.Data.(*workflow.DockerfileResult)
	require.True(t, ok, "Result should be DockerfileResult")
	assert.NotEmpty(t, dockerfileResult.Content, "Dockerfile content should not be empty")
	assert.Contains(t, dockerfileResult.Content, "FROM node:", "Should use Node.js base image")
	assert.Contains(t, dockerfileResult.Content, "COPY package.json", "Should copy package.json")
	assert.Contains(t, dockerfileResult.Content, "npm install", "Should run npm install")
	assert.Contains(t, dockerfileResult.Content, "EXPOSE 3000", "Should expose port 3000")

	// Verify dockerfile was written to disk
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	assert.FileExists(t, dockerfilePath, "Dockerfile should be created")

	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Equal(t, dockerfileResult.Content, string(content), "File content should match result")
}

func TestWorkflowIntegration_MultiStepScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a temporary Python Flask project
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "app.py"), []byte(`
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(`
Flask==2.3.0
Werkzeug==2.3.0
`), 0644)
	require.NoError(t, err)

	// Create workflow state
	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger:  logger,
		Results: make(map[string]*workflow.StepResult),
	}

	// Step 1: Analyze
	t.Run("Analyze", func(t *testing.T) {
		analyzeStep := NewAnalyzeStep()
		result, err := analyzeStep.Execute(ctx, state)
		require.NoError(t, err, "Analyze step should succeed")

		analyzeResult, ok := result.Data.(*workflow.AnalyzeResult)
		require.True(t, ok, "Result should be AnalyzeResult")
		assert.Equal(t, "python", analyzeResult.Language, "Should detect Python")
		assert.Equal(t, "flask", analyzeResult.Framework, "Should detect Flask")
		assert.Equal(t, 5000, analyzeResult.Port, "Should detect port 5000")

		state.Results["analyze"] = result
	})

	// Step 2: Generate Dockerfile
	t.Run("Dockerfile", func(t *testing.T) {
		dockerfileStep := NewDockerfileStep()
		result, err := dockerfileStep.Execute(ctx, state)
		require.NoError(t, err, "Dockerfile step should succeed")

		dockerfileResult, ok := result.Data.(*workflow.DockerfileResult)
		require.True(t, ok, "Result should be DockerfileResult")
		assert.Contains(t, dockerfileResult.Content, "FROM python:", "Should use Python base image")
		assert.Contains(t, dockerfileResult.Content, "pip install", "Should install dependencies")
		assert.Contains(t, dockerfileResult.Content, "EXPOSE 5000", "Should expose correct port")

		state.Results["dockerfile"] = result
	})

	// Step 3: Generate Manifests (simulated)
	t.Run("Manifests", func(t *testing.T) {
		manifestStep := NewManifestStep()
		result, err := manifestStep.Execute(ctx, state)
		require.NoError(t, err, "Manifest step should succeed")

		manifestResult, ok := result.Data.(*workflow.ManifestResult)
		require.True(t, ok, "Result should be ManifestResult")
		assert.NotEmpty(t, manifestResult.Manifests, "Should generate manifests")

		// Check for expected Kubernetes resources
		assert.Contains(t, manifestResult.Manifests, "deployment.yaml", "Should have deployment")
		assert.Contains(t, manifestResult.Manifests, "service.yaml", "Should have service")

		state.Results["manifest"] = result
	})

	// Verify all steps succeeded and state is consistent
	assert.Len(t, state.Results, 3, "Should have results for all 3 steps")
	for stepName, result := range state.Results {
		assert.True(t, result.Success, "Step %s should be successful", stepName)
		assert.NotNil(t, result.Data, "Step %s should have data", stepName)
	}
}

func TestWorkflowIntegration_ErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with invalid/empty directory
	tempDir := t.TempDir()

	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir, // Empty directory
		},
		Logger: logger,
	}

	analyzeStep := NewAnalyzeStep()
	result, err := analyzeStep.Execute(ctx, state)

	// Should handle gracefully - either succeed with "unknown" detection or fail cleanly
	if err != nil {
		assert.Contains(t, err.Error(), "no supported", "Should provide meaningful error")
	} else {
		// If it succeeds, should have reasonable defaults
		analyzeResult, ok := result.Data.(*workflow.AnalyzeResult)
		require.True(t, ok, "Result should be AnalyzeResult")
		assert.NotEmpty(t, analyzeResult.Language, "Should have some language detection")
	}
}

func TestWorkflowIntegration_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise for performance testing
	}))

	// Create a typical project structure
	tempDir := t.TempDir()

	// Create multiple files to simulate realistic project
	for i := 0; i < 50; i++ {
		err := os.WriteFile(filepath.Join(tempDir, "file"+string(rune(i))+".js"), []byte(`
console.log('File `+string(rune(i))+`');
module.exports = { test: true };
`), 0644)
		require.NoError(t, err)
	}

	err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{
  "name": "performance-test",
  "version": "1.0.0",
  "main": "index.js"
}`), 0644)
	require.NoError(t, err)

	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
	}

	// Measure analyze step performance
	start := time.Now()
	analyzeStep := NewAnalyzeStep()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := analyzeStep.Execute(ctx, state)
	elapsed := time.Since(start)

	require.NoError(t, err, "Analyze step should succeed")
	require.NotNil(t, result, "Result should not be nil")

	// Performance assertions - should be reasonably fast
	assert.Less(t, elapsed, 5*time.Second, "Analyze step should complete within 5 seconds")

	t.Logf("Analyze step completed in %v", elapsed)

	// Verify results are still correct despite performance focus
	analyzeResult, ok := result.Data.(*workflow.AnalyzeResult)
	require.True(t, ok, "Result should be AnalyzeResult")
	assert.Equal(t, "javascript", analyzeResult.Language, "Should still detect language correctly")
}

func TestWorkflowIntegration_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "app.py"), []byte("print('Hello')"), 0644)
	require.NoError(t, err)

	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
	}

	analyzeStep := NewAnalyzeStep()

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Allow some time for cancellation to take effect
	time.Sleep(10 * time.Millisecond)

	_, err = analyzeStep.Execute(ctx, state)

	// Should handle context cancellation gracefully
	if err != nil {
		assert.Contains(t, []string{
			context.DeadlineExceeded.Error(),
			context.Canceled.Error(),
			"context",
		}, err.Error(), "Should return context-related error")
	}
}
