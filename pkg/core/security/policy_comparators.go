package security

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// compareValues compares two values based on the given operator
func (pe *PolicyEngine) compareValues(actual interface{}, operator RuleOperator, expected interface{}) bool {
	switch operator {
	case OperatorEquals:
		return pe.compareEquals(actual, expected)
	case OperatorNotEquals:
		return !pe.compareEquals(actual, expected)
	case OperatorGreaterThan:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a > b })
	case OperatorGreaterThanOrEqual:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a >= b })
	case OperatorLessThan:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a < b })
	case OperatorLessThanOrEqual:
		return pe.compareNumeric(actual, expected, func(a, b float64) bool { return a <= b })
	case OperatorContains:
		return pe.compareString(actual, expected, strings.Contains)
	case OperatorNotContains:
		return !pe.compareString(actual, expected, strings.Contains)
	case OperatorMatches:
		return pe.compareRegex(actual, expected)
	case OperatorNotMatches:
		return !pe.compareRegex(actual, expected)
	default:
		pe.logger.Warn().Str("operator", string(operator)).Msg("Unknown operator")
		return false
	}
}

// compareEquals performs equality comparison with type coercion
func (pe *PolicyEngine) compareEquals(actual, expected interface{}) bool {
	// Handle nil cases
	if actual == nil || expected == nil {
		return actual == expected
	}

	// Try direct comparison first
	if actual == expected {
		return true
	}

	// Convert to strings for comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return actualStr == expectedStr
}

// compareNumeric compares numeric values using the provided comparison function
func (pe *PolicyEngine) compareNumeric(actual, expected interface{}, compareFn func(float64, float64) bool) bool {
	actualFloat := pe.toFloat64(actual)
	expectedFloat := pe.toFloat64(expected)
	return compareFn(actualFloat, expectedFloat)
}

// compareString compares string values using the provided comparison function
func (pe *PolicyEngine) compareString(actual, expected interface{}, compareFn func(string, string) bool) bool {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return compareFn(actualStr, expectedStr)
}

// compareRegex performs regex matching
func (pe *PolicyEngine) compareRegex(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	patternStr := fmt.Sprintf("%v", expected)

	regex, err := regexp.Compile(patternStr)
	if err != nil {
		pe.logger.Error().Err(err).Str("pattern", patternStr).Msg("Invalid regex pattern")
		return false
	}

	return regex.MatchString(actualStr)
}

// toFloat64 converts various types to float64
func (pe *PolicyEngine) toFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}
