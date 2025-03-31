package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path where manifests are expected to be found - uses GITHUB_WORKSPACE - requires checkout action step
var DefaultManifestPath = filepath.Join(os.Getenv("GITHUB_WORKSPACE"), "manifests")

// FindKubernetesManifests locates all .yml/yaml files in the specified directory path
// If no path is provided, DefaultManifestPath will be used
func FindKubernetesManifests(path string) ([]string, error) {
	// Use default path if none provided
	if path == "" {
		path = DefaultManifestPath
		fmt.Printf("Using default manifest path: %s\n", path)
	}

	var manifestPaths []string

	// Verify the path exists and is a directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error accessing directory %s: %v", path, err)
	}

	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	// Find all YAML files in the directory
	fmt.Printf("Looking for Kubernetes manifest files in directory: %s\n", path)

	err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			manifestPaths = append(manifestPaths, filePath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking manifest directory: %v", err)
	}

	return manifestPaths, nil
}

// Grabs all manifests in a "manifests" directory in the repo's root directory
func GetDefaultManifests() ([]string, error) {
	return FindKubernetesManifests("")
}

// InitializeManifests populates the K8sManifests field in PipelineState with manifests found in the specified path
// If path is empty, the default manifest path will be used
func InitializeManifests(state *PipelineState, path string) error {
	manifestPaths, err := FindKubernetesManifests(path)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	if len(manifestPaths) == 0 {
		fmt.Println("No Kubernetes manifest files found")
		return nil
	}

	fmt.Printf("Found %d Kubernetes manifest files\n", len(manifestPaths))

	for _, path := range manifestPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read manifest %s: %w", path, err)
		}

		name := filepath.Base(path)
		contentStr := string(content)
		isDeployment := strings.Contains(contentStr, "kind: Deployment")

		state.K8sManifests[name] = &K8sManifest{
			Name:             name,
			Content:          contentStr,
			isDeployed:       false,
			isDeploymentType: isDeployment,
		}

		fmt.Printf("Added manifest: %s \n", name)
	}

	return nil
}

func InitializeDefaultPathManifests(state *PipelineState) error {
	return InitializeManifests(state, "")
}
