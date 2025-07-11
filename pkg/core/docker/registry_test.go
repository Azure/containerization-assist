package docker

import (
	"errors"
	"testing"

	"log/slog"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/stretchr/testify/assert"
)

func TestRegistryManager_NormalizeImageRef(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		registry  string
		tag       string
		want      string
	}{
		{
			name:      "simple image with tag",
			imageName: "nginx",
			registry:  "",
			tag:       "latest",
			want:      "nginx:latest",
		},
		{
			name:      "simple image without tag",
			imageName: "nginx",
			registry:  "",
			tag:       "",
			want:      "nginx:latest",
		},
		{
			name:      "image with registry and tag",
			imageName: "myapp",
			registry:  "myregistry.azurecr.io",
			tag:       "v1.0",
			want:      "myregistry.azurecr.io/myapp:v1.0",
		},
		{
			name:      "image with registry no tag",
			imageName: "myapp",
			registry:  "docker.io",
			tag:       "",
			want:      "docker.io/myapp:latest",
		},
		{
			name:      "image with path in name",
			imageName: "library/alpine",
			registry:  "docker.io",
			tag:       "3.14",
			want:      "docker.io/library/alpine:3.14",
		},
		{
			name:      "gcr registry",
			imageName: "my-project/my-app",
			registry:  "gcr.io",
			tag:       "production",
			want:      "gcr.io/my-project/my-app:production",
		},
		{
			name:      "localhost registry",
			imageName: "test-image",
			registry:  "localhost:5000",
			tag:       "dev",
			want:      "localhost:5000/test-image:dev",
		},
		{
			name:      "empty image name",
			imageName: "",
			registry:  "myregistry.io",
			tag:       "latest",
			want:      "myregistry.io/:latest",
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.NormalizeImageRef(tt.imageName, tt.registry, tt.tag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegistryManager_categorizeError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		output       string
		expectedType string
	}{
		// Authentication errors
		{
			name:         "unauthorized error",
			err:          errors.New("unauthorized: authentication required"),
			output:       "",
			expectedType: "auth_error",
		},
		{
			name:         "authentication in output",
			err:          errors.New("push failed"),
			output:       "Error: authentication failed",
			expectedType: "auth_error",
		},
		{
			name:         "access denied",
			err:          errors.New("access denied"),
			output:       "",
			expectedType: "auth_error",
		},
		{
			name:         "permission denied",
			err:          errors.New("permission denied: repository does not exist or may require authentication"),
			output:       "",
			expectedType: "auth_error",
		},
		// Network errors
		{
			name:         "network timeout",
			err:          errors.New("network timeout"),
			output:       "",
			expectedType: "network_error",
		},
		{
			name:         "connection refused",
			err:          errors.New("connection refused"),
			output:       "",
			expectedType: "network_error",
		},
		{
			name:         "network in output",
			err:          errors.New("push failed"),
			output:       "network is unreachable",
			expectedType: "network_error",
		},
		{
			name:         "dial tcp timeout",
			err:          errors.New("dial tcp: lookup registry.example.com: i/o timeout"),
			output:       "",
			expectedType: "network_error",
		},
		// Not found errors
		{
			name:         "repository not found",
			err:          errors.New("repository not found"),
			output:       "",
			expectedType: "not_found",
		},
		{
			name:         "does not exist",
			err:          errors.New("image does not exist"),
			output:       "",
			expectedType: "not_found",
		},
		{
			name:         "not found in output",
			err:          errors.New("push failed"),
			output:       "Error: repository not found",
			expectedType: "not_found",
		},
		// Generic errors
		{
			name:         "generic error",
			err:          errors.New("unknown error occurred"),
			output:       "",
			expectedType: "push_error",
		},
		{
			name:         "empty error message",
			err:          errors.New(""),
			output:       "",
			expectedType: "push_error",
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.categorizeError(tt.err, tt.output)
			assert.Equal(t, tt.expectedType, got)
		})
	}
}

func TestRegistryManager_categorizePullError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		output       string
		expectedType string
	}{
		// Pull-specific not found errors
		{
			name:         "manifest unknown",
			err:          errors.New("manifest unknown"),
			output:       "",
			expectedType: "not_found",
		},
		{
			name:         "repository does not exist",
			err:          errors.New("pull access denied for myimage, repository does not exist"),
			output:       "",
			expectedType: "not_found",
		},
		{
			name:         "manifest unknown in output",
			err:          errors.New("pull failed"),
			output:       "Error response from daemon: manifest unknown: repository not found",
			expectedType: "not_found",
		},
		// Same error categories as push
		{
			name:         "auth error on pull",
			err:          errors.New("unauthorized: authentication required"),
			output:       "",
			expectedType: "auth_error",
		},
		{
			name:         "network error on pull",
			err:          errors.New("dial tcp: connection refused"),
			output:       "",
			expectedType: "network_error",
		},
		{
			name:         "generic pull error",
			err:          errors.New("unknown pull error"),
			output:       "",
			expectedType: "pull_error",
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.categorizePullError(tt.err, tt.output)
			assert.Equal(t, tt.expectedType, got)
		})
	}
}

func TestRegistryManager_validatePushInputs(t *testing.T) {
	tests := []struct {
		name      string
		imageRef  string
		options   PushOptions
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid image reference",
			imageRef:  "myregistry.io/myapp:v1.0",
			options:   PushOptions{},
			wantError: false,
		},
		{
			name:      "empty image reference",
			imageRef:  "",
			options:   PushOptions{},
			wantError: true,
			errorMsg:  "image reference is required",
		},
		{
			name:      "image starting with dash",
			imageRef:  "-invalid:latest",
			options:   PushOptions{},
			wantError: true,
			errorMsg:  "invalid image reference format",
		},
		{
			name:      "image ending with dash",
			imageRef:  "invalid-:latest",
			options:   PushOptions{},
			wantError: true,
			errorMsg:  "invalid image reference format",
		},
		{
			name:      "valid localhost registry",
			imageRef:  "localhost:5000/test:dev",
			options:   PushOptions{},
			wantError: false,
		},
		{
			name:      "valid gcr.io image",
			imageRef:  "gcr.io/my-project/my-app:latest",
			options:   PushOptions{},
			wantError: false,
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.validatePushInputs(tt.imageRef, tt.options)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistryManager_validatePullInputs(t *testing.T) {
	tests := []struct {
		name      string
		imageRef  string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid image reference",
			imageRef:  "nginx:latest",
			wantError: false,
		},
		{
			name:      "valid registry image",
			imageRef:  "docker.io/library/nginx:1.21",
			wantError: false,
		},
		{
			name:      "empty image reference",
			imageRef:  "",
			wantError: true,
			errorMsg:  "image reference is required",
		},
		{
			name:      "invalid format - starts with dash",
			imageRef:  "-nginx:latest",
			wantError: true,
			errorMsg:  "invalid image reference format",
		},
		{
			name:      "invalid format - ends with dash",
			imageRef:  "nginx-:latest",
			wantError: true,
			errorMsg:  "invalid image reference format",
		},
		{
			name:      "image with sha256",
			imageRef:  "nginx@sha256:abcdef123456",
			wantError: false,
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.validatePullInputs(tt.imageRef)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistryManager_validateTagInputs(t *testing.T) {
	tests := []struct {
		name        string
		sourceImage string
		targetImage string
		wantError   bool
		errorMsg    string
	}{
		{
			name:        "valid tag operation",
			sourceImage: "myapp:v1.0",
			targetImage: "myregistry.io/myapp:v1.0",
			wantError:   false,
		},
		{
			name:        "empty source image",
			sourceImage: "",
			targetImage: "myapp:latest",
			wantError:   true,
			errorMsg:    "source image is required",
		},
		{
			name:        "empty target image",
			sourceImage: "myapp:latest",
			targetImage: "",
			wantError:   true,
			errorMsg:    "target image is required",
		},
		{
			name:        "invalid source format",
			sourceImage: "-invalid:tag",
			targetImage: "valid:tag",
			wantError:   true,
			errorMsg:    "invalid image reference format",
		},
		{
			name:        "invalid target format",
			sourceImage: "valid:tag",
			targetImage: "invalid-:tag",
			wantError:   true,
			errorMsg:    "invalid image reference format",
		},
		{
			name:        "both images valid",
			sourceImage: "alpine:3.14",
			targetImage: "myregistry.azurecr.io/alpine:3.14-custom",
			wantError:   false,
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.validateTagInputs(tt.sourceImage, tt.targetImage)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistryManager_extractRegistry(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		want     string
	}{
		{
			name:     "docker hub image",
			imageRef: "nginx:latest",
			want:     "docker.io",
		},
		{
			name:     "explicit docker.io",
			imageRef: "docker.io/nginx:latest",
			want:     "docker.io",
		},
		{
			name:     "azure container registry",
			imageRef: "myregistry.azurecr.io/myapp:v1.0",
			want:     "myregistry.azurecr.io",
		},
		{
			name:     "gcr.io registry",
			imageRef: "gcr.io/my-project/my-app:latest",
			want:     "gcr.io",
		},
		{
			name:     "localhost registry",
			imageRef: "localhost:5000/test:latest",
			want:     "localhost:5000",
		},
		{
			name:     "library image",
			imageRef: "library/alpine:3.14",
			want:     "docker.io",
		},
		{
			name:     "nested path",
			imageRef: "myregistry.io/team/project/app:v2.1",
			want:     "myregistry.io",
		},
		{
			name:     "port in registry",
			imageRef: "registry.example.com:8080/app:latest",
			want:     "registry.example.com:8080",
		},
		{
			name:     "image with digest",
			imageRef: "nginx@sha256:abcdef",
			want:     "docker.io",
		},
	}

	rm := &RegistryManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.extractRegistry(tt.imageRef)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewRegistryManager tests the constructor
func TestNewRegistryManager(t *testing.T) {
	mockClients := &clients.Clients{}
	logger := slog.New(slog.NewTextHandler(nil, nil))

	rm := NewRegistryManager(mockClients, logger)

	assert.NotNil(t, rm)
	assert.Equal(t, mockClients, rm.clients)
	assert.NotNil(t, rm.logger)
}
