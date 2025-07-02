package utils

import (
	"strings"
	"testing"
)

func TestSecretScannerScanEnvironment(t *testing.T) {
	scanner := NewSecretScanner()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected []string // Expected sensitive variable names
	}{
		{
			name: "detect passwords",
			envVars: map[string]string{
				"DB_PASSWORD":    "secret123",
				"API_PASSWORD":   "token456",
				"USER_PASSWORD":  "pass789",
				"MYSQL_PASSWORD": "mysqlpass",
				"PASSWORD":       "simplepass",
			},
			expected: []string{"DB_PASSWORD", "API_PASSWORD", "USER_PASSWORD", "MYSQL_PASSWORD", "PASSWORD"},
		},
		{
			name: "detect tokens",
			envVars: map[string]string{
				"API_TOKEN":    "tok123",
				"ACCESS_TOKEN": "acc456",
				"GITHUB_TOKEN": "gh789",
				"AUTH_TOKEN":   "auth123",
				"TOKEN":        "simple",
			},
			expected: []string{"API_TOKEN", "ACCESS_TOKEN", "GITHUB_TOKEN", "AUTH_TOKEN", "TOKEN"},
		},
		{
			name: "detect API keys",
			envVars: map[string]string{
				"API_KEY":        "key123",
				"STRIPE_API_KEY": "sk_test_123",
				"AWS_ACCESS_KEY": "AKIA123",
				"SECRET_KEY":     "sec456",
			},
			expected: []string{"API_KEY", "STRIPE_API_KEY", "AWS_ACCESS_KEY", "SECRET_KEY"},
		},
		{
			name: "detect cloud provider secrets",
			envVars: map[string]string{
				"AWS_SECRET_ACCESS_KEY": "aws123",
				"AZURE_CLIENT_SECRET":   "azure456",
				"GCP_SERVICE_KEY":       "gcp789",
				"GOOGLE_API_KEY":        "google123",
			},
			expected: []string{"AWS_SECRET_ACCESS_KEY", "AZURE_CLIENT_SECRET", "GCP_SERVICE_KEY", "GOOGLE_API_KEY"},
		},
		{
			name: "detect database credentials",
			envVars: map[string]string{
				"DB_CONNECTION_STRING":      "postgres://user:pass@host",
				"DATABASE_URL":              "mysql://root:pass@localhost",
				"MONGODB_CONNECTION_STRING": "mongodb://user:pass@cluster",
			},
			expected: []string{"DB_CONNECTION_STRING", "DATABASE_URL", "MONGODB_CONNECTION_STRING"},
		},
		{
			name: "detect certificates",
			envVars: map[string]string{
				"TLS_CERT":        "-----BEGIN CERTIFICATE-----",
				"PRIVATE_KEY":     "-----BEGIN PRIVATE KEY-----",
				"SSL_CERTIFICATE": "cert_content",
			},
			expected: []string{"TLS_CERT", "PRIVATE_KEY", "SSL_CERTIFICATE"},
		},
		{
			name: "mixed with non-sensitive",
			envVars: map[string]string{
				"APP_NAME":    "myapp",
				"PORT":        "8080",
				"DB_PASSWORD": "secret",
				"LOG_LEVEL":   "info",
				"API_TOKEN":   "token123",
				"DEBUG":       "true",
			},
			expected: []string{"DB_PASSWORD", "API_TOKEN"},
		},
		{
			name: "no sensitive vars",
			envVars: map[string]string{
				"APP_NAME":  "myapp",
				"PORT":      "8080",
				"LOG_LEVEL": "info",
				"NODE_ENV":  "production",
				"VERSION":   "1.0.0",
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			detected := scanner.ScanEnvironment(tt.envVars)

			// Create map of detected names for easy lookup
			detectedNames := make(map[string]bool)
			for _, d := range detected {
				detectedNames[d.Name] = true
			}

			// Check all expected were detected
			for _, expected := range tt.expected {
				if !detectedNames[expected] {
					t.Errorf("Expected to detect %s but didn't", expected)
				}
			}

			// Check no unexpected detections
			if len(detected) != len(tt.expected) {
				t.Errorf("Expected %d detections, got %d", len(tt.expected), len(detected))
				for _, d := range detected {
					t.Logf("Detected: %s (pattern: %s)", d.Name, d.Pattern)
				}
			}
		})
	}
}

func TestSecretScannerRedaction(t *testing.T) {
	scanner := NewSecretScanner()

	envVars := map[string]string{
		"DB_PASSWORD": "supersecretpassword123",
		"API_KEY":     "sk_test_abcdef123456",
		"SHORT":       "abc",
	}

	detected := scanner.ScanEnvironment(envVars)

	// Check redaction
	for _, d := range detected {
		switch d.Name {
		case "DB_PASSWORD":
			if d.Redacted != "su***23" {
				t.Errorf("Expected 'su***23', got %s", d.Redacted)
			}
		case "API_KEY":
			if d.Redacted != "sk***56" {
				t.Errorf("Expected 'sk***56', got %s", d.Redacted)
			}
		case "SHORT":
			if d.Redacted != "***" {
				t.Errorf("Expected '***', got %s", d.Redacted)
			}
		}
	}
}

func TestSecretScannerSuggestedNames(t *testing.T) {
	scanner := NewSecretScanner()

	tests := []struct {
		envVar   string
		expected string
	}{
		{"DB_PASSWORD", "app-db-secrets"},
		{"API_TOKEN", "app-api-secrets"},
		{"STRIPE_API_KEY", "app-stripe-api-secrets"},
		{"AUTH_SECRET", "app-auth-secrets"},
		{"MYSQL_PASSWORD", "app-mysql-secrets"},
		{"SIMPLE_SECRET", "app-simple-secrets"},
	}

	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			envVars := map[string]string{tt.envVar: "value"}
			detected := scanner.ScanEnvironment(envVars)

			if len(detected) != 1 {
				t.Fatalf("Expected 1 detection, got %d", len(detected))
			}

			if detected[0].SuggestedName != tt.expected {
				t.Errorf("Expected suggested name %s, got %s", tt.expected, detected[0].SuggestedName)
			}
		})
	}
}

func TestSecretScannerCreateExternalizationPlan(t *testing.T) {
	scanner := NewSecretScanner()

	envVars := map[string]string{
		"APP_NAME":    "myapp",
		"PORT":        "8080",
		"DB_PASSWORD": "secret123",
		"API_TOKEN":   "token456",
		"LOG_LEVEL":   "info",
		"AWS_SECRET":  "aws789",
	}

	plan := scanner.CreateExternalizationPlan(envVars, "kubernetes-secrets")

	// Check detected secrets
	if len(plan.DetectedSecrets) != 3 {
		t.Errorf("Expected 3 detected secrets, got %d", len(plan.DetectedSecrets))
	}

	// Check secret references
	if len(plan.SecretReferences) != 3 {
		t.Errorf("Expected 3 secret references, got %d", len(plan.SecretReferences))
	}

	// Check ConfigMap entries (non-sensitive)
	if len(plan.ConfigMapEntries) != 3 {
		t.Errorf("Expected 3 ConfigMap entries, got %d", len(plan.ConfigMapEntries))
	}

	// Verify non-sensitive vars are in ConfigMap
	if plan.ConfigMapEntries["APP_NAME"] != "myapp" {
		t.Error("APP_NAME should be in ConfigMap")
	}
	if plan.ConfigMapEntries["PORT"] != "8080" {
		t.Error("PORT should be in ConfigMap")
	}
	if plan.ConfigMapEntries["LOG_LEVEL"] != "info" {
		t.Error("LOG_LEVEL should be in ConfigMap")
	}

	// Verify sensitive vars are NOT in ConfigMap
	if _, exists := plan.ConfigMapEntries["DB_PASSWORD"]; exists {
		t.Error("DB_PASSWORD should NOT be in ConfigMap")
	}
}

func TestSecretScannerGetRecommendedManager(t *testing.T) {
	scanner := NewSecretScanner()

	tests := []struct {
		name          string
		hasGitOps     bool
		cloudProvider string
		expected      string
	}{
		{"GitOps enabled", true, "", "sealed-secrets"},
		{"AWS provider", false, "aws", "external-secrets"},
		{"Azure provider", false, "azure", "external-secrets"},
		{"GCP provider", false, "gcp", "external-secrets"},
		{"Default", false, "", "kubernetes-secrets"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.GetRecommendedManager(tt.hasGitOps, tt.cloudProvider)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSecretScannerGenerateSecretManifest(t *testing.T) {
	scanner := NewSecretScanner()

	secrets := map[string]string{
		"DB_PASSWORD": "secret123",
		"API_TOKEN":   "token456",
	}

	manifest := scanner.GenerateSecretManifest("app-secrets", secrets, "production")

	// Check manifest contains required fields
	if !strings.Contains(manifest, "apiVersion: v1") {
		t.Error("Missing apiVersion")
	}
	if !strings.Contains(manifest, "kind: Secret") {
		t.Error("Missing kind")
	}
	if !strings.Contains(manifest, "name: app-secrets") {
		t.Error("Missing name")
	}
	if !strings.Contains(manifest, "namespace: production") {
		t.Error("Missing namespace")
	}
	if !strings.Contains(manifest, "type: Opaque") {
		t.Error("Missing type")
	}
	if !strings.Contains(manifest, "stringData:") {
		t.Error("Missing stringData")
	}

	// Check keys are lowercased
	if !strings.Contains(manifest, "db_password:") {
		t.Error("Missing db_password key")
	}
	if !strings.Contains(manifest, "api_token:") {
		t.Error("Missing api_token key")
	}

	// Check deterministic dummy values
	if !strings.Contains(manifest, "dummy-password-123") {
		t.Error("Missing dummy password value")
	}
	if !strings.Contains(manifest, "dummy-token-456") {
		t.Error("Missing dummy token value")
	}
}

func TestSecretScannerGenerateExternalSecretManifest(t *testing.T) {
	scanner := NewSecretScanner()

	mappings := map[string]string{
		"db_password": "prod/db/password",
		"api_token":   "prod/api/token",
	}

	manifest := scanner.GenerateExternalSecretManifest("app-secrets", "production", "vault-backend", mappings)

	// Check manifest structure
	if !strings.Contains(manifest, "apiVersion: external-secrets.io/v1beta1") {
		t.Error("Missing apiVersion")
	}
	if !strings.Contains(manifest, "kind: ExternalSecret") {
		t.Error("Missing kind")
	}
	if !strings.Contains(manifest, "name: app-secrets") {
		t.Error("Missing name")
	}
	if !strings.Contains(manifest, "namespace: production") {
		t.Error("Missing namespace")
	}

	// Check secret store reference
	if !strings.Contains(manifest, "secretStoreRef:") {
		t.Error("Missing secretStoreRef")
	}
	if !strings.Contains(manifest, "name: vault-backend") {
		t.Error("Missing secret store name")
	}

	// Check data mappings
	if !strings.Contains(manifest, "secretKey: db_password") {
		t.Error("Missing db_password mapping")
	}
	if !strings.Contains(manifest, "key: prod/db/password") {
		t.Error("Missing remote ref for db_password")
	}
}
