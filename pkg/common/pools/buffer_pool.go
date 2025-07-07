package pools

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
)

// ============================================================================
// WORKSTREAM DELTA: Memory Pool Optimization
// Reduces memory allocations in hot paths for <300μs P95 performance
// ============================================================================

// BufferPool provides reusable byte buffers to reduce GC pressure
var BufferPool = &bufferPool{
	pool: sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 4096) // Pre-allocate 4KB buffers
		},
	},
}

type bufferPool struct {
	pool sync.Pool
}

// Get retrieves a buffer from the pool
func (p *bufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool
func (p *bufferPool) Put(buf []byte) {
	// Don't retain very large buffers to prevent memory bloat
	if cap(buf) > 64*1024 {
		return
	}
	p.pool.Put(buf[:0]) // Reset length but keep capacity
}

// BytesBufferPool provides reusable bytes.Buffer instances
var BytesBufferPool = &bytesBufferPool{
	pool: sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	},
}

type bytesBufferPool struct {
	pool sync.Pool
}

// Get retrieves a bytes.Buffer from the pool
func (p *bytesBufferPool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put returns a bytes.Buffer to the pool
func (p *bytesBufferPool) Put(buf *bytes.Buffer) {
	// Don't retain very large buffers
	if buf.Cap() > 64*1024 {
		return
	}
	buf.Reset()
	p.pool.Put(buf)
}

// StringBuilderPool provides reusable string builders
var StringBuilderPool = &stringBuilderPool{
	pool: sync.Pool{
		New: func() interface{} {
			var sb strings.Builder
			sb.Grow(1024) // Pre-allocate 1KB
			return &sb
		},
	},
}

type stringBuilderPool struct {
	pool sync.Pool
}

// Get retrieves a strings.Builder from the pool
func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

// Put returns a strings.Builder to the pool
func (p *stringBuilderPool) Put(sb *strings.Builder) {
	// Don't retain very large builders
	if sb.Cap() > 16*1024 {
		return
	}
	sb.Reset()
	p.pool.Put(sb)
}

// JSONEncoderPool provides reusable JSON encoders
var JSONEncoderPool = &jsonEncoderPool{
	pool: sync.Pool{
		New: func() interface{} {
			buf := BytesBufferPool.Get()
			return json.NewEncoder(buf)
		},
	},
}

type jsonEncoderPool struct {
	pool sync.Pool
}

// Get retrieves a JSON encoder from the pool
func (p *jsonEncoderPool) Get() (*json.Encoder, *bytes.Buffer) {
	buf := BytesBufferPool.Get()
	encoder := json.NewEncoder(buf)
	return encoder, buf
}

// Put returns a JSON encoder to the pool
func (p *jsonEncoderPool) Put(encoder *json.Encoder, buf *bytes.Buffer) {
	BytesBufferPool.Put(buf)
	p.pool.Put(encoder)
}

// MapPool provides reusable maps for common use cases
var MapPool = &mapPool{
	stringInterfacePool: sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 16)
		},
	},
	stringStringPool: sync.Pool{
		New: func() interface{} {
			return make(map[string]string, 16)
		},
	},
}

type mapPool struct {
	stringInterfacePool sync.Pool
	stringStringPool    sync.Pool
}

// GetStringInterface retrieves a map[string]interface{} from the pool
func (p *mapPool) GetStringInterface() map[string]interface{} {
	return p.stringInterfacePool.Get().(map[string]interface{})
}

// PutStringInterface returns a map[string]interface{} to the pool
func (p *mapPool) PutStringInterface(m map[string]interface{}) {
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	// Don't retain very large maps
	if len(m) < 100 {
		p.stringInterfacePool.Put(m)
	}
}

// GetStringString retrieves a map[string]string from the pool
func (p *mapPool) GetStringString() map[string]string {
	return p.stringStringPool.Get().(map[string]string)
}

// PutStringString returns a map[string]string to the pool
func (p *mapPool) PutStringString(m map[string]string) {
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	// Don't retain very large maps
	if len(m) < 100 {
		p.stringStringPool.Put(m)
	}
}

// SlicePool provides reusable slices
var SlicePool = &slicePool{
	stringPool: sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 16)
		},
	},
	interfacePool: sync.Pool{
		New: func() interface{} {
			return make([]interface{}, 0, 16)
		},
	},
}

type slicePool struct {
	stringPool    sync.Pool
	interfacePool sync.Pool
}

// GetStringSlice retrieves a []string from the pool
func (p *slicePool) GetStringSlice() []string {
	return p.stringPool.Get().([]string)
}

// PutStringSlice returns a []string to the pool
func (p *slicePool) PutStringSlice(s []string) {
	// Don't retain very large slices
	if cap(s) > 1000 {
		return
	}
	p.stringPool.Put(s[:0])
}

// GetInterfaceSlice retrieves a []interface{} from the pool
func (p *slicePool) GetInterfaceSlice() []interface{} {
	return p.interfacePool.Get().([]interface{})
}

// PutInterfaceSlice returns a []interface{} to the pool
func (p *slicePool) PutInterfaceSlice(s []interface{}) {
	// Don't retain very large slices
	if cap(s) > 1000 {
		return
	}
	p.interfacePool.Put(s[:0])
}

// ============================================================================
// High-Level Helper Functions for Common Patterns
// ============================================================================

// WithBuffer executes a function with a buffer from the pool
func WithBuffer(fn func([]byte) error) error {
	buf := BufferPool.Get()
	defer BufferPool.Put(buf)
	return fn(buf)
}

// WithBytesBuffer executes a function with a bytes.Buffer from the pool
func WithBytesBuffer(fn func(*bytes.Buffer) error) error {
	buf := BytesBufferPool.Get()
	defer BytesBufferPool.Put(buf)
	return fn(buf)
}

// WithStringBuilder executes a function with a strings.Builder from the pool
func WithStringBuilder(fn func(*strings.Builder) string) string {
	sb := StringBuilderPool.Get()
	defer StringBuilderPool.Put(sb)
	return fn(sb)
}

// WithJSONEncoder executes a function with a JSON encoder from the pool
func WithJSONEncoder(fn func(*json.Encoder, *bytes.Buffer) error) error {
	encoder, buf := JSONEncoderPool.Get()
	defer JSONEncoderPool.Put(encoder, buf)
	return fn(encoder, buf)
}

// WithStringInterfaceMap executes a function with a map[string]interface{} from the pool
func WithStringInterfaceMap(fn func(map[string]interface{}) error) error {
	m := MapPool.GetStringInterface()
	defer MapPool.PutStringInterface(m)
	return fn(m)
}

// WithStringStringMap executes a function with a map[string]string from the pool
func WithStringStringMap(fn func(map[string]string) error) error {
	m := MapPool.GetStringString()
	defer MapPool.PutStringString(m)
	return fn(m)
}

// ============================================================================
// Performance Optimized JSON Operations
// ============================================================================

// FastJSONMarshal performs JSON marshaling using pooled resources
func FastJSONMarshal(v interface{}) ([]byte, error) {
	var result []byte
	var err error

	err = WithJSONEncoder(func(encoder *json.Encoder, buf *bytes.Buffer) error {
		if marshalErr := encoder.Encode(v); marshalErr != nil {
			return marshalErr
		}

		// Copy the result (buf will be returned to pool)
		result = make([]byte, buf.Len())
		copy(result, buf.Bytes())
		return nil
	})

	return result, err
}

// FastStringConcat performs string concatenation using pooled string builder
func FastStringConcat(parts ...string) string {
	return WithStringBuilder(func(sb *strings.Builder) string {
		for _, part := range parts {
			sb.WriteString(part)
		}
		return sb.String()
	})
}

// ============================================================================
// Performance Monitoring
// ============================================================================

// PoolStats provides statistics about pool usage
type PoolStats struct {
	BufferPoolSize        int `json:"buffer_pool_size"`
	BytesBufferPoolSize   int `json:"bytes_buffer_pool_size"`
	StringBuilderPoolSize int `json:"string_builder_pool_size"`
	MapPoolSize           int `json:"map_pool_size"`
	SlicePoolSize         int `json:"slice_pool_size"`
}

// GetPoolStats returns current pool statistics
// Note: This is an approximation since sync.Pool doesn't expose exact counts
func GetPoolStats() PoolStats {
	return PoolStats{
		// These are estimates - sync.Pool doesn't provide exact counts
		BufferPoolSize:        1, // Placeholder
		BytesBufferPoolSize:   1, // Placeholder
		StringBuilderPoolSize: 1, // Placeholder
		MapPoolSize:           1, // Placeholder
		SlicePoolSize:         1, // Placeholder
	}
}
