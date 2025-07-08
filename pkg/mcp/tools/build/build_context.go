package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// BuildContextInfo provides rich context for understanding the build environment
type BuildContextInfo struct {
	DockerfileExists        bool     `json:"dockerfile_exists"`
	BuildArgs               []string `json:"build_args"`        // List of build arguments used
	BaseImage               string   `json:"base_image"`        // Base image from Dockerfile
	FileCount               int      `json:"file_count"`        // Number of files in build context
	ContextSizeMB           float64  `json:"context_size_mb"`   // Size of build context in MB
	ContextSize             int64    `json:"context_size"`      // Size of build context in bytes
	HasDockerIgnore         bool     `json:"has_docker_ignore"` // Whether .dockerignore exists
	LayerCount              int      `json:"layer_count"`       // Number of layers in final image
	CacheEfficiency         string   `json:"cache_efficiency"`  // poor, good, excellent
	ImageSize               string   `json:"image_size"`        // small, medium, large
	Optimizations           []string `json:"optimizations"`     // Suggested performance improvements
	NextStepSuggestions     []string `json:"next_step_suggestions"`
	TroubleshootingTips     []string `json:"troubleshooting_tips"`
	DockerfileLines         int      `json:"dockerfile_lines"`         // Number of lines in Dockerfile
	BuildStages             int      `json:"build_stages"`             // Number of build stages
	ExposedPorts            []string `json:"exposed_ports"`            // Exposed ports from Dockerfile
	LargeFilesFound         []string `json:"large_files_found"`        // Large files in build context
	FilesInContext          []string `json:"files_in_context"`         // Files in build context
	BuildOptimizations      []string `json:"build_optimizations"`      // Build optimization suggestions
	SecurityRecommendations []string `json:"security_recommendations"` // Security recommendations
}

// BuildContextAnalyzer handles build context analysis and preparation
type BuildContextAnalyzer struct {
	logger *slog.Logger
}

// NewBuildContextAnalyzer creates a new build context analyzer
func NewBuildContextAnalyzer(logger *slog.Logger) *BuildContextAnalyzer {
	return &BuildContextAnalyzer{
		logger: logger,
	}
}

// AnalyzeBuildContext analyzes the Dockerfile and build context
func (bca *BuildContextAnalyzer) AnalyzeBuildContext(dockerfilePath string, buildContext string) *BuildContextInfo {
	info := &BuildContextInfo{
		DockerfileExists: false,
		BuildArgs:        []string{},
		ExposedPorts:     []string{},
		LargeFilesFound:  []string{},
		FilesInContext:   []string{},
	}
	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); err == nil {
		info.DockerfileExists = true
		// Parse Dockerfile for base image and exposed ports
		if content, err := os.ReadFile(dockerfilePath); err == nil {
			lines := strings.Split(string(content), "\n")
			info.DockerfileLines = len(lines)
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "FROM ") {
					parts := strings.Fields(trimmed)
					if len(parts) > 1 {
						info.BaseImage = parts[1]
						info.BuildStages++
					}
				}
				if strings.HasPrefix(trimmed, "EXPOSE ") {
					parts := strings.Fields(trimmed)
					if len(parts) > 1 {
						info.ExposedPorts = append(info.ExposedPorts, parts[1])
					}
				}
			}
		}
	}
	// Analyze build context directory
	bca.analyzeBuildContextDirectory(buildContext, info)
	// Add optimization suggestions based on analysis
	if info.ContextSizeMB > 100 {
		info.BuildOptimizations = append(info.BuildOptimizations, "Consider using .dockerignore to reduce build context size")
	}
	if !info.HasDockerIgnore {
		info.BuildOptimizations = append(info.BuildOptimizations, "Add .dockerignore file to exclude unnecessary files from build context")
	}
	if info.BuildStages == 1 && info.DockerfileLines > 50 {
		info.BuildOptimizations = append(info.BuildOptimizations, "Consider using multi-stage builds to reduce final image size")
	}
	return info
}

// analyzeBuildContextDirectory analyzes the build context directory
func (bca *BuildContextAnalyzer) analyzeBuildContextDirectory(contextPath string, info *BuildContextInfo) {
	var totalSize int64
	var fileCount int
	largeFileThreshold := int64(10 * 1024 * 1024) // 10MB
	err := filepath.Walk(contextPath, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		// Skip directories
		if fileInfo.IsDir() {
			return nil
		}
		// Check for .dockerignore
		if fileInfo.Name() == ".dockerignore" {
			info.HasDockerIgnore = true
		}
		relPath, _ := filepath.Rel(contextPath, path)
		info.FilesInContext = append(info.FilesInContext, relPath)
		fileCount++
		fileSize := fileInfo.Size()
		totalSize += fileSize
		// Track large files
		if fileSize > largeFileThreshold {
			info.LargeFilesFound = append(info.LargeFilesFound, fmt.Sprintf("%s (%.2fMB)", relPath, float64(fileSize)/(1024*1024)))
		}
		return nil
	})
	if err != nil {
		bca.logger.Warn("Error walking build context directory", "error", err)
	}
	info.FileCount = fileCount
	info.ContextSize = totalSize
	info.ContextSizeMB = float64(totalSize) / (1024 * 1024)
	// Set cache efficiency based on context size
	if info.ContextSizeMB < 50 {
		info.CacheEfficiency = "excellent"
	} else if info.ContextSizeMB < 200 {
		info.CacheEfficiency = "good"
	} else {
		info.CacheEfficiency = "poor"
	}
}

// GenerateBuildContext generates rich context information for AI understanding
func (bca *BuildContextAnalyzer) GenerateBuildContext(
	config mcptypes.BuildContextGenerateConfig,
) map[string]interface{} {
	contextInfo := map[string]interface{}{
		"session": map[string]interface{}{
			"id":        config.SessionID,
			"workspace": config.WorkspaceDir,
		},
		"build_config": map[string]interface{}{
			"image_name":      config.ImageName,
			"image_tag":       config.ImageTag,
			"full_image_ref":  fmt.Sprintf("%s:%s", config.ImageName, config.ImageTag),
			"dockerfile_path": config.DockerfilePath,
			"build_context":   config.BuildContext,
			"platform":        config.Platform,
			"build_args":      config.BuildArgs,
		},
		"environment": map[string]interface{}{
			"docker_available": true,    // Assumed since we're building
			"registry_config":  "local", // Default to local
		},
	}
	// Check if we're in a common project structure
	if _, err := os.Stat(filepath.Join(config.WorkspaceDir, "package.json")); err == nil {
		contextInfo["project_type"] = "node"
	} else if _, err := os.Stat(filepath.Join(config.WorkspaceDir, "go.mod")); err == nil {
		contextInfo["project_type"] = "go"
	} else if _, err := os.Stat(filepath.Join(config.WorkspaceDir, "requirements.txt")); err == nil {
		contextInfo["project_type"] = "python"
	}
	return contextInfo
}

// Helper methods for getting build configuration with defaults
// GetImageTag returns the image tag with default
func GetImageTag(tag string) string {
	if tag == "" {
		return "latest"
	}
	return tag
}

// GetPlatform returns the platform with default
func GetPlatform(platform string) string {
	if platform == "" {
		return "linux/amd64"
	}
	return platform
}

// GetBuildContext returns the build context path with validation
func GetBuildContext(buildContext string, workspaceDir string) (string, error) {
	if buildContext == "" {
		buildContext = workspaceDir
	}
	// Ensure absolute path
	if !filepath.IsAbs(buildContext) {
		buildContext = filepath.Join(workspaceDir, buildContext)
	}
	// Validate the path exists
	if _, err := os.Stat(buildContext); err != nil {
		return "", errors.NewError().Messagef("build context path does not exist: %s", buildContext).Build()
	}
	return buildContext, nil
}
