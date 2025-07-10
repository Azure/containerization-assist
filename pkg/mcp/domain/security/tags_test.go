package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationTags_GitURL(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagGitURL].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid https git url", "https://github.com/user/repo.git", false},
		{"valid ssh git url", "git@github.com:user/repo.git", false},
		{"valid github url without .git", "https://github.com/user/repo", false},
		{"invalid url", "not-a-url", true},
		{"empty string", "", true},
		{"ftp url", "ftp://example.com/repo.git", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_DockerImage(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagDockerImage].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple image", "nginx", false},
		{"image with tag", "nginx:latest", false},
		{"registry image", "registry.com/nginx:v1.0", false},
		{"localhost registry", "localhost/myapp", false},
		{"empty string", "", true},
		{"invalid characters", "invalid image name", true},
		{"double colon", "nginx::", true},
		{"multiple tags", "nginx:tag:extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_SessionID(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagSessionID].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid uuid", "123e4567-e89b-12d3-a456-426614174000", false},
		{"zero uuid", "00000000-0000-0000-0000-000000000000", false},
		{"empty string", "", true},
		{"invalid format", "not-a-uuid", true},
		{"short uuid", "123e4567-e89b-12d3-a456", true},
		{"invalid character", "123e4567-e89b-12d3-a456-42661417400g", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_Platform(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagPlatform].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"linux amd64", "linux/amd64", false},
		{"linux arm64", "linux/arm64", false},
		{"windows amd64", "windows/amd64", false},
		{"darwin arm64", "darwin/arm64", false},
		{"invalid platform", "invalid/platform", true},
		{"empty string", "", true},
		{"only os", "linux", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_K8sName(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagK8sName].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple name", "myapp", false},
		{"with hyphens", "my-app", false},
		{"with numbers", "app123", false},
		{"mixed", "my-app-123", false},
		{"uppercase", "MyApp", true}, // K8s names must be lowercase
		{"underscore", "my_app", true},
		{"start with hyphen", "-myapp", true},
		{"end with hyphen", "myapp-", true},
		{"empty string", "", true},
		{"too long", "this-is-a-very-long-name-that-exceeds-the-maximum-length-of-63-characters-allowed", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_SecurePath(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagSecurePath].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"safe path", "/safe/path/to/file", false},
		{"relative path", "relative/path", false},
		{"app data path", "/app/data/config.yaml", false},
		{"path traversal", "../../../etc/passwd", true},
		{"etc directory", "/etc/shadow", true},
		{"sys directory", "/sys/class/net", true},
		{"proc directory", "/proc/cpuinfo", true},
		{"root directory", "/root/.ssh/id_rsa", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_NoSensitive(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagNoSensitive].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal content", "normal content", false},
		{"build env", "BUILD_ENV=production", false},
		{"debug flag", "DEBUG=false", false},
		{"password", "password=secret123", true},
		{"api key", "api_key=AKIA1234567890ABCDEF", true},
		{"secret token", "SECRET_TOKEN=abc123def456", true},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_NoInjection(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagNoInjection].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal text", "normal text", false},
		{"app name", "app_name", false},
		{"some value", "some-value", false},
		{"sql injection", "'; DROP TABLE users; --", true},
		{"command injection", "$(rm -rf /)", true},
		{"xss", "<script>alert('xss')</script>", true},
		{"javascript injection", "javascript:alert(1)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationTags_Port(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagPort].Validator

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{"valid port int", 8080, false},
		{"valid port string", "8080", false},
		{"port 1", 1, false},
		{"port 65535", 65535, false},
		{"port 0", 0, true},
		{"port too high", 99999, true},
		{"negative port", -1, true},
		{"empty string", "", true},
		{"non-numeric string", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommonValidationTags_Registry(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()

	// Test that all expected tags are present
	expectedTags := []string{
		TagGitURL, TagImageName, TagDockerImage, TagDockerTag, TagPlatform,
		TagK8sName, TagNamespace, TagSessionID, TagFilePath, TagSecurePath,
		TagNoSensitive, TagNoInjection, TagPort, TagEndpoint, TagRegistryURL,
		TagResourceSpec, TagDomain, TagK8sSelector, TagVulnType,
	}

	for _, tag := range expectedTags {
		t.Run("tag_"+tag, func(t *testing.T) {
			// Disabled parallel due to shared state
			tagDef, exists := tags[tag]
			assert.True(t, exists, "Tag %s should exist in registry", tag)
			assert.NotEmpty(t, tagDef.Description, "Tag %s should have description", tag)
			assert.NotNil(t, tagDef.Validator, "Tag %s should have validator function", tag)
		})
	}
}

func TestValidationTags_RegistryURL(t *testing.T) {
	// Disabled parallel due to shared state
	tests := []struct {
		name      string
		value     interface{}
		shouldErr bool
	}{
		{"valid_registry", "myregistry.azurecr.io", false},
		{"valid_registry_with_port", "localhost:5000", false},
		{"valid_hostname", "docker.io", false},
		{"valid_subdomain", "my.registry.com", false},
		{"registry_with_numbers", "registry123.example.com", false},
		{"localhost_no_port", "localhost", false},
		{"empty_string", "", true},
		{"invalid_characters", "registry@domain.com", true},
		{"starts_with_hyphen", "-registry.com", true},
		{"ends_with_hyphen", "registry.com-", true},
		{"invalid_port", "registry.com:99999", true},
		{"empty_port", "registry.com:", true},
		{"non_numeric_port", "registry.com:abc", true},
		{"multiple_colons", "registry.com:5000:extra", true},
		{"spaces", "registry .com", true},
		{"non_string", 123, true},
	}

	validator := CommonValidationTags()[TagRegistryURL].Validator

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.value, "test_field", map[string]interface{}{})
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for value %v, but got none", tt.value)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error for value %v, but got: %v", tt.value, err)
			}
		})
	}
}

func TestValidationTags_VulnType(t *testing.T) {
	// Disabled parallel due to shared state
	tags := CommonValidationTags()
	validator := tags[TagVulnType].Validator

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid os", "os", false},
		{"valid library", "library", false},
		{"valid application", "application", false},
		{"valid config", "config", false},
		{"valid secret", "secret", false},
		{"valid malware", "malware", false},
		{"valid all", "all", false},
		{"case insensitive OS", "OS", false},
		{"case insensitive Library", "Library", false},
		{"invalid type", "invalid", true},
		{"empty string", "", true},
		{"unknown type", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Disabled parallel due to shared state
			err := validator(tt.input, "test_field", nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
