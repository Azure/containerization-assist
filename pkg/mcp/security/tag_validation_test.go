package security

import (
	"context"
	"testing"
)

// TestStructValidation tests the tag-based validation system
func TestStructValidation(t *testing.T) {
	// Note: Cannot run in parallel due to shared global state in StructValidator
	validator := NewStructValidator()

	// Test struct with validation tags
	type TestConfig struct {
		Name      string   `validate:"required,min=3,max=20"`
		Email     string   `validate:"required,email"`
		Port      int      `validate:"required,min=1,max=65535"`
		ImageName string   `validate:"required,image_name"`
		Namespace string   `validate:"k8s_name"`
		Tags      []string `validate:"required,min=1"`
	}

	tests := []struct {
		name      string
		config    TestConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: TestConfig{
				Name:      "test-app",
				Email:     "test@example.com",
				Port:      8080,
				ImageName: "nginx:latest",
				Namespace: "default",
				Tags:      []string{"web", "app"},
			},
			wantError: false,
		},
		{
			name: "missing required field",
			config: TestConfig{
				Email:     "test@example.com",
				Port:      8080,
				ImageName: "nginx:latest",
				Namespace: "default",
				Tags:      []string{"web", "app"},
			},
			wantError: true,
		},
		{
			name: "invalid email",
			config: TestConfig{
				Name:      "test-app",
				Email:     "invalid-email",
				Port:      8080,
				ImageName: "nginx:latest",
				Namespace: "default",
				Tags:      []string{"web", "app"},
			},
			wantError: true,
		},
		{
			name: "port out of range",
			config: TestConfig{
				Name:      "test-app",
				Email:     "test@example.com",
				Port:      70000,
				ImageName: "nginx:latest",
				Namespace: "default",
				Tags:      []string{"web", "app"},
			},
			wantError: true,
		},
		{
			name: "invalid k8s name",
			config: TestConfig{
				Name:      "test-app",
				Email:     "test@example.com",
				Port:      8080,
				ImageName: "nginx:latest",
				Namespace: "Invalid-Namespace",
				Tags:      []string{"web", "app"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validator.ValidateStruct(context.Background(), &tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateStruct() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestFieldValidation tests individual field validation
func TestFieldValidation(t *testing.T) {
	// Note: Cannot run in parallel due to shared global state in StructValidator
	validator := NewStructValidator()

	tests := []struct {
		name      string
		value     interface{}
		tag       string
		wantError bool
	}{
		{
			name:      "valid email",
			value:     "test@example.com",
			tag:       "required,email",
			wantError: false,
		},
		{
			name:      "invalid email",
			value:     "invalid",
			tag:       "required,email",
			wantError: true,
		},
		{
			name:      "valid port",
			value:     8080,
			tag:       "required,min=1,max=65535",
			wantError: false,
		},
		{
			name:      "port too high",
			value:     70000,
			tag:       "required,min=1,max=65535",
			wantError: true,
		},
		{
			name:      "valid image name",
			value:     "nginx:latest",
			tag:       "required,image_name",
			wantError: false,
		},
		{
			name:      "invalid image name",
			value:     "invalid@image:tag",
			tag:       "required,image_name",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validator.ValidateField(tt.value, tt.tag)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateField() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// BenchmarkStructValidation benchmarks the validation performance
func BenchmarkStructValidation(b *testing.B) {
	validator := NewStructValidator()

	type BenchConfig struct {
		Name      string `validate:"required,min=3,max=20"`
		Email     string `validate:"required,email"`
		Port      int    `validate:"required,min=1,max=65535"`
		ImageName string `validate:"required,image_name"`
	}

	config := BenchConfig{
		Name:      "test-app",
		Email:     "test@example.com",
		Port:      8080,
		ImageName: "nginx:latest",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateStruct(context.Background(), &config)
	}
}
