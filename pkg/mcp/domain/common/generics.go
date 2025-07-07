package common

import (
	"context"
	"sync"
	"time"
)

// Result type to replace interface{} returns with proper error handling
type Result[T any] struct {
	Data  T     `json:"data,omitempty"`
	Error error `json:"error,omitempty"`
}

// NewResult creates a successful result
func NewResult[T any](data T) Result[T] {
	return Result[T]{Data: data}
}

// NewError creates an error result
func NewError[T any](err error) Result[T] {
	var zero T
	return Result[T]{Data: zero, Error: err}
}

// IsOk returns true if the result contains no error
func (r Result[T]) IsOk() bool {
	return r.Error == nil
}

// Unwrap returns the data or panics if there's an error
func (r Result[T]) Unwrap() T {
	if r.Error != nil {
		panic(r.Error)
	}
	return r.Data
}

// UnwrapOr returns the data or the provided default if there's an error
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.Error != nil {
		return defaultValue
	}
	return r.Data
}

// Option type for nullable values (replacing *interface{})
type Option[T any] struct {
	Value T    `json:"value,omitempty"`
	Valid bool `json:"valid"`
}

// Some creates an Option with a value
func Some[T any](value T) Option[T] {
	return Option[T]{Value: value, Valid: true}
}

// None creates an empty Option
func None[T any]() Option[T] {
	var zero T
	return Option[T]{Value: zero, Valid: false}
}

// IsSome returns true if the option has a value
func (o Option[T]) IsSome() bool {
	return o.Valid
}

// IsNone returns true if the option has no value
func (o Option[T]) IsNone() bool {
	return !o.Valid
}

// Unwrap returns the value or panics if None
func (o Option[T]) Unwrap() T {
	if !o.Valid {
		panic("called Unwrap on None option")
	}
	return o.Value
}

// UnwrapOr returns the value or the provided default
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if !o.Valid {
		return defaultValue
	}
	return o.Value
}

// Predicate for filtering operations
type Predicate[T any] func(T) bool

// Transform for mapping operations
type Transform[T, U any] func(T) U

// Reducer for folding operations
type Reducer[T, U any] func(accumulator U, item T) U

// Cache interface with generics (replacing interface{} cache)
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V) error
	Delete(key K) error
	Clear()
	Size() int
	Keys() []K
}

// MemoryCache implements Cache interface with type safety
type MemoryCache[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

// NewMemoryCache creates a new type-safe memory cache
func NewMemoryCache[K comparable, V any]() *MemoryCache[K, V] {
	return &MemoryCache[K, V]{
		data: make(map[K]V),
	}
}

func (c *MemoryCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.data[key]
	return val, ok
}

func (c *MemoryCache[K, V]) Set(key K, value V) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *MemoryCache[K, V]) Delete(key K) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *MemoryCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[K]V)
}

func (c *MemoryCache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

func (c *MemoryCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}

// Type constraints for common patterns

// Number constraint for numeric types
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Ordered constraint for comparable types
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~string
}

// SchemaValue constraint for JSON schema values
type SchemaValue interface {
	~string | ~int | ~int64 | ~float64 | ~bool |
		[]any | map[string]any | []string | []int | []float64
}

// Validatable constraint for types that can be validated
type Validatable interface {
	Validate() error
}

// Serializable constraint for types that can be serialized
type Serializable interface {
	Serialize() ([]byte, error)
}

// ValidationResult replaces map[string]interface{} validation results
type ValidationResult[T any] struct {
	Valid    bool                `json:"valid"`
	Data     T                   `json:"data,omitempty"`
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
	Context  map[string]string   `json:"context,omitempty"`
	Duration time.Duration       `json:"duration,omitempty"`
}

type ValidationError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Field   string            `json:"field,omitempty"`
	Context map[string]string `json:"context,omitempty"`
}

type ValidationWarning struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Field   string            `json:"field,omitempty"`
	Context map[string]string `json:"context,omitempty"`
}

// AddError adds a validation error
func (vr *ValidationResult[T]) AddError(code, message string, context map[string]string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Code:    code,
		Message: message,
		Context: context,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResult[T]) AddWarning(code, message string, context map[string]string) {
	vr.Warnings = append(vr.Warnings, ValidationWarning{
		Code:    code,
		Message: message,
		Context: context,
	})
}

// IsValid returns true if validation passed
func (vr *ValidationResult[T]) IsValid() bool {
	return vr.Valid && len(vr.Errors) == 0
}

// ToolResult replaces interface{} tool results
type ToolResult[T any] struct {
	Success   bool              `json:"success"`
	Data      T                 `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Duration  time.Duration     `json:"duration,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewToolResult creates a successful tool result
func NewToolResult[T any](data T) ToolResult[T] {
	return ToolResult[T]{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewToolError creates an error tool result
func NewToolError[T any](err error) ToolResult[T] {
	var zero T
	return ToolResult[T]{
		Success:   false,
		Data:      zero,
		Error:     err.Error(),
		Timestamp: time.Now(),
	}
}

// Tool interface with generic input/output types
type Tool[I any, O any] interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input I) (O, error)
	Validate(input I) error
}

// Utility functions for working with slices

// Map applies a transform function to each element
func Map[T, U any](slice []T, fn Transform[T, U]) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = fn(item)
	}
	return result
}

// Filter returns elements that satisfy the predicate
func Filter[T any](slice []T, fn Predicate[T]) []T {
	var result []T
	for _, item := range slice {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Reduce folds the slice using the reducer function
func Reduce[T, U any](slice []T, initial U, fn Reducer[T, U]) U {
	result := initial
	for _, item := range slice {
		result = fn(result, item)
	}
	return result
}

// Find returns the first element that satisfies the predicate
func Find[T any](slice []T, fn Predicate[T]) Option[T] {
	for _, item := range slice {
		if fn(item) {
			return Some(item)
		}
	}
	return None[T]()
}

// Contains checks if the slice contains the value
func Contains[T comparable](slice []T, value T) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// Common type aliases for frequently used generic types
type StringCache = Cache[string, string]
type ConfigCache = Cache[string, map[string]any]
type ManifestValidation = ValidationResult[map[string]any]
type StringResult = Result[string]
type StringOption = Option[string]
