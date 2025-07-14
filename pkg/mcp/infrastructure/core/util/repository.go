// Package util provides repository-related utility functions
package util

import (
	"strings"
)

// ExtractRepoName extracts repository name from Git URL
func ExtractRepoName(repoURL string) string {
	if repoURL == "" {
		return "app"
	}

	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "app"
	}

	name := parts[len(parts)-1]
	return strings.TrimSuffix(name, ".git")
}

// ExtractDeploymentName extracts deployment name from image reference
func ExtractDeploymentName(imageRef string) string {
	if imageRef == "" {
		return "unknown-deployment"
	}

	// Extract the image name part after the last '/'
	parts := strings.Split(imageRef, "/")
	imageName := parts[len(parts)-1]

	// Remove tag if present
	if idx := strings.Index(imageName, ":"); idx != -1 {
		imageName = imageName[:idx]
	}

	// Replace underscores and dots with hyphens for valid K8s names
	deploymentName := strings.ReplaceAll(imageName, "_", "-")
	deploymentName = strings.ReplaceAll(deploymentName, ".", "-")

	return deploymentName
}
