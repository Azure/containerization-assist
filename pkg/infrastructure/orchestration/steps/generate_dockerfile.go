package steps

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/containerization-assist/pkg/domain/errors"
)

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

	// Generate Dockerfile based on detected language and framework
	dockerfile := generateDockerfileForLanguage(analyzeResult.Language, analyzeResult.Framework, analyzeResult.Port, logger)

	// Determine base image
	baseImage := getBaseImageForLanguage(analyzeResult.Language, analyzeResult.Framework)

	return &DockerfileResult{
		Content:     dockerfile,
		Path:        "Dockerfile",
		BaseImage:   baseImage,
		ExposedPort: analyzeResult.Port,
	}, nil
}

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
		return generateGenericDockerfile(port, logger)
	}
}

// generateGoDockerfile creates optimized Dockerfile for Go applications
func generateGoDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
# Copy go.mod first, go.sum if it exists
COPY go.mod ./
# Copy go.sum only if it exists (using wildcard that doesn't fail if missing)
COPY go.su[m] ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
`)

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString("CMD [\"./main\"]\n")

	return dockerfile.String()
}

// generateJavaDockerfile creates optimized Dockerfile for Java applications
func generateJavaDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Simple detection: if it has JAR files anywhere, it's a standard Java app, otherwise servlet
	isServlet := true

	// Walk through all directories to find JAR files
	err := filepath.WalkDir(".", func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jar") && !strings.Contains(info.Name(), "-sources") && !strings.Contains(info.Name(), "-javadoc") {
			isServlet = false
			return filepath.SkipAll // Found a JAR, stop walking
		}
		return nil
	})

	// If walk failed, default to servlet
	if err != nil {
		isServlet = true
	}

	if isServlet {
		// Use Tomcat for servlet applications
		dockerfile.WriteString(`# Build stage
FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /app
COPY . .

# Build the application
RUN if [ -f "mvnw" ]; then \
      chmod +x mvnw && ./mvnw clean package -DskipTests; \
    elif [ -f "gradlew" ]; then \
      chmod +x gradlew && ./gradlew build -x test; \
    elif [ -f "pom.xml" ]; then \
      mvn clean package -DskipTests; \
    elif [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then \
      gradle build -x test; \
    else \
      echo "No build file found" && exit 1; \
    fi

# Runtime stage - Tomcat for servlet applications
FROM tomcat:9.0-jre17-temurin
# Remove default webapps
RUN rm -rf /usr/local/tomcat/webapps/*
# Copy WAR file to Tomcat webapps as ROOT for root context
COPY --from=builder /app/target/*.war /usr/local/tomcat/webapps/ROOT.war
`)

		if port > 0 {
			dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
		} else {
			dockerfile.WriteString("EXPOSE 8080\n") // Default Tomcat port
		}

		dockerfile.WriteString(`# Start Tomcat
CMD ["catalina.sh", "run"]
`)
	} else {
		// Standard Java application (executable JAR)
		dockerfile.WriteString(`# Build stage
FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /app
COPY . .

# Build the application
RUN if [ -f "mvnw" ]; then \
      chmod +x mvnw && ./mvnw clean package -DskipTests; \
    elif [ -f "gradlew" ]; then \
      chmod +x gradlew && ./gradlew build -x test; \
    elif [ -f "pom.xml" ]; then \
      mvn clean package -DskipTests; \
    elif [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then \
      gradle build -x test; \
    else \
      echo "No build file found" && exit 1; \
    fi

# Runtime stage
FROM tomcat:9.0-jre17-temurin
WORKDIR /app

# Copy built artifacts from builder stage
# Use a shell script to find and copy the built artifact
RUN --mount=from=builder,source=/app,target=/build \
    find /build -name '*.jar' -not -name '*-sources.jar' -not -name '*-javadoc.jar' | head -1 | xargs -I {} cp {} /app/app.jar

`)

		if port > 0 {
			dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
		} else {
			dockerfile.WriteString("EXPOSE 8080\n") // Default Java web app port
		}

		dockerfile.WriteString(`# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]
`)
	}

	return dockerfile.String()
}

// generateNodeDockerfile creates optimized Dockerfile for Node.js applications
func generateNodeDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
`)

	// Framework-specific optimizations
	if strings.Contains(framework, "next") {
		dockerfile.WriteString("RUN npm run build\n")
	}

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 3000\n") // Default Node.js port
	}

	dockerfile.WriteString(`CMD ["npm", "start"]
`)

	return dockerfile.String()
}

// generatePythonDockerfile creates optimized Dockerfile for Python applications
func generatePythonDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
`)

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 5000\n") // Default Flask/Python port
	}

	// Framework-specific commands
	if strings.Contains(framework, "django") {
		dockerfile.WriteString(`CMD ["python", "manage.py", "runserver", "0.0.0.0:8000"]
`)
	} else if strings.Contains(framework, "fastapi") {
		dockerfile.WriteString(`CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
`)
	} else {
		dockerfile.WriteString(`CMD ["python", "app.py"]
`)
	}

	return dockerfile.String()
}

// generateRustDockerfile creates optimized Dockerfile for Rust applications
func generateRustDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`# Build stage
FROM rust:1.70 AS builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs
RUN cargo build --release
COPY . .
RUN cargo build --release

# Runtime stage
FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/target/release/* ./
`)

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString(`CMD ["./main"]
`)

	return dockerfile.String()
}

// generatePHPDockerfile creates optimized Dockerfile for PHP applications
func generatePHPDockerfile(framework string, port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`FROM php:8.2-apache
WORKDIR /var/www/html
COPY . .
RUN chown -R www-data:www-data /var/www/html
`)

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 80\n")
	}

	dockerfile.WriteString(`CMD ["apache2-foreground"]
`)

	return dockerfile.String()
}

func generateGenericDockerfile(port int, logger *slog.Logger) string {
	var dockerfile strings.Builder

	dockerfile.WriteString(`FROM alpine:latest
WORKDIR /app
COPY . .
`)

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString(`CMD ["./start.sh"]
`)

	return dockerfile.String()
}

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
