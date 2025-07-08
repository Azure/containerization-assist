package analyze

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Azure/container-kit/pkg/core/analysis"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// ContextGenerator generates containerization context and assessments
type ContextGenerator struct {
	logger *slog.Logger
}

// NewContextGenerator creates a new context generator
func NewContextGenerator(logger *slog.Logger) *ContextGenerator {
	return &ContextGenerator{
		logger: logger.With("component", "context_generator"),
	}
}

// GenerateContainerizationAssessment generates a comprehensive containerization assessment
func (c *ContextGenerator) GenerateContainerizationAssessment(
	analysisResult *analysis.AnalysisResult,
	analysisContext *AnalysisContext,
) (*ContainerizationAssessment, error) {

	if analysisResult == nil || analysisContext == nil {
		return nil, errors.NewError().Messagef("analysis result and context are required").Build()
	}

	assessment := &ContainerizationAssessment{
		ReadinessScore:      c.calculateReadinessScore(analysisResult, analysisContext),
		StrengthAreas:       c.identifyStrengthAreas(analysisResult, analysisContext),
		ChallengeAreas:      c.identifyChallengeAreas(analysisResult, analysisContext),
		RecommendedApproach: c.determineRecommendedApproach(analysisResult, analysisContext),
		TechnologyStack:     c.assessTechnologyStack(analysisResult, analysisContext),
		RiskAnalysis:        c.analyzeContainerizationRisks(analysisResult, analysisContext),
		DeploymentOptions:   c.generateDeploymentOptions(analysisResult, analysisContext),
	}

	return assessment, nil
}

// calculateReadinessScore calculates containerization readiness (0-100)
func (c *ContextGenerator) calculateReadinessScore(analysis *analysis.AnalysisResult, ctx *AnalysisContext) int {
	score := 50 // Base score

	// Language support
	supportedLanguages := map[string]int{
		"Go":         10,
		"Python":     10,
		"JavaScript": 10,
		"Java":       8,
		"C#":         8,
		"Ruby":       8,
		"PHP":        7,
		"Rust":       9,
	}
	if bonus, ok := supportedLanguages[analysis.Language]; ok {
		score += bonus
	}

	// Dependencies present (indicates package manager)
	if len(analysis.Dependencies) > 0 {
		score += 10
	}

	// Entry point identified
	if len(ctx.EntryPointsFound) > 0 {
		score += 10
	}

	// Has tests
	if len(ctx.TestFilesFound) > 0 {
		score += 5
	}

	// Has CI/CD
	if ctx.HasCI {
		score += 5
	}

	// Already has Dockerfile
	if len(ctx.DockerFiles) > 0 {
		score += 15
	}

	// Has documentation
	if ctx.HasReadme {
		score += 5
	}

	// Penalize missing entry points
	if len(ctx.EntryPointsFound) == 0 && analysis.Language != "" {
		score -= 10
	}

	// Penalize very large repositories
	if ctx.RepositorySize > 100*1024*1024 { // 100MB
		score -= 5
	}

	// Ensure score is within bounds
	if score > 100 {
		score = 100
	} else if score < 0 {
		score = 0
	}

	return score
}

// identifyStrengthAreas identifies containerization strengths
func (c *ContextGenerator) identifyStrengthAreas(analysis *analysis.AnalysisResult, ctx *AnalysisContext) []string {
	strengths := []string{}

	if analysis.Language != "" {
		strengths = append(strengths, fmt.Sprintf("Clear %s application structure identified", analysis.Language))
	}

	if len(analysis.Dependencies) > 0 {
		strengths = append(strengths, "Clear dependency management structure")
	}

	if len(ctx.EntryPointsFound) > 0 {
		strengths = append(strengths, "Clear application entry points found")
	}

	if len(ctx.TestFilesFound) > 0 {
		strengths = append(strengths, "Test suite present for validation")
	}

	if ctx.HasCI {
		strengths = append(strengths, "CI/CD configuration detected")
	}

	if len(ctx.DockerFiles) > 0 {
		strengths = append(strengths, "Existing containerization artifacts found")
	}

	if len(ctx.ConfigFilesFound) > 0 {
		strengths = append(strengths, "Configuration management structure in place")
	}

	if analysis.Framework != "" {
		strengths = append(strengths, fmt.Sprintf("Well-known framework (%s) with established patterns", analysis.Framework))
	}

	return strengths
}

// identifyChallengeAreas identifies potential challenges
func (c *ContextGenerator) identifyChallengeAreas(analysis *analysis.AnalysisResult, ctx *AnalysisContext) []string {
	challenges := []string{}

	if len(ctx.EntryPointsFound) == 0 {
		challenges = append(challenges, "No clear entry point identified")
	}

	if len(ctx.DatabaseFiles) > 0 {
		challenges = append(challenges, "Database dependencies require external services")
	}

	if ctx.RepositorySize > 50*1024*1024 { // 50MB
		challenges = append(challenges, "Large repository size may lead to bigger images")
	}

	if len(ctx.ConfigFilesFound) > 5 {
		challenges = append(challenges, "Multiple configuration files need environment mapping")
	}

	if len(analysis.Dependencies) > 50 {
		challenges = append(challenges, "Large number of dependencies may increase build time")
	}

	if !ctx.HasCI && !ctx.HasReadme {
		challenges = append(challenges, "Limited documentation for build/run instructions")
	}

	return challenges
}

// determineRecommendedApproach determines the recommended containerization approach
func (c *ContextGenerator) determineRecommendedApproach(analysis *analysis.AnalysisResult, ctx *AnalysisContext) string {
	if len(ctx.DockerFiles) > 0 {
		return "Optimize existing Dockerfile and add multi-stage build if not present"
	}

	switch analysis.Language {
	case "Go":
		return "Multi-stage build with Alpine Linux for minimal image size"
	case "Python":
		return "Multi-stage build with slim Python image and virtual environment"
	case "JavaScript", "TypeScript":
		if analysis.Framework == "Next.js" || analysis.Framework == "React" {
			return "Multi-stage build with Node.js and nginx for static hosting"
		}
		return "Node.js Alpine image with production dependencies only"
	case "Java":
		return "Multi-stage build with Maven/Gradle and JRE slim image"
	case "C#":
		return "Multi-stage build with .NET SDK and runtime images"
	default:
		return "Standard containerization with appropriate base image"
	}
}

// assessTechnologyStack assesses the technology stack
func (c *ContextGenerator) assessTechnologyStack(analysis *analysis.AnalysisResult, ctx *AnalysisContext) TechnologyStackAssessment {
	assessment := TechnologyStackAssessment{
		Language:  analysis.Language,
		Framework: analysis.Framework,
	}

	// Base image options
	switch analysis.Language {
	case "Go":
		assessment.BaseImageOptions = []string{"golang:alpine", "scratch (for static binaries)", "distroless/static"}
		assessment.BuildStrategy = "Multi-stage build with Go modules"
	case "Python":
		assessment.BaseImageOptions = []string{"python:3-slim", "python:3-alpine", "python:3-slim-bullseye"}
		assessment.BuildStrategy = "Multi-stage build with pip or poetry"
	case "JavaScript", "TypeScript":
		assessment.BaseImageOptions = []string{"node:lts-alpine", "node:lts-slim", "nginx:alpine (for static sites)"}
		assessment.BuildStrategy = "Multi-stage build with npm/yarn/pnpm"
	case "Java":
		assessment.BaseImageOptions = []string{"openjdk:17-slim", "eclipse-temurin:17-jre", "amazoncorretto:17"}
		assessment.BuildStrategy = "Multi-stage build with Maven or Gradle"
	case "C#":
		assessment.BaseImageOptions = []string{"mcr.microsoft.com/dotnet/runtime", "mcr.microsoft.com/dotnet/aspnet"}
		assessment.BuildStrategy = "Multi-stage build with .NET SDK"
	default:
		assessment.BaseImageOptions = []string{"ubuntu:22.04", "alpine:latest", "debian:bullseye-slim"}
		assessment.BuildStrategy = "Standard build process"
	}

	// Security considerations
	assessment.SecurityConsiderations = []string{
		"Run as non-root user",
		"Use specific version tags instead of 'latest'",
		"Minimize attack surface with minimal base images",
		"Scan for vulnerabilities regularly",
	}

	if len(ctx.DatabaseFiles) > 0 {
		assessment.SecurityConsiderations = append(assessment.SecurityConsiderations,
			"Secure database credentials using secrets management")
	}

	return assessment
}

// analyzeContainerizationRisks analyzes potential risks
func (c *ContextGenerator) analyzeContainerizationRisks(analysis *analysis.AnalysisResult, ctx *AnalysisContext) []ContainerizationRisk {
	risks := []ContainerizationRisk{}

	// Large image size risk
	if ctx.RepositorySize > 100*1024*1024 {
		risks = append(risks, ContainerizationRisk{
			Area:       "Image Size",
			Risk:       "Large repository may result in bloated container images",
			Impact:     "high",
			Mitigation: "Use multi-stage builds and .dockerignore to exclude unnecessary files",
		})
	}

	// Missing entry point risk
	if len(ctx.EntryPointsFound) == 0 {
		risks = append(risks, ContainerizationRisk{
			Area:       "Application Startup",
			Risk:       "No clear entry point identified for container startup",
			Impact:     "high",
			Mitigation: "Identify and document the main application entry point",
		})
	}

	// Database dependency risk
	if len(ctx.DatabaseFiles) > 0 {
		risks = append(risks, ContainerizationRisk{
			Area:       "Data Persistence",
			Risk:       "Application requires database which needs separate management",
			Impact:     "medium",
			Mitigation: "Use external database service or StatefulSet for data persistence",
		})
	}

	// Configuration management risk
	if len(ctx.ConfigFilesFound) > 3 {
		risks = append(risks, ContainerizationRisk{
			Area:       "Configuration",
			Risk:       "Multiple configuration files may complicate deployment",
			Impact:     "medium",
			Mitigation: "Use environment variables or ConfigMaps for configuration",
		})
	}

	// Security risk for missing non-root user
	risks = append(risks, ContainerizationRisk{
		Area:       "Security",
		Risk:       "Running container as root user poses security risks",
		Impact:     "high",
		Mitigation: "Create and use non-root user in Dockerfile",
	})

	return risks
}

// generateDeploymentOptions generates deployment recommendations
func (c *ContextGenerator) generateDeploymentOptions(analysis *analysis.AnalysisResult, ctx *AnalysisContext) []DeploymentRecommendation {
	options := []DeploymentRecommendation{
		{
			Strategy: "Kubernetes Deployment",
			Pros: []string{
				"Scalability and load balancing",
				"Self-healing capabilities",
				"Rolling updates with zero downtime",
				"Rich ecosystem of tools",
			},
			Cons: []string{
				"Complexity for simple applications",
				"Requires Kubernetes knowledge",
				"Resource overhead",
			},
			Complexity: "moderate",
			UseCase:    "Production workloads requiring high availability",
		},
		{
			Strategy: "Docker Compose",
			Pros: []string{
				"Simple to understand and use",
				"Good for development environments",
				"Easy multi-container orchestration",
				"Minimal learning curve",
			},
			Cons: []string{
				"Not suitable for production at scale",
				"Limited to single host",
				"No built-in scaling",
			},
			Complexity: "simple",
			UseCase:    "Development and testing environments",
		},
	}

	// Add serverless option for suitable applications
	if c.isSuitableForServerless(analysis, ctx) {
		options = append(options, DeploymentRecommendation{
			Strategy: "Serverless (Cloud Run, Lambda, etc.)",
			Pros: []string{
				"No infrastructure management",
				"Automatic scaling",
				"Pay-per-use pricing",
				"Built-in high availability",
			},
			Cons: []string{
				"Cold start latency",
				"Vendor lock-in",
				"Limited execution time",
				"Stateless only",
			},
			Complexity: "simple",
			UseCase:    "Event-driven and API workloads",
		})
	}

	return options
}

// isSuitableForServerless checks if app is suitable for serverless
func (c *ContextGenerator) isSuitableForServerless(analysis *analysis.AnalysisResult, ctx *AnalysisContext) bool {
	// Check for serverless-friendly languages
	serverlessLanguages := []string{"Go", "Python", "JavaScript", "TypeScript", "Java", "C#"}
	languageSupported := false
	for _, lang := range serverlessLanguages {
		if analysis.Language == lang {
			languageSupported = true
			break
		}
	}

	// Check for serverless-friendly frameworks
	serverlessFrameworks := []string{"Express", "FastAPI", "Flask", "Gin", "Spring Boot"}
	frameworkSupported := false
	for _, fw := range serverlessFrameworks {
		if strings.Contains(analysis.Framework, fw) {
			frameworkSupported = true
			break
		}
	}

	// No database files is better for serverless
	noDatabaseFiles := len(ctx.DatabaseFiles) == 0

	return languageSupported && (frameworkSupported || noDatabaseFiles)
}
