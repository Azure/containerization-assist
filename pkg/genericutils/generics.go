package genericutils

import (
	"reflect"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// MapGet safely retrieves a value from a map with the specified type.
// Returns the typed value and true if found and type matches, zero value and false otherwise.
func MapGet[T any](m map[string]interface{}, key string) (T, bool) {
	var zero T

	value, exists := m[key]
	if !exists {
		return zero, false
	}

	// Try direct type assertion
	if typed, ok := value.(T); ok {
		return typed, true
	}

	// Handle nil values
	if value == nil {
		return zero, false
	}

	return zero, false
}

// MapGetWithDefault retrieves a value from a map or returns a default value.
func MapGetWithDefault[T any](m map[string]interface{}, key string, defaultValue T) T {
	if value, ok := MapGet[T](m, key); ok {
		return value
	}
	return defaultValue
}

// SafeCast performs a type-safe cast with error handling.
// Returns the casted value and nil on success, zero value and error on failure.
func SafeCast[T any](value interface{}) (T, error) {
	var zero T

	if value == nil {
		return zero, mcperrors.NewError().Messagef("cannot cast nil to %T", zero).WithLocation().Build()
	}

	if casted, ok := value.(T); ok {
		return casted, nil
	}

	return zero, mcperrors.NewError().Messagef("cannot cast %T to %T", value, zero).WithLocation(

	// TypedMap is a generic wrapper around map[string]interface{} that provides type-safe access.
	).Build()
}

type TypedMap[K comparable, V any] struct {
	data map[K]V
}

// NewTypedMap creates a new TypedMap.
func NewTypedMap[K comparable, V any]() *TypedMap[K, V] {
	return &TypedMap[K, V]{
		data: make(map[K]V),
	}
}

// NewTypedMapFrom creates a TypedMap from an existing map.
func NewTypedMapFrom[K comparable, V any](m map[K]V) *TypedMap[K, V] {
	return &TypedMap[K, V]{
		data: m,
	}
}

// Get retrieves a value from the map.
func (tm *TypedMap[K, V]) Get(key K) (V, bool) {
	value, exists := tm.data[key]
	return value, exists
}

// GetOrDefault retrieves a value or returns a default.
func (tm *TypedMap[K, V]) GetOrDefault(key K, defaultValue V) V {
	if value, exists := tm.data[key]; exists {
		return value
	}
	return defaultValue
}

// Set stores a value in the map.
func (tm *TypedMap[K, V]) Set(key K, value V) {
	tm.data[key] = value
}

// Delete removes a key from the map.
func (tm *TypedMap[K, V]) Delete(key K) {
	delete(tm.data, key)
}

// Has checks if a key exists in the map.
func (tm *TypedMap[K, V]) Has(key K) bool {
	_, exists := tm.data[key]
	return exists
}

// Keys returns all keys in the map.
func (tm *TypedMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(tm.data))
	for k := range tm.data {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values in the map.
func (tm *TypedMap[K, V]) Values() []V {
	values := make([]V, 0, len(tm.data))
	for _, v := range tm.data {
		values = append(values, v)
	}
	return values
}

// Len returns the number of items in the map.
func (tm *TypedMap[K, V]) Len() int {
	return len(tm.data)
}

// Clear removes all items from the map.
func (tm *TypedMap[K, V]) Clear() {
	tm.data = make(map[K]V)
}

// ToMap returns the underlying map.
func (tm *TypedMap[K, V]) ToMap() map[K]V {
	// Return a copy to prevent external modification
	result := make(map[K]V, len(tm.data))
	for k, v := range tm.data {
		result[k] = v
	}
	return result
}

// Optional represents a value that may or may not be present.
type Optional[T any] struct {
	value   T
	present bool
}

// NewOptional creates an Optional with a value.
func NewOptional[T any](value T) Optional[T] {
	return Optional[T]{
		value:   value,
		present: true,
	}
}

// EmptyOptional creates an empty Optional.
func EmptyOptional[T any]() Optional[T] {
	return Optional[T]{
		present: false,
	}
}

// IsPresent returns true if the Optional contains a value.
func (o Optional[T]) IsPresent() bool {
	return o.present
}

// Get returns the value and whether it's present.
func (o Optional[T]) Get() (T, bool) {
	return o.value, o.present
}

// OrElse returns the value if present, otherwise returns the provided default.
func (o Optional[T]) OrElse(defaultValue T) T {
	if o.present {
		return o.value
	}
	return defaultValue
}

// OrElseGet returns the value if present, otherwise calls the provider function.
func (o Optional[T]) OrElseGet(provider func() T) T {
	if o.present {
		return o.value
	}
	return provider()
}

// Result represents a value or an error.
type Result[T any] struct {
	value T
	err   error
}

// Ok creates a successful Result.
func Ok[T any](value T) Result[T] {
	return Result[T]{
		value: value,
		err:   nil,
	}
}

// Err creates an error Result.
func Err[T any](err error) Result[T] {
	var zero T
	return Result[T]{
		value: zero,
		err:   err,
	}
}

// IsOk returns true if the Result contains a value.
func (r Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr returns true if the Result contains an error.
func (r Result[T]) IsErr() bool {
	return r.err != nil
}

// Get returns the value and whether the result is ok.
func (r Result[T]) Get() (T, bool) {
	return r.value, r.err == nil
}

// Expect returns the value or panics with a custom message if there's an error.
// Prefer this over Unwrap() when you want to provide context.
func (r Result[T]) Expect(msg string) T {
	if r.err != nil {
		panic(mcperrors.NewError().Messagef("%s: %w", msg, r.err).WithLocation().Build())
	}
	return r.value
}

// UnwrapOr returns the value or a default if there's an error.
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// ExpectErr returns the error or panics with a custom message if there's no error.
func (r Result[T]) ExpectErr(msg string) error {
	if r.err == nil {
		panic(mcperrors.NewError().Messagef("%s: called ExpectErr on ok Result", msg).WithLocation().Build())
	}
	return r.err
}

// Map transforms the value if the Result is Ok.
func (r Result[T]) Map(fn func(T) T) Result[T] {
	if r.err != nil {
		return r
	}
	return Ok(fn(r.value))
}

// MapErr transforms the error if the Result is Err.
func (r Result[T]) MapErr(fn func(error) error) Result[T] {
	if r.err == nil {
		return r
	}
	return Err[T](fn(r.err))
}

// Slice utilities

// Filter returns a new slice containing only elements that match the predicate.
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0)
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms each element in a slice.
func Map[T, U any](slice []T, transform func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = transform(item)
	}
	return result
}

// Reduce aggregates slice elements into a single value.
func Reduce[T, U any](slice []T, initial U, reducer func(U, T) U) U {
	result := initial
	for _, item := range slice {
		result = reducer(result, item)
	}
	return result
}

// Find returns the first element matching the predicate.
func Find[T any](slice []T, predicate func(T) bool) Optional[T] {
	for _, item := range slice {
		if predicate(item) {
			return NewOptional(item)
		}
	}
	return EmptyOptional[T]()
}

// Contains checks if a slice contains an element.
func Contains[T comparable](slice []T, target T) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

// Unique returns a slice with duplicate elements removed.
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]bool)
	result := make([]T, 0)

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// GroupBy groups slice elements by a key function.
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	result := make(map[K][]T)

	for _, item := range slice {
		key := keyFn(item)
		result[key] = append(result[key], item)
	}

	return result
}

// Ptr returns a pointer to the value.
// Useful for inline pointer creation.
func Ptr[T any](v T) *T {
	return &v
}

// DerefOr dereferences a pointer or returns a default value if nil.
func DerefOr[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// Zero returns the zero value for type T.
func Zero[T any]() T {
	var zero T
	return zero
}

// IsZero checks if a value is the zero value for its type.
func IsZero[T comparable](value T) bool {
	var zero T
	return value == zero
}

// Coalesce returns the first non-zero value.
func Coalesce[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// FirstNonNil returns the first non-nil pointer.
func FirstNonNil[T any](ptrs ...*T) *T {
	for _, ptr := range ptrs {
		if ptr != nil {
			return ptr
		}
	}
	return nil
}

// ConvertMap converts a map from one type to another using converter functions.
func ConvertMap[K1, K2 comparable, V1, V2 any](
	source map[K1]V1,
	keyConverter func(K1) K2,
	valueConverter func(V1) V2,
) map[K2]V2 {
	result := make(map[K2]V2, len(source))
	for k, v := range source {
		result[keyConverter(k)] = valueConverter(v)
	}
	return result
}

// Keys returns the keys of a map as a slice.
func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the values of a map as a slice.
func Values[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// TryConvert attempts to convert between compatible types using reflection.
// This is a fallback for complex conversions.
func TryConvert[T any](value interface{}) (T, bool) {
	var target T
	targetType := reflect.TypeOf(target)
	valueType := reflect.TypeOf(value)

	if valueType == nil {
		return target, false
	}

	if valueType.ConvertibleTo(targetType) {
		converted := reflect.ValueOf(value).Convert(targetType)
		if result, ok := converted.Interface().(T); ok {
			return result, true
		}
	}

	return target, false
}
