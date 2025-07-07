package validators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// ImageValidator validates Docker image references and configurations
type ImageValidator struct {
	*BaseValidatorImpl
	trustedRegistries []string
	deprecatedImages  map[string]string
}

// NewImageValidator creates a new image validator
func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		BaseValidatorImpl: NewBaseValidator("image", "1.0.0", []string{"docker_image", "container_image"}),
		trustedRegistries: []string{
			"docker.io",
			"registry.hub.docker.com",
			"gcr.io",
			"ghcr.io",
			"quay.io",
			"registry.gitlab.com",
		},
		deprecatedImages: map[string]string{
			"centos:6":     "CentOS 6 is EOL. Use CentOS 7 or later, or consider Rocky Linux/AlmaLinux",
			"centos:7":     "CentOS 7 is approaching EOL. Consider CentOS Stream, Rocky Linux, or AlmaLinux",
			"ubuntu:14.04": "Ubuntu 14.04 is EOL. Use Ubuntu 18.04 or later",
			"ubuntu:16.04": "Ubuntu 16.04 is EOL. Use Ubuntu 18.04 or later",
			"debian:8":     "Debian 8 (Jessie) is EOL. Use Debian 10 (Buster) or later",
			"debian:9":     "Debian 9 (Stretch) is EOL. Use Debian 10 (Buster) or later",
			"python:2":     "Python 2 is EOL. Use Python 3",
			"python:2.7":   "Python 2.7 is EOL. Use Python 3",
			"node:8":       "Node.js 8 is EOL. Use Node.js 14 or later",
			"node:10":      "Node.js 10 is EOL. Use Node.js 14 or later",
			"node:12":      "Node.js 12 is EOL. Use Node.js 14 or later",
			"java:8":       "The 'java' image is deprecated. Use 'openjdk:8' or later",
			"java:7":       "The 'java' image is deprecated and Java 7 is EOL. Use 'openjdk:8' or later",
		},
	}
}

// Validate validates image references and configurations
func (i *ImageValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := i.BaseValidatorImpl.Validate(ctx, data, options)

	// Determine validation type
	validationType := ""
	if options != nil && options.Context != nil {
		if vt, ok := options.Context["validation_type"].(string); ok {
			validationType = vt
		}
	}

	switch validationType {
	case "image_reference":
		i.validateImageReference(data, result, options)
	case "image_list":
		i.validateImageList(data, result, options)
	case "base_image":
		i.validateBaseImage(data, result, options)
	default:
		// Auto-detect validation type
		i.autoDetectAndValidate(data, result, options)
	}

	return result
}

// validateImageReference validates a single image reference
func (i *ImageValidator) validateImageReference(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	imageRef, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_IMAGE_TYPE",
			"Image reference must be a string",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return
	}

	// Parse image reference
	image := i.parseImageReference(imageRef)

	// Validate image format
	if !i.isValidImageFormat(imageRef) {
		result.AddError(core.NewError(
			"INVALID_IMAGE_FORMAT",
			fmt.Sprintf("Invalid image reference format: %s", imageRef),
			core.ErrTypeFormat,
			core.SeverityHigh,
		))
		return
	}

	// Check for latest tag
	if image.Tag == "latest" || image.Tag == "" {
		warning := core.NewWarning(
			"LATEST_TAG_USED",
			"Using 'latest' tag or no tag (defaults to latest) is not recommended for production",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Use specific version tags for reproducibility")
		result.AddWarning(warning)
	}

	// Check for deprecated images
	i.checkDeprecatedImage(image, result)

	// Validate registry trust
	if !options.ShouldSkipRule("trusted_registry") {
		i.validateRegistryTrust(image, result, options)
	}

	// Check for vulnerable base images
	if !options.ShouldSkipRule("vulnerability_check") {
		i.checkKnownVulnerabilities(image, result)
	}

	// Validate digest if present
	if image.Digest != "" {
		i.validateDigest(image.Digest, result)
	}
}

// validateImageList validates multiple image references
func (i *ImageValidator) validateImageList(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	images, ok := data.([]string)
	if !ok {
		// Try to extract from map
		if m, ok := data.(map[string]interface{}); ok {
			if imgs, ok := m["images"].([]string); ok {
				images = imgs
			}
		}
		if images == nil {
			result.AddError(core.NewError(
				"INVALID_IMAGE_LIST",
				"Image list must be a string array",
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			return
		}
	}

	// Track statistics
	tagStats := make(map[string]int)
	registryStats := make(map[string]int)

	for _, imageRef := range images {
		// Validate each image
		singleResult := i.Validate(context.Background(), imageRef, &core.ValidationOptions{
			Context: map[string]interface{}{
				"validation_type": "image_reference",
			},
		})

		// Merge results
		result.Merge(singleResult)

		// Collect stats
		image := i.parseImageReference(imageRef)
		tagStats[image.Tag]++
		registryStats[image.Registry]++
	}

	// Add summary info
	if tagStats["latest"] > 0 {
		result.AddWarning(core.NewWarning(
			"MULTIPLE_LATEST_TAGS",
			fmt.Sprintf("Found %d images using 'latest' tag", tagStats["latest"]),
		))
	}

	// Check for consistency
	if len(registryStats) > 1 {
		result.AddWarning(core.NewWarning(
			"MULTIPLE_REGISTRIES",
			fmt.Sprintf("Images are from %d different registries. Consider using a single registry for consistency", len(registryStats)),
		))
	}
}

// validateBaseImage validates base image in FROM instruction context
func (i *ImageValidator) validateBaseImage(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Extract base image info
	type BaseImageInfo struct {
		Image    string
		Stage    string
		Line     int
		Alias    string
		Platform string
	}

	var baseImage *BaseImageInfo
	switch v := data.(type) {
	case string:
		baseImage = &BaseImageInfo{Image: v}
	case *BaseImageInfo:
		baseImage = v
	case map[string]interface{}:
		baseImage = &BaseImageInfo{
			Image:    getImageStringFromMap(v, "image"),
			Stage:    getImageStringFromMap(v, "stage"),
			Line:     getImageIntFromMap(v, "line"),
			Alias:    getImageStringFromMap(v, "alias"),
			Platform: getImageStringFromMap(v, "platform"),
		}
	default:
		result.AddError(core.NewError(
			"INVALID_BASE_IMAGE_DATA",
			"Base image data format not recognized",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return
	}

	// Validate the image reference
	i.validateImageReference(baseImage.Image, result, options)

	// Additional base image specific checks
	image := i.parseImageReference(baseImage.Image)

	// Check for minimal/distroless images
	if strings.Contains(image.Name, "distroless") || strings.Contains(image.Name, "minimal") {
		result.AddSuggestion("Good choice! Using minimal/distroless base images reduces attack surface")
	}

	// Check for Alpine-specific issues
	if strings.Contains(image.Name, "alpine") {
		warning := core.NewWarning(
			"ALPINE_COMPATIBILITY",
			"Alpine Linux uses musl libc which may cause compatibility issues with some software",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Test thoroughly or consider using debian-slim if you encounter issues")
		result.AddWarning(warning)
	}

	// Check for scratch image
	if image.Name == "scratch" {
		warning := core.NewWarning(
			"SCRATCH_IMAGE",
			"Using 'scratch' as base image requires static binaries",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Ensure your application is statically compiled")
		result.AddWarning(warning)
	}

	// Platform-specific validation
	if baseImage.Platform != "" {
		i.validatePlatform(baseImage.Platform, result)
	}
}

// autoDetectAndValidate attempts to detect the validation type
func (i *ImageValidator) autoDetectAndValidate(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	switch v := data.(type) {
	case string:
		i.validateImageReference(data, result, options)
	case []string:
		i.validateImageList(data, result, options)
	case map[string]interface{}:
		// Check if it's a base image info
		if _, hasImage := v["image"]; hasImage {
			i.validateBaseImage(data, result, options)
		} else if _, hasImages := v["images"]; hasImages {
			i.validateImageList(data, result, options)
		}
	default:
		result.AddError(core.NewError(
			"UNSUPPORTED_IMAGE_DATA_TYPE",
			fmt.Sprintf("Unsupported image data type: %T", data),
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
	}
}

// Helper methods

type ParsedImage struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	Digest    string
	FullName  string
}

func (i *ImageValidator) parseImageReference(imageRef string) *ParsedImage {
	parsed := &ParsedImage{
		FullName: imageRef,
		Tag:      "latest", // Default tag
	}

	// Extract digest if present
	if idx := strings.Index(imageRef, "@"); idx > 0 {
		parsed.Digest = imageRef[idx+1:]
		imageRef = imageRef[:idx]
	}

	// Extract tag if present
	if idx := strings.LastIndex(imageRef, ":"); idx > 0 {
		// Make sure it's not a port number in registry
		afterColon := imageRef[idx+1:]
		if !strings.Contains(afterColon, "/") {
			parsed.Tag = afterColon
			imageRef = imageRef[:idx]
		}
	}

	// Parse registry and namespace
	parts := strings.Split(imageRef, "/")
	switch len(parts) {
	case 1:
		// Just image name (e.g., "nginx")
		parsed.Name = parts[0]
		parsed.Registry = "docker.io"
		parsed.Namespace = "library"
	case 2:
		// Could be namespace/image or registry/image
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			// It's a registry
			parsed.Registry = parts[0]
			parsed.Name = parts[1]
		} else {
			// It's namespace/image
			parsed.Registry = "docker.io"
			parsed.Namespace = parts[0]
			parsed.Name = parts[1]
		}
	case 3:
		// registry/namespace/image
		parsed.Registry = parts[0]
		parsed.Namespace = parts[1]
		parsed.Name = parts[2]
	}

	return parsed
}

func (i *ImageValidator) isValidImageFormat(imageRef string) bool {
	// Comprehensive regex for Docker image references
	// Supports: [registry[:port]/]namespace/image[:tag][@digest]
	imageRegex := regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))*(:[0-9]+)?\/)?([a-z0-9]+(?:[._-][a-z0-9]+)*\/)?([a-z0-9]+(?:[._-][a-z0-9]+)*)(?::([\w][\w.-]{0,127}))?(?:@([A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*:[[:xdigit:]]{32,}))?$`)
	return imageRegex.MatchString(imageRef)
}

func (i *ImageValidator) checkDeprecatedImage(image *ParsedImage, result *core.NonGenericResult) {
	// Check full image:tag combination
	fullImage := fmt.Sprintf("%s:%s", image.Name, image.Tag)
	if msg, deprecated := i.deprecatedImages[fullImage]; deprecated {
		result.AddWarning(core.NewWarning(
			"DEPRECATED_IMAGE",
			fmt.Sprintf("Image '%s' is deprecated: %s", fullImage, msg),
		))
		return
	}

	// Check image name without tag for generic deprecations
	if msg, deprecated := i.deprecatedImages[image.Name]; deprecated {
		result.AddWarning(core.NewWarning(
			"DEPRECATED_IMAGE",
			fmt.Sprintf("Image '%s' is deprecated: %s", image.Name, msg),
		))
	}
}

func (i *ImageValidator) validateRegistryTrust(image *ParsedImage, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Check if custom trusted registries are provided
	trustedRegistries := i.trustedRegistries
	if options != nil && options.Context != nil {
		if customRegistries, ok := options.Context["trusted_registries"].([]string); ok {
			trustedRegistries = customRegistries
		}
	}

	// Check if registry is trusted
	trusted := false
	for _, registry := range trustedRegistries {
		if image.Registry == registry || strings.HasSuffix(image.Registry, "."+registry) {
			trusted = true
			break
		}
	}

	if !trusted {
		warning := core.NewWarning(
			"UNTRUSTED_REGISTRY",
			fmt.Sprintf("Image is from untrusted registry: %s", image.Registry),
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Consider using images from trusted registries or add this registry to trusted list")
		result.AddWarning(warning)
	}

	// Check for insecure registries (non-HTTPS)
	if !strings.Contains(image.Registry, ".") && image.Registry != "localhost" {
		result.AddWarning(core.NewWarning(
			"POSSIBLE_INSECURE_REGISTRY",
			fmt.Sprintf("Registry '%s' may be using insecure HTTP", image.Registry),
		))
	}
}

func (i *ImageValidator) checkKnownVulnerabilities(image *ParsedImage, result *core.NonGenericResult) {
	// Known vulnerable images (simplified check)
	vulnerableImages := map[string]string{
		"elasticsearch:1":    "Elasticsearch 1.x has known vulnerabilities",
		"elasticsearch:2":    "Elasticsearch 2.x has known vulnerabilities",
		"jenkins:1":          "Jenkins 1.x has known vulnerabilities",
		"wordpress:4":        "WordPress 4.x may have vulnerabilities. Use latest 5.x or 6.x",
		"drupal:7":           "Drupal 7 has reached end of life",
		"ghost:0":            "Ghost 0.x has known vulnerabilities",
		"redmine:2":          "Redmine 2.x has known vulnerabilities",
		"gitlab/gitlab-ce:8": "GitLab 8.x has known vulnerabilities",
		"mysql:5.5":          "MySQL 5.5 has reached end of life",
		"postgres:9":         "PostgreSQL 9.x has reached end of life",
		"mongo:2":            "MongoDB 2.x has known vulnerabilities",
		"redis:2":            "Redis 2.x has known vulnerabilities",
		"memcached:1.4":      "Memcached 1.4.x has known vulnerabilities",
		"nginx:1.10":         "nginx 1.10 has reached end of life",
		"httpd:2.2":          "Apache httpd 2.2 has reached end of life",
	}

	imageWithTag := fmt.Sprintf("%s:%s", image.Name, image.Tag)
	for pattern, message := range vulnerableImages {
		if strings.HasPrefix(imageWithTag, pattern) {
			result.AddError(core.NewError(
				"VULNERABLE_IMAGE",
				fmt.Sprintf("Image '%s' has known vulnerabilities: %s", imageWithTag, message),
				core.ErrTypeSecurity,
				core.SeverityHigh,
			))
			break
		}
	}
}

func (i *ImageValidator) validateDigest(digest string, result *core.NonGenericResult) {
	// Validate digest format (algorithm:hex)
	digestRegex := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*:[[:xdigit:]]{32,}$`)
	if !digestRegex.MatchString(digest) {
		result.AddError(core.NewError(
			"INVALID_DIGEST_FORMAT",
			fmt.Sprintf("Invalid digest format: %s", digest),
			core.ErrTypeFormat,
			core.SeverityMedium,
		))
		return
	}

	// Check for weak algorithms
	if strings.HasPrefix(digest, "sha1:") || strings.HasPrefix(digest, "md5:") {
		result.AddWarning(core.NewWarning(
			"WEAK_DIGEST_ALGORITHM",
			"Using weak digest algorithm. Consider using sha256 or stronger",
		))
	}
}

func (i *ImageValidator) validatePlatform(platform string, result *core.NonGenericResult) {
	// Validate platform format
	platformRegex := regexp.MustCompile(`^[a-z0-9]+(?:/[a-z0-9]+)?(?:/v[0-9]+)?$`)
	if !platformRegex.MatchString(platform) {
		result.AddError(core.NewError(
			"INVALID_PLATFORM_FORMAT",
			fmt.Sprintf("Invalid platform format: %s", platform),
			core.ErrTypeFormat,
			core.SeverityMedium,
		))
		return
	}

	// Check for common platforms
	knownPlatforms := []string{
		"linux/amd64", "linux/arm64", "linux/arm/v7", "linux/arm/v6",
		"linux/386", "linux/ppc64le", "linux/s390x", "linux/mips64le",
		"windows/amd64", "windows/386",
		"darwin/amd64", "darwin/arm64",
	}

	known := false
	for _, kp := range knownPlatforms {
		if platform == kp {
			known = true
			break
		}
	}

	if !known {
		result.AddWarning(core.NewWarning(
			"UNCOMMON_PLATFORM",
			fmt.Sprintf("Platform '%s' is not commonly used. Ensure build support exists", platform),
		))
	}
}

// SetTrustedRegistries updates the list of trusted registries
func (i *ImageValidator) SetTrustedRegistries(registries []string) {
	i.trustedRegistries = registries
}

// AddTrustedRegistry adds a registry to the trusted list
func (i *ImageValidator) AddTrustedRegistry(registry string) {
	i.trustedRegistries = append(i.trustedRegistries, registry)
}

// Helper functions - renamed to avoid conflict
func getImageStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getImageIntFromMap(m map[string]interface{}, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
