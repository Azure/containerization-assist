package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test struct for tag-based validation
type TestAnalyzeStruct struct {
	SessionID  string `validate:"required,session_id"`
	RepoURL    string `validate:"required,git_url"`
	BranchName string `validate:"omitempty,git_branch"`
	TargetPath string `validate:"omitempty,secure_path"`
}

type TestBuildStruct struct {
	SessionID string `validate:"required,session_id"`
	ImageRef  string `validate:"required,docker_image"`
	Platform  string `validate:"omitempty,platform"`
	Timeout   int    `validate:"omitempty,min=30,max=3600"`
}

type TestDeployStruct struct {
	SessionID     string `validate:"required,session_id"`
	AppName       string `validate:"required,k8s_name"`
	Namespace     string `validate:"omitempty,namespace"`
	ImageRef      string `validate:"required,docker_image"`
	CPURequest    string `validate:"omitempty,resource_spec"`
	MemoryRequest string `validate:"omitempty,resource_spec"`
	IngressHost   string `validate:"omitempty,domain"`
}

type TestScanStruct struct {
	SessionID    string   `validate:"required,session_id"`
	ImageName    string   `validate:"required,docker_image"`
	ScanPath     string   `validate:"omitempty,secure_path"`
	Severity     string   `validate:"omitempty,severity"`
	VulnTypes    []string `validate:"omitempty,dive,vuln_type"`
	FilePatterns []string `validate:"omitempty,dive,file_pattern"`
}

func TestValidateTaggedStruct_Analyze(t *testing.T) {
	tests := []struct {
		name    string
		data    TestAnalyzeStruct
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid analyze struct",
			data: TestAnalyzeStruct{
				SessionID:  "123e4567-e89b-12d3-a456-426614174000",
				RepoURL:    "https://github.com/example/repo.git",
				BranchName: "main",
				TargetPath: "/app/workspace",
			},
			wantErr: false,
		},
		{
			name: "missing session ID",
			data: TestAnalyzeStruct{
				RepoURL: "https://github.com/example/repo.git",
			},
			wantErr: true,
			errMsg:  "SessionID is required",
		},
		{
			name: "invalid git URL",
			data: TestAnalyzeStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				RepoURL:   "not-a-git-url",
			},
			wantErr: true,
			errMsg:  "must be a valid Git URL",
		},
		{
			name: "invalid session ID format",
			data: TestAnalyzeStruct{
				SessionID: "not-a-uuid",
				RepoURL:   "https://github.com/example/repo.git",
			},
			wantErr: true,
			errMsg:  "must be a valid UUID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaggedStruct(&tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaggedStruct_Build(t *testing.T) {
	tests := []struct {
		name    string
		data    TestBuildStruct
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid build struct",
			data: TestBuildStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				ImageRef:  "nginx:latest",
				Platform:  "linux/amd64",
				Timeout:   600,
			},
			wantErr: false,
		},
		{
			name: "invalid platform",
			data: TestBuildStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				ImageRef:  "nginx:latest",
				Platform:  "invalid/platform",
			},
			wantErr: true,
			errMsg:  "must be a valid platform",
		},
		{
			name: "timeout out of range",
			data: TestBuildStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				ImageRef:  "nginx:latest",
				Timeout:   10, // Less than min 30
			},
			wantErr: true,
			errMsg:  "must be at least 30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaggedStruct(&tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaggedStruct_Deploy(t *testing.T) {
	tests := []struct {
		name    string
		data    TestDeployStruct
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid deploy struct",
			data: TestDeployStruct{
				SessionID:     "123e4567-e89b-12d3-a456-426614174000",
				AppName:       "my-app",
				Namespace:     "default",
				ImageRef:      "myregistry.com/myapp:v1.0.0",
				CPURequest:    "100m",
				MemoryRequest: "128Mi",
				IngressHost:   "myapp.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid k8s name",
			data: TestDeployStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				AppName:   "My_App", // Contains uppercase and underscore
				ImageRef:  "nginx:latest",
			},
			wantErr: true,
			errMsg:  "must be a valid Kubernetes name",
		},
		{
			name: "invalid resource spec",
			data: TestDeployStruct{
				SessionID:  "123e4567-e89b-12d3-a456-426614174000",
				AppName:    "my-app",
				ImageRef:   "nginx:latest",
				CPURequest: "invalid",
			},
			wantErr: true,
			errMsg:  "must be a valid Kubernetes resource specification",
		},
		{
			name: "invalid domain",
			data: TestDeployStruct{
				SessionID:   "123e4567-e89b-12d3-a456-426614174000",
				AppName:     "my-app",
				ImageRef:    "nginx:latest",
				IngressHost: "not a domain",
			},
			wantErr: true,
			errMsg:  "must be a valid domain name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaggedStruct(&tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaggedStruct_Scan(t *testing.T) {
	tests := []struct {
		name    string
		data    TestScanStruct
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid scan struct",
			data: TestScanStruct{
				SessionID:    "123e4567-e89b-12d3-a456-426614174000",
				ImageName:    "nginx:latest",
				ScanPath:     "/workspace/scan",
				Severity:     "HIGH",
				VulnTypes:    []string{"os", "library"},
				FilePatterns: []string{"*.js", "*.py"},
			},
			wantErr: false,
		},
		{
			name: "invalid severity",
			data: TestScanStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				ImageName: "nginx:latest",
				Severity:  "INVALID",
			},
			wantErr: true,
			errMsg:  "must be one of: LOW, MEDIUM, HIGH, CRITICAL",
		},
		{
			name: "path traversal in scan path",
			data: TestScanStruct{
				SessionID: "123e4567-e89b-12d3-a456-426614174000",
				ImageName: "nginx:latest",
				ScanPath:  "../../../etc/passwd",
			},
			wantErr: true,
			errMsg:  "path cannot contain '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaggedStruct(&tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaggedStruct_EdgeCases(t *testing.T) {
	t.Run("nil struct", func(t *testing.T) {
		err := ValidateTaggedStruct(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "struct cannot be nil")
	})

	t.Run("nil pointer", func(t *testing.T) {
		var ptr *TestAnalyzeStruct
		err := ValidateTaggedStruct(ptr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "struct pointer cannot be nil")
	})

	t.Run("non-struct type", func(t *testing.T) {
		notAStruct := "not a struct"
		err := ValidateTaggedStruct(notAStruct)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected struct")
	})

	t.Run("empty struct", func(t *testing.T) {
		type EmptyStruct struct{}
		err := ValidateTaggedStruct(&EmptyStruct{})
		assert.NoError(t, err) // Empty struct should pass
	})

	t.Run("struct with no validation tags", func(t *testing.T) {
		type NoValidationStruct struct {
			Field1 string
			Field2 int
		}
		err := ValidateTaggedStruct(&NoValidationStruct{
			Field1: "value",
			Field2: 42,
		})
		assert.NoError(t, err) // Should pass as no validation required
	})
}
