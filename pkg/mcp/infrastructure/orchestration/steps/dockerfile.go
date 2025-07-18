// Package steps provides orchestration steps for container-kit workflows.
// This file contains Dockerfile generation functionality using Go templates
// instead of string building patterns for improved maintainability.
package steps

import (
"bytes"
"fmt"
"log/slog"
"os"
"path/filepath"
"strings"
"text/template"

"github.com/Azure/container-kit/pkg/common/errors"
"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps/dockerfile"
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

// TemplateData contains data for Dockerfile templates
type TemplateData struct {
Language         string
Framework        string
LanguageVersion  string
FrameworkVersion string
Port             int
DefaultPort      int
IsServlet        bool
HasBuildStep     bool
HasNextFramework bool
IsDjango         bool
IsFastAPI        bool
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
// Prepare template data
data := &TemplateData{
Language:         language,
Framework:        framework,
LanguageVersion:  getVersionForLanguage(language, languageVersion),
FrameworkVersion: frameworkVersion,
Port:             port,
HasNextFramework: strings.Contains(framework, "next"),
IsDjango:         strings.Contains(framework, "django"),
IsFastAPI:        strings.Contains(framework, "fastapi"),
}

// Set servlet flag for Java
if language == "java" {
data.IsServlet = strings.Contains(strings.ToLower(framework), "servlet") ||
strings.Contains(strings.ToLower(framework), "jsp") ||
strings.Contains(strings.ToLower(framework), "war")
}

// Select template based on language
templateName := language
switch language {
case "javascript", "typescript":
templateName = "node"
case "go", "java", "python", "rust", "php":
// Use language as-is
default:
templateName = "generic"
logger.Warn("Unknown language, using generic template", "language", language)
}

// Get and parse template
tmplStr, exists := dockerfile.GetTemplate(templateName)
if !exists {
logger.Error("Template not found", "template", templateName)
return generateGenericDockerfile(port, logger)
}

tmpl, err := template.New(templateName).Parse(tmplStr)
if err != nil {
logger.Error("Failed to parse template", "template", templateName, "error", err)
return generateGenericDockerfile(port, logger)
}

// Execute template
var buf bytes.Buffer
if err := tmpl.Execute(&buf, data); err != nil {
logger.Error("Failed to execute template", "template", templateName, "error", err)
return generateGenericDockerfile(port, logger)
}

result := buf.String()
logger.Debug("Generated Dockerfile from template",
"template", templateName,
"language", language,
"framework", framework,
"version", data.LanguageVersion,
"lines", len(strings.Split(result, "\n")))

return result
}

// getVersionForLanguage returns the appropriate version string for a language
func getVersionForLanguage(language, languageVersion string) string {
switch language {
case "go":
if languageVersion != "" {
cleanVersion := strings.TrimPrefix(languageVersion, "v")
if cleanVersion != "" {
return cleanVersion
}
}
return "1.24"
case "java":
if languageVersion != "" {
if majorVersion := strings.Split(languageVersion, ".")[0]; majorVersion != "" {
return majorVersion
}
}
return "17"
case "javascript", "typescript":
if languageVersion != "" {
cleanVersion := strings.TrimSpace(languageVersion)
cleanVersion = strings.Trim(cleanVersion, "^~>=<")
if parts := strings.Split(cleanVersion, "."); len(parts) > 0 && parts[0] != "" {
return parts[0]
}
}
return "18"
case "python":
if languageVersion != "" {
cleanVersion := strings.TrimSpace(languageVersion)
cleanVersion = strings.Trim(cleanVersion, "^~>=<")
if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
return parts[0] + "." + parts[1]
}
}
return "3.11"
case "rust":
if languageVersion != "" {
cleanVersion := strings.TrimSpace(languageVersion)
cleanVersion = strings.TrimPrefix(cleanVersion, "v")
if cleanVersion != "" {
return cleanVersion
}
}
return "1.70"
case "php":
if languageVersion != "" {
cleanVersion := strings.TrimSpace(languageVersion)
cleanVersion = strings.Trim(cleanVersion, "^~>=<")
if parts := strings.Split(cleanVersion, "."); len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
return parts[0] + "." + parts[1]
}
}
return "8.2"
default:
return "latest"
}
}

// generateGenericDockerfile creates a basic Dockerfile for unknown languages
func generateGenericDockerfile(port int, logger *slog.Logger) string {
data := &TemplateData{
Port: port,
}

tmpl, err := template.New("generic").Parse(dockerfile.GenericTemplate)
if err != nil {
logger.Error("Failed to parse generic template", "error", err)
return `FROM alpine:latest
WORKDIR /app
COPY . .
CMD ["./start.sh"]`
}

var buf bytes.Buffer
if err := tmpl.Execute(&buf, data); err != nil {
logger.Error("Failed to execute generic template", "error", err)
return `FROM alpine:latest
WORKDIR /app
COPY . .
CMD ["./start.sh"]`
}

logger.Debug("Generated generic Dockerfile")
return buf.String()
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
