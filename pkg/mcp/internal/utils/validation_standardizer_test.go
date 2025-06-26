package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardizedValidationMixin_StandardValidateImageRef(t *testing.T) {
	tests := []struct {
		name          string
		imageRef      string
		fieldName     string
		expectValid   bool
		expectedError string
		errorCode     string
	}{
		{
			name:        "valid image with tag",
			imageRef:    "nginx:latest",
			fieldName:   "image",
			expectValid: true,
		},
		{
			name:        "valid image with version tag",
			imageRef:    "alpine:3.14",
			fieldName:   "image",
			expectValid: true,
		},
		{
			name:        "valid image with registry",
			imageRef:    "docker.io/library/nginx:1.21",
			fieldName:   "image",
			expectValid: true,
		},
		{
			name:          "empty image reference",
			imageRef:      "",
			fieldName:     "image",
			expectValid:   false,
			expectedError: "Image reference cannot be empty",
			errorCode:     "IMAGE_REF_REQUIRED",
		},
		{
			name:          "missing tag",
			imageRef:      "nginx",
			fieldName:     "image",
			expectValid:   false,
			expectedError: "Image reference must include a tag",
			errorCode:     "IMAGE_REF_NO_TAG",
		},
		{
			name:          "empty tag",
			imageRef:      "nginx:",
			fieldName:     "image",
			expectValid:   false,
			expectedError: "Image tag cannot be empty",
			errorCode:     "IMAGE_TAG_EMPTY",
		},
		{
			name:          "only colon",
			imageRef:      ":",
			fieldName:     "image",
			expectValid:   false,
			expectedError: "Image name cannot be empty",
			errorCode:     "IMAGE_NAME_EMPTY",
		},
		{
			name:          "colon at start",
			imageRef:      ":latest",
			fieldName:     "image",
			expectValid:   false,
			expectedError: "Image name cannot be empty",
			errorCode:     "IMAGE_NAME_EMPTY",
		},
		{
			name:        "image with sha256 digest",
			imageRef:    "nginx@sha256:abcdef123456",
			fieldName:   "image",
			expectValid: true, // @ is treated as part of image name, not a separator
		},
		{
			name:        "complex registry path",
			imageRef:    "myregistry.azurecr.io/team/project/app:v2.1.3",
			fieldName:   "image",
			expectValid: true,
		},
		{
			name:        "localhost registry",
			imageRef:    "localhost:5000/myapp:dev",
			fieldName:   "image",
			expectValid: true,
		},
	}

	svm := NewStandardizedValidationMixin(zerolog.Nop())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svm.StandardValidateImageRef(tt.imageRef, tt.fieldName)

			assert.Equal(t, tt.expectValid, result.Valid)

			if !tt.expectValid {
				assert.True(t, result.HasErrors())
				if tt.expectedError != "" {
					firstError := result.GetFirstError()
					assert.NotNil(t, firstError)
					assert.Contains(t, firstError.Message, tt.expectedError)
					if tt.errorCode != "" {
						assert.Equal(t, tt.errorCode, firstError.Code)
					}
				}
			} else {
				assert.False(t, result.HasErrors())
			}
		})
	}
}

func TestStandardizedValidationMixin_StandardValidatePath(t *testing.T) {
	// Create temp directory for tests
	tempDir := t.TempDir()

	// Create test files and directories
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0600))

	testDir := filepath.Join(tempDir, "testdir")
	require.NoError(t, os.Mkdir(testDir, 0755))

	readOnlyFile := filepath.Join(tempDir, "readonly.txt")
	require.NoError(t, os.WriteFile(readOnlyFile, []byte("readonly"), 0400))

	tests := []struct {
		name          string
		path          string
		fieldName     string
		requirements  PathRequirements
		expectValid   bool
		expectedError string
		errorCode     string
	}{
		{
			name:      "valid existing file",
			path:      testFile,
			fieldName: "file",
			requirements: PathRequirements{
				Required:  true,
				MustExist: true,
			},
			expectValid: true,
		},
		{
			name:      "valid existing directory",
			path:      testDir,
			fieldName: "directory",
			requirements: PathRequirements{
				Required:        true,
				MustExist:       true,
				MustBeDirectory: true,
			},
			expectValid: true,
		},
		{
			name:      "empty path when required",
			path:      "",
			fieldName: "path",
			requirements: PathRequirements{
				Required: true,
			},
			expectValid:   false,
			expectedError: "Path field 'path' is required",
			errorCode:     "PATH_REQUIRED",
		},
		{
			name:      "non-existent path when must exist",
			path:      filepath.Join(tempDir, "nonexistent.txt"),
			fieldName: "path",
			requirements: PathRequirements{
				MustExist: true,
			},
			expectValid:   false,
			expectedError: "Path does not exist",
			errorCode:     "PATH_NOT_FOUND",
		},
		{
			name:      "file when expecting directory",
			path:      testFile,
			fieldName: "directory",
			requirements: PathRequirements{
				MustExist:       true,
				MustBeDirectory: true,
			},
			expectValid:   false,
			expectedError: "Path must be a directory",
			errorCode:     "PATH_MUST_BE_DIRECTORY",
		},
		{
			name:      "directory when expecting file",
			path:      testDir,
			fieldName: "file",
			requirements: PathRequirements{
				MustExist:  true,
				MustBeFile: true,
			},
			expectValid:   false,
			expectedError: "Path must be a file",
			errorCode:     "PATH_MUST_BE_FILE",
		},
		{
			name:      "path traversal attempt",
			path:      filepath.Join(tempDir, "..", "..", "etc", "passwd"),
			fieldName: "path",
			requirements: PathRequirements{
				Required: true,
			},
			expectValid: true, // Path traversal is allowed, just cleaned
		},
		{
			name:      "absolute path",
			path:      "/tmp/test",
			fieldName: "path",
			requirements: PathRequirements{
				Required: true,
			},
			expectValid: true,
		},
		{
			name:      "path with redundant elements",
			path:      filepath.Join(tempDir, ".", "test", "..", "test.txt"),
			fieldName: "path",
			requirements: PathRequirements{
				MustExist: true,
			},
			expectValid: true, // Should be cleaned and validated
		},
	}

	svm := NewStandardizedValidationMixin(zerolog.Nop())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svm.StandardValidatePath(tt.path, tt.fieldName, tt.requirements)

			assert.Equal(t, tt.expectValid, result.Valid)

			if !tt.expectValid {
				assert.True(t, result.HasErrors())
				if tt.expectedError != "" {
					firstError := result.GetFirstError()
					assert.NotNil(t, firstError)
					assert.Contains(t, firstError.Message, tt.expectedError)
					if tt.errorCode != "" {
						assert.Equal(t, tt.errorCode, firstError.Code)
					}
				}
			}
		})
	}
}

func TestStandardizedValidationMixin_StandardValidateRequiredFields(t *testing.T) {
	type TestStruct struct {
		Name     string
		Age      int
		Email    string
		Optional *string
	}

	tests := []struct {
		name          string
		args          interface{}
		requiredTags  []string
		expectValid   bool
		expectedError string
	}{
		{
			name: "all required fields present",
			args: TestStruct{
				Name:  "John",
				Age:   30,
				Email: "john@example.com",
			},
			requiredTags: []string{"Name", "Email"},
			expectValid:  true,
		},
		{
			name: "missing required field",
			args: TestStruct{
				Name: "John",
				Age:  30,
			},
			requiredTags:  []string{"Name", "Email"},
			expectValid:   false,
			expectedError: "Required field 'Email' cannot be empty",
		},
		{
			name: "empty required string",
			args: TestStruct{
				Name:  "",
				Age:   30,
				Email: "john@example.com",
			},
			requiredTags:  []string{"Name", "Email"},
			expectValid:   false,
			expectedError: "Required field 'Name' cannot be empty",
		},
		{
			name: "zero value int not checked",
			args: TestStruct{
				Name:  "John",
				Age:   0,
				Email: "john@example.com",
			},
			requiredTags:  []string{"Name", "Email", "Age"},
			expectValid:   false, // Zero int values ARE considered empty by isEmptyValue
			expectedError: "Required field 'Age' cannot be empty",
		},
		{
			name: "nil pointer field",
			args: TestStruct{
				Name:     "John",
				Email:    "john@example.com",
				Optional: nil,
			},
			requiredTags:  []string{"Name", "Email", "Optional"},
			expectValid:   false,
			expectedError: "Required field 'Optional' cannot be empty",
		},
		{
			name:         "empty required tags",
			args:         TestStruct{Name: "John"},
			requiredTags: []string{},
			expectValid:  true,
		},
		{
			name:          "non-existent field",
			args:          TestStruct{Name: "John"},
			requiredTags:  []string{"NonExistent"},
			expectValid:   false, // Non-existent fields cause an error
			expectedError: "Required field 'NonExistent' not found",
		},
	}

	svm := NewStandardizedValidationMixin(zerolog.Nop())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svm.StandardValidateRequiredFields(tt.args, tt.requiredTags)

			assert.Equal(t, tt.expectValid, result.Valid)

			if !tt.expectValid {
				assert.True(t, result.HasErrors())
				if tt.expectedError != "" {
					firstError := result.GetFirstError()
					assert.NotNil(t, firstError)
					assert.Contains(t, firstError.Message, tt.expectedError)
				}
			}
		})
	}
}

func TestValidationResult_Methods(t *testing.T) {
	t.Run("AddError", func(t *testing.T) {
		result := &ValidationResult{Valid: true}

		result.AddError("field1", "error message", "ERROR_CODE", "value1")

		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "field1", result.Errors[0].Field)
		assert.Equal(t, "error message", result.Errors[0].Message)
		assert.Equal(t, "ERROR_CODE", result.Errors[0].Code)
		assert.Equal(t, "high", result.Errors[0].Severity)
	})

	t.Run("AddWarning", func(t *testing.T) {
		result := &ValidationResult{Valid: true}

		result.AddWarning("field2", "warning message", "WARN_CODE", "value2")

		assert.True(t, result.Valid) // Warnings don't invalidate
		assert.Len(t, result.Warnings, 1)
		assert.Equal(t, "field2", result.Warnings[0].Field)
		assert.Equal(t, "warning message", result.Warnings[0].Message)
		assert.Equal(t, "WARN_CODE", result.Warnings[0].Code)
		assert.Equal(t, "medium", result.Warnings[0].Severity)
	})

	t.Run("AddInfo", func(t *testing.T) {
		result := &ValidationResult{Valid: true}

		result.AddInfo("field3", "info message", "INFO_CODE", "value3")

		assert.True(t, result.Valid) // Info doesn't invalidate
		assert.Len(t, result.Info, 1)
		assert.Equal(t, "field3", result.Info[0].Field)
		assert.Equal(t, "info message", result.Info[0].Message)
		assert.Equal(t, "INFO_CODE", result.Info[0].Code)
		assert.Equal(t, "low", result.Info[0].Severity)
	})

	t.Run("HasErrors", func(t *testing.T) {
		result := &ValidationResult{Valid: true}

		assert.False(t, result.HasErrors())

		result.AddError("field", "error", "CODE", nil)
		assert.True(t, result.HasErrors())
	})

	t.Run("GetFirstError", func(t *testing.T) {
		result := &ValidationResult{Valid: true}

		assert.Nil(t, result.GetFirstError())

		result.AddError("field1", "error1", "CODE1", nil)
		result.AddError("field2", "error2", "CODE2", nil)

		firstError := result.GetFirstError()
		assert.NotNil(t, firstError)
		assert.Equal(t, "field1", firstError.Field)
		assert.Equal(t, "error1", firstError.Message)
	})
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{
		Field:   "testField",
		Message: "test error message",
		Code:    "TEST_ERROR",
	}

	errMsg := ve.Error()
	assert.Equal(t, "validation failed for field 'testField': test error message", errMsg)
}
