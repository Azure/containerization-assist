package testutil

import (
	"testing"
)

type TestStruct struct {
	Success bool
	Error   string
	Value   int
}

func TestGetFieldValue(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		fieldName string
		wantValue interface{}
		wantOk    bool
	}{
		{
			name: "get bool field from struct",
			input: TestStruct{
				Success: true,
				Error:   "test error",
				Value:   42,
			},
			fieldName: "Success",
			wantValue: true,
			wantOk:    true,
		},
		{
			name: "get string field from struct",
			input: TestStruct{
				Success: false,
				Error:   "validation failed",
				Value:   100,
			},
			fieldName: "Error",
			wantValue: "validation failed",
			wantOk:    true,
		},
		{
			name: "get int field from struct",
			input: TestStruct{
				Success: true,
				Error:   "",
				Value:   999,
			},
			fieldName: "Value",
			wantValue: 999,
			wantOk:    true,
		},
		{
			name: "get field from pointer to struct",
			input: &TestStruct{
				Success: true,
				Error:   "pointer test",
				Value:   123,
			},
			fieldName: "Error",
			wantValue: "pointer test",
			wantOk:    true,
		},
		{
			name:      "field not found",
			input:     TestStruct{Success: true},
			fieldName: "NonExistent",
			wantValue: nil,
			wantOk:    false,
		},
		{
			name:      "nil pointer",
			input:     (*TestStruct)(nil),
			fieldName: "Success",
			wantValue: nil,
			wantOk:    false,
		},
		{
			name:      "non-struct type",
			input:     "not a struct",
			fieldName: "Success",
			wantValue: nil,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := getFieldValue(tt.input, tt.fieldName)
			if gotOk != tt.wantOk {
				t.Errorf("getFieldValue() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
			if gotValue != tt.wantValue {
				t.Errorf("getFieldValue() gotValue = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestAssertSuccessWithGetFieldValue(t *testing.T) {
	// Test that AssertSuccess uses getFieldValue correctly
	type SuccessResult struct {
		Success bool
		Error   string
	}

	t.Run("success case", func(t *testing.T) {
		mockT := &testing.T{}
		result := SuccessResult{Success: true}
		AssertSuccess(mockT, result, nil)
		// If getFieldValue works, this should not fail
	})
}
