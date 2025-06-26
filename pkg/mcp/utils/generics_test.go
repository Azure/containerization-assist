package utils

import (
	"errors"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

func TestMapGet(t *testing.T) {
	m := map[string]interface{}{
		"string": types.TestStringHello,
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"nil":    nil,
		"slice":  []string{"a", "b", "c"},
		"map":    map[string]int{"one": 1, "two": 2},
	}

	tests := []struct {
		name     string
		key      string
		want     interface{}
		wantOk   bool
		testFunc func() (interface{}, bool)
	}{
		{
			name:   "get string",
			key:    "string",
			want:   types.TestStringHello,
			wantOk: true,
			testFunc: func() (interface{}, bool) {
				return MapGet[string](m, "string")
			},
		},
		{
			name:   "get int",
			key:    "int",
			want:   42,
			wantOk: true,
			testFunc: func() (interface{}, bool) {
				return MapGet[int](m, "int")
			},
		},
		{
			name:   "get float",
			key:    "float",
			want:   3.14,
			wantOk: true,
			testFunc: func() (interface{}, bool) {
				return MapGet[float64](m, "float")
			},
		},
		{
			name:   "get bool",
			key:    "bool",
			want:   true,
			wantOk: true,
			testFunc: func() (interface{}, bool) {
				return MapGet[bool](m, "bool")
			},
		},
		{
			name:   "get non-existent key",
			key:    "notfound",
			want:   "",
			wantOk: false,
			testFunc: func() (interface{}, bool) {
				return MapGet[string](m, "notfound")
			},
		},
		{
			name:   "get wrong type",
			key:    "string",
			want:   0,
			wantOk: false,
			testFunc: func() (interface{}, bool) {
				return MapGet[int](m, "string")
			},
		},
		{
			name:   "get nil value",
			key:    "nil",
			want:   "",
			wantOk: false,
			testFunc: func() (interface{}, bool) {
				return MapGet[string](m, "nil")
			},
		},
		{
			name:   "get slice",
			key:    "slice",
			want:   []string{"a", "b", "c"},
			wantOk: true,
			testFunc: func() (interface{}, bool) {
				return MapGet[[]string](m, "slice")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tt.testFunc()

			if ok != tt.wantOk {
				t.Errorf("MapGet() ok = %v, want %v", ok, tt.wantOk)
			}

			if tt.wantOk {
				switch want := tt.want.(type) {
				case []string:
					gotSlice, ok := got.([]string)
					if !ok {
						t.Errorf("MapGet() returned wrong type, got %T", got)
					} else if len(gotSlice) != len(want) {
						t.Errorf("MapGet() got = %v, want %v", got, tt.want)
					}
				default:
					if got != tt.want {
						t.Errorf("MapGet() got = %v, want %v", got, tt.want)
					}
				}
			}
		})
	}
}

func TestMapGetWithDefault(t *testing.T) {
	m := map[string]interface{}{
		"exists": "value",
		"number": 42,
	}

	// Test existing key
	got := MapGetWithDefault(m, "exists", "default")
	if got != "value" {
		t.Errorf("MapGetWithDefault() = %v, want %v", got, "value")
	}

	// Test non-existent key
	got = MapGetWithDefault(m, "notfound", "default")
	if got != "default" {
		t.Errorf("MapGetWithDefault() = %v, want %v", got, "default")
	}

	// Test wrong type
	gotInt := MapGetWithDefault(m, "exists", 99)
	if gotInt != 99 {
		t.Errorf("MapGetWithDefault() = %v, want %v", gotInt, 99)
	}
}

func TestSafeCast(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:    "cast string",
			value:   types.TestStringHello,
			want:    types.TestStringHello,
			wantErr: false,
		},
		{
			name:    "cast int",
			value:   42,
			want:    42,
			wantErr: false,
		},
		{
			name:    "cast nil",
			value:   nil,
			want:    "",
			wantErr: true,
		},
		{
			name:    "cast wrong type",
			value:   "string",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.want.(type) {
			case string:
				got, err := SafeCast[string](tt.value)
				if (err != nil) != tt.wantErr {
					t.Errorf("SafeCast() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("SafeCast() = %v, want %v", got, tt.want)
				}
			case int:
				got, err := SafeCast[int](tt.value)
				if (err != nil) != tt.wantErr {
					t.Errorf("SafeCast() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("SafeCast() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestOptional(t *testing.T) {
	// Test with value
	opt := NewOptional(types.TestStringHello)
	if !opt.IsPresent() {
		t.Error("Optional should be present")
	}

	val, ok := opt.Get()
	if !ok || val != types.TestStringHello {
		t.Errorf("Get() = %v, %v; want hello, true", val, ok)
	}

	if opt.OrElse("default") != types.TestStringHello {
		t.Error("OrElse should return the value when present")
	}

	// Test empty
	empty := EmptyOptional[string]()
	if empty.IsPresent() {
		t.Error("Empty optional should not be present")
	}

	if empty.OrElse("default") != "default" {
		t.Error("OrElse should return default when empty")
	}

	// Test OrElseGet
	called := false
	result := empty.OrElseGet(func() string {
		called = true
		return "generated"
	})

	if !called || result != "generated" {
		t.Error("OrElseGet should call provider function")
	}
}

func TestResult(t *testing.T) {
	// Test Ok
	ok := Ok("success")
	if !ok.IsOk() || ok.IsErr() {
		t.Error("Ok Result should be ok")
	}

	if value, isOk := ok.Get(); !isOk || value != "success" {
		t.Error("Get should return the value and true for ok result")
	}

	// Test Err
	err := Err[string](errors.New("failure"))
	if err.IsOk() || !err.IsErr() {
		t.Error("Err Result should be err")
	}

	if err.UnwrapOr("default") != "default" {
		t.Error("UnwrapOr should return default on error")
	}

	// Test Map
	mapped := ok.Map(func(s string) string {
		return s + "!"
	})

	if value, isOk := mapped.Get(); !isOk || value != "success!" {
		t.Error("Map should transform the value")
	}

	// Map on error should not transform
	errMapped := err.Map(func(s string) string {
		return s + "!"
	})

	if errMapped.IsOk() {
		t.Error("Mapping error Result should still be error")
	}
}

func TestSliceUtilities(t *testing.T) {
	// Test Filter
	nums := []int{1, 2, 3, 4, 5}
	evens := Filter(nums, func(n int) bool { return n%2 == 0 })

	if len(evens) != 2 || evens[0] != 2 || evens[1] != 4 {
		t.Errorf("Filter() = %v, want [2, 4]", evens)
	}

	// Test Map
	doubled := Map(nums, func(n int) int { return n * 2 })
	expected := []int{2, 4, 6, 8, 10}

	for i, v := range doubled {
		if v != expected[i] {
			t.Errorf("Map() index %d = %v, want %v", i, v, expected[i])
		}
	}

	// Test Reduce
	sum := Reduce(nums, 0, func(acc, n int) int { return acc + n })
	if sum != 15 {
		t.Errorf("Reduce() = %v, want 15", sum)
	}

	// Test Find
	found := Find(nums, func(n int) bool { return n > 3 })
	if !found.IsPresent() || found.OrElse(0) != 4 {
		t.Error("Find should return first matching element")
	}

	notFound := Find(nums, func(n int) bool { return n > 10 })
	if notFound.IsPresent() {
		t.Error("Find should return empty when no match")
	}

	// Test Contains
	if !Contains(nums, 3) {
		t.Error("Contains should find existing element")
	}

	if Contains(nums, 10) {
		t.Error("Contains should not find non-existent element")
	}

	// Test Unique
	withDups := []int{1, 2, 2, 3, 3, 3, 4, 5, 5}
	unique := Unique(withDups)

	if len(unique) != 5 {
		t.Errorf("Unique() returned %d elements, want 5", len(unique))
	}

	// Test GroupBy
	words := []string{"apple", "apricot", "banana", "berry", "cherry"}
	byFirstLetter := GroupBy(words, func(s string) rune {
		return rune(s[0])
	})

	if len(byFirstLetter['a']) != 2 {
		t.Error("GroupBy should group 'apple' and 'apricot' together")
	}

	if len(byFirstLetter['b']) != 2 {
		t.Error("GroupBy should group 'banana' and 'berry' together")
	}
}

func TestTypedMap(t *testing.T) {
	tm := NewTypedMap[string, int]()

	// Test Set and Get
	tm.Set("one", 1)
	tm.Set("two", 2)

	if val, ok := tm.Get("one"); !ok || val != 1 {
		t.Errorf("Get() = %v, %v; want 1, true", val, ok)
	}

	// Test GetOrDefault
	if tm.GetOrDefault("three", 99) != 99 {
		t.Error("GetOrDefault should return default for missing key")
	}

	// Test Has
	if !tm.Has("one") || tm.Has("three") {
		t.Error("Has() not working correctly")
	}

	// Test Keys and Values
	keys := tm.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}

	values := tm.Values()
	if len(values) != 2 {
		t.Errorf("Values() returned %d values, want 2", len(values))
	}

	// Test Delete
	tm.Delete("one")
	if tm.Has("one") {
		t.Error("Delete should remove the key")
	}

	// Test Clear
	tm.Clear()
	if tm.Len() != 0 {
		t.Error("Clear should remove all items")
	}
}

func TestPointerUtilities(t *testing.T) {
	// Test Ptr
	p := Ptr(types.TestStringHello)
	if *p != types.TestStringHello {
		t.Error("Ptr should return pointer to value")
	}

	// Test DerefOr
	if DerefOr(p, "default") != types.TestStringHello {
		t.Error("DerefOr should return dereferenced value")
	}

	var nilPtr *string
	if DerefOr(nilPtr, "default") != "default" {
		t.Error("DerefOr should return default for nil pointer")
	}

	// Test FirstNonNil
	var nil1, nil2 *int
	three := Ptr(3)
	four := Ptr(4)

	result := FirstNonNil(nil1, nil2, three, four)
	if result == nil || *result != 3 {
		t.Error("FirstNonNil should return first non-nil pointer")
	}
}

func TestUtilityFunctions(t *testing.T) {
	// Test Zero
	zeroInt := Zero[int]()
	if zeroInt != 0 {
		t.Error("Zero should return zero value")
	}

	zeroString := Zero[string]()
	if zeroString != "" {
		t.Error("Zero should return empty string")
	}

	// Test IsZero
	if !IsZero(0) || !IsZero("") {
		t.Error("IsZero should detect zero values")
	}

	if IsZero(1) || IsZero(types.TestStringHello) {
		t.Error("IsZero should not detect non-zero values")
	}

	// Test Coalesce
	result := Coalesce("", "", types.TestStringHello, "world")
	if result != types.TestStringHello {
		t.Error("Coalesce should return first non-zero value")
	}

	// Test ConvertMap
	source := map[string]int{"one": 1, "two": 2}
	converted := ConvertMap(
		source,
		func(k string) string { return "key_" + k },
		func(v int) string { return string(rune('0' + v)) },
	)

	if converted["key_one"] != "1" || converted["key_two"] != "2" {
		t.Error("ConvertMap should convert keys and values")
	}
}
