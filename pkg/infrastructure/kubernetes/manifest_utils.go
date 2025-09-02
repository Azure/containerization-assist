package kubernetes

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	mcperrors "github.com/Azure/containerization-assist/pkg/domain/errors"
	"github.com/Azure/containerization-assist/templates"
)

const DefaultImageAndTag = "localhost:5001/app:latest"

const SNAPSHOT_DIR_NAME = ".containerization-assist-snapshots"
const MANIFEST_DIR_NAME = "manifests"

// Path where manifests are expected to be found - uses GITHUB_WORKSPACE - requires checkout action step
var DefaultManifestAbsolutePath = filepath.Join(os.Getenv("GITHUB_WORKSPACE"), MANIFEST_DIR_NAME)

const ManifestObjectDelimiter = "---"

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
	embeddedBasePath := path.Join(MANIFEST_TEMPLATE_DIR, string(templateName))
	filesToCopy := []string{"deployment.yaml", "service.yaml", "configmap.yaml", "secret.yaml"}

	manifestsDir := filepath.Join(targetDir, MANIFEST_TARGET_DIR)
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return mcperrors.New(mcperrors.CodeIoError, "k8s", fmt.Sprintf("creating manifests directory %q: %v", manifestsDir, err), err)
	}

	for _, filename := range filesToCopy {
		embeddedPath := path.Join(embeddedBasePath, filename)
		data, err := templates.Templates.ReadFile(embeddedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return mcperrors.New(mcperrors.CodeIoError, "k8s", fmt.Sprintf("reading embedded file %q: %v", embeddedPath, err), err)
		}

		destPath := filepath.Join(manifestsDir, filename)
		updatedData := strings.ReplaceAll(string(data), DefaultImageAndTag, imageNameAndTag)
		data = []byte(updatedData)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return mcperrors.New(mcperrors.CodeIoError, "k8s", fmt.Sprintf("writing file %q: %v", destPath, err), err)
		}
	}
	return nil
}
