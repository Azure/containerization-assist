package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
)

// BuildStrategy represents a build strategy for implementation
type BuildStrategy struct {
	Name                    string
	Type                    string
	Confidence              string
	Description             string
	Language                string
	Framework               string
	Features                []string
	Optimizations           []string
	SecurityRecommendations []string
}

// ContextOptimization represents build context optimization
type ContextOptimization struct {
	OriginalSize    int64
	OptimizedSize   int64
	ExcludedFiles   []string
	Recommendations []string
}

// Monitor represents build monitoring
type Monitor struct {
	StartTime    time.Time
	BuildContext interface{}
	Stages       []interface{}
	Metrics      interface{}
}

// Build strategy implementations

// detectBuildStrategy analyzes the workspace and determines the best build strategy
func (cmd *ConsolidatedBuildCommand) detectBuildStrategy(ctx context.Context, workspaceDir string) (*BuildStrategy, error) {
	// Check for existing Dockerfile
	dockerfilePath := filepath.Join(workspaceDir, "Dockerfile")
	if fileExists(dockerfilePath) {
		return cmd.analyzeDockerfile(dockerfilePath)
	}

	// Check for language-specific build files
	return cmd.detectLanguageStrategy(workspaceDir)
}

// analyzeDockerfile analyzes an existing Dockerfile to determine build strategy
func (cmd *ConsolidatedBuildCommand) analyzeDockerfile(dockerfilePath string) (*BuildStrategy, error) {
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	dockerfile := string(content)

	// Determine strategy based on Dockerfile content
	strategy := &BuildStrategy{
		Name:        "dockerfile",
		Type:        "dockerfile",
		Confidence:  "high",
		Description: "Use existing Dockerfile",
		Features: []string{
			"multi-stage",
			"caching",
			"security",
		},
	}

	// Analyze for multi-stage builds
	if cmd.isMultiStageDockerfile(dockerfile) {
		strategy.Features = append(strategy.Features, "multi-stage")
		strategy.Optimizations = append(strategy.Optimizations, "Multi-stage build detected")
	}

	// Analyze for BuildKit features
	if cmd.usesBuildKitFeatures(dockerfile) {
		strategy.Features = append(strategy.Features, "buildkit")
		strategy.Optimizations = append(strategy.Optimizations, "BuildKit features detected")
	}

	// Analyze for security best practices
	securityIssues := cmd.analyzeDockerfileSecurity(dockerfile)
	if len(securityIssues) > 0 {
		strategy.SecurityRecommendations = securityIssues
	}

	return strategy, nil
}

// detectLanguageStrategy detects build strategy based on language and framework
func (cmd *ConsolidatedBuildCommand) detectLanguageStrategy(workspaceDir string) (*BuildStrategy, error) {
	// Language detection logic
	language := cmd.detectPrimaryLanguage(workspaceDir)
	framework := cmd.detectFrameworkForLanguage(workspaceDir, language)

	strategy := &BuildStrategy{
		Name:        fmt.Sprintf("%s-%s", language, framework),
		Type:        "generated",
		Confidence:  "medium",
		Description: fmt.Sprintf("Generated strategy for %s with %s", language, framework),
		Language:    language,
		Framework:   framework,
	}

	// Language-specific optimizations
	switch language {
	case "go":
		strategy.Features = append(strategy.Features, "multi-stage", "caching")
		strategy.Optimizations = append(strategy.Optimizations,
			"Use multi-stage build with Go modules caching",
			"Minimize final image size with distroless base",
		)
	case "node":
		strategy.Features = append(strategy.Features, "multi-stage", "caching")
		strategy.Optimizations = append(strategy.Optimizations,
			"Use multi-stage build with npm/yarn caching",
			"Optimize node_modules with production dependencies only",
		)
	case "python":
		strategy.Features = append(strategy.Features, "multi-stage", "caching")
		strategy.Optimizations = append(strategy.Optimizations,
			"Use multi-stage build with pip caching",
			"Minimize image size with appropriate Python base image",
		)
	case "java":
		strategy.Features = append(strategy.Features, "multi-stage", "caching")
		strategy.Optimizations = append(strategy.Optimizations,
			"Use multi-stage build with Maven/Gradle caching",
			"Optimize JVM settings for container environment",
		)
	}

	return strategy, nil
}

// detectPrimaryLanguage detects the primary programming language in the workspace
func (cmd *ConsolidatedBuildCommand) detectPrimaryLanguage(workspaceDir string) string {
	languageFiles := map[string][]string{
		"go":     {"go.mod", "go.sum", "main.go"},
		"node":   {"package.json", "package-lock.json", "yarn.lock"},
		"python": {"requirements.txt", "setup.py", "pyproject.toml", "Pipfile"},
		"java":   {"pom.xml", "build.gradle", "gradlew"},
		"dotnet": {"*.csproj", "*.sln", "project.json"},
		"ruby":   {"Gemfile", "Gemfile.lock"},
		"php":    {"composer.json", "composer.lock"},
		"rust":   {"Cargo.toml", "Cargo.lock"},
		"cpp":    {"CMakeLists.txt", "Makefile", "*.cpp", "*.hpp"},
	}

	for language, files := range languageFiles {
		for _, file := range files {
			if cmd.findFileInWorkspace(workspaceDir, file) {
				return language
			}
		}
	}

	return "unknown"
}

// detectFrameworkForLanguage detects the framework for a specific language
func (cmd *ConsolidatedBuildCommand) detectFrameworkForLanguage(workspaceDir, language string) string {
	switch language {
	case "go":
		return cmd.detectGoFramework(workspaceDir)
	case "node":
		return cmd.detectNodeFramework(workspaceDir)
	case "python":
		return cmd.detectPythonFramework(workspaceDir)
	case "java":
		return cmd.detectJavaFramework(workspaceDir)
	default:
		return "unknown"
	}
}

// detectGoFramework detects Go framework
func (cmd *ConsolidatedBuildCommand) detectGoFramework(workspaceDir string) string {
	// Check go.mod for framework dependencies
	goModPath := filepath.Join(workspaceDir, "go.mod")
	if !fileExists(goModPath) {
		return "unknown"
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "unknown"
	}

	goMod := string(content)

	frameworks := map[string]string{
		"github.com/gin-gonic/gin": "gin",
		"github.com/gorilla/mux":   "gorilla",
		"github.com/labstack/echo": "echo",
		"github.com/gofiber/fiber": "fiber",
		"github.com/beego/beego":   "beego",
		"github.com/revel/revel":   "revel",
		"go.uber.org/fx":           "fx",
		"github.com/go-chi/chi":    "chi",
	}

	for dep, framework := range frameworks {
		if strings.Contains(goMod, dep) {
			return framework
		}
	}

	return "standard"
}

// detectNodeFramework detects Node.js framework
func (cmd *ConsolidatedBuildCommand) detectNodeFramework(workspaceDir string) string {
	packageJsonPath := filepath.Join(workspaceDir, "package.json")
	if !fileExists(packageJsonPath) {
		return "unknown"
	}

	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return "unknown"
	}

	packageJson := string(content)

	frameworks := map[string]string{
		"\"express\"": "express",
		"\"next\"":    "nextjs",
		"\"react\"":   "react",
		"\"vue\"":     "vue",
		"\"angular\"": "angular",
		"\"nestjs\"":  "nestjs",
		"\"koa\"":     "koa",
		"\"fastify\"": "fastify",
		"\"nuxt\"":    "nuxt",
		"\"gatsby\"":  "gatsby",
	}

	for dep, framework := range frameworks {
		if strings.Contains(packageJson, dep) {
			return framework
		}
	}

	return "node"
}

// detectPythonFramework detects Python framework
func (cmd *ConsolidatedBuildCommand) detectPythonFramework(workspaceDir string) string {
	// Check requirements.txt
	requirementsPath := filepath.Join(workspaceDir, "requirements.txt")
	if fileExists(requirementsPath) {
		content, err := os.ReadFile(requirementsPath)
		if err == nil {
			requirements := string(content)

			frameworks := map[string]string{
				"Django":    "django",
				"Flask":     "flask",
				"FastAPI":   "fastapi",
				"Tornado":   "tornado",
				"Pyramid":   "pyramid",
				"CherryPy":  "cherrypy",
				"Bottle":    "bottle",
				"Sanic":     "sanic",
				"Quart":     "quart",
				"Starlette": "starlette",
			}

			for dep, framework := range frameworks {
				if strings.Contains(requirements, dep) {
					return framework
				}
			}
		}
	}

	// Check for framework-specific files
	if fileExists(filepath.Join(workspaceDir, "manage.py")) {
		return "django"
	}
	if fileExists(filepath.Join(workspaceDir, "app.py")) {
		return "flask"
	}

	return "python"
}

// detectJavaFramework detects Java framework
func (cmd *ConsolidatedBuildCommand) detectJavaFramework(workspaceDir string) string {
	// Check pom.xml for Maven dependencies
	pomPath := filepath.Join(workspaceDir, "pom.xml")
	if fileExists(pomPath) {
		content, err := os.ReadFile(pomPath)
		if err == nil {
			pom := string(content)

			frameworks := map[string]string{
				"spring-boot":      "spring-boot",
				"spring-framework": "spring",
				"quarkus":          "quarkus",
				"micronaut":        "micronaut",
				"vertx":            "vertx",
				"dropwizard":       "dropwizard",
				"spark-core":       "spark",
				"jersey":           "jersey",
			}

			for dep, framework := range frameworks {
				if strings.Contains(pom, dep) {
					return framework
				}
			}
		}
	}

	// Check build.gradle for Gradle dependencies
	gradlePath := filepath.Join(workspaceDir, "build.gradle")
	if fileExists(gradlePath) {
		content, err := os.ReadFile(gradlePath)
		if err == nil {
			gradle := string(content)

			frameworks := map[string]string{
				"spring-boot":      "spring-boot",
				"spring-framework": "spring",
				"quarkus":          "quarkus",
				"micronaut":        "micronaut",
				"vertx":            "vertx",
			}

			for dep, framework := range frameworks {
				if strings.Contains(gradle, dep) {
					return framework
				}
			}
		}
	}

	return "java"
}

// Docker build optimization methods

// isMultiStageDockerfile checks if the Dockerfile uses multi-stage builds
func (cmd *ConsolidatedBuildCommand) isMultiStageDockerfile(dockerfile string) bool {
	// Look for multiple FROM statements or FROM ... AS ... patterns
	fromPattern := regexp.MustCompile(`(?m)^FROM\s+.*\s+AS\s+\w+`)
	return fromPattern.MatchString(dockerfile)
}

// usesBuildKitFeatures checks if the Dockerfile uses BuildKit features
func (cmd *ConsolidatedBuildCommand) usesBuildKitFeatures(dockerfile string) bool {
	buildKitFeatures := []string{
		"--mount=type=cache",
		"--mount=type=secret",
		"--mount=type=ssh",
		"--mount=type=bind",
		"syntax=",
		"RUN --mount",
	}

	for _, feature := range buildKitFeatures {
		if strings.Contains(dockerfile, feature) {
			return true
		}
	}

	return false
}

// analyzeDockerfileSecurity analyzes Dockerfile for security issues
func (cmd *ConsolidatedBuildCommand) analyzeDockerfileSecurity(dockerfile string) []string {
	var issues []string

	// Check for root user usage
	if !strings.Contains(dockerfile, "USER ") {
		issues = append(issues, "Consider adding a non-root user with USER instruction")
	}

	// Check for latest tag usage
	if strings.Contains(dockerfile, ":latest") {
		issues = append(issues, "Avoid using 'latest' tag, pin specific versions")
	}

	// Check for package manager cache cleanup
	if strings.Contains(dockerfile, "apt-get install") && !strings.Contains(dockerfile, "rm -rf /var/lib/apt/lists/*") {
		issues = append(issues, "Clean up apt cache to reduce image size")
	}

	// Check for exposed ports
	if strings.Contains(dockerfile, "EXPOSE") {
		issues = append(issues, "Review exposed ports for security implications")
	}

	// Check for secrets in environment variables
	envPattern := regexp.MustCompile(`(?m)^ENV\s+.*(?:PASSWORD|SECRET|KEY|TOKEN)`)
	if envPattern.MatchString(dockerfile) {
		issues = append(issues, "Avoid hardcoding secrets in environment variables")
	}

	// Check for COPY/ADD without .dockerignore
	if strings.Contains(dockerfile, "COPY .") || strings.Contains(dockerfile, "ADD .") {
		issues = append(issues, "Use .dockerignore to exclude unnecessary files")
	}

	return issues
}

// Build optimization methods

// optimizeBuildContext analyzes and optimizes the build context
func (cmd *ConsolidatedBuildCommand) optimizeBuildContext(ctx context.Context, workspaceDir string) (*ContextOptimization, error) {
	optimization := &ContextOptimization{
		OriginalSize:    0,
		OptimizedSize:   0,
		ExcludedFiles:   []string{},
		Recommendations: []string{},
	}

	// Calculate original context size
	originalSize, err := cmd.calculateDirectorySize(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory size: %w", err)
	}
	optimization.OriginalSize = originalSize

	// Check for .dockerignore file
	dockerignorePath := filepath.Join(workspaceDir, ".dockerignore")
	if !fileExists(dockerignorePath) {
		optimization.Recommendations = append(optimization.Recommendations,
			"Create .dockerignore file to exclude unnecessary files")
	}

	// Analyze common files that should be excluded
	commonExclusions := []string{
		".git",
		"node_modules",
		"*.log",
		"*.tmp",
		".env",
		".env.local",
		".DS_Store",
		"Thumbs.db",
		"*.swp",
		"*.swo",
		".vscode",
		".idea",
		"target",
		"build",
		"dist",
		"__pycache__",
		"*.pyc",
		".pytest_cache",
		".coverage",
		".nyc_output",
		"coverage",
		"test-results",
		"*.test",
		"*.out",
	}

	for _, pattern := range commonExclusions {
		if cmd.findFileInWorkspace(workspaceDir, pattern) {
			optimization.ExcludedFiles = append(optimization.ExcludedFiles, pattern)
		}
	}

	// Estimate optimized size (rough calculation)
	optimization.OptimizedSize = optimization.OriginalSize - int64(len(optimization.ExcludedFiles)*1024*1024) // Rough estimate
	if optimization.OptimizedSize < 0 {
		optimization.OptimizedSize = optimization.OriginalSize / 2 // Conservative estimate
	}

	return optimization, nil
}

// generateDockerfile generates an optimized Dockerfile for the detected language and framework
func (cmd *ConsolidatedBuildCommand) generateDockerfile(ctx context.Context, workspaceDir, language, framework string) (string, error) {
	switch language {
	case "go":
		return cmd.generateGoDockerfile(framework), nil
	case "node":
		return cmd.generateNodeDockerfile(framework), nil
	case "python":
		return cmd.generatePythonDockerfile(framework), nil
	case "java":
		return cmd.generateJavaDockerfile(framework), nil
	default:
		return cmd.generateGenericDockerfile(language), nil
	}
}

// generateGoDockerfile generates an optimized Dockerfile for Go applications
func (cmd *ConsolidatedBuildCommand) generateGoDockerfile(framework string) string {
	return `# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Change ownership and switch to non-root user
RUN chown appuser:appgroup main
USER appuser

# Expose port
EXPOSE 8080

# Command to run
CMD ["./main"]`
}

// generateNodeDockerfile generates an optimized Dockerfile for Node.js applications
func (cmd *ConsolidatedBuildCommand) generateNodeDockerfile(framework string) string {
	return `# syntax=docker/dockerfile:1

# Build stage
FROM node:18-alpine AS builder

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci --only=production

# Copy source code
COPY . .

# Build the application (if build script exists)
RUN npm run build --if-present

# Final stage
FROM node:18-alpine

# Install dumb-init for proper signal handling
RUN apk add --no-cache dumb-init

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \
    adduser -u 1001 -S nodejs -G nodejs

# Set working directory
WORKDIR /app

# Copy node_modules from builder stage
COPY --from=builder --chown=nodejs:nodejs /app/node_modules ./node_modules

# Copy application files
COPY --from=builder --chown=nodejs:nodejs /app .

# Switch to non-root user
USER nodejs

# Expose port
EXPOSE 3000

# Use dumb-init to handle signals properly
ENTRYPOINT ["dumb-init", "--"]

# Command to run
CMD ["node", "index.js"]`
}

// generatePythonDockerfile generates an optimized Dockerfile for Python applications
func (cmd *ConsolidatedBuildCommand) generatePythonDockerfile(framework string) string {
	return `# syntax=docker/dockerfile:1

# Build stage
FROM python:3.11-slim AS builder

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements file
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Final stage
FROM python:3.11-slim

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Create non-root user
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -m appuser

# Set working directory
WORKDIR /app

# Copy Python packages from builder stage
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy application files
COPY --chown=appuser:appgroup . .

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8000

# Command to run
CMD ["python", "app.py"]`
}

// generateJavaDockerfile generates an optimized Dockerfile for Java applications
func (cmd *ConsolidatedBuildCommand) generateJavaDockerfile(framework string) string {
	return `# syntax=docker/dockerfile:1

# Build stage
FROM maven:3.8-openjdk-17 AS builder

# Set working directory
WORKDIR /app

# Copy pom.xml
COPY pom.xml .

# Download dependencies
RUN mvn dependency:go-offline -B

# Copy source code
COPY src ./src

# Build the application
RUN mvn clean package -DskipTests

# Final stage
FROM openjdk:17-jre-slim

# Create non-root user
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -m appuser

# Set working directory
WORKDIR /app

# Copy JAR file from builder stage
COPY --from=builder --chown=appuser:appgroup /app/target/*.jar app.jar

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Command to run
CMD ["java", "-jar", "app.jar"]`
}

// generateGenericDockerfile generates a generic Dockerfile
func (cmd *ConsolidatedBuildCommand) generateGenericDockerfile(language string) string {
	return `# syntax=docker/dockerfile:1

FROM alpine:latest

# Install common packages
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy application files
COPY . .

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Command to run (customize based on your application)
CMD ["./app"]`
}

// Utility methods

// findFileInWorkspace searches for a file or pattern in the workspace
func (cmd *ConsolidatedBuildCommand) findFileInWorkspace(workspaceDir, pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(workspaceDir, pattern))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// calculateDirectorySize calculates the total size of a directory
func (cmd *ConsolidatedBuildCommand) calculateDirectorySize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// Note: fileExists is defined in common.go

// Performance and monitoring methods

// monitorBuildProgress monitors build progress and performance
func (cmd *ConsolidatedBuildCommand) monitorBuildProgress(ctx context.Context, buildCtx interface{}) *Monitor {
	monitor := &Monitor{
		StartTime:    time.Now(),
		BuildContext: buildCtx,
		Stages:       []interface{}{},
		Metrics:      interface{}(nil),
	}

	// Start monitoring goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Update metrics
				// monitor.Metrics.ElapsedTime = time.Since(monitor.StartTime)
				// Add more metrics collection here
			}
		}
	}()

	return monitor
}

// optimizeBuildPerformance provides build performance optimization recommendations
func (cmd *ConsolidatedBuildCommand) optimizeBuildPerformance(ctx context.Context, result *build.BuildResult) []string {
	var recommendations []string

	// Check build duration
	if result.Duration > 5*time.Minute {
		recommendations = append(recommendations, "Consider using multi-stage builds to improve build speed")
	}

	// Check cache utilization
	if result.Metadata.CacheHits < result.Metadata.CacheMisses {
		recommendations = append(recommendations, "Optimize Dockerfile layer ordering for better caching")
	}

	// Check image size
	if result.Size > 500*1024*1024 { // 500MB
		recommendations = append(recommendations, "Consider using smaller base images or multi-stage builds to reduce image size")
	}

	// Check build context size
	if result.Metadata.ResourceUsage.DiskIO > 100*1024*1024 { // 100MB
		recommendations = append(recommendations, "Use .dockerignore to reduce build context size")
	}

	return recommendations
}

// validateBuildEnvironment validates the build environment
func (cmd *ConsolidatedBuildCommand) validateBuildEnvironment(ctx context.Context, workspaceDir string) error {
	// Check if Docker is available
	if cmd.dockerClient == nil {
		return fmt.Errorf("Docker client not available")
	}

	// Check workspace directory
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return fmt.Errorf("workspace directory does not exist: %s", workspaceDir)
	}

	// Check disk space
	// This is a simplified check - you might want to implement more sophisticated disk space checking

	return nil
}
