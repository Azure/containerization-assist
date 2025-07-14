//go:build integration

package steps_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeStep_RealRepository demonstrates integration testing with real file system
func TestAnalyzeStep_RealRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name              string
		setupFunc         func(t *testing.T, dir string)
		expectedLang      string
		expectedFramework string
		expectedPort      int
	}{
		{
			name: "Go project with gin",
			setupFunc: func(t *testing.T, dir string) {
				// Create go.mod
				goMod := `module test-app
go 1.21

require github.com/gin-gonic/gin v1.9.0`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644))

				// Create main.go
				mainGo := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})
	r.Run(":8080")
}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0644))
			},
			expectedLang:      "go",
			expectedFramework: "gin",
			expectedPort:      8080,
		},
		{
			name: "Node.js project with Express",
			setupFunc: func(t *testing.T, dir string) {
				// Create package.json
				packageJSON := `{
  "name": "test-app",
  "version": "1.0.0",
  "main": "index.js",
  "scripts": {
    "start": "node index.js"
  },
  "dependencies": {
    "express": "^4.18.0"
  }
}`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644))

				// Create index.js
				indexJS := `const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ message: 'hello' });
});

app.listen(PORT, () => {
  console.log('Server running on port', PORT);
});`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte(indexJS), 0644))
			},
			expectedLang:      "javascript",
			expectedFramework: "express",
			expectedPort:      3000,
		},
		{
			name: "Python Flask project",
			setupFunc: func(t *testing.T, dir string) {
				// Create requirements.txt
				requirements := `Flask==2.3.0
gunicorn==20.1.0`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644))

				// Create app.py
				appPy := `from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def hello():
    return jsonify({"message": "hello"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte(appPy), 0644))
			},
			expectedLang:      "python",
			expectedFramework: "flask",
			expectedPort:      5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			logger := testutil.NewTestLogger(t)
			tempDir := t.TempDir()

			// Setup project structure
			tt.setupFunc(t, tempDir)

			// Create analyze step
			step := steps.NewAnalyzeStep()

			// Create workflow state
			state := &workflow.State{
				Request: &workflow.ContainerizeRequest{
					RepoPath: tempDir,
				},
				Logger: logger,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Act
			result, err := step.Execute(ctx, state)

			// Assert
			testutil.AssertNoError(t, err, "analyze step execution")
			require.NotNil(t, result)
			assert.True(t, result.Success)

			analyzeResult, ok := result.Data.(*workflow.AnalyzeResult)
			require.True(t, ok, "result should be AnalyzeResult")

			assert.Equal(t, tt.expectedLang, analyzeResult.Language)
			assert.Equal(t, tt.expectedFramework, analyzeResult.Framework)
			assert.Equal(t, tt.expectedPort, analyzeResult.Port)

			// Verify logs
			testutil.AssertLogged(t, logger, "Analyzing repository")
			testutil.AssertLogged(t, logger, tt.expectedLang)
		})
	}
}

// TestDockerfileGeneration_Integration demonstrates testing Dockerfile generation
func TestDockerfileGeneration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Arrange
	logger := testutil.NewTestLogger(t)
	tempDir := t.TempDir()

	// Create a simple Go project
	goMod := `module test-app
go 1.21`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644))

	mainGo := `package main
func main() {
	println("Hello, World!")
}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainGo), 0644))

	// Create workflow state with analyze result
	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
		Results: map[string]*workflow.StepResult{
			"analyze": {
				Success: true,
				Data: &workflow.AnalyzeResult{
					Language:    "go",
					Framework:   "standard",
					BuildSystem: "go mod",
					Port:        8080,
				},
			},
		},
	}

	// Create dockerfile step
	step := steps.NewDockerfileStep()
	ctx := context.Background()

	// Act
	result, err := step.Execute(ctx, state)

	// Assert
	testutil.AssertNoError(t, err, "dockerfile generation")
	require.NotNil(t, result)
	assert.True(t, result.Success)

	dockerfileResult, ok := result.Data.(*workflow.DockerfileResult)
	require.True(t, ok)

	// Verify Dockerfile content
	testutil.AssertContains(t, dockerfileResult.Content, "FROM golang:", "base image")
	testutil.AssertContains(t, dockerfileResult.Content, "go mod download", "dependency download")
	testutil.AssertContains(t, dockerfileResult.Content, "go build", "build command")
	testutil.AssertContains(t, dockerfileResult.Content, "EXPOSE 8080", "port exposure")

	// Verify Dockerfile was written to disk
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	assert.FileExists(t, dockerfilePath)

	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Equal(t, dockerfileResult.Content, string(content))
}

// TestWorkflowIntegration_EndToEnd demonstrates a complete workflow integration test
func TestWorkflowIntegration_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require Docker to be available
	// It's marked as integration but would need additional setup
	t.Skip("End-to-end test requires Docker daemon")

	// Example of what an end-to-end test would look like:
	/*
		// Arrange
		logger := testutil.NewTestLogger(t)
		tempDir := t.TempDir()

		// Create test project
		createTestProject(t, tempDir)

		// Create all steps
		analyzeStep := steps.NewAnalyzeStep()
		dockerfileStep := steps.NewDockerfileStep()
		buildStep := steps.NewBuildStep()

		// Create workflow state
		state := createWorkflowState(tempDir, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Act - Execute steps in sequence
		steps := []workflow.Step{analyzeStep, dockerfileStep, buildStep}
		for _, step := range steps {
			result, err := step.Execute(ctx, state)
			require.NoError(t, err, "step %s failed", step.Name())
			state.Results[step.Name()] = result
		}

		// Assert
		buildResult := state.Results["build"].Data.(*workflow.BuildResult)
		testutil.AssertImageRef(t, buildResult.ImageRef)
	*/
}

// TestPerformance_AnalyzeStep demonstrates performance testing for integration tests
func TestPerformance_AnalyzeStep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a large project structure
	tempDir := t.TempDir()

	// Create many files to test performance
	for i := 0; i < 100; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf(`package main
// File %d
func function%d() {
	println("Function %d")
}`, i, i, i)
		require.NoError(t, os.WriteFile(filename, []byte(content), 0644))
	}

	// Add go.mod
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, "go.mod"),
		[]byte("module perf-test\ngo 1.21"),
		0644,
	))

	logger := testutil.NewDiscardLogger()
	step := steps.NewAnalyzeStep()
	state := &workflow.State{
		Request: &workflow.ContainerizeRequest{
			RepoPath: tempDir,
		},
		Logger: logger,
	}

	ctx := context.Background()

	// Measure performance
	start := time.Now()
	result, err := step.Execute(ctx, state)
	duration := time.Since(start)

	// Assert
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Performance assertion - should complete within reasonable time
	assert.Less(t, duration, 5*time.Second,
		"Analysis of 100 files should complete within 5 seconds, took %v", duration)

	t.Logf("Analyzed 100 files in %v", duration)
}
