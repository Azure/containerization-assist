package docker

import (
	"fmt"
	"strings"
	"testing"

	"log/slog"

	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDockerfile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	tests := []struct {
		name           string
		dockerfile     string
		expectedValid  bool
		expectedErrors int
		expectedWarns  int
		checkError     func(t *testing.T, errors []validation.Error)
		checkWarning   func(t *testing.T, warnings []validation.Warning)
	}{
		{
			name:           "empty dockerfile",
			dockerfile:     "",
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				assert.Equal(t, "DOCKERFILE_EMPTY", errors[0].Code)
				assert.Contains(t, errors[0].Message, "empty")
			},
		},
		{
			name:           "whitespace only dockerfile",
			dockerfile:     "   \n\t\n   ",
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
		},
		{
			name: "valid minimal dockerfile",
			dockerfile: `FROM node:16-alpine
WORKDIR /app
COPY . .
RUN npm install
CMD ["node", "index.js"]`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0, // node:16-alpine has a specific tag, /app is absolute
		},
		{
			name: "missing FROM instruction",
			dockerfile: `WORKDIR /app
COPY . .
RUN npm install`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				assert.Equal(t, "DOCKERFILE_NO_FROM", errors[0].Code)
				assert.Contains(t, errors[0].Message, "must start with FROM")
			},
		},
		{
			name: "invalid instruction",
			dockerfile: `FROM node:16
INVALID_INSTRUCTION something`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				assert.Equal(t, "UNKNOWN_INSTRUCTION", errors[0].Code)
				assert.Contains(t, errors[0].Message, "Unknown instruction")
			},
		},
		{
			name: "FROM without image name",
			dockerfile: `FROM
WORKDIR /app`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				assert.Equal(t, "FROM_MISSING_IMAGE", errors[0].Code)
				assert.Equal(t, "FROM", errors[0].Field)
				assert.Contains(t, errors[0].Message, "requires an image name")
			},
		},
		{
			name:           "FROM with latest tag warning",
			dockerfile:     `FROM ubuntu:latest`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Equal(t, "FROM_LATEST_TAG", warnings[0].Code)
				assert.Contains(t, warnings[0].Message, "latest")
			},
		},
		{
			name:           "FROM without tag warning",
			dockerfile:     `FROM ubuntu`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Equal(t, "FROM_LATEST_TAG", warnings[0].Code)
				assert.Contains(t, warnings[0].Suggestion, "specific version")
			},
		},
		{
			name: "RUN apt-get without update warning",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get install -y curl`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  2, // apt-get update + cache cleanup warnings
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				foundUpdate := false
				foundCleanup := false
				for _, w := range warnings {
					if strings.Contains(w.Message, "apt-get update") {
						foundUpdate = true
					}
					if strings.Contains(w.Message, "cache") {
						foundCleanup = true
					}
				}
				assert.True(t, foundUpdate, "Should warn about apt-get update")
				assert.True(t, foundCleanup, "Should warn about cache cleanup")
			},
		},
		{
			name: "RUN apt-get with update but no cleanup",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1, // Only cache cleanup warning
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Contains(t, warnings[0].Message, "cache")
			},
		},
		{
			name: "RUN apt-get best practices",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "COPY without enough arguments",
			dockerfile: `FROM node:16
COPY`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				assert.Equal(t, "COPY_MISSING_ARGS", errors[0].Code)
				assert.Equal(t, "COPY", errors[0].Field)
			},
		},
		{
			name: "ADD vs COPY warning",
			dockerfile: `FROM node:16
ADD package.json /app/`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				if warnings[0].Field != "" {
					assert.Equal(t, "ADD", warnings[0].Field)
				}
				assert.Contains(t, warnings[0].Message, "COPY is preferred")
			},
		},
		{
			name: "ADD for URL is OK",
			dockerfile: `FROM node:16
ADD https://example.com/file.tar.gz /app/`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "ADD for tar file is OK",
			dockerfile: `FROM node:16
ADD app.tar /app/`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "EXPOSE without port",
			dockerfile: `FROM node:16
EXPOSE`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				if errors[0].Field != "" {
					assert.Equal(t, "EXPOSE", errors[0].Field)
				}
				assert.Contains(t, errors[0].Message, "port number")
			},
		},
		{
			name: "EXPOSE with valid ports",
			dockerfile: `FROM node:16
EXPOSE 80
EXPOSE 443/tcp
EXPOSE 8080/udp`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "EXPOSE with invalid port format",
			dockerfile: `FROM node:16
EXPOSE abc
EXPOSE 80/invalid`,
			expectedValid:  false,
			expectedErrors: 2,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				for _, err := range errors {
					if err.Field != "" {
						assert.Equal(t, "EXPOSE", err.Field)
					}
					assert.Contains(t, err.Message, "Invalid port format")
				}
			},
		},
		{
			name: "USER without username",
			dockerfile: `FROM node:16
USER`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				if errors[0].Field != "" {
					assert.Equal(t, "USER", errors[0].Field)
				}
			},
		},
		{
			name: "USER root warning",
			dockerfile: `FROM node:16
USER root`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Equal(t, "USER_ROOT_SECURITY", warnings[0].Code)
				assert.Contains(t, warnings[0].Message, "root user")
			},
		},
		{
			name: "USER non-root",
			dockerfile: `FROM node:16
USER node`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "WORKDIR without path",
			dockerfile: `FROM node:16
WORKDIR`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				if errors[0].Field != "" {
					assert.Equal(t, "WORKDIR", errors[0].Field)
				}
			},
		},
		{
			name: "WORKDIR relative path warning",
			dockerfile: `FROM node:16
WORKDIR app`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				if warnings[0].Field != "" {
					assert.Equal(t, "WORKDIR", warnings[0].Field)
				}
				assert.Contains(t, warnings[0].Message, "absolute paths")
			},
		},
		{
			name: "WORKDIR absolute path",
			dockerfile: `FROM node:16
WORKDIR /app`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "CMD without command",
			dockerfile: `FROM node:16
CMD`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				if errors[0].Field != "" {
					assert.Equal(t, "CMD", errors[0].Field)
				}
			},
		},
		{
			name: "ENTRYPOINT without command",
			dockerfile: `FROM node:16
ENTRYPOINT`,
			expectedValid:  false,
			expectedErrors: 1,
			expectedWarns:  0,
			checkError: func(t *testing.T, errors []validation.Error) {
				if errors[0].Field != "" {
					assert.Equal(t, "ENTRYPOINT", errors[0].Field)
				}
			},
		},
		{
			name: "multiple CMD warning",
			dockerfile: `FROM node:16
CMD ["node", "app1.js"]
CMD ["node", "app2.js"]`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Equal(t, "MULTIPLE_CMD", warnings[0].Code)
				assert.Contains(t, warnings[0].Message, "Multiple CMD")
			},
		},
		{
			name: "multiple ENTRYPOINT warning",
			dockerfile: `FROM node:16
ENTRYPOINT ["node"]
ENTRYPOINT ["npm", "start"]`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  1,
			checkWarning: func(t *testing.T, warnings []validation.Warning) {
				assert.Equal(t, "MULTIPLE_ENTRYPOINT", warnings[0].Code)
				assert.Contains(t, warnings[0].Message, "Multiple ENTRYPOINT")
			},
		},
		{
			name: "comments and empty lines",
			dockerfile: `# Base image
FROM node:16

# Set working directory
WORKDIR /app

# Copy files
COPY . .

# Install dependencies
RUN npm install

# Start application
CMD ["node", "index.js"]`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name: "all valid instructions",
			dockerfile: `FROM node:16-alpine
LABEL maintainer="test@example.com"
ARG BUILD_VERSION=1.0
ENV NODE_ENV=production
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
USER node
HEALTHCHECK --interval=30s --timeout=3s CMD node healthcheck.js
VOLUME ["/app/data"]
STOPSIGNAL SIGTERM
CMD ["node", "server.js"]`,
			expectedValid:  true,
			expectedErrors: 0,
			expectedWarns:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateDockerfile(tt.dockerfile)

			assert.Equal(t, tt.expectedValid, result.Valid, "Validity mismatch")
			assert.Len(t, result.Errors, tt.expectedErrors, "Error count mismatch")
			assert.Len(t, result.Warnings, tt.expectedWarns, "Warning count mismatch")

			if tt.checkError != nil && len(result.Errors) > 0 {
				tt.checkError(t, result.Errors)
			}

			if tt.checkWarning != nil && len(result.Warnings) > 0 {
				tt.checkWarning(t, result.Warnings)
			}
		})
	}
}

func TestValidateStructure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	tests := []struct {
		name           string
		instructions   []string
		expectedErrors int
		expectedWarns  int
	}{
		{
			name:           "empty instructions",
			instructions:   []string{},
			expectedErrors: 1,
			expectedWarns:  0,
		},
		{
			name:           "not starting with FROM",
			instructions:   []string{"RUN", "CMD"},
			expectedErrors: 1,
			expectedWarns:  0,
		},
		{
			name:           "valid structure",
			instructions:   []string{"FROM", "RUN", "CMD"},
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name:           "multiple CMD",
			instructions:   []string{"FROM", "CMD", "CMD", "RUN"},
			expectedErrors: 0,
			expectedWarns:  1,
		},
		{
			name:           "multiple ENTRYPOINT",
			instructions:   []string{"FROM", "ENTRYPOINT", "ENTRYPOINT"},
			expectedErrors: 0,
			expectedWarns:  1,
		},
		{
			name:           "multiple CMD and ENTRYPOINT",
			instructions:   []string{"FROM", "CMD", "ENTRYPOINT", "CMD", "ENTRYPOINT"},
			expectedErrors: 0,
			expectedWarns:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &BuildValidationResult{
				Valid:    true,
				Errors:   make([]validation.Error, 0),
				Warnings: make([]validation.Warning, 0),
			}

			validator.validateStructure(tt.instructions, result)

			assert.Len(t, result.Errors, tt.expectedErrors, "Error count mismatch")
			assert.Len(t, result.Warnings, tt.expectedWarns, "Warning count mismatch")
		})
	}
}

func TestAddGeneralSuggestions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	tests := []struct {
		name               string
		dockerfile         string
		expectedSuggestion string
		shouldContain      bool
	}{
		{
			name: "suggest HEALTHCHECK",
			dockerfile: `FROM node:16
CMD ["node", "app.js"]`,
			expectedSuggestion: "HEALTHCHECK",
			shouldContain:      true,
		},
		{
			name: "no HEALTHCHECK suggestion if present",
			dockerfile: `FROM node:16
HEALTHCHECK CMD curl -f http://localhost/ || exit 1
CMD ["node", "app.js"]`,
			expectedSuggestion: "HEALTHCHECK",
			shouldContain:      false,
		},
		{
			name: "suggest multi-stage for npm",
			dockerfile: `FROM node:16
RUN npm install
CMD ["node", "app.js"]`,
			expectedSuggestion: "multi-stage",
			shouldContain:      true,
		},
		{
			name: "suggest multi-stage for go",
			dockerfile: `FROM golang:1.17
RUN go build -o app
CMD ["./app"]`,
			expectedSuggestion: "multi-stage",
			shouldContain:      true,
		},
		{
			name: "suggest multi-stage for maven",
			dockerfile: `FROM maven:3.8
RUN mvn package
CMD ["java", "-jar", "app.jar"]`,
			expectedSuggestion: "multi-stage",
			shouldContain:      true,
		},
		{
			name: "no multi-stage suggestion if already using it",
			dockerfile: `FROM node:16 AS builder
RUN npm install
FROM node:16-alpine
COPY --from=builder /app .
CMD ["node", "app.js"]`,
			expectedSuggestion: "multi-stage",
			shouldContain:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &BuildValidationResult{
				Details: make(map[string]interface{}),
			}

			validator.addGeneralSuggestions(tt.dockerfile, result)

			found := false
			if suggestions, ok := result.Details["suggestions"].([]string); ok {
				for _, suggestion := range suggestions {
					if strings.Contains(suggestion, tt.expectedSuggestion) {
						found = true
						break
					}
				}
			}

			if tt.shouldContain {
				assert.True(t, found, "Expected suggestion containing '%s' not found", tt.expectedSuggestion)
			} else {
				assert.False(t, found, "Unexpected suggestion containing '%s' found", tt.expectedSuggestion)
			}

			// Always check for standard suggestions
			suggestions, _ := result.Details["suggestions"].([]string)
			assert.True(t, len(suggestions) > 0, "Should have at least some suggestions")

			// Check for dockerignore suggestion
			foundDockerignore := false
			for _, suggestion := range suggestions {
				if strings.Contains(suggestion, "dockerignore") {
					foundDockerignore = true
					break
				}
			}
			assert.True(t, foundDockerignore, "Should always suggest .dockerignore")
		})
	}
}

func TestValidationContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	dockerfile := `FROM node:16
WORKDIR /app
COPY . .
RUN npm install
EXPOSE 3000
CMD ["node", "app.js"]`

	result := validator.ValidateDockerfile(dockerfile)

	require.NotNil(t, result.Metadata.Context)

	lineCountStr, ok := result.Metadata.Context["line_count"]
	assert.True(t, ok, "line_count should be present")
	assert.Equal(t, "6", lineCountStr)

	totalSizeStr, ok := result.Metadata.Context["total_size"]
	assert.True(t, ok, "total_size should be present")
	assert.Equal(t, fmt.Sprintf("%d", len(dockerfile)), totalSizeStr)
}

func TestComplexDockerfileValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	// Complex multi-stage Dockerfile with various patterns
	dockerfile := `# Build stage
FROM golang:1.17-alpine AS builder
ARG VERSION=dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o app .

# Runtime stage
FROM alpine:3.14
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/app .
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
CMD ["serve"]`

	result := validator.ValidateDockerfile(dockerfile)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Len(t, result.Warnings, 1) // One warning for multiple CMD (ENTRYPOINT + CMD)

	// Should not suggest multi-stage since it's already using it
	if suggestions, ok := result.Details["suggestions"].([]string); ok {
		for _, suggestion := range suggestions {
			assert.NotContains(t, suggestion, "multi-stage")
		}
	}
}

func TestEdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	tests := []struct {
		name       string
		dockerfile string
	}{
		{
			name: "dockerfile with only comments",
			dockerfile: `# This is a comment
# Another comment
# Yet another comment`,
		},
		{
			name: "dockerfile with mixed case instructions",
			dockerfile: `from node:16
From ubuntu:20.04
FROM alpine:3.14`,
		},
		{
			name: "dockerfile with tabs and spaces",
			dockerfile: `FROM	node:16
		WORKDIR		/app
	COPY	.	.`,
		},
		{
			name: "dockerfile with line continuations",
			dockerfile: `FROM node:16
RUN apt-get update \
    && apt-get install -y \
        curl \
        git \
    && rm -rf /var/lib/apt/lists/*`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := validator.ValidateDockerfile(tt.dockerfile)
			assert.NotNil(t, result)
		})
	}
}

func TestCheckDockerInstallation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	validator := NewValidator(logger)

	// This test will pass or fail based on whether Docker is installed
	// In CI/CD environments, Docker should be available
	err := validator.CheckDockerInstallation()

	// We can't assert the result since it depends on the environment
	// But we can check that the function doesn't panic
	if err != nil {
		t.Logf("Docker check returned error (expected if Docker not installed): %v", err)
		assert.Contains(t, err.Error(), "docker")
	} else {
		t.Log("Docker is installed and accessible")
	}
}
