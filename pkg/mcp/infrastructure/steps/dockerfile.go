package steps

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// DockerfileResult contains the generated Dockerfile and metadata
type DockerfileResult struct {
	Content     string            `json:"content"`
	Path        string            `json:"path"`
	BaseImage   string            `json:"base_image"`
	BuildArgs   map[string]string `json:"build_args,omitempty"`
	ExposedPort int               `json:"exposed_port,omitempty"`
}

// GenerateDockerfile creates an optimized Dockerfile based on analysis results
func GenerateDockerfile(analyzeResult *AnalyzeResult, logger *slog.Logger) (*DockerfileResult, error) {
	if analyzeResult == nil {
		return nil, errors.New(errors.CodeInvalidParameter, "dockerfile", "analyze result is required", nil)
	}

	logger.Info("Generating Dockerfile",
		"language", analyzeResult.Language,
		"framework", analyzeResult.Framework,
		"port", analyzeResult.Port)

	// Generate Dockerfile based on detected language and framework
	dockerfile := generateDockerfileForLanguage(analyzeResult.Language, analyzeResult.Framework, analyzeResult.Port, logger)

	// Determine base image
	baseImage := getBaseImageForLanguage(analyzeResult.Language, analyzeResult.Framework)

	logger.Info("Dockerfile generated successfully",
		"base_image", baseImage,
		"lines", len(strings.Split(dockerfile, "\n")),
		"port", analyzeResult.Port)

	return &DockerfileResult{
		Content:     dockerfile,
		Path:        "Dockerfile",
		BaseImage:   baseImage,
		ExposedPort: analyzeResult.Port,
	}, nil
}

// generateDockerfileForLanguage creates language-specific Dockerfiles
func generateDockerfileForLanguage(language, framework string, port int, logger *slog.Logger) string {
	switch language {
	case "go":
		return generateGoDockerfile(port, logger)
	case "java":
		return generateJavaDockerfile(framework, port, logger)
	case "javascript", "typescript":
		return generateNodeDockerfile(framework, port, logger)
	case "python":
		return generatePythonDockerfile(framework, port, logger)
	case "rust":
		return generateRustDockerfile(port, logger)
	case "php":
		return generatePHPDockerfile(framework, port, logger)
	default:
		logger.Warn("Unknown language, generating generic Dockerfile", "language", language)
		return generateGenericDockerfile(port, logger)
	}
}

// generateGoDockerfile creates optimized Dockerfile for Go applications
func generateGoDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("# Build stage\n")
	dockerfile.WriteString("FROM golang:1.21-alpine AS builder\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("# Copy go.mod first, go.sum if it exists\n")
	dockerfile.WriteString("COPY go.mod ./\n")
	dockerfile.WriteString("# Copy go.sum only if it exists (using wildcard that doesn't fail if missing)\n")
	dockerfile.WriteString("COPY go.su[m] ./\n")
	dockerfile.WriteString("RUN go mod download\n")
	dockerfile.WriteString("COPY . .\n")
	dockerfile.WriteString("RUN CGO_ENABLED=0 GOOS=linux go build -o main .\n\n")

	dockerfile.WriteString("# Runtime stage\n")
	dockerfile.WriteString("FROM alpine:latest\n")
	dockerfile.WriteString("RUN apk --no-cache add ca-certificates\n")
	dockerfile.WriteString("WORKDIR /root/\n")
	dockerfile.WriteString("COPY --from=builder /app/main .\n")

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString("CMD [\"./main\"]\n")

	logger.Debug("Generated Go Dockerfile with multi-stage build")
	return dockerfile.String()
}

// generateJavaDockerfile creates optimized Dockerfile for Java applications
func generateJavaDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Check if this is a servlet application (WAR file)
	isServlet := strings.Contains(strings.ToLower(framework), "servlet") || 
		strings.Contains(strings.ToLower(framework), "jsp") ||
		strings.Contains(strings.ToLower(framework), "war")

	if isServlet {
		// Use Tomcat for servlet applications
		dockerfile.WriteString("# Build stage\n")
		dockerfile.WriteString("FROM maven:3.9-eclipse-temurin-17 AS builder\n")
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY . .\n")

		// Build the application
		dockerfile.WriteString("# Build the application\n")
		dockerfile.WriteString("RUN if [ -f \"mvnw\" ]; then \\\n")
		dockerfile.WriteString("      chmod +x mvnw && ./mvnw clean package -DskipTests; \\\n")
		dockerfile.WriteString("    elif [ -f \"gradlew\" ]; then \\\n")
		dockerfile.WriteString("      chmod +x gradlew && ./gradlew build -x test; \\\n")
		dockerfile.WriteString("    elif [ -f \"pom.xml\" ]; then \\\n")
		dockerfile.WriteString("      mvn clean package -DskipTests; \\\n")
		dockerfile.WriteString("    elif [ -f \"build.gradle\" ] || [ -f \"build.gradle.kts\" ]; then \\\n")
		dockerfile.WriteString("      gradle build -x test; \\\n")
		dockerfile.WriteString("    else \\\n")
		dockerfile.WriteString("      echo \"No build file found\" && exit 1; \\\n")
		dockerfile.WriteString("    fi\n\n")

		// Runtime stage with Tomcat
		dockerfile.WriteString("# Runtime stage - Tomcat for servlet applications\n")
		dockerfile.WriteString("FROM tomcat:10-jre17-temurin-jammy\n")
		dockerfile.WriteString("# Remove default webapps\n")
		dockerfile.WriteString("RUN rm -rf /usr/local/tomcat/webapps/*\n")
		dockerfile.WriteString("# Copy WAR file to Tomcat webapps as ROOT for root context\n")
		dockerfile.WriteString("COPY --from=builder /app/target/*.war /usr/local/tomcat/webapps/ROOT.war\n")

		if port > 0 {
			dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
		} else {
			dockerfile.WriteString("EXPOSE 8080\n") // Default Tomcat port
		}

		dockerfile.WriteString("# Start Tomcat\n")
		dockerfile.WriteString("CMD [\"catalina.sh\", \"run\"]\n")
	} else {
		// Standard Java application (executable JAR)
		dockerfile.WriteString("# Build stage\n")
		dockerfile.WriteString("FROM maven:3.9-eclipse-temurin-17 AS builder\n")
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY . .\n")

		// Handle different build systems
		dockerfile.WriteString("# Build the application\n")
		dockerfile.WriteString("RUN if [ -f \"mvnw\" ]; then \\\n")
		dockerfile.WriteString("      chmod +x mvnw && ./mvnw clean package -DskipTests; \\\n")
		dockerfile.WriteString("    elif [ -f \"gradlew\" ]; then \\\n")
		dockerfile.WriteString("      chmod +x gradlew && ./gradlew build -x test; \\\n")
		dockerfile.WriteString("    elif [ -f \"pom.xml\" ]; then \\\n")
		dockerfile.WriteString("      mvn clean package -DskipTests; \\\n")
		dockerfile.WriteString("    elif [ -f \"build.gradle\" ] || [ -f \"build.gradle.kts\" ]; then \\\n")
		dockerfile.WriteString("      gradle build -x test; \\\n")
		dockerfile.WriteString("    else \\\n")
		dockerfile.WriteString("      echo \"No build file found\" && exit 1; \\\n")
		dockerfile.WriteString("    fi\n\n")

		// Runtime stage
		dockerfile.WriteString("# Runtime stage\n")
		dockerfile.WriteString("FROM eclipse-temurin:17-jre-alpine\n")
		dockerfile.WriteString("WORKDIR /app\n")

		// Copy built artifacts from builder stage - handle both Maven and Gradle outputs
		dockerfile.WriteString("# Copy built artifacts from builder stage\n")
		dockerfile.WriteString("# Use a shell script to find and copy the built artifact\n")
		dockerfile.WriteString("RUN --mount=from=builder,source=/app,target=/build \\\n")
		dockerfile.WriteString("    find /build -name '*.jar' -not -name '*-sources.jar' -not -name '*-javadoc.jar' | head -1 | xargs -I {} cp {} /app/app.jar\n\n")

		if port > 0 {
			dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
		} else {
			dockerfile.WriteString("EXPOSE 8080\n") // Default Java web app port
		}

		dockerfile.WriteString("# Run the application\n")
		dockerfile.WriteString("ENTRYPOINT [\"java\", \"-jar\", \"app.jar\"]\n")
	}

	logger.Debug("Generated Java Dockerfile", "framework", framework, "isServlet", isServlet)
	return dockerfile.String()
}

// generateNodeDockerfile creates optimized Dockerfile for Node.js applications
func generateNodeDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("FROM node:18-alpine\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("COPY package*.json ./\n")
	dockerfile.WriteString("RUN npm ci --only=production\n")
	dockerfile.WriteString("COPY . .\n")

	// Framework-specific optimizations
	if strings.Contains(framework, "next") {
		dockerfile.WriteString("RUN npm run build\n")
	}

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 3000\n") // Default Node.js port
	}

	dockerfile.WriteString("CMD [\"npm\", \"start\"]\n")

	logger.Debug("Generated Node.js Dockerfile", "framework", framework)
	return dockerfile.String()
}

// generatePythonDockerfile creates optimized Dockerfile for Python applications
func generatePythonDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("FROM python:3.11-slim\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("COPY requirements.txt .\n")
	dockerfile.WriteString("RUN pip install --no-cache-dir -r requirements.txt\n")
	dockerfile.WriteString("COPY . .\n")

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 5000\n") // Default Flask/Python port
	}

	// Framework-specific commands
	if strings.Contains(framework, "django") {
		dockerfile.WriteString("CMD [\"python\", \"manage.py\", \"runserver\", \"0.0.0.0:8000\"]\n")
	} else if strings.Contains(framework, "fastapi") {
		dockerfile.WriteString("CMD [\"uvicorn\", \"main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]\n")
	} else {
		dockerfile.WriteString("CMD [\"python\", \"app.py\"]\n")
	}

	logger.Debug("Generated Python Dockerfile", "framework", framework)
	return dockerfile.String()
}

// generateRustDockerfile creates optimized Dockerfile for Rust applications
func generateRustDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("# Build stage\n")
	dockerfile.WriteString("FROM rust:1.70 AS builder\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("COPY Cargo.toml Cargo.lock ./\n")
	dockerfile.WriteString("RUN mkdir src && echo 'fn main() {}' > src/main.rs\n")
	dockerfile.WriteString("RUN cargo build --release\n")
	dockerfile.WriteString("COPY . .\n")
	dockerfile.WriteString("RUN cargo build --release\n\n")

	dockerfile.WriteString("# Runtime stage\n")
	dockerfile.WriteString("FROM debian:bookworm-slim\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("COPY --from=builder /app/target/release/* ./\n")

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString("CMD [\"./main\"]\n")

	logger.Debug("Generated Rust Dockerfile with multi-stage build")
	return dockerfile.String()
}

// generatePHPDockerfile creates optimized Dockerfile for PHP applications
func generatePHPDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("FROM php:8.2-apache\n")
	dockerfile.WriteString("WORKDIR /var/www/html\n")
	dockerfile.WriteString("COPY . .\n")
	dockerfile.WriteString("RUN chown -R www-data:www-data /var/www/html\n")

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 80\n")
	}

	dockerfile.WriteString("CMD [\"apache2-foreground\"]\n")

	logger.Debug("Generated PHP Dockerfile", "framework", framework)
	return dockerfile.String()
}

// generateGenericDockerfile creates a basic Dockerfile for unknown languages
func generateGenericDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString("FROM alpine:latest\n")
	dockerfile.WriteString("WORKDIR /app\n")
	dockerfile.WriteString("COPY . .\n")

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString("CMD [\"./start.sh\"]\n")

	logger.Debug("Generated generic Dockerfile")
	return dockerfile.String()
}

// getBaseImageForLanguage returns the base image used for a language
func getBaseImageForLanguage(language, framework string) string {
	switch language {
	case "go":
		return "golang:1.21-alpine"
	case "java":
		return "openjdk:17-jdk-slim"
	case "javascript", "typescript":
		return "node:18-alpine"
	case "python":
		return "python:3.11-slim"
	case "rust":
		return "rust:1.70"
	case "php":
		return "php:8.2-apache"
	default:
		return "alpine:latest"
	}
}

// WriteDockerfile writes the Dockerfile content to the specified path
func WriteDockerfile(repoPath, content string, logger *slog.Logger) error {
	dockerfilePath := filepath.Join(repoPath, "Dockerfile")

	logger.Info("Writing Dockerfile", "path", dockerfilePath)

	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		return errors.New(errors.CodeIoError, "dockerfile", "failed to write Dockerfile", err)
	}

	logger.Info("Dockerfile written successfully", "path", dockerfilePath, "size", len(content))
	return nil
}
