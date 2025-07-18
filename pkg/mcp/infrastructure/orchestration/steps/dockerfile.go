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
	Content          string            `json:"content"`
	Path             string            `json:"path"`
	BaseImage        string            `json:"base_image"`
	LanguageVersion  string            `json:"language_version,omitempty"`
	FrameworkVersion string            `json:"framework_version,omitempty"`
	BuildArgs        map[string]string `json:"build_args,omitempty"`
	ExposedPort      int               `json:"exposed_port,omitempty"`
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

	// Extract version information from analysis if available
	var languageVersion, frameworkVersion string
	if analysis, ok := analyzeResult.Analysis["language_version"].(string); ok {
		languageVersion = analysis
	}
	if analysis, ok := analyzeResult.Analysis["framework_version"].(string); ok {
		frameworkVersion = analysis
	}

	// Generate Dockerfile based on detected language, framework, and versions
	dockerfile := generateDockerfileForLanguage(analyzeResult.Language, analyzeResult.Framework, analyzeResult.Port, languageVersion, frameworkVersion, logger)

	// Determine base image with version
	baseImage := getBaseImageForLanguage(analyzeResult.Language, analyzeResult.Framework, languageVersion)

	logger.Info("Dockerfile generated successfully",
		"base_image", baseImage,
		"language_version", languageVersion,
		"framework_version", frameworkVersion,
		"lines", len(strings.Split(dockerfile, "\n")),
		"port", analyzeResult.Port)

	return &DockerfileResult{
		Content:          dockerfile,
		Path:             "Dockerfile",
		BaseImage:        baseImage,
		LanguageVersion:  languageVersion,
		FrameworkVersion: frameworkVersion,
		ExposedPort:      analyzeResult.Port,
	}, nil
}

// generateDockerfileForLanguage creates language-specific Dockerfiles with version support
func generateDockerfileForLanguage(language, framework string, port int, languageVersion, frameworkVersion string, logger *slog.Logger) string {
	switch language {
	case "go":
		return generateGoDockerfile(port, languageVersion, logger)
	case "java":
		return generateJavaDockerfile(framework, port, languageVersion, frameworkVersion, logger)
	case "javascript", "typescript":
		return generateNodeDockerfile(framework, port, languageVersion, frameworkVersion, logger)
	case "python":
		return generatePythonDockerfile(framework, port, languageVersion, frameworkVersion, logger)
	case "rust":
		return generateRustDockerfile(port, languageVersion, logger)
	case "php":
		return generatePHPDockerfile(framework, port, languageVersion, logger)
	default:
		logger.Warn("Unknown language, generating generic Dockerfile", "language", language)
		return generateGenericDockerfile(port, logger)
	}
}

// generateGoDockerfile creates optimized Dockerfile for Go applications with version support
func generateGoDockerfile(port int, languageVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine Go version - use detected version or fallback to latest stable
	goVersion := "1.24-alpine"
	if languageVersion != "" {
		// Clean version string and ensure it works with Docker
		cleanVersion := strings.TrimPrefix(languageVersion, "v")
		if cleanVersion != "" {
			goVersion = cleanVersion + "-alpine"
		}
	}

	dockerfile.WriteString(fmt.Sprintf(`# Build stage
FROM golang:%s AS builder
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
`, goVersion))

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString("CMD [\"./main\"]\n")

	logger.Debug("Generated Go Dockerfile with multi-stage build", "go_version", goVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generateJavaDockerfile creates optimized Dockerfile for Java applications with version support
func generateJavaDockerfile(framework string, port int, languageVersion, frameworkVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine Java version - use detected version or fallback to stable version
	javaVersion := "17"
	if languageVersion != "" {
		// Extract major version from detected version (e.g., "11.0.1" -> "11")
		if majorVersion := strings.Split(languageVersion, ".")[0]; majorVersion != "" {
			javaVersion = majorVersion
		}
	}

	// Check if this is a servlet application (WAR file)
	isServlet := strings.Contains(strings.ToLower(framework), "servlet") ||
		strings.Contains(strings.ToLower(framework), "jsp") ||
		strings.Contains(strings.ToLower(framework), "war")

	if isServlet {
		// Use Tomcat for servlet applications
		dockerfile.WriteString(fmt.Sprintf(`# Build stage
FROM maven:3.9-eclipse-temurin-%s AS builder
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
FROM tomcat:10-jre%s-temurin-jammy
# Remove default webapps
RUN rm -rf /usr/local/tomcat/webapps/*
# Copy WAR file to Tomcat webapps as ROOT for root context
COPY --from=builder /app/target/*.war /usr/local/tomcat/webapps/ROOT.war
`, javaVersion, javaVersion))

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
		dockerfile.WriteString(fmt.Sprintf(`# Build stage
FROM maven:3.9-eclipse-temurin-%s AS builder
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
FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app

# Copy built artifacts from builder stage
# Use a shell script to find and copy the built artifact
RUN --mount=from=builder,source=/app,target=/build \
    find /build -name '*.jar' -not -name '*-sources.jar' -not -name '*-javadoc.jar' | head -1 | xargs -I {} cp {} /app/app.jar

`, javaVersion, javaVersion))

		if port > 0 {
			dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
		} else {
			dockerfile.WriteString("EXPOSE 8080\n") // Default Java web app port
		}

		dockerfile.WriteString(`# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]
`)
	}

	logger.Debug("Generated Java Dockerfile", "framework", framework, "isServlet", isServlet, "java_version", javaVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generateNodeDockerfile creates optimized Dockerfile for Node.js applications with version support
func generateNodeDockerfile(framework string, port int, languageVersion, frameworkVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine Node.js version - use detected version or fallback to stable version
	nodeVersion := "18-alpine"
	if languageVersion != "" {
		// Clean version string - handle versions like ">=16.0.0", "^18.0.0", etc.
		cleanVersion := strings.TrimSpace(languageVersion)
		cleanVersion = strings.Trim(cleanVersion, "^~>=<")
		// Extract major version
		if parts := strings.Split(cleanVersion, "."); len(parts) > 0 && parts[0] != "" {
			majorVersion := parts[0]
			nodeVersion = majorVersion + "-alpine"
		}
	}

	dockerfile.WriteString(fmt.Sprintf(`FROM node:%s
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
`, nodeVersion))

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

	logger.Debug("Generated Node.js Dockerfile", "framework", framework, "node_version", nodeVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generatePythonDockerfile creates optimized Dockerfile for Python applications with version support
func generatePythonDockerfile(framework string, port int, languageVersion, frameworkVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine Python version - use detected version or fallback to stable version
	pythonVersion := "3.11-slim"
	if languageVersion != "" {
		// Clean version string - handle versions like ">=3.9", "^3.11", etc.
		cleanVersion := strings.TrimSpace(languageVersion)
		cleanVersion = strings.Trim(cleanVersion, "^~>=<")
		// Extract major.minor version
		if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			majorMinor := parts[0] + "." + parts[1]
			pythonVersion = majorMinor + "-slim"
		}
	}

	dockerfile.WriteString(fmt.Sprintf(`FROM python:%s
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
`, pythonVersion))

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

	logger.Debug("Generated Python Dockerfile", "framework", framework, "python_version", pythonVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generateRustDockerfile creates optimized Dockerfile for Rust applications with version support
func generateRustDockerfile(port int, languageVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine Rust version - use detected version or fallback to stable version
	rustVersion := "1.70"
	if languageVersion != "" {
		// Clean version string and use it
		cleanVersion := strings.TrimSpace(languageVersion)
		cleanVersion = strings.TrimPrefix(cleanVersion, "v")
		if cleanVersion != "" {
			rustVersion = cleanVersion
		}
	}

	dockerfile.WriteString(fmt.Sprintf(`# Build stage
FROM rust:%s AS builder
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
`, rustVersion))

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	}

	dockerfile.WriteString(`CMD ["./main"]
`)

	logger.Debug("Generated Rust Dockerfile with multi-stage build", "rust_version", rustVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generatePHPDockerfile creates optimized Dockerfile for PHP applications with version support
func generatePHPDockerfile(framework string, port int, languageVersion string, logger *slog.Logger) string {
	var dockerfile strings.Builder

	// Determine PHP version - use detected version or fallback to stable version
	phpVersion := "8.2-apache"
	if languageVersion != "" {
		// Clean version string and use major.minor version
		cleanVersion := strings.TrimSpace(languageVersion)
		cleanVersion = strings.Trim(cleanVersion, "^~>=<")
		if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			majorMinor := parts[0] + "." + parts[1]
			phpVersion = majorMinor + "-apache"
		}
	}

	dockerfile.WriteString(fmt.Sprintf(`FROM php:%s
WORKDIR /var/www/html
COPY . .
RUN chown -R www-data:www-data /var/www/html
`, phpVersion))

	if port > 0 {
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	} else {
		dockerfile.WriteString("EXPOSE 80\n")
	}

	dockerfile.WriteString(`CMD ["apache2-foreground"]
`)

	logger.Debug("Generated PHP Dockerfile", "framework", framework, "php_version", phpVersion, "detected_version", languageVersion)
	return dockerfile.String()
}

// generateGenericDockerfile creates a basic Dockerfile for unknown languages
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

	logger.Debug("Generated generic Dockerfile")
	return dockerfile.String()
}

// getBaseImageForLanguage returns the base image used for a language with version support
func getBaseImageForLanguage(language, framework, languageVersion string) string {
	switch language {
	case "go":
		if languageVersion != "" {
			cleanVersion := strings.TrimPrefix(languageVersion, "v")
			if cleanVersion != "" {
				return fmt.Sprintf("golang:%s-alpine", cleanVersion)
			}
		}
		return "golang:1.24-alpine"
	case "java":
		javaVersion := "17"
		if languageVersion != "" {
			if majorVersion := strings.Split(languageVersion, ".")[0]; majorVersion != "" {
				javaVersion = majorVersion
			}
		}
		return fmt.Sprintf("openjdk:%s-jdk-slim", javaVersion)
	case "javascript", "typescript":
		if languageVersion != "" {
			cleanVersion := strings.TrimSpace(languageVersion)
			cleanVersion = strings.Trim(cleanVersion, "^~>=<")
			if parts := strings.Split(cleanVersion, "."); len(parts) > 0 && parts[0] != "" {
				majorVersion := parts[0]
				return fmt.Sprintf("node:%s-alpine", majorVersion)
			}
		}
		return "node:18-alpine"
	case "python":
		if languageVersion != "" {
			cleanVersion := strings.TrimSpace(languageVersion)
			cleanVersion = strings.Trim(cleanVersion, "^~>=<")
			if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
				majorMinor := parts[0] + "." + parts[1]
				return fmt.Sprintf("python:%s-slim", majorMinor)
			}
		}
		return "python:3.11-slim"
	case "rust":
		if languageVersion != "" {
			cleanVersion := strings.TrimPrefix(languageVersion, "v")
			if cleanVersion != "" {
				return fmt.Sprintf("rust:%s", cleanVersion)
			}
		}
		return "rust:1.70"
	case "php":
		if languageVersion != "" {
			cleanVersion := strings.TrimSpace(languageVersion)
			cleanVersion = strings.Trim(cleanVersion, "^~>=<")
			if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
				majorMinor := parts[0] + "." + parts[1]
				return fmt.Sprintf("php:%s-apache", majorMinor)
			}
		}
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
