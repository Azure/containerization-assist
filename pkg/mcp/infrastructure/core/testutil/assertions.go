package testutil

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// AssertSuccess verifies that an operation completed successfully
func AssertSuccess(t *testing.T, result interface{}, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check if result has a Success field
	if successField, ok := getFieldValue(result, "Success"); ok {
		if success, ok := successField.(bool); ok && !success {
			if errorField, ok := getFieldValue(result, "Error"); ok {
				t.Fatalf("Expected success=true, got false. Error: %v", errorField)
			} else {
				t.Fatalf("Expected success=true, got false")
			}
		}
	}
}

// AssertFailure verifies that an operation failed with expected error
func AssertFailure(t *testing.T, result interface{}, err error, expectedError string) {
	t.Helper()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error containing %q, got: %v", expectedError, err)
	}
}

// AssertRichError verifies Rich error properties
func AssertRichError(t *testing.T, err error, expectedCode errors.Code) {
	t.Helper()

	richErr, ok := err.(*errors.Rich)
	if !ok {
		t.Fatalf("Expected Rich error, got %T", err)
	}

	if richErr.Code != expectedCode {
		t.Errorf("Expected code %v, got %v", expectedCode, richErr.Code)
	}
}

// getFieldValue uses reflection to get a field value from an interface
func getFieldValue(v interface{}, fieldName string) (interface{}, bool) {
	rv := reflect.ValueOf(v)

	// Handle pointer types
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}

	// Check if it's a struct
	if rv.Kind() != reflect.Struct {
		return nil, false
	}

	// Look for the field
	field := rv.FieldByName(fieldName)
	if !field.IsValid() {
		return nil, false
	}

	// Return the field value
	return field.Interface(), true
}

// AssertDuration verifies that a duration is within expected bounds
func AssertDuration(t *testing.T, actual, expected, tolerance time.Duration) {
	t.Helper()

	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}

	if diff > tolerance {
		t.Errorf("Duration %v not within %v of expected %v", actual, tolerance, expected)
	}
}

// AssertImageRef verifies a Docker image reference format
func AssertImageRef(t *testing.T, imageRef string) {
	t.Helper()

	if imageRef == "" {
		t.Error("Image reference is empty")
		return
	}

	// Basic validation - should contain registry/repo:tag or repo:tag
	parts := strings.Split(imageRef, ":")
	if len(parts) != 2 {
		t.Errorf("Invalid image reference format: %s", imageRef)
	}
}

// AssertEventPublished verifies that an event was published
func AssertEventPublished(t *testing.T, events []interface{}, eventType string) {
	t.Helper()

	found := false
	for _, event := range events {
		if e, ok := event.(interface{ EventType() string }); ok {
			if e.EventType() == eventType {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("Event type %q not found in published events", eventType)
	}
}

// AssertNoError is a simple helper for checking no error occurred
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// AssertError is a simple helper for checking an error occurred
func AssertError(t *testing.T, err error, context string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s: expected error, got nil", context)
	}
}

// AssertContains verifies a string contains a substring
func AssertContains(t *testing.T, haystack, needle, context string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected to contain %q, got %q", context, needle, haystack)
	}
}

// AssertMapHasKey verifies a map contains a key
func AssertMapHasKey(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()

	if _, exists := m[key]; !exists {
		t.Errorf("Map missing expected key %q", key)
	}
}
