package utils

import (
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// Example showing how to replace reflection-based validation with type-safe validation
// This file demonstrates the pattern for the AtomicAnalyzeRepositoryArgs struct

// AtomicAnalyzeRepositoryArgs with validation methods (example)
type ExampleAtomicAnalyzeRepositoryArgs struct {
	types.BaseToolArgs
	RepoURL      string `json:"repo_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch       string `json:"branch,omitempty" description:"Git branch to analyze (default: main)"`
	Context      string `json:"context,omitempty" description:"Additional context about the application"`
	LanguageHint string `json:"language_hint,omitempty" description:"Primary programming language hint"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
}

// ValidateRequired implements type-safe required field validation
// This replaces the reflection-based StandardValidateRequiredFields method
func (args *ExampleAtomicAnalyzeRepositoryArgs) ValidateRequired() error {
	validator := NewTypedValidator(zerolog.Nop()) // In real usage, pass actual logger

	// Validate required fields with type safety
	if err := validator.ValidateString(args.SessionID, "session_id", true); err != nil {
		return err
	}

	if err := validator.ValidateString(args.RepoURL, "repo_url", true); err != nil {
		return err
	}

	return nil
}

// ValidatePaths implements type-safe path validation
// This replaces the reflection-based path validation methods
func (args *ExampleAtomicAnalyzeRepositoryArgs) ValidatePaths() error {
	validator := NewTypedValidator(zerolog.Nop()) // In real usage, pass actual logger

	// If RepoURL is a local path, validate it
	if !IsURL(args.RepoURL) {
		pathReqs := PathRequirements{
			Required:        true,
			MustExist:       true,
			MustBeDirectory: true,
			MustBeReadable:  true,
		}
		if err := validator.ValidatePath(args.RepoURL, "repo_url", pathReqs); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFormat implements type-safe format validation
// This replaces reflection-based format checking
func (args *ExampleAtomicAnalyzeRepositoryArgs) ValidateFormat() error {
	validator := NewTypedValidator(zerolog.Nop()) // In real usage, pass actual logger

	// Validate RepoURL format (either valid URL or valid local path)
	if IsURL(args.RepoURL) {
		if err := validator.ValidateString(args.RepoURL, "repo_url", true, ValidURL()); err != nil {
			return err
		}
	}
	// Local path validation is handled in ValidatePaths for non-URL cases

	// Validate optional fields if present
	if args.Branch != "" {
		if err := validator.ValidateString(args.Branch, "branch", false,
			MinLength(1),
			MaxLength(100),
			ContainsNoUnsafeChars(),
		); err != nil {
			return err
		}
	}

	if args.LanguageHint != "" {
		if err := validator.ValidateString(args.LanguageHint, "language_hint", false,
			MinLength(1),
			MaxLength(50),
			ContainsNoUnsafeChars(),
		); err != nil {
			return err
		}
	}

	return nil
}

// ValidateComplete performs complete validation
// This is the main entry point replacing reflection-based validation
func (args *ExampleAtomicAnalyzeRepositoryArgs) ValidateComplete() error {
	validator := NewTypedValidator(zerolog.Nop()) // In real usage, pass actual logger
	return validator.ValidateStruct(args)
}

// Example showing batch validation
func (args *ExampleAtomicAnalyzeRepositoryArgs) ValidateWithBatch() []error {
	validator := NewTypedValidator(zerolog.Nop()) // In real usage, pass actual logger

	return validator.BatchValidate(
		func() error { return args.ValidateRequired() },
		func() error { return args.ValidatePaths() },
		func() error { return args.ValidateFormat() },
	)
}

// Migration helper for existing atomic tools
// This shows how to replace reflection-based validation calls
func MigrateFromReflectionValidation() {
	// OLD WAY (reflection-based):
	// standardMixin := NewStandardizedValidationMixin(logger)
	// result := standardMixin.StandardValidateRequiredFields(args, []string{"session_id", "repo_url"})
	// if !result.Valid {
	//     return result.ToError()
	// }

	// NEW WAY (type-safe):
	args := &ExampleAtomicAnalyzeRepositoryArgs{
		// ... populate fields
	}

	if err := args.ValidateComplete(); err != nil {
		// Handle validation error
		_ = err
	}
}

// Example validation rules that can be reused across tools
var CommonValidationRules = struct {
	SessionID     func() error
	ImageName     func(string) error
	ContainerPort func(int) error
}{
	SessionID: func() error {
		// Common session ID validation logic
		return nil
	},
	ImageName: func(name string) error {
		validator := NewTypedValidator(zerolog.Nop())
		return validator.ValidateString(name, "image_name", true,
			MinLength(1),
			MaxLength(100),
			ValidDockerImage(),
		)
	},
	ContainerPort: func(port int) error {
		validator := NewTypedValidator(zerolog.Nop())
		return validator.ValidateInt(port, "port", true,
			MinValue(1),
			MaxValue(65535),
		)
	},
}
