package mcperror

import (
	"fmt"

	v20250326 "github.com/localrivet/gomcp/mcp/v20250326"
)

// ExampleUsage demonstrates how to use the mcperror package
func ExampleUsage() {
	// Example 1: Creating a simple MCP error
	err1 := New(v20250326.ErrorCodeInvalidArguments, "invalid image name format")
	fmt.Printf("Simple error: %v\n", err1)

	// Example 2: Creating an error with structured data
	err2 := NewWithData(v20250326.ErrorCodeInvalidArguments, "missing required field", map[string]interface{}{
		"field":          "image_name",
		"provided_value": "",
	})
	fmt.Printf("Error with data: %v\n", err2)

	// Example 3: Using convenience functions
	err3 := NewSessionNotFound("abc123")
	fmt.Printf("Session error: %v\n", err3)

	err4 := NewBuildFailed("dockerfile syntax error on line 15")
	fmt.Printf("Build error: %v\n", err4)

	// Example 4: Converting a regular Go error to MCP error
	regularErr := fmt.Errorf("connection timeout")
	mcpErr := FromError(regularErr)
	fmt.Printf("Converted error: %v (code: %s)\n", mcpErr, mcpErr.Code)

	// Example 5: Checking error types
	if IsSessionError(err3) {
		fmt.Println("err3 is a session error")
	}

	if IsBuildError(err4) {
		fmt.Println("err4 is a build error")
	}

	// Example 6: Getting error category information
	if category, ok := GetErrorCategory(err2.Code); ok {
		fmt.Printf("Error category: %s\n", category.Name)
		fmt.Printf("Retryable: %t\n", category.Retryable)
		fmt.Printf("Recovery steps: %v\n", category.RecoverySteps)
	}

	// Example 7: Using error in MCP response
	errorResponse := err1.ToMCPErrorResponse("request-123")
	fmt.Printf("MCP error response: %+v\n", errorResponse)
}

// ExampleToolUsage shows how to use mcperror in a tool function
func ExampleToolUsage(sessionID, imageName string) error {
	// Validate inputs
	if sessionID == "" {
		return NewRequiredFieldMissing("session_id")
	}

	if imageName == "" {
		return NewRequiredFieldMissing("image_name")
	}

	// Simulate some business logic
	if sessionID == "invalid" {
		return NewSessionNotFound(sessionID)
	}

	if imageName == "bad-format" {
		return NewWithData(v20250326.ErrorCodeInvalidArguments, "invalid image name format", map[string]interface{}{
			"image_name": imageName,
			"reason":     "contains invalid characters",
		})
	}

	// Simulate a build failure
	if imageName == "fail-build" {
		return NewBuildFailed("missing base image")
	}

	return nil
}

// ExampleErrorHandling demonstrates error handling patterns
func ExampleErrorHandling() {
	err := ExampleToolUsage("invalid", "my-app")

	if err != nil {
		// Convert to MCP error if needed
		mcpErr := FromError(err)

		// Get user-friendly message
		message := GetUserFriendlyMessage(mcpErr)
		fmt.Printf("User message: %s\n", message)

		// Check if retryable
		if ShouldRetry(mcpErr) {
			fmt.Println("This error can be retried")
		} else {
			fmt.Println("This error requires manual intervention")
		}

		// Get recovery steps
		steps := GetRecoverySteps(mcpErr)
		fmt.Printf("Recovery steps: %v\n", steps)

		// Handle specific error types
		if IsSessionError(err) {
			fmt.Println("Handling session error...")
			// Maybe create a new session
		} else if IsValidationError(err) {
			fmt.Println("Handling validation error...")
			// Maybe prompt user for correct input
		}
	}
}
