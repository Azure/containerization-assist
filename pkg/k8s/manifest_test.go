package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteManifestsFromTemplate_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	err := WriteManifestsFromTemplate(ManifestsBasic, tmpDir, "myrepo/myapp:1.2.3")
	if err != nil {
		t.Fatalf("failed to write manifests: %v", err)
	}

	expectedFiles := []string{"deployment.yaml", "service.yaml", "configmap.yaml", "secret.yaml"}
	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, "manifests", f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected manifest file %s to exist: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("manifest file %s is empty", path)
		}
		if info.Name() == "deployment.yaml" {
			// Check that the image name was replaced in deployment.yaml
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("failed to read deployment.yaml: %v", err)
			}
			if string(data) == "" || !strings.Contains(string(data), "myrepo/myapp:1.2.3") {
				t.Errorf("deployment.yaml does not contain the expected image name")
			}
		}
	}
}
