package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/rs/zerolog"
)

// StandardizedValidationMixin provides consistent validation patterns across all atomic tools
type StandardizedValidationMixin struct {
	logger zerolog.Logger
}

// NewStandardizedValidationMixin creates a new standardized validation mixin
func NewStandardizedValidationMixin(logger zerolog.Logger) *StandardizedValidationMixin {
	return &StandardizedValidationMixin{
		logger: logger.With().Str("component", "validation_mixin").Logger(),
	}
}

// ValidatedSession contains session information that has been validated
type ValidatedSession struct {
	ID           string
	WorkspaceDir string
	Session      interface{} // The actual session object
}

// ValidationError represents a standardized validation error
type ValidationError struct {
	Field       string            `json:"field"`
	Value       interface{}       `json:"value"`
	Constraint  string            `json:"constraint"`
	Message     string            `json:"message"`
	Code        string            `json:"code"`
	Severity    string            `json:"severity"`
	Context     map[string]string `json:"context"`
	Suggestions []string          `json:"suggestions"`
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", ve.Field, ve.Message)
}

// ValidationResult contains the results of validation
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors"`
	Warnings []*ValidationError `json:"warnings"`
	Info     []*ValidationError `json:"info"`
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(field, message, code string, value interface{}) {
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:    field,
		Value:    value,
		Message:  message,
		Code:     code,
		Severity: "high",
	})
	vr.Valid = false
}

// AddWarning adds a validation warning
func (vr *ValidationResult) AddWarning(field, message, code string, value interface{}) {
	vr.Warnings = append(vr.Warnings, &ValidationError{
		Field:    field,
		Value:    value,
		Message:  message,
		Code:     code,
		Severity: "medium",
	})
}

// AddInfo adds validation info
func (vr *ValidationResult) AddInfo(field, message, code string, value interface{}) {
	vr.Info = append(vr.Info, &ValidationError{
		Field:    field,
		Value:    value,
		Message:  message,
		Code:     code,
		Severity: "low",
	})
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// GetFirstError returns the first validation error or nil
func (vr *ValidationResult) GetFirstError() *ValidationError {
	if len(vr.Errors) > 0 {
		return vr.Errors[0]
	}
	return nil
}

// StandardValidateSession performs standard session validation
func (svm *StandardizedValidationMixin) StandardValidateSession(
	ctx context.Context,
	sessionManager interface{ GetSession(sessionID string) (interface{}, error) },
	sessionID string,
) (*ValidatedSession, error) {
	// Basic validation
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("INVALID_INPUT: session_id is required and cannot be empty")
	}

	// Get session
	session, err := sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("SESSION_NOT_FOUND: Failed to get session: %v", err)
	}

	// Get workspace directory using reflection or interface
	workspaceDir := ""
	if sessionWithWorkspace, ok := session.(interface{ GetWorkspaceDir() string }); ok {
		workspaceDir = sessionWithWorkspace.GetWorkspaceDir()
	} else {
		// Fallback: try reflection to extract SessionID field for workspace calculation
		if sessionStruct, ok := session.(*sessiontypes.SessionState); ok {
			workspaceDir = filepath.Join("/tmp", "sessions", sessionStruct.SessionID)
		}
	}

	return &ValidatedSession{
		ID:           sessionID,
		WorkspaceDir: workspaceDir,
		Session:      session,
	}, nil
}

// StandardValidateRequiredFields validates required fields using reflection
func (svm *StandardizedValidationMixin) StandardValidateRequiredFields(
	args interface{},
	requiredFields []string,
) *ValidationResult {
	result := &ValidationResult{Valid: true}

	argValue := reflect.ValueOf(args)
	if argValue.Kind() == reflect.Ptr {
		argValue = argValue.Elem()
	}

	for _, fieldName := range requiredFields {
		field := argValue.FieldByName(fieldName)
		if !field.IsValid() {
			result.AddError(
				fieldName,
				fmt.Sprintf("Required field '%s' not found", fieldName),
				"FIELD_NOT_FOUND",
				nil,
			)
			continue
		}

		if svm.isEmptyValue(field) {
			result.AddError(
				fieldName,
				fmt.Sprintf("Required field '%s' cannot be empty", fieldName),
				"FIELD_REQUIRED",
				field.Interface(),
			)
		}
	}

	return result
}

// StandardValidatePath validates file/directory paths
func (svm *StandardizedValidationMixin) StandardValidatePath(
	path, fieldName string,
	requirements PathRequirements,
) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if path == "" {
		if requirements.Required {
			result.AddError(
				fieldName,
				fmt.Sprintf("Path field '%s' is required", fieldName),
				"PATH_REQUIRED",
				path,
			)
		}
		return result
	}

	// Clean and validate path
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		result.AddWarning(
			fieldName,
			fmt.Sprintf("Path contains redundant elements, cleaned to: %s", cleanPath),
			"PATH_CLEANED",
			path,
		)
	}

	// Check if path exists
	stat, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			if requirements.MustExist {
				result.AddError(
					fieldName,
					fmt.Sprintf("Path does not exist: %s", cleanPath),
					"PATH_NOT_FOUND",
					cleanPath,
				)
			}
		} else {
			result.AddError(
				fieldName,
				fmt.Sprintf("Cannot access path: %s (%v)", cleanPath, err),
				"PATH_ACCESS_ERROR",
				cleanPath,
			)
		}
		return result
	}

	// Validate path type
	if requirements.MustBeFile && stat.IsDir() {
		result.AddError(
			fieldName,
			fmt.Sprintf("Path must be a file, but is a directory: %s", cleanPath),
			"PATH_MUST_BE_FILE",
			cleanPath,
		)
	}

	if requirements.MustBeDirectory && !stat.IsDir() {
		result.AddError(
			fieldName,
			fmt.Sprintf("Path must be a directory, but is a file: %s", cleanPath),
			"PATH_MUST_BE_DIRECTORY",
			cleanPath,
		)
	}

	// Check permissions
	if requirements.MustBeReadable {
		if err := svm.checkReadPermission(cleanPath); err != nil {
			result.AddError(
				fieldName,
				fmt.Sprintf("Path is not readable: %s (%v)", cleanPath, err),
				"PATH_NOT_READABLE",
				cleanPath,
			)
		}
	}

	if requirements.MustBeWritable {
		if err := svm.checkWritePermission(cleanPath); err != nil {
			result.AddError(
				fieldName,
				fmt.Sprintf("Path is not writable: %s (%v)", cleanPath, err),
				"PATH_NOT_WRITABLE",
				cleanPath,
			)
		}
	}

	return result
}

// PathRequirements defines requirements for path validation
type PathRequirements struct {
	Required          bool
	MustExist         bool
	MustBeFile        bool
	MustBeDirectory   bool
	MustBeReadable    bool
	MustBeWritable    bool
	AllowedExtensions []string
}

// StandardValidateImageRef validates Docker image references
func (svm *StandardizedValidationMixin) StandardValidateImageRef(
	imageRef, fieldName string,
) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if imageRef == "" {
		result.AddError(
			fieldName,
			"Image reference cannot be empty",
			"IMAGE_REF_REQUIRED",
			imageRef,
		)
		return result
	}

	// Basic format validation
	parts := strings.Split(imageRef, ":")
	if len(parts) < 2 {
		result.AddError(
			fieldName,
			"Image reference must include a tag (e.g., image:tag)",
			"IMAGE_REF_NO_TAG",
			imageRef,
		)
	}

	// Validate image name part
	imageName := parts[0]
	if imageName == "" {
		result.AddError(
			fieldName,
			"Image name cannot be empty",
			"IMAGE_NAME_EMPTY",
			imageRef,
		)
	}

	// Validate tag part
	if len(parts) >= 2 {
		tag := parts[1]
		if tag == "" {
			result.AddError(
				fieldName,
				"Image tag cannot be empty",
				"IMAGE_TAG_EMPTY",
				imageRef,
			)
		}
	}

	return result
}

// ConvertValidationToRichError converts a ValidationResult to a RichError
func (svm *StandardizedValidationMixin) ConvertValidationToRichError(
	result *ValidationResult,
	operation, stage string,
) *types.RichError {
	if result.Valid {
		return nil
	}

	firstError := result.GetFirstError()
	if firstError == nil {
		return nil
	}

	// Create a RichError using the types package instead of the errors package
	builtError := types.NewRichError(firstError.Code, firstError.Message, types.ErrTypeValidation)

	// Manually add context information
	builtError.Context.Operation = operation
	builtError.Context.Stage = stage

	// Add diagnostics for all errors
	for i, validationError := range result.Errors {
		if builtError.Context.Metadata == nil {
			builtError.Context.Metadata = types.NewErrorMetadata("", "", "")
		}
		builtError.Context.Metadata.AddCustom(fmt.Sprintf("validation_error_%d", i), fmt.Sprintf("Field: %s, Error: %s", validationError.Field, validationError.Message))
	}

	// Add resolution steps
	if len(result.Errors) > 0 {
		builtError.Resolution.ImmediateSteps = append(builtError.Resolution.ImmediateSteps,
			types.ResolutionStep{
				Order:       1,
				Action:      "Check input parameters",
				Description: "Check input parameters for correctness",
				Expected:    "All parameters should be valid",
			},
			types.ResolutionStep{
				Order:       2,
				Action:      "Provide required fields",
				Description: "Ensure all required fields are provided",
				Expected:    "All required fields should have valid values",
			},
		)
		if len(result.Errors) > 1 {
			builtError.Resolution.ImmediateSteps = append(builtError.Resolution.ImmediateSteps,
				types.ResolutionStep{
					Order:       3,
					Action:      "Fix validation errors",
					Description: fmt.Sprintf("Fix all %d validation errors", len(result.Errors)),
					Expected:    "All validation errors should be resolved",
				},
			)
		}
	}

	return builtError
}

// Helper methods

func (svm *StandardizedValidationMixin) isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Bool:
		return false // booleans are never "empty"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	default:
		return false
	}
}

func (svm *StandardizedValidationMixin) checkReadPermission(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func (svm *StandardizedValidationMixin) checkWritePermission(path string) error {
	// For directories, try to create a temp file
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		tempFile := filepath.Join(path, ".write_test")
		file, err := os.Create(tempFile)
		if err != nil {
			return err
		}
		file.Close()
		os.Remove(tempFile)
		return nil
	}

	// For files, try to open for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

