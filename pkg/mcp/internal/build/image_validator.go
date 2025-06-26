package build

import (
	"fmt"
	"strings"

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
	// TODO: Integrate with actual vulnerability database (e.g., CVE database, Trivy, Grype)
	// Currently checking against hardcoded list of known outdated versions
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

	// TODO: Parse COPY --from instructions to accurately detect stage references
	// Currently marking all named stages as potentially referenced (conservative approach)
	for _, img := range images {
		if img.StageName != "" {
			references[img.StageName] = true
		}
	}

	return references
}
