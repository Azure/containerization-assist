package common

import (
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

func TestResult(t *testing.T) {
	// Test successful result
	result := NewResult("test data")
	if !result.IsOk() {
		t.Error("Expected result to be Ok")
	}
	if result.Unwrap() != "test data" {
		t.Error("Expected data to be 'test data'")
	}

	// Test error result
	errResult := NewError[string](errors.Validation("test", "test error"))
	if errResult.IsOk() {
		t.Error("Expected result to be error")
	}
	if errResult.UnwrapOr("default") != "default" {
		t.Error("Expected default value")
	}
}

func TestOption(t *testing.T) {
	// Test Some
	some := Some("value")
	if !some.IsSome() {
		t.Error("Expected Some to be some")
	}
	if some.Unwrap() != "value" {
		t.Error("Expected value to be 'value'")
	}

	// Test None
	none := None[string]()
	if !none.IsNone() {
		t.Error("Expected None to be none")
	}
	if none.UnwrapOr("default") != "default" {
		t.Error("Expected default value")
	}
}

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache[string, int]()

	// Test Set and Get
	err := cache.Set("key1", 42)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	value, ok := cache.Get("key1")
	if !ok {
		t.Error("Expected key to exist")
	}
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}

	// Test missing key
	_, ok = cache.Get("missing")
	if ok {
		t.Error("Expected key to not exist")
	}

	// Test Size
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	// Test Delete
	err = cache.Delete("key1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}
}

func TestValidationResult(t *testing.T) {
	result := &ValidationResult[string]{
		Valid: true,
		Data:  "test data",
	}

	if !result.IsValid() {
		t.Error("Expected result to be valid")
	}

	result.AddError("test_error", "Test error message", map[string]string{
		"field": "test_field",
	})

	if result.IsValid() {
		t.Error("Expected result to be invalid after adding error")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

func TestSliceUtilities(t *testing.T) {
	// Test Map
	numbers := []int{1, 2, 3, 4, 5}
	doubled := Map(numbers, func(x int) int { return x * 2 })
	expected := []int{2, 4, 6, 8, 10}

	for i, v := range doubled {
		if v != expected[i] {
			t.Errorf("Expected %d, got %d at index %d", expected[i], v, i)
		}
	}

	// Test Filter
	evens := Filter(numbers, func(x int) bool { return x%2 == 0 })
	expectedEvens := []int{2, 4}

	if len(evens) != len(expectedEvens) {
		t.Errorf("Expected %d evens, got %d", len(expectedEvens), len(evens))
	}

	// Test Find
	found := Find(numbers, func(x int) bool { return x > 3 })
	if !found.IsSome() {
		t.Error("Expected to find a value")
	}
	if found.Unwrap() != 4 {
		t.Errorf("Expected 4, got %d", found.Unwrap())
	}

	// Test Contains
	if !Contains(numbers, 3) {
		t.Error("Expected slice to contain 3")
	}
	if Contains(numbers, 10) {
		t.Error("Expected slice to not contain 10")
	}

	// Test Reduce
	sum := Reduce(numbers, 0, func(acc, x int) int { return acc + x })
	if sum != 15 {
		t.Errorf("Expected sum to be 15, got %d", sum)
	}
}

func TestToolResult(t *testing.T) {
	// Test successful result
	result := NewToolResult("success data")
	if !result.Success {
		t.Error("Expected result to be successful")
	}
	if result.Data != "success data" {
		t.Error("Expected data to be 'success data'")
	}

	// Test error result
	errResult := NewToolError[string](errors.Validation("test", "tool error"))
	if errResult.Success {
		t.Error("Expected result to be unsuccessful")
	}
	if !strings.Contains(errResult.Error, "tool error") {
		t.Errorf("Expected error message to contain 'tool error', got '%s'", errResult.Error)
	}
}
