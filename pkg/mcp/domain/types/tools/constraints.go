package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Constraint represents a validation constraint
type Constraint interface {
	Validate(value interface{}) error
	Description() string
}

// StringConstraint provides validation for string values
type StringConstraint struct {
	MinLength    *int
	MaxLength    *int
	Pattern      *regexp.Regexp
	Enum         []string
	Format       StringFormat
	Required     bool
	NonEmpty     bool
	Alphanumeric bool
}

// StringFormat represents common string formats
type StringFormat string

const (
	FormatEmail     StringFormat = "email"
	FormatURL       StringFormat = "url"
	FormatUUID      StringFormat = "uuid"
	FormatBase64    StringFormat = "base64"
	FormatDockerTag StringFormat = "docker-tag"
	FormatK8sName   StringFormat = "k8s-name"
	FormatFilePath  StringFormat = "file-path"
	FormatDirPath   StringFormat = "dir-path"
)

// Validate validates a string value against the constraint
func (c StringConstraint) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.NewError().Messagef("expected string, got %T", value).Build()
	}

	if c.Required && str == "" {
		return errors.NewError().Messagef("value is required").Build()
	}

	if c.NonEmpty && strings.TrimSpace(str) == "" {
		return errors.NewError().Messagef("value cannot be empty").Build()
	}

	if c.MinLength != nil && len(str) < *c.MinLength {
		return errors.NewError().Messagef("minimum length is %d, got %d", *c.MinLength, len(str)).Build()
	}

	if c.MaxLength != nil && len(str) > *c.MaxLength {
		return errors.NewError().Messagef("maximum length is %d, got %d", *c.MaxLength, len(str)).Build()
	}

	if c.Pattern != nil && !c.Pattern.MatchString(str) {
		return errors.NewError().Messagef("value does not match required pattern: %s", c.Pattern.String()).Build()
	}

	if len(c.Enum) > 0 {
		found := false
		for _, allowed := range c.Enum {
			if str == allowed {
				found = true
				break
			}
		}
		if !found {
			return errors.NewError().Messagef("value must be one of: %s", strings.Join(c.Enum, ", ")).Build()
		}
	}

	if c.Alphanumeric && !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(str) {
		return errors.NewError().Messagef("value must be alphanumeric").Build()
	}

	if c.Format != "" {
		if err := c.validateFormat(str); err != nil {
			return err
		}
	}

	return nil
}

// validateFormat validates string against specific formats
func (c StringConstraint) validateFormat(str string) error {
	switch c.Format {
	case FormatEmail:
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(str) {
			return errors.NewError().Messagef("invalid email format").Build()
		}
	case FormatURL:
		if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
			return errors.NewError().Messagef("invalid URL format (must start with http:// or https://)").Build()
		}
	case FormatUUID:
		uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidRegex.MatchString(str) {
			return errors.NewError().Messagef("invalid UUID format").Build()
		}
	case FormatBase64:
		base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
		if !base64Regex.MatchString(str) {
			return errors.NewError().Messagef("invalid base64 format").Build()
		}
	case FormatDockerTag:
		tagRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*[a-zA-Z0-9]$`)
		if !tagRegex.MatchString(str) {
			return errors.NewError().Messagef("invalid Docker tag format").Build()
		}
	case FormatK8sName:
		nameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
		if !nameRegex.MatchString(str) {
			return errors.NewError().Messagef("invalid Kubernetes name format").Build()
		}
	case FormatFilePath:
		if strings.Contains(str, "..") || strings.Contains(str, "//") {
			return errors.NewError().Messagef("invalid file path (contains .. or //)").Build()
		}
	case FormatDirPath:
		if strings.Contains(str, "..") || strings.Contains(str, "//") {
			return errors.NewError().Messagef("invalid directory path (contains .. or //)").Build()
		}
	}
	return nil
}

// Description returns a human-readable description of the constraint
func (c StringConstraint) Description() string {
	var parts []string

	if c.Required {
		parts = append(parts, "required")
	}
	if c.NonEmpty {
		parts = append(parts, "non-empty")
	}
	if c.MinLength != nil {
		parts = append(parts, fmt.Sprintf("min length: %d", *c.MinLength))
	}
	if c.MaxLength != nil {
		parts = append(parts, fmt.Sprintf("max length: %d", *c.MaxLength))
	}
	if c.Pattern != nil {
		parts = append(parts, fmt.Sprintf("pattern: %s", c.Pattern.String()))
	}
	if len(c.Enum) > 0 {
		parts = append(parts, fmt.Sprintf("allowed values: %s", strings.Join(c.Enum, ", ")))
	}
	if c.Alphanumeric {
		parts = append(parts, "alphanumeric only")
	}
	if c.Format != "" {
		parts = append(parts, fmt.Sprintf("format: %s", c.Format))
	}

	if len(parts) == 0 {
		return "string value"
	}
	return fmt.Sprintf("string (%s)", strings.Join(parts, ", "))
}

// NumberConstraint provides validation for numeric values
type NumberConstraint struct {
	Min          *float64
	Max          *float64
	ExclusiveMin bool
	ExclusiveMax bool
	MultipleOf   *float64
	Required     bool
	Positive     bool
	NonNegative  bool
}

// Validate validates a numeric value against the constraint
func (c NumberConstraint) Validate(value interface{}) error {
	num, err := c.convertToFloat64(value)
	if err != nil {
		return err
	}

	validators := []func(float64) error{
		c.validateMinimum,
		c.validateMaximum,
		c.validateMultipleOf,
		c.validatePositive,
		c.validateNonNegative,
	}

	for _, validator := range validators {
		if err := validator(num); err != nil {
			return err
		}
	}

	return nil
}

// convertToFloat64 converts various numeric types to float64
func (c NumberConstraint) convertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		num, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, errors.NewError().Messagef("cannot parse '%s' as number", v).Build()
		}
		return num, nil
	default:
		return 0, errors.NewError().Messagef("expected number, got %T", value).Build()
	}
}

// validateMinimum checks minimum value constraints
func (c NumberConstraint) validateMinimum(num float64) error {
	if c.Min == nil {
		return nil
	}

	if c.ExclusiveMin && num <= *c.Min {
		return errors.NewError().Messagef("value must be greater than %g", *c.Min).Build()
	}

	if !c.ExclusiveMin && num < *c.Min {
		return errors.NewError().Messagef("value must be greater than or equal to %g", *c.Min).Build()
	}

	return nil
}

// validateMaximum checks maximum value constraints
func (c NumberConstraint) validateMaximum(num float64) error {
	if c.Max == nil {
		return nil
	}

	if c.ExclusiveMax && num >= *c.Max {
		return errors.NewError().Messagef("value must be less than %g", *c.Max).Build()
	}

	if !c.ExclusiveMax && num > *c.Max {
		return errors.NewError().Messagef("value must be less than or equal to %g", *c.Max).Build()
	}

	return nil
}

// validateMultipleOf checks multiple-of constraints
func (c NumberConstraint) validateMultipleOf(num float64) error {
	if c.MultipleOf == nil || *c.MultipleOf == 0 {
		return nil
	}

	remainder := num / *c.MultipleOf
	if remainder != float64(int(remainder)) {
		return errors.NewError().Messagef("value must be a multiple of %g", *c.MultipleOf).Build()
	}

	return nil
}

// validatePositive checks positive value constraints
func (c NumberConstraint) validatePositive(num float64) error {
	if c.Positive && num <= 0 {
		return errors.NewError().Messagef("value must be positive").Build()
	}
	return nil
}

// validateNonNegative checks non-negative value constraints
func (c NumberConstraint) validateNonNegative(num float64) error {
	if c.NonNegative && num < 0 {
		return errors.NewError().Messagef("value must be non-negative").Build()
	}
	return nil
}

func (c NumberConstraint) Description() string {
	var parts []string

	if c.Required {
		parts = append(parts, "required")
	}
	if c.Min != nil {
		op := ">="
		if c.ExclusiveMin {
			op = ">"
		}
		parts = append(parts, fmt.Sprintf("%s %g", op, *c.Min))
	}
	if c.Max != nil {
		op := "<="
		if c.ExclusiveMax {
			op = "<"
		}
		parts = append(parts, fmt.Sprintf("%s %g", op, *c.Max))
	}
	if c.MultipleOf != nil {
		parts = append(parts, fmt.Sprintf("multiple of %g", *c.MultipleOf))
	}
	if c.Positive {
		parts = append(parts, "positive")
	}
	if c.NonNegative {
		parts = append(parts, "non-negative")
	}

	if len(parts) == 0 {
		return "number"
	}
	return fmt.Sprintf("number (%s)", strings.Join(parts, ", "))
}

// ArrayConstraint provides validation for array/slice values
type ArrayConstraint struct {
	MinItems    *int
	MaxItems    *int
	UniqueItems bool
	ItemType    reflect.Type
	Required    bool
}

// Validate validates an array value against the constraint
func (c ArrayConstraint) Validate(value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return errors.NewError().Messagef("expected array/slice, got %T", value).Build()
	}

	length := v.Len()

	if c.MinItems != nil && length < *c.MinItems {
		return errors.NewError().Messagef("minimum items is %d, got %d", *c.MinItems, length).Build()
	}

	if c.MaxItems != nil && length > *c.MaxItems {
		return errors.NewError().Messagef("maximum items is %d, got %d", *c.MaxItems, length).Build()
	}

	if c.UniqueItems {
		seen := make(map[interface{}]bool)
		for i := 0; i < length; i++ {
			item := v.Index(i).Interface()
			if seen[item] {
				return errors.NewError().Messagef("duplicate item found: %v", item).Build()
			}
			seen[item] = true
		}
	}

	if c.ItemType != nil {
		for i := 0; i < length; i++ {
			item := v.Index(i)
			if !item.Type().AssignableTo(c.ItemType) {
				return errors.NewError().Messagef("item at index %d: expected type %s, got %s", i, c.ItemType, item.Type()).Build()
			}
		}
	}

	return nil
}

// Description returns a human-readable description of the constraint
func (c ArrayConstraint) Description() string {
	var parts []string

	if c.Required {
		parts = append(parts, "required")
	}
	if c.MinItems != nil {
		parts = append(parts, fmt.Sprintf("min items: %d", *c.MinItems))
	}
	if c.MaxItems != nil {
		parts = append(parts, fmt.Sprintf("max items: %d", *c.MaxItems))
	}
	if c.UniqueItems {
		parts = append(parts, "unique items")
	}
	if c.ItemType != nil {
		parts = append(parts, fmt.Sprintf("item type: %s", c.ItemType))
	}

	if len(parts) == 0 {
		return "array"
	}
	return fmt.Sprintf("array (%s)", strings.Join(parts, ", "))
}

// BooleanConstraint provides validation for boolean values
type BooleanConstraint struct {
	Required bool
}

// Validate validates a boolean value against the constraint
func (c BooleanConstraint) Validate(value interface{}) error {
	switch v := value.(type) {
	case bool:
		return nil
	case string:
		if _, err := strconv.ParseBool(v); err != nil {
			return errors.NewError().Messagef("cannot parse '%s' as boolean", v).Build()
		}
		return nil
	default:
		return errors.NewError().Messagef("expected boolean, got %T", value).WithLocation(

		// Description returns a human-readable description of the constraint
		).Build()
	}
}

func (c BooleanConstraint) Description() string {
	if c.Required {
		return "boolean (required)"
	}
	return "boolean"
}

// DateTimeConstraint provides validation for date/time values
type DateTimeConstraint struct {
	After    *time.Time
	Before   *time.Time
	Format   string
	Required bool
}

// Validate validates a date/time value against the constraint
func (c DateTimeConstraint) Validate(value interface{}) error {
	var t time.Time
	var err error

	switch v := value.(type) {
	case time.Time:
		t = v
	case string:
		format := c.Format
		if format == "" {
			format = time.RFC3339
		}
		t, err = time.Parse(format, v)
		if err != nil {
			return errors.NewError().Messagef("cannot parse '%s' as date/time (format: %s)", v, format).Build()
		}
	case int64:
		t = time.Unix(v, 0)
	default:
		return errors.NewError().Messagef("expected date/time, got %T", value).Build()
	}

	if c.After != nil && t.Before(*c.After) {
		return errors.NewError().Messagef("date/time must be after %s", c.After.Format(time.RFC3339)).Build()
	}

	if c.Before != nil && t.After(*c.Before) {
		return errors.NewError().Messagef("date/time must be before %s", c.Before.Format(time.RFC3339)).Build(

		// Description returns a human-readable description of the constraint
		)
	}

	return nil
}

func (c DateTimeConstraint) Description() string {
	var parts []string

	if c.Required {
		parts = append(parts, "required")
	}
	if c.Format != "" {
		parts = append(parts, fmt.Sprintf("format: %s", c.Format))
	}
	if c.After != nil {
		parts = append(parts, fmt.Sprintf("after %s", c.After.Format(time.RFC3339)))
	}
	if c.Before != nil {
		parts = append(parts, fmt.Sprintf("before %s", c.Before.Format(time.RFC3339)))
	}

	if len(parts) == 0 {
		return "date/time"
	}
	return fmt.Sprintf("date/time (%s)", strings.Join(parts, ", "))
}

// CompoundConstraint combines multiple constraints with AND logic
type CompoundConstraint struct {
	Constraints []Constraint
}

// Validate validates a value against all constraints
func (c CompoundConstraint) Validate(value interface{}) error {
	for i, constraint := range c.Constraints {
		if err := constraint.Validate(value); err != nil {
			return errors.NewError().Message(fmt.Sprintf("constraint %d failed", i+1)).Cause(err).Build()
		}
	}
	return nil
}

// Description returns a human-readable description of the constraint
func (c CompoundConstraint) Description() string {
	var parts []string
	for _, constraint := range c.Constraints {
		parts = append(parts, constraint.Description())
	}
	return fmt.Sprintf("all of: %s", strings.Join(parts, ", "))
}

// Helper functions for creating constraints

// Required creates a required constraint for any type
func Required() Constraint {
	return StringConstraint{Required: true}
}

// MinLen creates a minimum length constraint for strings
func MinLen(min int) StringConstraint {
	return StringConstraint{MinLength: &min}
}

// MaxLen creates a maximum length constraint for strings
func MaxLen(max int) StringConstraint {
	return StringConstraint{MaxLength: &max}
}

// Pattern creates a regex pattern constraint for strings
func Pattern(pattern string) StringConstraint {
	return StringConstraint{Pattern: regexp.MustCompile(pattern)}
}

// Enum creates an enumeration constraint for strings
func Enum(values ...string) StringConstraint {
	return StringConstraint{Enum: values}
}

// Min creates a minimum value constraint for numbers
func Min(min float64) NumberConstraint {
	return NumberConstraint{Min: &min}
}

// Max creates a maximum value constraint for numbers
func Max(max float64) NumberConstraint {
	return NumberConstraint{Max: &max}
}

// Positive creates a positive number constraint
func Positive() NumberConstraint {
	return NumberConstraint{Positive: true}
}

// NonNegative creates a non-negative number constraint
func NonNegative() NumberConstraint {
	return NumberConstraint{NonNegative: true}
}
