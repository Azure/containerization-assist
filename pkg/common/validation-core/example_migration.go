package validation

import (
	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Example of migrating from reflection-based to generic validation

// ===== OLD APPROACH (using reflection) =====

// Before: Using ValidateRequiredFields with reflection
type OldToolArgs struct {
	SessionID string            `json:"session_id"`
	ImageRef  string            `json:"image_ref"`
	BuildArgs map[string]string `json:"build_args,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
}

// Old validation approach would use reflection:
// err := validation.ValidateRequiredFields(args)

// ===== NEW APPROACH (type-safe, no reflection) =====

// After: Using type-safe validation
type NewToolArgs struct {
	SessionID string            `json:"session_id"`
	ImageRef  string            `json:"image_ref"`
	BuildArgs map[string]string `json:"build_args,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
}

// Implement validation method on the struct
func (args *NewToolArgs) Validate() error {
	sv := NewStructValidator()

	// Validate required fields
	sv.ValidateString("session_id", args.SessionID, true)
	sv.ValidateString("image_ref", args.ImageRef, true)

	// Validate optional fields with patterns
	if args.ImageRef != "" {
		sv.ValidateStringWithPattern("image_ref", args.ImageRef, false,
			func(s string) bool {
				// Docker image reference validation
				return len(s) > 0 && len(s) < 256
			},
			"valid Docker image reference (max 256 chars)",
		)
	}

	// Validate maps and slices
	sv.ValidateMap("build_args", len(args.BuildArgs), false)
	sv.ValidateSlice("tags", len(args.Tags), false)

	return sv.GetError()
}

// Example of using the new validation
func ExampleUsage() error {
	args := &NewToolArgs{
		SessionID: "123",
		ImageRef:  "nginx:latest",
	}

	// Simple one-liner validation
	if err := args.Validate(); err != nil {
		return mcperrors.NewError().Messagef("validation failed: %w", err).WithLocation().Build(

		// ===== MIGRATION PATTERN =====
		)
	}

	return nil
}

// For structs that don't implement Validate(), use the generic approach:
func ValidateAnyStruct(sessionID, imageRef string, buildArgs map[string]string) error {
	return ValidateRequiredFieldsGeneric(func(sv *StructValidator) {
		sv.ValidateString("session_id", sessionID, true)
		sv.ValidateString("image_ref", imageRef, true)
		sv.ValidateMap("build_args", len(buildArgs), false)
	})
}

// ===== BENEFITS =====
// 1. No reflection overhead
// 2. Type-safe at compile time
// 3. Clear validation logic
// 4. Better performance
// 5. Easier to test and debug
