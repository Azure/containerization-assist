package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestSecretGenerator_GenerateSecret(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	tests := []struct {
		name    string
		options SecretOptions
		wantErr bool
		verify  func(t *testing.T, result *SecretGenerationResult)
	}{
		{
			name: "opaque secret with string data",
			options: SecretOptions{
				Name:      "test-secret",
				Namespace: "default",
				Type:      SecretTypeOpaque,
				StringData: map[string]string{
					"username": "admin",
					"password": "secret123",
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				assert.Equal(t, "test-secret", result.Secret.Metadata.Name)
				assert.Equal(t, "default", result.Secret.Metadata.Namespace)
				assert.Equal(t, string(SecretTypeOpaque), result.Secret.Type)
				assert.Equal(t, "admin", result.Secret.StringData["username"])
				assert.Equal(t, "secret123", result.Secret.StringData["password"])
			},
		},
		{
			name: "opaque secret with binary data",
			options: SecretOptions{
				Name:      "binary-secret",
				Namespace: "test",
				Type:      SecretTypeOpaque,
				Data: map[string][]byte{
					"data.bin": {0x01, 0x02, 0x03, 0x04},
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				expected := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04})
				assert.Equal(t, expected, result.Secret.Data["data.bin"])
			},
		},
		{
			name: "docker registry secret",
			options: SecretOptions{
				Name:      "docker-secret",
				Namespace: "default",
				Type:      SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"docker.io":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				assert.Equal(t, string(SecretTypeDockerConfigJson), result.Secret.Type)
				assert.NotEmpty(t, result.Secret.Data[".dockerconfigjson"])
			},
		},
		{
			name: "basic auth secret",
			options: SecretOptions{
				Name:      "basic-auth",
				Namespace: "default",
				Type:      SecretTypeBasicAuth,
				StringData: map[string]string{
					"username": "testuser",
					"password": "testpass",
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				assert.Equal(t, string(SecretTypeBasicAuth), result.Secret.Type)
				assert.Equal(t, "testuser", result.Secret.StringData["username"])
				assert.Equal(t, "testpass", result.Secret.StringData["password"])
			},
		},
		{
			name: "TLS secret",
			options: SecretOptions{
				Name:      "tls-secret",
				Namespace: "default",
				Type:      SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIC..."),
					"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nMIIE..."),
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				assert.Equal(t, string(SecretTypeTLS), result.Secret.Type)
				assert.NotEmpty(t, result.Secret.Data["tls.crt"])
				assert.NotEmpty(t, result.Secret.Data["tls.key"])
			},
		},
		{
			name: "secret with labels and annotations",
			options: SecretOptions{
				Name:      "labeled-secret",
				Namespace: "default",
				Type:      SecretTypeOpaque,
				Labels: map[string]string{
					"app": "myapp",
					"env": "prod",
				},
				Annotations: map[string]string{
					"description": "Production credentials",
				},
				StringData: map[string]string{
					"key": "value",
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.Secret)
				assert.Equal(t, "myapp", result.Secret.Metadata.Labels["app"])
				assert.Equal(t, "prod", result.Secret.Metadata.Labels["env"])
				assert.Equal(t, "containerization-assist", result.Secret.Metadata.Labels["kubernetes.azure.com/generator"])
				assert.Equal(t, "Production credentials", result.Secret.Metadata.Annotations["description"])
			},
		},
		{
			name: "missing name validation error",
			options: SecretOptions{
				Type: SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.Error)
				assert.Equal(t, "validation_error", result.Error.Type)
				assert.Contains(t, result.Error.Message, "name is required")
			},
		},
		{
			name: "invalid name format",
			options: SecretOptions{
				Name: "Invalid_Name",
				Type: SecretTypeOpaque,
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.Error)
				assert.Equal(t, "validation_error", result.Error.Type)
				assert.Contains(t, result.Error.Message, "invalid secret name")
			},
		},
		{
			name: "missing docker config data",
			options: SecretOptions{
				Name: "docker-invalid",
				Type: SecretTypeDockerConfigJson,
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.Error)
				assert.Equal(t, "processing_error", result.Error.Type)
				assert.Contains(t, result.Error.Message, ".dockerconfigjson data")
			},
		},
		{
			name: "missing basic auth credentials",
			options: SecretOptions{
				Name:       "basic-invalid",
				Type:       SecretTypeBasicAuth,
				StringData: map[string]string{},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.Error)
				assert.Equal(t, "processing_error", result.Error.Type)
				assert.Contains(t, result.Error.Message, "username and password")
			},
		},
		{
			name: "missing TLS data",
			options: SecretOptions{
				Name: "tls-invalid",
				Type: SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("cert"),
					// Missing tls.key
				},
			},
			verify: func(t *testing.T, result *SecretGenerationResult) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.Error)
				assert.Equal(t, "processing_error", result.Error.Type)
				assert.Contains(t, result.Error.Message, "tls.crt and tls.key")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sg.GenerateSecret(ctx, tt.options)
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.verify(t, result)
		})
	}
}

func TestSecretGenerator_GenerateDockerRegistrySecret(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	result, err := sg.GenerateDockerRegistrySecret(
		ctx,
		"docker-creds",
		"default",
		"docker.io",
		"testuser",
		"testpass",
		"test@example.com",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "docker-creds", result.Secret.Metadata.Name)
	assert.Equal(t, string(SecretTypeDockerConfigJson), result.Secret.Type)

	// Verify the docker config structure
	dockerConfigJSON := result.Secret.Data[".dockerconfigjson"]
	decodedData, err := base64.StdEncoding.DecodeString(dockerConfigJSON)
	require.NoError(t, err)

	var dockerConfig map[string]interface{}
	err = json.Unmarshal(decodedData, &dockerConfig)
	require.NoError(t, err)

	auths := dockerConfig["auths"].(map[string]interface{})
	serverAuth := auths["docker.io"].(map[string]interface{})
	assert.Equal(t, "testuser", serverAuth["username"])
	assert.Equal(t, "testpass", serverAuth["password"])
	assert.Equal(t, "test@example.com", serverAuth["email"])

	// Verify auth field
	expectedAuth := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
	assert.Equal(t, expectedAuth, serverAuth["auth"])
}

func TestSecretGenerator_GenerateTLSSecret(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	certPEM := []byte("-----BEGIN CERTIFICATE-----\ntest cert\n-----END CERTIFICATE-----")
	keyPEM := []byte("-----BEGIN PRIVATE KEY-----\ntest key\n-----END PRIVATE KEY-----")

	result, err := sg.GenerateTLSSecret(ctx, "tls-test", "default", certPEM, keyPEM)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "tls-test", result.Secret.Metadata.Name)
	assert.Equal(t, string(SecretTypeTLS), result.Secret.Type)

	// Verify base64 encoding
	expectedCert := base64.StdEncoding.EncodeToString(certPEM)
	expectedKey := base64.StdEncoding.EncodeToString(keyPEM)
	assert.Equal(t, expectedCert, result.Secret.Data["tls.crt"])
	assert.Equal(t, expectedKey, result.Secret.Data["tls.key"])
}

func TestSecretGenerator_GenerateBasicAuthSecret(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	result, err := sg.GenerateBasicAuthSecret(ctx, "auth-test", "default", "admin", "secret123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "auth-test", result.Secret.Metadata.Name)
	assert.Equal(t, string(SecretTypeBasicAuth), result.Secret.Type)
	assert.Equal(t, "admin", result.Secret.StringData["username"])
	assert.Equal(t, "secret123", result.Secret.StringData["password"])
}

func TestSecretGenerator_YAMLMarshaling(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	options := SecretOptions{
		Name:      "yaml-test",
		Namespace: "test-ns",
		Type:      SecretTypeOpaque,
		StringData: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Labels: map[string]string{
			"app": "test",
		},
	}

	result, err := sg.GenerateSecret(ctx, options)
	require.NoError(t, err)
	require.True(t, result.Success)

	// Marshal to YAML
	yamlData, err := yaml.Marshal(result.Secret)
	require.NoError(t, err)

	// Verify YAML structure
	yamlStr := string(yamlData)
	assert.Contains(t, yamlStr, "apiVersion: v1")
	assert.Contains(t, yamlStr, "kind: Secret")
	assert.Contains(t, yamlStr, "name: yaml-test")
	assert.Contains(t, yamlStr, "namespace: test-ns")
	assert.Contains(t, yamlStr, "type: Opaque")
	assert.Contains(t, yamlStr, "key1: value1")
	assert.Contains(t, yamlStr, "key2: value2")

	// Unmarshal back and verify
	var unmarshaled Secret
	err = yaml.Unmarshal(yamlData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "v1", unmarshaled.APIVersion)
	assert.Equal(t, "Secret", unmarshaled.Kind)
	assert.Equal(t, "yaml-test", unmarshaled.Metadata.Name)
}

func TestSecretGenerator_RoundTripIntegration(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	sg := NewSecretGenerator(logger)
	ctx := context.Background()

	// Test round-trip for different secret types
	testCases := []struct {
		name    string
		options SecretOptions
	}{
		{
			name: "opaque secret round-trip",
			options: SecretOptions{
				Name:      "opaque-rt",
				Namespace: "default",
				Type:      SecretTypeOpaque,
				StringData: map[string]string{
					"database": "postgres://user:pass@host:5432/db",
					"api-key":  "sk-1234567890abcdef",
				},
			},
		},
		{
			name: "docker config round-trip",
			options: SecretOptions{
				Name:      "docker-rt",
				Namespace: "docker",
				Type:      SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"gcr.io":{"auth":"_json_key:..."}}}`),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate secret
			result, err := sg.GenerateSecret(ctx, tc.options)
			require.NoError(t, err)
			require.True(t, result.Success)

			// Marshal to YAML
			yamlData, err := yaml.Marshal(result.Secret)
			require.NoError(t, err)

			// Unmarshal from YAML
			var loaded map[string]interface{}
			err = yaml.Unmarshal(yamlData, &loaded)
			require.NoError(t, err)

			// Verify structure
			assert.Equal(t, "v1", loaded["apiVersion"])
			assert.Equal(t, "Secret", loaded["kind"])
			metadata := loaded["metadata"].(map[string]interface{})
			assert.Equal(t, tc.options.Name, metadata["name"])
			assert.Equal(t, tc.options.Namespace, metadata["namespace"])
		})
	}
}

func TestSecretGenerator_isValidKubernetesName(t *testing.T) {
	sg := &SecretGenerator{}

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid lowercase", "my-secret", true},
		{"valid with numbers", "secret-123", true},
		{"valid with dots", "my.secret.v1", true},
		{"valid single char", "a", true},
		{"invalid uppercase", "MySecret", false},
		{"invalid underscore", "my_secret", false},
		{"invalid special chars", "my@secret", false},
		{"invalid start dash", "-secret", false},
		{"invalid end dash", "secret-", false},
		{"invalid empty", "", false},
		{"invalid too long", strings.Repeat("a", 254), false},
		{"valid max length", strings.Repeat("a", 253), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sg.isValidKubernetesName(tt.input)
			assert.Equal(t, tt.valid, result)
		})
	}
}
