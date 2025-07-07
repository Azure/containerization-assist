// Package utils - Type conversion utilities
// This file consolidates type conversion functions from across pkg/mcp
package utils

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// String conversion utilities

// ToString converts various types to string
func ToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case time.Time:
		return v.Format(time.RFC3339)
	case time.Duration:
		return v.String()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ToStringSlice converts various types to string slice
func ToStringSlice(value interface{}) []string {
	if value == nil {
		return []string{}
	}

	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			result[i] = ToString(item)
		}
		return result
	case string:
		// Split on common delimiters
		if strings.Contains(v, ",") {
			return SplitAndTrim(v, ",")
		} else if strings.Contains(v, ";") {
			return SplitAndTrim(v, ";")
		} else if strings.Contains(v, " ") {
			return SplitAndTrim(v, " ")
		}
		return []string{v}
	default:
		// Use reflection for other slice types
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice {
			result := make([]string, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				result[i] = ToString(rv.Index(i).Interface())
			}
			return result
		}
		return []string{ToString(value)}
	}
}

// Integer conversion utilities

// ToInt converts various types to int
func ToInt(value interface{}) (int, error) {
	if value == nil {
		return 0, errors.NewError().Messagef("cannot convert nil to int").WithLocation().Build()
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		const maxInt = int64(^uint(0) >> 1)
		const minInt = -maxInt - 1
		if v > maxInt || v < minInt {
			return 0, errors.NewError().Messagef("value %d overflows int", v).WithLocation().Build()
		}
		return int(v), nil
	case uint:
		const maxInt = uint(^uint(0) >> 1)
		if v > maxInt {
			return 0, errors.NewError().Messagef("value %d overflows int", v).WithLocation().Build()
		}
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		if uint64(v) > uint64(^uint(0)>>1) {
			return 0, errors.NewError().Messagef("value %d overflows int", v).WithLocation().Build()
		}
		return int(v), nil
	case uint64:
		const maxInt = uint64(^uint(0) >> 1)
		if v > maxInt {
			return 0, errors.NewError().Messagef("value %d overflows int", v).WithLocation().Build()
		}
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, errors.NewError().Messagef("cannot convert %T to int", value).WithLocation(

		// ToInt64 converts various types to int64
		).Build()
	}
}

func ToInt64(value interface{}) (int64, error) {
	if value == nil {
		return 0, errors.NewError().Messagef("cannot convert nil to int64").WithLocation().Build()
	}

	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0, errors.NewError().Messagef("value %d overflows int64", v).WithLocation().Build()
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, errors.NewError().Messagef("cannot convert %T to int64", value).WithLocation(

		// Float conversion utilities
		).Build()
	}
}

// ToFloat64 converts various types to float64
func ToFloat64(value interface{}) (float64, error) {
	if value == nil {
		return 0, errors.NewError().Messagef("cannot convert nil to float64").WithLocation().Build()
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	default:
		return 0, errors.NewError().Messagef("cannot convert %T to float64", value).WithLocation(

		// Boolean conversion utilities
		).Build()
	}
}

// ToBool converts various types to bool
func ToBool(value interface{}) (bool, error) {
	if value == nil {
		return false, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		v = strings.ToLower(strings.TrimSpace(v))
		switch v {
		case "true", "t", "yes", "y", "1", "on", "enabled":
			return true, nil
		case "false", "f", "no", "n", "0", "off", "disabled", "":
			return false, nil
		default:
			return strconv.ParseBool(v)
		}
	case int, int8, int16, int32, int64:
		intVal, _ := ToInt64(v)
		return intVal != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		intVal, _ := ToInt64(v)
		return intVal != 0, nil
	case float32, float64:
		floatVal, _ := ToFloat64(v)
		return floatVal != 0, nil
	default:
		return false, errors.NewError().Messagef("cannot convert %T to bool", value).WithLocation(

		// Time conversion utilities
		).Build()
	}
}

// ToTime converts various types to time.Time
func ToTime(value interface{}) (time.Time, error) {
	if value == nil {
		return time.Time{}, errors.NewError().Messagef("cannot convert nil to time.Time").WithLocation().Build()
	}

	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// Try common time formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			time.RFC822,
			time.RFC822Z,
			time.RFC850,
			time.RFC1123,
			time.RFC1123Z,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
			"15:04:05",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, nil
			}
		}

		return time.Time{}, errors.NewError().Messagef("cannot parse time: %s", v).WithLocation().Build(

		// Assume Unix timestamp
		)
	case int64:

		return time.Unix(v, 0), nil
	case int:
		return time.Unix(int64(v), 0), nil
	default:
		return time.Time{}, errors.NewError().Messagef("cannot convert %T to time.Time", value).WithLocation(

		// ToDuration converts various types to time.Duration
		).Build()
	}
}

func ToDuration(value interface{}) (time.Duration, error) {
	if value == nil {
		return 0, errors.NewError().Messagef("cannot convert nil to duration").WithLocation().Build()
	}

	switch v := value.(type) {
	case time.Duration:
		return v, nil
	case string:
		return time.ParseDuration(v)
	case int64:
		return time.Duration(v), nil
	case int:
		return time.Duration(v), nil
	case float64:
		return time.Duration(v), nil
	default:
		return 0, errors.NewError().Messagef("cannot convert %T to duration", value).WithLocation(

		// Map conversion utilities
		).Build()
	}
}

// ToStringMap converts various types to map[string]string
func ToStringMap(value interface{}) (map[string]string, error) {
	if value == nil {
		return map[string]string{}, nil
	}

	switch v := value.(type) {
	case map[string]string:
		return v, nil
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for key, val := range v {
			result[key] = ToString(val)
		}
		return result, nil
	case map[interface{}]interface{}:
		result := make(map[string]string)
		for key, val := range v {
			result[ToString(key)] = ToString(val)
		}
		return result, nil
	default:
		return nil, errors.NewError().Messagef("cannot convert %T to map[string]string", value).WithLocation(

		// ToInterfaceMap converts various types to map[string]interface{}
		).Build()
	}
}

func ToInterfaceMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}

	switch v := value.(type) {
	case map[string]interface{}:
		return v, nil
	case map[string]string:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = val
		}
		return result, nil
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[ToString(key)] = val
		}
		return result, nil
	default:
		return nil, errors.NewError().Messagef("cannot convert %T to map[string]interface{}", value).WithLocation(

		// Type checking utilities
		).Build()
	}
}

// IsNil checks if a value is nil or a nil pointer/interface
func IsNil(value interface{}) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// IsEmpty checks if a value is empty (nil, zero value, empty string/slice/map)
func IsEmpty(value interface{}) bool {
	if IsNil(value) {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String:
		return rv.Len() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return rv.IsNil()
	case reflect.Struct:
		// Check if it's the zero value
		return rv.Interface() == reflect.Zero(rv.Type()).Interface()
	default:
		return false
	}
}

// IsNumeric checks if a value is a numeric type
func IsNumeric(value interface{}) bool {
	if value == nil {
		return false
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	case reflect.String:
		// Check if string represents a number
		str := rv.String()
		if _, err := strconv.ParseFloat(str, 64); err == nil {
			return true
		}
		if _, err := strconv.ParseInt(str, 10, 64); err == nil {
			return true
		}
		return false
	default:
		return false
	}
}

// GetType returns the type name of a value
func GetType(value interface{}) string {
	if value == nil {
		return "nil"
	}

	return reflect.TypeOf(value).String()
}

// GetKind returns the reflect.Kind of a value
func GetKind(value interface{}) reflect.Kind {
	if value == nil {
		return reflect.Invalid
	}

	return reflect.TypeOf(value).Kind()
}

// Safe conversion utilities (return default on error)

// ToIntSafe safely converts to int, returning default on error
func ToIntSafe(value interface{}, defaultValue int) int {
	if result, err := ToInt(value); err == nil {
		return result
	}
	return defaultValue
}

// ToInt64Safe safely converts to int64, returning default on error
func ToInt64Safe(value interface{}, defaultValue int64) int64 {
	if result, err := ToInt64(value); err == nil {
		return result
	}
	return defaultValue
}

// ToFloat64Safe safely converts to float64, returning default on error
func ToFloat64Safe(value interface{}, defaultValue float64) float64 {
	if result, err := ToFloat64(value); err == nil {
		return result
	}
	return defaultValue
}

// ToBoolSafe safely converts to bool, returning default on error
func ToBoolSafe(value interface{}, defaultValue bool) bool {
	if result, err := ToBool(value); err == nil {
		return result
	}
	return defaultValue
}

// ToTimeSafe safely converts to time.Time, returning default on error
func ToTimeSafe(value interface{}, defaultValue time.Time) time.Time {
	if result, err := ToTime(value); err == nil {
		return result
	}
	return defaultValue
}

// ToDurationSafe safely converts to time.Duration, returning default on error
func ToDurationSafe(value interface{}, defaultValue time.Duration) time.Duration {
	if result, err := ToDuration(value); err == nil {
		return result
	}
	return defaultValue
}

// ToStringMapSafe safely converts to map[string]string, returning default on error
func ToStringMapSafe(value interface{}, defaultValue map[string]string) map[string]string {
	if result, err := ToStringMap(value); err == nil {
		return result
	}
	return defaultValue
}
