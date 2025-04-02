package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Path where manifests are expected to be found - uses GITHUB_WORKSPACE - requires checkout action step
var DefaultManifestPath = filepath.Join(os.Getenv("GITHUB_WORKSPACE"), "manifests")

// FindKubernetesManifests locates all .yml/yaml files in the specified directory path
// If no path is provided, DefaultManifestPath will be used
// FindAndCheckK8sDeployments locates all .yml/.yaml files in the given path and checks if they are Kubernetes Deployments.
func FindAndCheckK8sDeployments(path string) ([]string, []string, error) {
	if path == "" {
		path = DefaultManifestPath
		fmt.Printf("Using default manifest path: %s\n", path)
	}

	var manifestPaths []string
	var deploymentFiles []string

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("error accessing directory %s: %v", path, err)
	}
	if !fileInfo.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a directory", path)
	}

	fmt.Printf("Looking for Kubernetes manifest files in directory: %s\n", path)

	err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			manifestPaths = append(manifestPaths, filePath)
			if isK8sDeployment(filePath) {
				deploymentFiles = append(deploymentFiles, filePath)
			}
		}
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("error walking manifest directory: %v", err)
	}

	return manifestPaths, deploymentFiles, nil
}

// isK8sDeployment checks if a given YAML file is a Kubernetes Deployment (supports multiple documents per file).
func isK8sDeployment(filePath string) bool {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return false
	}

	var documents []map[string]interface{}
	if err := yaml.Unmarshal(data, &documents); err != nil {
		log.Printf("Error parsing YAML: %v", err)
		return false
	}

	for _, doc := range documents {
		if doc != nil {
			u := &unstructured.Unstructured{Object: doc}
			if u.GetKind() == "Deployment" && u.GetAPIVersion() != "" && doc["spec"] != nil {
				return true
			}
		}
	}
	return false
}

// Grabs all manifests in a "manifests" directory in the repo's root directory
func GetDefaultManifests() ([]string, []string, error) {
	return FindAndCheckK8sDeployments("")
}

// InitializeManifests populates the K8sManifests field in PipelineState with manifests found in the specified path
// If path is empty, the default manifest path will be used
func InitializeManifests(state *PipelineState, path string) error {
	manifestPaths, deploymentFiles, err := FindAndCheckK8sDeployments(path)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	if len(deploymentFiles) == 0 {
		fmt.Println("No Kubernetes deployment files found")
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
			Path:             path,
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

// FormatManifestErrors returns a string containing all manifest errors with their names
func FormatManifestErrors(state *PipelineState) string {
	var errorBuilder strings.Builder

	for name, manifest := range state.K8sManifests {
		if manifest.errorLog != "" {
			errorBuilder.WriteString(fmt.Sprintf("\nManifest %q:\n%s\n", name, manifest.errorLog))
		}
	}

	return errorBuilder.String()
}

// GetPendingManifests returns a map of manifest names that still need to be deployed
func GetPendingManifests(state *PipelineState) map[string]bool {
	pendingManifests := make(map[string]bool)

	for name, manifest := range state.K8sManifests {
		if !manifest.isDeployed {
			pendingManifests[name] = true
		}
	}

	return pendingManifests
}
