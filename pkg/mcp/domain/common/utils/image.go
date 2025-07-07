// Package utils - Docker image reference utilities
// This file consolidates image processing functions from across pkg/mcp
package utils

import (
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ImageReference represents a parsed Docker image reference
type ImageReference struct {
	Registry   string `json:"registry,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Repository string `json:"repository"`
	Tag        string `json:"tag,omitempty"`
	Digest     string `json:"digest,omitempty"`
	Full       string `json:"full"`
}

// Constants for image validation
const (
	DefaultRegistry = "docker.io"
	DefaultTag      = "latest"
	LatestTag       = "latest"
)

// Common registries
var (
	KnownRegistries = []string{
		"docker.io",
		"registry-1.docker.io",
		"gcr.io",
		"us.gcr.io",
		"eu.gcr.io",
		"asia.gcr.io",
		"quay.io",
		"ghcr.io",
		"mcr.microsoft.com",
	}
)

// Regex patterns for image validation (consolidated from multiple files)
var (
	// Basic image name pattern
	imageNamePattern = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$`)

	// Tag pattern
	tagPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

	// Digest pattern (SHA256)
	digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

	// Registry hostname pattern
	registryPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](\:[0-9]+)?$`)

	// Full image reference pattern
	fullImagePattern = regexp.MustCompile(`^(?:([a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](?::[0-9]+)?)/)?([a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*)(?::((?:[a-zA-Z0-9][a-zA-Z0-9._/-]*)|latest))?(?:@(sha256:[a-f0-9]{64}))?$`)
)

// ParseImageReference parses a Docker image reference into components
// Consolidates logic from push_image.go, build_image.go, tag_image_atomic.go
func ParseImageReference(imageRef string) (*ImageReference, error) {
	if imageRef == "" {
		return nil, errors.NewError().Messagef("image reference cannot be empty").WithLocation(

		// Normalize the image reference
		).Build()
	}

	imageRef = strings.TrimSpace(imageRef)

	ref := &ImageReference{
		Full: imageRef,
	}

	// Check for digest first
	if strings.Contains(imageRef, "@") {
		parts := strings.Split(imageRef, "@")
		if len(parts) != 2 {
			return nil, errors.NewError().Messagef("invalid digest format in image reference: %s", imageRef).WithLocation().Build()
		}
		imageRef = parts[0]
		ref.Digest = parts[1]

		if !digestPattern.MatchString(ref.Digest) {
			return nil, errors.NewError().Messagef("invalid digest format: %s", ref.Digest).WithLocation(

			// Check for tag
			).Build()
		}
	}

	if strings.Contains(imageRef, ":") {
		parts := strings.Split(imageRef, ":")
		if len(parts) < 2 {
			return nil, errors.NewError().Messagef("invalid tag format in image reference: %s", imageRef).WithLocation(

			// The last part is the tag, join the rest back
			).Build()
		}

		ref.Tag = parts[len(parts)-1]
		imageRef = strings.Join(parts[:len(parts)-1], ":")

		if !tagPattern.MatchString(ref.Tag) {
			return nil, errors.NewError().Messagef("invalid tag format: %s", ref.Tag).WithLocation().Build()
		}
	} else if ref.Digest == "" {
		ref.Tag = DefaultTag
	}

	// Parse registry and repository
	if strings.Contains(imageRef, "/") {
		parts := strings.Split(imageRef, "/")

		// Check if first part looks like a registry (contains . or :)
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			ref.Registry = parts[0]
			imageRef = strings.Join(parts[1:], "/")
		} else {
			ref.Registry = DefaultRegistry
		}

		// Handle namespace/repository structure
		repoParts := strings.Split(imageRef, "/")
		if len(repoParts) >= 2 {
			ref.Namespace = strings.Join(repoParts[:len(repoParts)-1], "/")
			ref.Repository = repoParts[len(repoParts)-1]
		} else {
			ref.Repository = imageRef
		}
	} else {
		ref.Registry = DefaultRegistry
		ref.Repository = imageRef
	}

	// Validate components
	if err := ref.Validate(); err != nil {
		return nil, err
	}

	return ref, nil
}

// Validate validates the image reference components
func (r *ImageReference) Validate() error {
	if r.Repository == "" {
		return errors.NewError().Messagef("repository name cannot be empty").WithLocation().Build()
	}

	if r.Registry != "" && !registryPattern.MatchString(r.Registry) {
		return errors.NewError().Messagef("invalid registry format: %s", r.Registry).WithLocation(

		// Validate repository name (without registry)
		).Build()
	}

	fullRepo := r.Repository
	if r.Namespace != "" {
		fullRepo = r.Namespace + "/" + r.Repository
	}

	if !imageNamePattern.MatchString(fullRepo) {
		return errors.NewError().Messagef("invalid repository name format: %s", fullRepo).WithLocation().Build()
	}

	if r.Tag != "" && !tagPattern.MatchString(r.Tag) {
		return errors.NewError().Messagef("invalid tag format: %s", r.Tag).WithLocation().Build()
	}

	if r.Digest != "" && !digestPattern.MatchString(r.Digest) {
		return errors.NewError().Messagef("invalid digest format: %s", r.Digest).WithLocation().Build(

		// String returns the full image reference
		)
	}

	return nil
}

func (r *ImageReference) String() string {
	var parts []string

	// Add registry
	if r.Registry != "" && r.Registry != DefaultRegistry {
		parts = append(parts, r.Registry)
	}

	// Add namespace and repository
	if r.Namespace != "" {
		parts = append(parts, r.Namespace+"/"+r.Repository)
	} else {
		parts = append(parts, r.Repository)
	}

	result := strings.Join(parts, "/")

	// Add tag
	if r.Tag != "" && r.Tag != DefaultTag {
		result += ":" + r.Tag
	}

	// Add digest
	if r.Digest != "" {
		result += "@" + r.Digest
	}

	return result
}

// WithTag returns a new image reference with the specified tag
func (r *ImageReference) WithTag(tag string) *ImageReference {
	newRef := *r
	newRef.Tag = tag
	newRef.Digest = "" // Clear digest when setting tag
	newRef.Full = newRef.String()
	return &newRef
}

// WithDigest returns a new image reference with the specified digest
func (r *ImageReference) WithDigest(digest string) *ImageReference {
	newRef := *r
	newRef.Digest = digest
	newRef.Tag = "" // Clear tag when setting digest
	newRef.Full = newRef.String()
	return &newRef
}

// GetRegistryURL returns the registry URL with protocol
func (r *ImageReference) GetRegistryURL() string {
	if r.Registry == "" || r.Registry == DefaultRegistry {
		return "https://index.docker.io/v1/"
	}

	// Check if it already has a protocol
	if strings.HasPrefix(r.Registry, "http://") || strings.HasPrefix(r.Registry, "https://") {
		return r.Registry
	}

	return "https://" + r.Registry
}

// Utility functions (consolidated from multiple files)

// NormalizeImageReference normalizes an image reference to a canonical form
func NormalizeImageReference(imageRef string) (string, error) {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return "", err
	}

	return parsed.String(), nil
}

// ExtractRegistry extracts the registry from an image reference
func ExtractRegistry(imageRef string) string {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return DefaultRegistry
	}

	if parsed.Registry == "" {
		return DefaultRegistry
	}

	return parsed.Registry
}

// ExtractRepository extracts the repository name from an image reference
func ExtractRepository(imageRef string) string {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return ""
	}

	if parsed.Namespace != "" {
		return parsed.Namespace + "/" + parsed.Repository
	}

	return parsed.Repository
}

// ExtractTag extracts the tag from an image reference
func ExtractTag(imageRef string) string {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return DefaultTag
	}

	if parsed.Tag == "" {
		return DefaultTag
	}

	return parsed.Tag
}

// ValidateImageReference validates a Docker image reference
func ValidateImageReference(imageRef string) error {
	_, err := ParseImageReference(imageRef)
	return err
}

// IsValidImageReference checks if an image reference is valid
func IsValidImageReference(imageRef string) bool {
	return ValidateImageReference(imageRef) == nil
}

// SanitizeImageReference creates a valid image reference from user input
func SanitizeImageReference(imageRef string) string {
	// Remove extra whitespace
	imageRef = strings.TrimSpace(imageRef)

	// Remove any invalid characters
	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9._/-:@]`)
	imageRef = invalidChars.ReplaceAllString(imageRef, "")

	// Ensure it has a tag if no digest
	if !strings.Contains(imageRef, ":") && !strings.Contains(imageRef, "@") {
		imageRef += ":" + DefaultTag
	}

	return imageRef
}

// BuildFullImageReference builds a full image reference from components
func BuildFullImageReference(registry, namespace, repository, tag string) string {
	var parts []string

	if registry != "" && registry != DefaultRegistry {
		parts = append(parts, registry)
	}

	if namespace != "" {
		parts = append(parts, namespace+"/"+repository)
	} else {
		parts = append(parts, repository)
	}

	result := strings.Join(parts, "/")

	if tag != "" && tag != DefaultTag {
		result += ":" + tag
	}

	return result
}

// IsOfficialImage checks if the image is an official Docker Hub image
func IsOfficialImage(imageRef string) bool {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return false
	}

	// Official images have no namespace and use docker.io registry
	return parsed.Registry == DefaultRegistry && parsed.Namespace == ""
}

// IsLatestTag checks if the image uses the latest tag
func IsLatestTag(imageRef string) bool {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return false
	}

	return parsed.Tag == LatestTag || parsed.Tag == ""
}

// HasDigest checks if the image reference includes a digest
func HasDigest(imageRef string) bool {
	parsed, err := ParseImageReference(imageRef)
	if err != nil {
		return false
	}

	return parsed.Digest != ""
}

// GetImageSize returns a placeholder for image size (would need Docker API)
func GetImageSize(imageRef string) (int64, error) {
	// This would require Docker API integration
	// Placeholder for future implementation
	return 0, errors.NewError().Messagef("image size lookup not implemented").WithLocation(

	// CompareImageReferences compares two image references for equality
	).Build()
}

func CompareImageReferences(ref1, ref2 string) bool {
	parsed1, err1 := ParseImageReference(ref1)
	parsed2, err2 := ParseImageReference(ref2)

	if err1 != nil || err2 != nil {
		return false
	}

	return parsed1.String() == parsed2.String()
}
