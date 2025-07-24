// Package registry provides a generic registry implementation for managing collections of typed items
//
// This package provides a thread-safe generic map implementation used throughout the application
// for storing and retrieving typed data. It should not be confused with the registrar package,
// which handles MCP-specific tool and resource registration.
package registry

import (
	"fmt"
	"sync"
)

// Registry provides a generic, thread-safe registry for managing items of type T
type Registry[T any] struct {
	mu   sync.RWMutex
	data map[string]T
}

// New creates a new registry for items of type T
func New[T any]() *Registry[T] {
	return &Registry[T]{
		data: make(map[string]T),
	}
}

// Add registers an item with the given key
func (r *Registry[T]) Add(key string, item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = item
}

// Get retrieves an item by key
func (r *Registry[T]) Get(key string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, exists := r.data[key]
	return item, exists
}

// All returns a copy of all items in the registry
// This uses copy-on-iterate for thread safety
func (r *Registry[T]) All() map[string]T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid unsafe caller references
	result := make(map[string]T, len(r.data))
	for key, value := range r.data {
		result[key] = value
	}
	return result
}

// Keys returns all registered keys
func (r *Registry[T]) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.data))
	for key := range r.data {
		keys = append(keys, key)
	}
	return keys
}

// Exists checks if a key is registered
func (r *Registry[T]) Exists(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.data[key]
	return exists
}

// Remove removes an item by key
func (r *Registry[T]) Remove(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[key]; exists {
		delete(r.data, key)
		return true
	}
	return false
}

// Size returns the number of registered items
func (r *Registry[T]) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.data)
}

// Clear removes all items from the registry
func (r *Registry[T]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = make(map[string]T)
}

// GetOrError retrieves an item by key or returns an error if not found
func (r *Registry[T]) GetOrError(key string) (T, error) {
	item, exists := r.Get(key)
	if !exists {
		var zero T
		return zero, fmt.Errorf("item not found: %s", key)
	}
	return item, nil
}
