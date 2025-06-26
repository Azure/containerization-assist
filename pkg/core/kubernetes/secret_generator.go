// Package kubernetes provides core Kubernetes operations for secrets generation
package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"sigs.k8s.io/yaml"
)

// SecretGenerator provides Kubernetes secret generation operations
type SecretGenerator struct {
	logger zerolog.Logger
}

// NewSecretGenerator creates a new secret generator
func NewSecretGenerator(logger zerolog.Logger) *SecretGenerator {
	return &SecretGenerator{
		logger: logger.With().Str("component", "k8s_secret_generator").Logger(),
	}
}

// SecretType represents different types of Kubernetes secrets
type SecretType string

const (
	SecretTypeOpaque              SecretType = "Opaque"
	SecretTypeServiceAccountToken SecretType = "kubernetes.io/service-account-token"
	SecretTypeDockerConfigJson    SecretType = "kubernetes.io/dockerconfigjson"
	SecretTypeDockercfg           SecretType = "kubernetes.io/dockercfg"
	SecretTypeBasicAuth           SecretType = "kubernetes.io/basic-auth"
	SecretTypeSSHAuth             SecretType = "kubernetes.io/ssh-auth"
	SecretTypeTLS                 SecretType = "kubernetes.io/tls"
	SecretTypeBootstrapToken      SecretType = "bootstrap.kubernetes.io/token"
)

// SecretOptions contains options for secret generation
type SecretOptions struct {
	Name        string
	Namespace   string
	Type        SecretType
	Data        map[string][]byte
	StringData  map[string]string
	Labels      map[string]string
	Annotations map[string]string
}

// SecretGenerationResult contains the result of secret generation
type SecretGenerationResult struct {
	Success  bool          `json:"success"`
	Secret   *Secret       `json:"secret"`
	Path     string        `json:"path,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    *SecretError  `json:"error,omitempty"`
}

// Secret represents a Kubernetes Secret
type Secret struct {
	APIVersion string            `yaml:"apiVersion" json:"apiVersion"`
	Kind       string            `yaml:"kind" json:"kind"`
	Metadata   SecretMetadata    `yaml:"metadata" json:"metadata"`
	Type       string            `yaml:"type" json:"type"`
	Data       map[string]string `yaml:"data,omitempty" json:"data,omitempty"`
	StringData map[string]string `yaml:"stringData,omitempty" json:"stringData,omitempty"`
}

// SecretMetadata represents secret metadata
type SecretMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace" json:"namespace"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// SecretError provides detailed secret generation error information
type SecretError struct {
	Type    string                 `json:"type"`
	Message string                 `json:"message"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// GenerateSecret generates a Kubernetes secret
func (sg *SecretGenerator) GenerateSecret(ctx context.Context, options SecretOptions) (*SecretGenerationResult, error) {
	startTime := time.Now()

	result := &SecretGenerationResult{}

	sg.logger.Info().
		Str("name", options.Name).
		Str("namespace", options.Namespace).
		Str("type", string(options.Type)).
		Msg("Starting secret generation")

	// Validate inputs
	if err := sg.validateSecretOptions(options); err != nil {
		result.Error = &SecretError{
			Type:    "validation_error",
			Message: err.Error(),
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Create secret object
	secret := &Secret{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata: SecretMetadata{
			Name:        options.Name,
			Namespace:   options.Namespace,
			Labels:      options.Labels,
			Annotations: options.Annotations,
		},
		Type: string(options.Type),
	}

	// Add default labels if not present
	if secret.Metadata.Labels == nil {
		secret.Metadata.Labels = make(map[string]string)
	}
	secret.Metadata.Labels["kubernetes.azure.com/generator"] = "container-kit"

	// Process data based on type
	switch options.Type {
	case SecretTypeOpaque:
		if err := sg.processOpaqueSecret(secret, options); err != nil {
			result.Error = &SecretError{
				Type:    "processing_error",
				Message: err.Error(),
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

	case SecretTypeDockerConfigJson:
		if err := sg.processDockerConfigSecret(secret, options); err != nil {
			result.Error = &SecretError{
				Type:    "processing_error",
				Message: err.Error(),
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

	case SecretTypeBasicAuth:
		if err := sg.processBasicAuthSecret(secret, options); err != nil {
			result.Error = &SecretError{
				Type:    "processing_error",
				Message: err.Error(),
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

	case SecretTypeTLS:
		if err := sg.processTLSSecret(secret, options); err != nil {
			result.Error = &SecretError{
				Type:    "processing_error",
				Message: err.Error(),
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

	default:
		// For other types, just encode the data
		if err := sg.processGenericSecret(secret, options); err != nil {
			result.Error = &SecretError{
				Type:    "processing_error",
				Message: err.Error(),
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	result.Success = true
	result.Secret = secret
	result.Duration = time.Since(startTime)

	sg.logger.Info().
		Str("name", options.Name).
		Dur("duration", result.Duration).
		Msg("Secret generation completed successfully")

	return result, nil
}

// SaveSecretToFile saves a secret to a YAML file
func (sg *SecretGenerator) SaveSecretToFile(secret *Secret, outputPath string) error {
	// Marshal to YAML
	yamlData, err := yaml.Marshal(secret)
	if err != nil {
		return fmt.Errorf("failed to marshal secret to YAML: %v", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := sg.ensureDirectory(dir); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Write to file
	if err := sg.writeFile(outputPath, yamlData); err != nil {
		return fmt.Errorf("failed to write secret file: %v", err)
	}

	sg.logger.Info().
		Str("path", outputPath).
		Str("name", secret.Metadata.Name).
		Msg("Secret saved to file")

	return nil
}

// GenerateDockerRegistrySecret generates a Docker registry secret
func (sg *SecretGenerator) GenerateDockerRegistrySecret(ctx context.Context, name, namespace, server, username, password, email string) (*SecretGenerationResult, error) {
	dockerConfig := map[string]interface{}{
		"auths": map[string]interface{}{
			server: map[string]interface{}{
				"username": username,
				"password": password,
				"email":    email,
				"auth":     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
			},
		},
	}

	dockerConfigJSON, err := sg.marshalJSON(dockerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker config: %v", err)
	}

	options := SecretOptions{
		Name:      name,
		Namespace: namespace,
		Type:      SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": dockerConfigJSON,
		},
	}

	return sg.GenerateSecret(ctx, options)
}

// GenerateTLSSecret generates a TLS secret
func (sg *SecretGenerator) GenerateTLSSecret(ctx context.Context, name, namespace string, certPEM, keyPEM []byte) (*SecretGenerationResult, error) {
	options := SecretOptions{
		Name:      name,
		Namespace: namespace,
		Type:      SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}

	return sg.GenerateSecret(ctx, options)
}

// GenerateBasicAuthSecret generates a basic authentication secret
func (sg *SecretGenerator) GenerateBasicAuthSecret(ctx context.Context, name, namespace, username, password string) (*SecretGenerationResult, error) {
	options := SecretOptions{
		Name:      name,
		Namespace: namespace,
		Type:      SecretTypeBasicAuth,
		StringData: map[string]string{
			"username": username,
			"password": password,
		},
	}

	return sg.GenerateSecret(ctx, options)
}

// Helper methods

func (sg *SecretGenerator) validateSecretOptions(options SecretOptions) error {
	if options.Name == "" {
		return fmt.Errorf("secret name is required")
	}

	if options.Namespace == "" {
		options.Namespace = "default"
	}

	if options.Type == "" {
		options.Type = SecretTypeOpaque
	}

	// Validate name format
	if !sg.isValidKubernetesName(options.Name) {
		return fmt.Errorf("invalid secret name: must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character")
	}

	return nil
}

func (sg *SecretGenerator) processOpaqueSecret(secret *Secret, options SecretOptions) error {
	// Process both Data and StringData
	if len(options.Data) > 0 {
		secret.Data = make(map[string]string)
		for key, value := range options.Data {
			secret.Data[key] = base64.StdEncoding.EncodeToString(value)
		}
	}

	if len(options.StringData) > 0 {
		secret.StringData = options.StringData
	}

	return nil
}

func (sg *SecretGenerator) processDockerConfigSecret(secret *Secret, options SecretOptions) error {
	// Validate required field
	if _, ok := options.Data[".dockerconfigjson"]; !ok {
		return fmt.Errorf("docker config secret requires .dockerconfigjson data")
	}

	secret.Data = make(map[string]string)
	secret.Data[".dockerconfigjson"] = base64.StdEncoding.EncodeToString(options.Data[".dockerconfigjson"])

	return nil
}

func (sg *SecretGenerator) processBasicAuthSecret(secret *Secret, options SecretOptions) error {
	// Check for required fields in StringData
	username, hasUsername := options.StringData["username"]
	password, hasPassword := options.StringData["password"]

	if !hasUsername || !hasPassword {
		return fmt.Errorf("basic auth secret requires username and password")
	}

	secret.StringData = map[string]string{
		"username": username,
		"password": password,
	}

	return nil
}

func (sg *SecretGenerator) processTLSSecret(secret *Secret, options SecretOptions) error {
	// Validate required fields
	cert, hasCert := options.Data["tls.crt"]
	key, hasKey := options.Data["tls.key"]

	if !hasCert || !hasKey {
		return fmt.Errorf("TLS secret requires tls.crt and tls.key data")
	}

	secret.Data = make(map[string]string)
	secret.Data["tls.crt"] = base64.StdEncoding.EncodeToString(cert)
	secret.Data["tls.key"] = base64.StdEncoding.EncodeToString(key)

	return nil
}

func (sg *SecretGenerator) processGenericSecret(secret *Secret, options SecretOptions) error {
	// Process both Data and StringData
	if len(options.Data) > 0 {
		secret.Data = make(map[string]string)
		for key, value := range options.Data {
			secret.Data[key] = base64.StdEncoding.EncodeToString(value)
		}
	}

	if len(options.StringData) > 0 {
		secret.StringData = options.StringData
	}

	return nil
}

func (sg *SecretGenerator) isValidKubernetesName(name string) bool {
	// Simple validation - in production, use proper regex
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// Must start and end with alphanumeric
	if !sg.isAlphanumeric(string(name[0])) || !sg.isAlphanumeric(string(name[len(name)-1])) {
		return false
	}

	// Check all characters
	for _, c := range name {
		if !sg.isAlphanumeric(string(c)) && c != '-' && c != '.' {
			return false
		}
	}

	return true
}

func (sg *SecretGenerator) isAlphanumeric(s string) bool {
	return (s >= "a" && s <= "z") || (s >= "0" && s <= "9")
}

func (sg *SecretGenerator) ensureDirectory(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func (sg *SecretGenerator) writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func (sg *SecretGenerator) marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
