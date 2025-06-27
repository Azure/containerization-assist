package k8s

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/logger"
	"github.com/Azure/container-kit/templates"
	"sigs.k8s.io/yaml"
)

const DefaultImageAndTag = "localhost:5001/app:latest"

const SNAPSHOT_DIR_NAME = ".container-kit-snapshots"
const MANIFEST_DIR_NAME = "manifests"

// Path where manifests are expected to be found - uses GITHUB_WORKSPACE - requires checkout action step
var DefaultManifestAbsolutePath = filepath.Join(os.Getenv("GITHUB_WORKSPACE"), MANIFEST_DIR_NAME)

func (o K8sObject) IsDeployment() bool {
	return o.ApiVersion == "apps/v1" && o.Kind == "Deployment"
}

const ManifestObjectDelimiter = "---"

// FindKubernetesManifests locates all .yml/yaml files in the specified directory path
// If no path is provided, DefaultManifestPath will be used
// FindK8sObjects locates all .yml/.yaml files in the given path and checks if they are Kubernetes Deployments.
func FindK8sObjects(path string) ([]K8sObject, error) {
	if path == "" {
		path = DefaultManifestAbsolutePath
		logger.Infof("Using default manifest path: %s", path)
	}

	var k8sObjects []K8sObject

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error accessing directory %s: %v", path, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	logger.Infof("Finding Kubernetes manifest files in directory: %s", path)

	err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == SNAPSHOT_DIR_NAME {
			return filepath.SkipDir
		}

		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file %s: %w", filePath, err)
			}
			o, err := ReadK8sObjects(fileContent)
			if err != nil {
				logger.Debugf("Skipping file %s: %v", filePath, err)
				return nil // Skip files with errors instead of failing
			}

			// Validate that this is actually a Kubernetes manifest by checking required fields
			if o.Kind == "" || o.ApiVersion == "" || o.Metadata.Name == "" {
				logger.Debugf("Skipping file %s: not a valid Kubernetes manifest (missing required fields)", filePath)
				return nil
			}

			o.ManifestPath = filePath
			k8sObjects = append(k8sObjects, o)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking manifest directory: %v", err)
	}
	return k8sObjects, nil
}

func ReadK8sObjects(content []byte) (K8sObject, error) {
	var o K8sObject
	if strings.Contains(string(content), ManifestObjectDelimiter) {
		return o, fmt.Errorf("multi-object manifests are not yet supported")
	}
	err := yaml.Unmarshal(content, &o)
	if err != nil {
		return o, fmt.Errorf("unmarshaling yaml as k8s object: %w", err)
	}
	o.Content = content
	return o, nil
}

type K8sObject struct {
	ApiVersion             string      `yaml:"apiVersion"`
	Kind                   string      `yaml:"kind"`
	Metadata               K8sMetadata `yaml:"metadata"`
	Content                []byte
	ManifestPath           string
	IsSuccessfullyDeployed bool
	ErrorLog               string
}

type K8sMetadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

type ManifestsName string

const (
	ManifestsBasic ManifestsName = "manifest-basic" // Basic manifests for a deployment, service, configmap and secret
)

const MANIFEST_TEMPLATE_DIR = "manifests"
const MANIFEST_TARGET_DIR = "manifests"

func WriteManifestsFromTemplate(templateName ManifestsName, targetDir string, imageNameAndTag string) error {
	basePath := filepath.Join(MANIFEST_TEMPLATE_DIR, string(templateName))
	filesToCopy := []string{"deployment.yaml", "service.yaml", "configmap.yaml", "secret.yaml"}

	manifestsDir := filepath.Join(targetDir, MANIFEST_TARGET_DIR)
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return fmt.Errorf("creating manifests directory %q: %w", manifestsDir, err)
	}

	for _, filename := range filesToCopy {
		embeddedPath := filepath.Join(basePath, filename)
		data, err := templates.Templates.ReadFile(embeddedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logger.Debugf("Template file %s does not exist, skipping", embeddedPath)
				continue
			}
			return fmt.Errorf("reading embedded file %q: %w", embeddedPath, err)
		}

		destPath := filepath.Join(manifestsDir, filename)
		logger.Infof("Updating image name to %s in deployment manifest: %s", imageNameAndTag, destPath)
		data = []byte(strings.ReplaceAll(string(data), DefaultImageAndTag, imageNameAndTag))

		logger.Debugf("Writing manifest file: %s", destPath)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("writing file %q: %w", destPath, err)
		}
	}
	return nil
}
