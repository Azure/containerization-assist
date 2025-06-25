package customizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

func TestDeploymentCustomizer_CustomizeDeployment(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	customizer := NewDeploymentCustomizer(logger)

	// Create a test deployment manifest
	tempDir := t.TempDir()
	deploymentPath := filepath.Join(tempDir, "deployment.yaml")

	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: placeholder
        ports:
        - containerPort: 8080
`

	if err := os.WriteFile(deploymentPath, []byte(deploymentYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := kubernetes.CustomizeOptions{
		ImageRef:  "myregistry/myapp:v1.0.0",
		Namespace: "production",
		Replicas:  3,
		EnvVars: map[string]string{
			"ENV":  "production",
			"PORT": "8080",
		},
		Labels: map[string]string{
			"version": "v1.0.0",
			"team":    "backend",
		},
	}

	err := CustomizeDeployment(deploymentPath, opts)
	if err != nil {
		t.Fatalf("CustomizeDeployment failed: %v", err)
	}

	// Read and verify the updated manifest
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatal(err)
	}

	var deployment map[string]interface{}
	if err := yaml.Unmarshal(content, &deployment); err != nil {
		t.Fatal(err)
	}

	// Check namespace
	if metadata, ok := deployment["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "production" {
			t.Errorf("Expected namespace 'production', got '%s'", ns)
		}
	}

	// Check replicas
	if spec, ok := deployment["spec"].(map[string]interface{}); ok {
		if replicas, ok := spec["replicas"].(int); ok && replicas != 3 {
			t.Errorf("Expected replicas 3, got %d", replicas)
		}
	}
}

func TestServiceCustomizer_CustomizeService(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	customizer := NewServiceCustomizer(logger)

	// Create a test service manifest
	tempDir := t.TempDir()
	servicePath := filepath.Join(tempDir, "service.yaml")

	serviceYAML := `apiVersion: v1
kind: Service
metadata:
  name: test-app
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
`

	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := ServiceCustomizationOptions{
		ServiceType: "LoadBalancer",
		ServicePorts: []ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: 8080,
				Protocol:   "TCP",
			},
			{
				Name:       "https",
				Port:       443,
				TargetPort: 8443,
				Protocol:   "TCP",
			},
		},
		LoadBalancerIP:  "10.0.0.100",
		SessionAffinity: "ClientIP",
		Namespace:       "production",
		Labels: map[string]string{
			"environment": "prod",
		},
	}

	err := CustomizeService(servicePath, opts)
	if err != nil {
		t.Fatalf("CustomizeService failed: %v", err)
	}

	// Read and verify the updated manifest
	content, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatal(err)
	}

	var service map[string]interface{}
	if err := yaml.Unmarshal(content, &service); err != nil {
		t.Fatal(err)
	}

	// Check service type
	if spec, ok := service["spec"].(map[string]interface{}); ok {
		if serviceType, ok := spec["type"].(string); ok && serviceType != "LoadBalancer" {
			t.Errorf("Expected service type 'LoadBalancer', got '%s'", serviceType)
		}

		// Check session affinity
		if affinity, ok := spec["sessionAffinity"].(string); ok && affinity != "ClientIP" {
			t.Errorf("Expected session affinity 'ClientIP', got '%s'", affinity)
		}

		// Check loadBalancerIP
		if lbIP, ok := spec["loadBalancerIP"].(string); ok && lbIP != "10.0.0.100" {
			t.Errorf("Expected loadBalancerIP '10.0.0.100', got '%s'", lbIP)
		}

		// Check ports
		if ports, ok := spec["ports"].([]interface{}); ok && len(ports) != 2 {
			t.Errorf("Expected 2 ports, got %d", len(ports))
		}
	}
}

func TestConfigMapCustomizer_CustomizeConfigMap(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	customizer := NewConfigMapCustomizer(logger)

	// Create a test configmap manifest
	tempDir := t.TempDir()
	configMapPath := filepath.Join(tempDir, "configmap.yaml")

	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data: {}
`

	if err := os.WriteFile(configMapPath, []byte(configMapYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := kubernetes.CustomizeOptions{
		Namespace: "production",
		EnvVars: map[string]string{
			"DATABASE_URL": "postgres://localhost/myapp",
			"REDIS_URL":    "redis://localhost:6379",
			"LOG_LEVEL":    "info",
		},
		Labels: map[string]string{
			"app":     "myapp",
			"version": "v1.0.0",
		},
	}

	err := CustomizeConfigMap(configMapPath, opts)
	if err != nil {
		t.Fatalf("CustomizeConfigMap failed: %v", err)
	}

	// Read and verify the updated manifest
	content, err := os.ReadFile(configMapPath)
	if err != nil {
		t.Fatal(err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		t.Fatal(err)
	}

	// Check namespace
	if metadata, ok := configMap["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "production" {
			t.Errorf("Expected namespace 'production', got '%s'", ns)
		}
	}

	// Check data
	if data, ok := configMap["data"].(map[string]interface{}); ok {
		if dbURL, ok := data["DATABASE_URL"].(string); ok && dbURL != "postgres://localhost/myapp" {
			t.Errorf("Expected DATABASE_URL to be set correctly")
		}
		if len(data) != 3 {
			t.Errorf("Expected 3 data items, got %d", len(data))
		}
	}
}

func TestIngressCustomizer_CustomizeIngress(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	customizer := NewIngressCustomizer(logger)

	// Create a test ingress manifest
	tempDir := t.TempDir()
	ingressPath := filepath.Join(tempDir, "ingress.yaml")

	ingressYAML := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  namespace: default
spec:
  rules: []
`

	if err := os.WriteFile(ingressPath, []byte(ingressYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := IngressCustomizationOptions{
		IngressHosts: []IngressHost{
			{
				Host: "myapp.example.com",
				Paths: []IngressPath{
					{
						Path:        "/",
						PathType:    "Prefix",
						ServiceName: "myapp-service",
						ServicePort: 80,
					},
				},
			},
		},
		IngressTLS: []IngressTLS{
			{
				Hosts:      []string{"myapp.example.com"},
				SecretName: "myapp-tls",
			},
		},
		IngressClass: "nginx",
		Namespace:    "production",
		Labels: map[string]string{
			"app": "myapp",
		},
	}

	err := CustomizeIngress(ingressPath, opts)
	if err != nil {
		t.Fatalf("CustomizeIngress failed: %v", err)
	}

	// Read and verify the updated manifest
	content, err := os.ReadFile(ingressPath)
	if err != nil {
		t.Fatal(err)
	}

	var ingress map[string]interface{}
	if err := yaml.Unmarshal(content, &ingress); err != nil {
		t.Fatal(err)
	}

	// Check namespace
	if metadata, ok := ingress["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "production" {
			t.Errorf("Expected namespace 'production', got '%s'", ns)
		}
	}

	// Check spec
	if spec, ok := ingress["spec"].(map[string]interface{}); ok {
		// Check ingress class
		if ingressClassName, ok := spec["ingressClassName"].(string); ok && ingressClassName != "nginx" {
			t.Errorf("Expected ingressClassName 'nginx', got '%s'", ingressClassName)
		}

		// Check rules
		if rules, ok := spec["rules"].([]interface{}); ok && len(rules) != 1 {
			t.Errorf("Expected 1 rule, got %d", len(rules))
		}

		// Check TLS
		if tls, ok := spec["tls"].([]interface{}); ok && len(tls) != 1 {
			t.Errorf("Expected 1 TLS entry, got %d", len(tls))
		}
	}
}

func TestSecretCustomizer_CustomizeSecret(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	customizer := NewSecretCustomizer(logger)

	// Create a test secret manifest
	tempDir := t.TempDir()
	secretPath := filepath.Join(tempDir, "secret.yaml")

	secretYAML := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
data: {}
`

	if err := os.WriteFile(secretPath, []byte(secretYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := SecretCustomizationOptions{
		Namespace: "production",
		Labels: map[string]string{
			"app":  "myapp",
			"type": "credentials",
		},
	}

	err := CustomizeSecret(secretPath, opts)
	if err != nil {
		t.Fatalf("CustomizeSecret failed: %v", err)
	}

	// Read and verify the updated manifest
	content, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}

	var secret map[string]interface{}
	if err := yaml.Unmarshal(content, &secret); err != nil {
		t.Fatal(err)
	}

	// Check namespace
	if metadata, ok := secret["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "production" {
			t.Errorf("Expected namespace 'production', got '%s'", ns)
		}

		// Check labels
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			if app, ok := labels["app"].(string); ok && app != "myapp" {
				t.Errorf("Expected label app='myapp', got '%s'", app)
			}
		}
	}
}
