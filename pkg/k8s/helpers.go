package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewDeployment returns a minimal Deployment K8sObject in YAML form.
func NewDeployment(name, image string, port int32, replicas int32, env map[string]string, cpu, memory, livenessPath, readinessPath string) *K8sObject {
	envYaml := ""
	if len(env) > 0 {
		for k, v := range env {
			envYaml += fmt.Sprintf("        - name: %s\n          value: \"%s\"\n", k, v)
		}
	}

	manifest := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: %s
          image: %s
          ports:
            - containerPort: %d
%s          resources:
            requests:
              cpu: "%s"
              memory: "%s"`, name, replicas, name, name, name, image, port, envYaml, cpu, memory)

	return &K8sObject{
		ApiVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   K8sMetadata{Name: name},
		Content:    []byte(manifest),
	}
}

// NewService returns a Service object targeting the given port.
func NewService(name string, port int32, ingress bool) *K8sObject {
	svcType := "ClusterIP"
	if ingress {
		svcType = "LoadBalancer"
	}
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  selector:
    app: %s
  type: %s
  ports:
    - port: %d
      targetPort: %d`, name, name, svcType, port, port)

	return &K8sObject{
		ApiVersion: "v1",
		Kind:       "Service",
		Metadata:   K8sMetadata{Name: name},
		Content:    []byte(manifest),
	}
}

// NewConfigMap converts env map into a ConfigMap.
func NewConfigMap(name string, env map[string]string) *K8sObject {
	var b strings.Builder
	for k, v := range env {
		b.WriteString(fmt.Sprintf("  %s: \"%s\"\n", k, v))
	}
	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-config
data:
%s`, name, b.String())

	return &K8sObject{
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   K8sMetadata{Name: name + "-config"},
		Content:    []byte(manifest),
	}
}

// WriteK8sObjectsToFiles writes the provided objects to YAML files inside dir/manifests.
func WriteK8sObjectsToFiles(objs map[string]*K8sObject, dir string) error {
	manifestsDir := filepath.Join(dir, MANIFEST_DIR_NAME)
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return fmt.Errorf("create manifests dir: %w", err)
	}
	for key, obj := range objs {
		filename := fmt.Sprintf("%s.yaml", key)
		path := filepath.Join(manifestsDir, filename)
		if err := os.WriteFile(path, obj.Content, 0644); err != nil {
			return fmt.Errorf("write manifest %s: %w", path, err)
		}
		obj.ManifestPath = path
	}
	return nil
}
