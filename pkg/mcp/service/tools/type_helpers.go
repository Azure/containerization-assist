package tools

// Type conversion helpers for handling JSON unmarshaling type variations.
// JSON unmarshaling converts all numbers to float64, but our code often needs ints.
// These helpers provide safe, idiomatic conversions.

// GetInt safely extracts an int from an interface{} that may be float64 or int.
// Returns 0 if the value is nil or not a number.
func GetInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	default:
		return 0
	}
}

// GetString safely extracts a string from an interface{}.
// Returns empty string if the value is nil or not a string.
func GetString(v interface{}) string {
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	return ""
}

// GetStringSlice safely extracts a []string from an interface{}.
// Handles both []string and []interface{} (from JSON unmarshaling).
// Returns empty slice if the value is nil or not an array.
func GetStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}

	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return []string{}
	}
}

// GetBool safely extracts a bool from an interface{}.
// Returns false if the value is nil or not a bool.
func GetBool(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GetFloat64 safely extracts a float64 from an interface{}.
// Returns 0.0 if the value is nil or not a number.
func GetFloat64(v interface{}) float64 {
	if v == nil {
		return 0.0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0.0
	}
}

// GetMap safely extracts a map[string]interface{} from an interface{}.
// Returns nil if the value is nil or not a map.
func GetMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// IncrementCounter safely increments a counter value in a map.
// Handles both int and float64 types (from JSON).
func IncrementCounter(m map[string]interface{}, key string) int {
	current := GetInt(m[key])
	current++
	m[key] = current
	return current
}
