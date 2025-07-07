package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOptimizer_OptimizeBuild(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	optimizer := NewBuildOptimizer(logger)
	ctx := context.Background()

	// Create temporary test files
	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

	tests := []struct {
		name                string
		dockerfile          string
		expectedRecommends  int
		checkRecommendation func(t *testing.T, recommendations []OptimizationRecommendation)
	}{
		{
			name: "layer ordering issue",
			dockerfile: `FROM node:16
COPY . .
RUN npm install
RUN npm run build`,
			expectedRecommends: 1,
			checkRecommendation: func(t *testing.T, recommendations []OptimizationRecommendation) {
				found := false
				for _, rec := range recommendations {
					if rec.Type == "layer_ordering" {
						found = true
						assert.Equal(t, "high", rec.Priority)
						assert.Contains(t, rec.Description, "Source code is copied before dependency installation")
					}
				}
				assert.True(t, found, "Should detect layer ordering issue")
			},
		},
		{
			name: "cache busting with ADD",
			dockerfile: `FROM ubuntu:20.04
ADD https://example.com/file.tar.gz /tmp/
RUN tar -xzf /tmp/file.tar.gz`,
			expectedRecommends: 1,
			checkRecommendation: func(t *testing.T, recommendations []OptimizationRecommendation) {
				found := false
				for _, rec := range recommendations {
					if rec.Type == "cache_busting" {
						found = true
						assert.Equal(t, "medium", rec.Priority)
						assert.Contains(t, rec.Description, "ADD with remote URL")
					}
				}
				assert.True(t, found, "Should detect cache busting with ADD")
			},
		},
		{
			name: "multi-stage opportunity",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y gcc make build-essential
COPY . /app
WORKDIR /app
RUN make build
CMD ["/app/bin/myapp"]`,
			expectedRecommends: 1,
			checkRecommendation: func(t *testing.T, recommendations []OptimizationRecommendation) {
				found := false
				for _, rec := range recommendations {
					if rec.Type == "multi_stage" {
						found = true
						assert.Equal(t, "high", rec.Priority)
						assert.Contains(t, rec.Description, "Build tools present in final image")
					}
				}
				assert.True(t, found, "Should detect multi-stage opportunity")
			},
		},
		{
			name: "npm install optimization",
			dockerfile: `FROM node:16
COPY package*.json ./
RUN npm install
COPY . .`,
			expectedRecommends: 1,
			checkRecommendation: func(t *testing.T, recommendations []OptimizationRecommendation) {
				found := false
				for _, rec := range recommendations {
					if rec.Type == "package_manager" {
						found = true
						assert.Equal(t, "medium", rec.Priority)
						assert.Contains(t, rec.Description, "npm install is slower")
					}
				}
				assert.True(t, found, "Should recommend npm ci")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test Dockerfile
			err := os.WriteFile(dockerfilePath, []byte(tt.dockerfile), 0644)
			require.NoError(t, err)

			result, err := optimizer.OptimizeBuild(ctx, dockerfilePath, tmpDir)
			require.NoError(t, err)
			assert.NotNil(t, result)

			if tt.expectedRecommends > 0 {
				assert.GreaterOrEqual(t, len(result.Recommendations), tt.expectedRecommends)
			}

			if tt.checkRecommendation != nil {
				tt.checkRecommendation(t, result.Recommendations)
			}

			// Check that cache strategy is generated
			assert.NotEmpty(t, result.CacheStrategy.CacheMode)
			assert.NotEmpty(t, result.CacheStrategy.CacheKey)

			// Check that layer strategy is generated
			assert.NotEmpty(t, result.LayerStrategy.OptimalOrder)
			assert.True(t, result.LayerStrategy.CombineCommands)
		})
	}
}

func TestCacheManager_GenerateStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cacheManager := NewCacheManager(logger)

	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

	// Create test files
	dockerfile := `FROM node:16
COPY package*.json ./
RUN npm ci
COPY . .`

	err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644)
	require.NoError(t, err)

	// Create dependency files
	packageJSON := `{"name": "test-app", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644)
	require.NoError(t, err)

	strategy := cacheManager.GenerateStrategy(dockerfilePath, tmpDir)

	assert.Equal(t, "max", strategy.CacheMode)
	assert.True(t, strategy.LayerCaching)
	assert.NotEmpty(t, strategy.CacheKey)
	assert.Len(t, strategy.CacheFrom, 2)
	assert.Len(t, strategy.CacheTo, 2)
	assert.Contains(t, strategy.BuildKitFeatures, "inline-cache")
	assert.Contains(t, strategy.BuildKitFeatures, "cache-mounts")
}

func TestLayerOptimizer_GenerateStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	layerOptimizer := NewLayerOptimizer(logger)

	strategy := layerOptimizer.GenerateStrategy("/path/to/Dockerfile")

	assert.NotEmpty(t, strategy.OptimalOrder)
	assert.True(t, strategy.CombineCommands)
	assert.True(t, strategy.MinimizeLayers)
	assert.True(t, strategy.CleanupCommands)
	assert.NotEmpty(t, strategy.SquashPoints)

	// Check optimal order contains expected steps
	expectedSteps := []string{
		"FROM base_image",
		"RUN install_system_dependencies",
		"COPY dependency_files",
		"RUN install_app_dependencies",
		"COPY source_code",
		"RUN build_application",
		"CMD start_application",
	}

	for _, step := range expectedSteps {
		assert.Contains(t, strategy.OptimalOrder, step)
	}
}

func TestContextOptimizer_Analyze(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	contextOptimizer := NewContextOptimizer(logger)

	tests := []struct {
		name               string
		setupContext       func(dir string)
		expectDockerignore bool
		expectLargeFiles   bool
		minRecommendations int
	}{
		{
			name: "missing dockerignore",
			setupContext: func(dir string) {
				// Create many files without .dockerignore
				for i := 0; i < 150; i++ {
					filename := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
					os.WriteFile(filename, []byte("test content"), 0644)
				}
			},
			expectDockerignore: true,
			expectLargeFiles:   false,
			minRecommendations: 1,
		},
		{
			name: "large files in context",
			setupContext: func(dir string) {
				// Create .dockerignore
				os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte("*.log"), 0644)

				// Create large file
				largeContent := make([]byte, 15*1024*1024) // 15MB
				os.WriteFile(filepath.Join(dir, "large.bin"), largeContent, 0644)
			},
			expectDockerignore: false,
			expectLargeFiles:   true,
			minRecommendations: 1,
		},
		{
			name: "well-optimized context",
			setupContext: func(dir string) {
				// Create .dockerignore
				dockerignore := `node_modules
*.log
.git
coverage`
				os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte(dockerignore), 0644)

				// Create small source files
				os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			expectDockerignore: false,
			expectLargeFiles:   false,
			minRecommendations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupContext(tmpDir)

			strategy, err := contextOptimizer.Analyze(tmpDir)
			require.NoError(t, err)

			assert.Equal(t, tt.minRecommendations, len(strategy.Recommendations))

			if tt.expectDockerignore {
				found := false
				for _, rec := range strategy.Recommendations {
					if rec.Title == "Missing .dockerignore file" {
						found = true
						assert.Equal(t, "high", rec.Priority)
					}
				}
				assert.True(t, found, "Should recommend creating .dockerignore")
			}

			if tt.expectLargeFiles {
				found := false
				for _, rec := range strategy.Recommendations {
					if rec.Title == "Large files in build context" {
						found = true
						assert.Equal(t, "medium", rec.Priority)
					}
				}
				assert.True(t, found, "Should detect large files")
			}

			assert.True(t, strategy.IncludeOnlyNeeded)
		})
	}
}

func TestOptimizer_CalculateImprovements(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	optimizer := NewBuildOptimizer(logger)

	result := &OptimizationResult{
		Recommendations: []OptimizationRecommendation{
			{Type: "layer_ordering", Priority: "high"},
			{Type: "cache_busting", Priority: "medium"},
			{Type: "multi_stage", Priority: "high"},
			{Type: "package_manager", Priority: "low"},
		},
		ContextStrategy: ContextStrategy{
			EstimatedSizeReduction: 25,
		},
	}

	improvements := optimizer.calculateImprovements(result)

	// high = 30%, medium = 15%, low = 5%
	// 2 high + 1 medium + 1 low = 60 + 15 + 5 = 80, capped at 70
	assert.Equal(t, 70, improvements.BuildTimeReduction)
	assert.Equal(t, 25, improvements.ImageSizeReduction)
	assert.Equal(t, 40, improvements.CacheHitRateIncrease)
}

func TestOptimizer_isUnnecessaryFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	optimizer := NewContextOptimizer(logger)

	tests := []struct {
		path        string
		unnecessary bool
	}{
		{".git/refs/heads/main", true},
		{"node_modules/package/index.js", true},
		{"vendor/github.com/pkg/lib.go", true},
		{"__pycache__/module.pyc", true},
		{".coverage/index.html", true},
		{"logs/app.log", true},
		{".DS_Store", true},
		{".env", true},
		{"test/unit_test.go", true},
		{"docs/README.md", true},

		// Necessary files
		{"main.go", false},
		{"package.json", false},
		{"Dockerfile", false},
		{"src/app.js", false},
		{"config/production.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := optimizer.isUnnecessaryFile(tt.path)
			assert.Equal(t, tt.unnecessary, result)
		})
	}
}
