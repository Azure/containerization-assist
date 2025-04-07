package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Path where manifests are expected to be found - uses GITHUB_WORKSPACE - requires checkout action step
var DefaultManifestPath = filepath.Join(os.Getenv("GITHUB_WORKSPACE"), "manifests")

type K8sMetadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}
type K8sObject struct {
	ApiVersion         string      `yaml:"apiVersion"`
	Kind               string      `yaml:"kind"`
	Metadata           K8sMetadata `yaml:"metadata"`
	RawContent         []byte
	SourceManifestPath string
}

func (o K8sObject) IsDeployment() bool {
	return o.ApiVersion == "apps/v1" && o.Kind == "Deployment"
}

const ManifestObjectDelimiter = "---"

// FindKubernetesManifests locates all .yml/yaml files in the specified directory path
// If no path is provided, DefaultManifestPath will be used
// FindK8sObjects locates all .yml/.yaml files in the given path and checks if they are Kubernetes Deployments.
func FindK8sObjects(path string) ([]K8sObject, error) {
	if path == "" {
		path = DefaultManifestPath
		fmt.Printf("Using default manifest path: %s\n", path)
	}

	var k8sObjects []K8sObject

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error accessing directory %s: %v", path, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	fmt.Printf("Looking for Kubernetes manifest files in directory: %s\n", path)

	err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", filePath, err)
			}
			o, err := readK8sObjects(fileContent)
			if err != nil {
				return fmt.Errorf("reading k8s object: %w", err)
			}
			o.SourceManifestPath = filePath
			k8sObjects = append(k8sObjects, o)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking manifest directory: %v", err)
	}
	return k8sObjects, nil
}

func readK8sObjects(content []byte) (K8sObject, error) {
	var o K8sObject
	if strings.Contains(string(content), ManifestObjectDelimiter) {
		return o, fmt.Errorf("multi-object manifests are not yet supported")
	}
	err := yaml.Unmarshal(content, &o)
	if err != nil {
		return o, fmt.Errorf("unmarshaling yaml as k8s object: %w", err)
	}
	o.RawContent = content
	return o, nil
}

// InitializeManifests populates the K8sManifests field in PipelineState with manifests found in the specified path
// If path is empty, the default manifest path will be used
func InitializeManifests(state *PipelineState, path string) error {
	k8sObjects, err := FindK8sObjects(path)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}

	if len(k8sObjects) == 0 {
		fmt.Println("No Kubernetes deployment files found")
	}

	fmt.Printf("Found %d Kubernetes object files\n", len(k8sObjects))

	for _, o := range k8sObjects {
		contentStr := string(o.RawContent)
		isDeployment := o.IsDeployment()
		name := filepath.Base(o.SourceManifestPath)

		state.K8sManifests[name] = &K8sManifest{
			Name:             name,
			Content:          contentStr,
			Path:             path,
			isDeployed:       false,
			isDeploymentType: isDeployment,
		}

		fmt.Printf("Added manifest: %s of Kind %s\n", name, o.Kind)
	}

	return nil
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
