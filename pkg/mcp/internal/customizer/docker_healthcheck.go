package docker

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// HealthCheckGenerator generates health checks for different languages/frameworks
type HealthCheckGenerator struct {
	logger zerolog.Logger
}

// NewHealthCheckGenerator creates a new health check generator
func NewHealthCheckGenerator(logger zerolog.Logger) *HealthCheckGenerator {
	return &HealthCheckGenerator{
		logger: logger.With().Str("component", "healthcheck_generator").Logger(),
	}
}

// Generate generates a health check based on language and framework
func (g *HealthCheckGenerator) Generate(language, framework string) string {
	switch strings.ToLower(language) {
	case "go":
		return g.generateGoHealthCheck()
	case "python":
		return g.generatePythonHealthCheck(framework)
	case "javascript", "typescript":
		return g.generateNodeHealthCheck(framework)
	case "java":
		return g.generateJavaHealthCheck(framework)
	case "c#", "csharp":
		return g.generateDotNetHealthCheck(framework)
	default:
		// Generic health check
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost/ || exit 1"
	}
}

// generateGoHealthCheck generates a health check for Go applications
func (g *HealthCheckGenerator) generateGoHealthCheck() string {
	return `# Health check for Go application
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1`
}

// generatePythonHealthCheck generates a health check for Python applications
func (g *HealthCheckGenerator) generatePythonHealthCheck(framework string) string {
	switch strings.ToLower(framework) {
	case "django":
		return `# Health check for Django application
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health/').read()" || exit 1`
	case "flask", "fastapi":
		return `# Health check for Flask/FastAPI application
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:5000/health').read()" || exit 1`
	default:
		return `# Health check for Python application
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/').read()" || exit 1`
	}
}

// generateNodeHealthCheck generates a health check for Node.js applications
func (g *HealthCheckGenerator) generateNodeHealthCheck(framework string) string {
	switch strings.ToLower(framework) {
	case "express":
		return `# Health check for Express application
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3000/health', (res) => { process.exit(res.statusCode === 200 ? 0 : 1); })"`
	case "next.js", "nextjs":
		return `# Health check for Next.js application
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3000/api/health', (res) => { process.exit(res.statusCode === 200 ? 0 : 1); })"`
	default:
		return `# Health check for Node.js application
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3000/', (res) => { process.exit(res.statusCode === 200 ? 0 : 1); })"`
	}
}

// generateJavaHealthCheck generates a health check for Java applications
func (g *HealthCheckGenerator) generateJavaHealthCheck(framework string) string {
	if strings.Contains(strings.ToLower(framework), "spring") {
		return `# Health check for Spring Boot application
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 \
  CMD curl -f http://localhost:8080/actuator/health || exit 1`
	}
	return `# Health check for Java application
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1`
}

// generateDotNetHealthCheck generates a health check for .NET applications
func (g *HealthCheckGenerator) generateDotNetHealthCheck(framework string) string {
	return `# Health check for .NET application
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:5000/health || exit 1`
}

// GenerateWithPort generates a health check with a specific port
func (g *HealthCheckGenerator) GenerateWithPort(language, framework string, port int) string {
	baseCheck := g.Generate(language, framework)
	// Replace default ports with the specified port
	portStr := fmt.Sprintf(":%d", port)
	replacements := map[string]string{
		":8080": portStr,
		":8000": portStr,
		":5000": portStr,
		":3000": portStr,
	}

	for oldPort, newPort := range replacements {
		if strings.Contains(baseCheck, oldPort) {
			baseCheck = strings.ReplaceAll(baseCheck, oldPort, newPort)
			break
		}
	}

	return baseCheck
}
