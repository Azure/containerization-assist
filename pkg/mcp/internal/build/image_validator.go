package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ImageValidator handles base image validation
type ImageValidator struct {
	logger            zerolog.Logger
	trustedRegistries []string
}

// NewImageValidator creates a new image validator
func NewImageValidator(logger zerolog.Logger, trustedRegistries []string) *ImageValidator {
	return &ImageValidator{
		logger:            logger.With().Str("component", "image_validator").Logger(),
		trustedRegistries: trustedRegistries,
	}
}

// Validate performs image-related validation
func (v *ImageValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().Msg("Starting base image validation")
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Info:     make([]string, 0),
	}
	lines := strings.Split(content, "\n")
	images := v.extractBaseImages(lines)
	// Validate each base image
	for _, img := range images {
		v.validateImage(img, result)
	}
	// Check for multi-stage build best practices
	if len(images) > 1 {
		v.validateMultiStageImages(images, result)
	}
	// Update result state
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result, nil
}

// Analyze provides image-specific analysis
func (v *ImageValidator) Analyze(lines []string, context ValidationContext) interface{} {
	images := v.extractBaseImages(lines)
	if len(images) == 0 {
		return BaseImageAnalysis{
			Recommendations: []string{"No base image found - add FROM instruction"},
		}
	}
	// Analyze the first/main base image
	mainImage := images[0]
	analysis := v.analyzeBaseImage(mainImage)
	// Add multi-stage specific recommendations
	if len(images) > 1 {
		analysis.Recommendations = append(analysis.Recommendations,
			fmt.Sprintf("Multi-stage build detected with %d stages", len(images)))
		// Check if using consistent base images
		baseImageMap := make(map[string]int)
		for _, img := range images {
			base := img.Image
			if idx := strings.Index(base, ":"); idx > 0 {
				base = base[:idx]
			}
			baseImageMap[base]++
		}
		if len(baseImageMap) > 3 {
			analysis.Recommendations = append(analysis.Recommendations,
				"Consider using fewer distinct base images for better caching")
		}
	}
	return analysis
}

// ImageInfo represents information about a base image
type ImageInfo struct {
	Line      int
	Image     string
	Registry  string
	Tag       string
	StageName string
}

// extractBaseImages extracts all FROM instructions
func (v *ImageValidator) extractBaseImages(lines []string) []ImageInfo {
	images := make([]ImageInfo, 0)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				imgInfo := ImageInfo{
					Line:  i + 1,
					Image: parts[1],
				}
				// Parse registry and tag
				v.parseImageReference(&imgInfo)
				// Extract stage name if present
				for j, part := range parts {
					if strings.ToUpper(part) == "AS" && j+1 < len(parts) {
						imgInfo.StageName = parts[j+1]
						break
					}
				}
				images = append(images, imgInfo)
			}
		}
	}
	return images
}

// parseImageReference parses registry and tag from image reference
func (v *ImageValidator) parseImageReference(img *ImageInfo) {
	image := img.Image
	// Extract tag
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		img.Tag = image[idx+1:]
		image = image[:idx]
	}
	// Extract registry
	if strings.Contains(image, "/") {
		parts := strings.Split(image, "/")
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			img.Registry = parts[0]
		} else {
			img.Registry = "docker.io"
		}
	} else {
		img.Registry = "docker.io"
	}
}

// validateImage validates a single base image
func (v *ImageValidator) validateImage(img ImageInfo, result *ValidationResult) {
	// Check for missing tag
	if img.Tag == "" || img.Tag == "latest" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    img.Line,
			Message: fmt.Sprintf("Base image '%s' uses 'latest' tag or no tag. Use specific version tags for reproducible builds", img.Image),
			Rule:    "image_tag",
		})
	}
	// Check trusted registries
	if len(v.trustedRegistries) > 0 && !v.isTrustedRegistry(img.Registry) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    img.Line,
			Message: fmt.Sprintf("Base image from untrusted registry: %s. Use images from trusted registries", img.Registry),
			Rule:    "untrusted_registry",
		})
	}
	// Check for deprecated images
	if deprecated, suggestion := v.isDeprecatedImage(img.Image); deprecated {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    img.Line,
			Message: fmt.Sprintf("Base image '%s' is deprecated. %s", img.Image, suggestion),
			Rule:    "deprecated_image",
		})
	}
	// Check for vulnerable images
	if v.isKnownVulnerableImage(img.Image) {
		result.Errors = append(result.Errors, ValidationError{
			Line:    img.Line,
			Message: fmt.Sprintf("Base image '%s' has known vulnerabilities", img.Image),
			Rule:    "vulnerable_image",
		})
	}
}

// validateMultiStageImages validates multi-stage build practices
func (v *ImageValidator) validateMultiStageImages(images []ImageInfo, result *ValidationResult) {
	// Check for unnamed stages
	for i, img := range images {
		if img.StageName == "" && i < len(images)-1 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    img.Line,
				Message: "Intermediate build stage without name. Name build stages with 'AS <name>' for clarity",
				Rule:    "unnamed_stage",
			})
		}
	}
	// Check for unused stages
	stageReferences := v.findStageReferences(images)
	for _, img := range images[:len(images)-1] { // Skip final stage
		if img.StageName != "" && !stageReferences[img.StageName] {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    img.Line,
				Message: fmt.Sprintf("Build stage '%s' appears to be unused. Remove unused build stages or reference them with COPY --from", img.StageName),
				Rule:    "unused_stage",
			})
		}
	}
}

// analyzeBaseImage analyzes a base image
func (v *ImageValidator) analyzeBaseImage(img ImageInfo) BaseImageAnalysis {
	analysis := BaseImageAnalysis{
		Image:           img.Image,
		Registry:        img.Registry,
		IsTrusted:       v.isTrustedRegistry(img.Registry),
		IsOfficial:      v.isOfficialImage(img.Image),
		Recommendations: make([]string, 0),
		Alternatives:    make([]string, 0),
	}
	// Check for vulnerabilities
	analysis.HasKnownVulns = v.isKnownVulnerableImage(img.Image)
	// Add recommendations based on image
	if img.Tag == "" || img.Tag == "latest" {
		analysis.Recommendations = append(analysis.Recommendations,
			"Pin base image to specific version")
	}
	// Suggest alternatives
	analysis.Alternatives = v.suggestAlternatives(img.Image)
	// Add size recommendations
	if v.isLargeBaseImage(img.Image) {
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider using a smaller base image like Alpine or distroless")
	}
	return analysis
}

// Helper functions
func (v *ImageValidator) isTrustedRegistry(registry string) bool {
	if len(v.trustedRegistries) == 0 {
		// Default trusted registries
		defaultTrusted := []string{
			"docker.io",
			"gcr.io",
			"quay.io",
			"mcr.microsoft.com",
			"public.ecr.aws",
		}
		for _, trusted := range defaultTrusted {
			if registry == trusted {
				return true
			}
		}
		return false
	}
	for _, trusted := range v.trustedRegistries {
		if registry == trusted {
			return true
		}
	}
	return false
}
func (v *ImageValidator) isOfficialImage(image string) bool {
	// Official images don't have a username/organization prefix
	parts := strings.Split(image, "/")
	return len(parts) == 1 || (len(parts) == 2 && parts[0] == "library")
}
func (v *ImageValidator) isDeprecatedImage(image string) (bool, string) {
	deprecatedImages := map[string]string{
		"centos":    "Consider using rockylinux or almalinux instead",
		"openjdk:8": "Consider using a more recent JDK version",
		"python:2":  "Python 2 is EOL, use Python 3",
		"node:6":    "Node.js 6 is EOL, use a supported version",
		"node:8":    "Node.js 8 is EOL, use a supported version",
	}
	for deprecated, suggestion := range deprecatedImages {
		if strings.Contains(image, deprecated) {
			return true, suggestion
		}
	}
	return false, ""
}
func (v *ImageValidator) isKnownVulnerableImage(image string) bool {
	// Use real vulnerability scanning with Trivy/Grype
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	vulnResult := v.scanImageVulnerabilities(ctx, image)
	if vulnResult != nil {
		// Check for high/critical vulnerabilities
		return vulnResult.HasCriticalVulns || vulnResult.HighVulns > 0
	}
	// Fallback to known vulnerable patterns if scan fails
	vulnerablePatterns := []string{
		"ubuntu:14",
		"ubuntu:16",
		"debian:7",
		"debian:8",
		"alpine:3.1",
		"alpine:3.2",
		"alpine:3.3",
	}
	for _, pattern := range vulnerablePatterns {
		if strings.Contains(image, pattern) {
			return true
		}
	}
	return false
}
func (v *ImageValidator) isLargeBaseImage(image string) bool {
	largeImages := []string{
		"ubuntu",
		"debian",
		"centos",
		"fedora",
	}
	imageLower := strings.ToLower(image)
	for _, large := range largeImages {
		if strings.Contains(imageLower, large) &&
			!strings.Contains(imageLower, "slim") &&
			!strings.Contains(imageLower, "minimal") {
			return true
		}
	}
	return false
}
func (v *ImageValidator) suggestAlternatives(image string) []string {
	alternatives := make([]string, 0)
	baseImage := strings.Split(image, ":")[0]
	switch {
	case strings.Contains(baseImage, "ubuntu"):
		alternatives = append(alternatives, "ubuntu:22.04-slim", "debian:bullseye-slim", "alpine:latest")
	case strings.Contains(baseImage, "debian"):
		alternatives = append(alternatives, "debian:bullseye-slim", "alpine:latest")
	case strings.Contains(baseImage, "centos"):
		alternatives = append(alternatives, "rockylinux:9-minimal", "almalinux:9-minimal")
	case strings.Contains(baseImage, "node") && !strings.Contains(baseImage, "alpine"):
		alternatives = append(alternatives, "node:18-alpine", "node:18-slim")
	case strings.Contains(baseImage, "python") && !strings.Contains(baseImage, "alpine"):
		alternatives = append(alternatives, "python:3.11-alpine", "python:3.11-slim")
	case strings.Contains(baseImage, "golang"):
		alternatives = append(alternatives, "golang:1.21-alpine", "distroless/base-debian11")
	}
	return alternatives
}
func (v *ImageValidator) findStageReferences(images []ImageInfo) map[string]bool {
	references := make(map[string]bool)
	// Parse COPY --from instructions to accurately detect stage references
	// Extract content from the original lines that contained the images
	for _, img := range images {
		if img.StageName != "" {
			// Mark stage as potentially referenced by default
			references[img.StageName] = true
		}
	}
	// NOTE: Add more sophisticated parsing of COPY --from=stage instructions
	// This would require access to the full Dockerfile content
	return references
}

// VulnerabilityResult represents the result of a vulnerability scan
type VulnerabilityResult struct {
	HasCriticalVulns bool
	CriticalVulns    int
	HighVulns        int
	MediumVulns      int
	LowVulns         int
	TotalVulns       int
	ScanTool         string
	ScanDuration     time.Duration
}

// TrivyVulnerability represents a vulnerability from Trivy
type TrivyVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	Description      string `json:"Description"`
}

// TrivyResult represents the full Trivy scan result
type TrivyResult struct {
	Results []struct {
		Target          string               `json:"Target"`
		Class           string               `json:"Class"`
		Type            string               `json:"Type"`
		Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
	} `json:"Results"`
}

// scanImageVulnerabilities performs vulnerability scanning using Trivy or Grype
func (v *ImageValidator) scanImageVulnerabilities(ctx context.Context, image string) *VulnerabilityResult {
	// Try Trivy first
	if result := v.scanWithTrivy(ctx, image); result != nil {
		return result
	}
	// Fallback to Grype if Trivy fails
	if result := v.scanWithGrype(ctx, image); result != nil {
		return result
	}
	v.logger.Warn().Str("image", image).Msg("No vulnerability scanners available")
	return nil
}

// scanWithTrivy performs vulnerability scanning using Trivy
func (v *ImageValidator) scanWithTrivy(ctx context.Context, image string) *VulnerabilityResult {
	startTime := time.Now()
	// Check if Trivy is available
	if err := exec.Command("trivy", "--version").Run(); err != nil {
		v.logger.Debug().Msg("Trivy not available")
		return nil
	}
	v.logger.Info().Str("image", image).Msg("Scanning image with Trivy")
	// Run Trivy scan
	cmd := exec.CommandContext(ctx, "trivy", "image", "--format", "json", "--quiet", image)
	output, err := cmd.Output()
	if err != nil {
		v.logger.Warn().Err(err).Str("image", image).Msg("Trivy scan failed")
		return nil
	}
	// Parse Trivy output
	var trivyResult TrivyResult
	if err := json.Unmarshal(output, &trivyResult); err != nil {
		v.logger.Warn().Err(err).Msg("Failed to parse Trivy output")
		return nil
	}
	// Count vulnerabilities by severity
	result := &VulnerabilityResult{
		ScanTool:     "trivy",
		ScanDuration: time.Since(startTime),
	}
	for _, res := range trivyResult.Results {
		for _, vuln := range res.Vulnerabilities {
			result.TotalVulns++
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				result.CriticalVulns++
				result.HasCriticalVulns = true
			case "HIGH":
				result.HighVulns++
			case "MEDIUM":
				result.MediumVulns++
			case "LOW":
				result.LowVulns++
			}
		}
	}
	v.logger.Info().
		Str("image", image).
		Int("total", result.TotalVulns).
		Int("critical", result.CriticalVulns).
		Int("high", result.HighVulns).
		Dur("duration", result.ScanDuration).
		Msg("Trivy scan completed")
	return result
}

// scanWithGrype performs vulnerability scanning using Grype
func (v *ImageValidator) scanWithGrype(ctx context.Context, image string) *VulnerabilityResult {
	startTime := time.Now()
	// Check if Grype is available
	if err := exec.Command("grype", "--version").Run(); err != nil {
		v.logger.Debug().Msg("Grype not available")
		return nil
	}
	v.logger.Info().Str("image", image).Msg("Scanning image with Grype")
	// Run Grype scan
	cmd := exec.CommandContext(ctx, "grype", "-o", "json", image)
	output, err := cmd.Output()
	if err != nil {
		v.logger.Warn().Err(err).Str("image", image).Msg("Grype scan failed")
		return nil
	}
	// Parse Grype output (simplified - Grype has different JSON format)
	var grypeResult map[string]interface{}
	if err := json.Unmarshal(output, &grypeResult); err != nil {
		v.logger.Warn().Err(err).Msg("Failed to parse Grype output")
		return nil
	}
	// Count vulnerabilities by severity (simplified parsing)
	result := &VulnerabilityResult{
		ScanTool:     "grype",
		ScanDuration: time.Since(startTime),
	}
	if matches, ok := grypeResult["matches"].([]interface{}); ok {
		for _, match := range matches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				result.TotalVulns++
				if vuln, ok := matchMap["vulnerability"].(map[string]interface{}); ok {
					if severity, ok := vuln["severity"].(string); ok {
						switch strings.ToUpper(severity) {
						case "CRITICAL":
							result.CriticalVulns++
							result.HasCriticalVulns = true
						case "HIGH":
							result.HighVulns++
						case "MEDIUM":
							result.MediumVulns++
						case "LOW":
							result.LowVulns++
						}
					}
				}
			}
		}
	}
	v.logger.Info().
		Str("image", image).
		Int("total", result.TotalVulns).
		Int("critical", result.CriticalVulns).
		Int("high", result.HighVulns).
		Dur("duration", result.ScanDuration).
		Msg("Grype scan completed")
	return result
}
