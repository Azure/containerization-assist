package logging

import (
	"sync"
	"time"
)

// RingBuffer implements a circular buffer for log entries.
type RingBuffer struct {
	mu      sync.RWMutex
	buffer  []LogEntry
	size    int
	head    int
	tail    int
	full    bool
	metrics RingBufferMetrics
}

// RingBufferMetrics tracks performance metrics for the ring buffer.
type RingBufferMetrics struct {
	TotalWrites  int64
	TotalReads   int64
	Overflows    int64
	CurrentSize  int
	WriteLatency time.Duration
	ReadLatency  time.Duration
}

// NewRingBuffer creates a new ring buffer with the specified size.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]LogEntry, size),
		size:   size,
	}
}

// Write implements io.Writer interface for the ring buffer.
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	start := time.Now()
	defer func() {
		rb.mu.Lock()
		rb.metrics.WriteLatency = time.Since(start)
		rb.metrics.TotalWrites++
		rb.mu.Unlock()
	}()

	// For simplicity, we'll just store the raw bytes as a log entry
	// In a real implementation, you might want to parse the log format
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LevelInfo, // Default level
		Message:   string(p),
		Fields:    make(map[string]interface{}),
	}

	rb.Add(entry)
	return len(p), nil
}

// Add adds a log entry to the ring buffer.
func (rb *RingBuffer) Add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.head] = entry

	if rb.full {
		rb.tail = (rb.tail + 1) % rb.size
		rb.metrics.Overflows++
	}

	rb.head = (rb.head + 1) % rb.size

	if rb.head == rb.tail {
		rb.full = true
	}

	rb.metrics.CurrentSize = rb.currentSize()
}

// GetAll returns all log entries in the buffer.
func (rb *RingBuffer) GetAll() []LogEntry {
	start := time.Now()
	defer func() {
		rb.mu.RLock()
		rb.metrics.ReadLatency = time.Since(start)
		rb.metrics.TotalReads++
		rb.mu.RUnlock()
	}()

	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full && rb.head == rb.tail {
		return nil
	}

	var result []LogEntry

	if rb.full {
		// Buffer is full, read from tail to head
		for i := rb.tail; i != rb.head; i = (i + 1) % rb.size {
			result = append(result, rb.buffer[i])
		}
	} else {
		// Buffer is not full, read from 0 to head
		for i := 0; i < rb.head; i++ {
			result = append(result, rb.buffer[i])
		}
	}

	return result
}

// GetRecent returns the most recent n log entries.
func (rb *RingBuffer) GetRecent(n int) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	all := rb.getAllUnsafe()
	if len(all) <= n {
		return all
	}

	return all[len(all)-n:]
}

// Clear clears the ring buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.head = 0
	rb.tail = 0
	rb.full = false
	rb.metrics.CurrentSize = 0
}

// Size returns the current number of entries in the buffer.
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.currentSize()
}

// Capacity returns the maximum capacity of the buffer.
func (rb *RingBuffer) Capacity() int {
	return rb.size
}

// IsFull returns true if the buffer is full.
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.full
}

// IsEmpty returns true if the buffer is empty.
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return !rb.full && rb.head == rb.tail
}

// GetMetrics returns performance metrics for the ring buffer.
func (rb *RingBuffer) GetMetrics() RingBufferMetrics {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	metrics := rb.metrics
	metrics.CurrentSize = rb.currentSize()
	return metrics
}

// currentSize returns the current size without locking (internal use).
func (rb *RingBuffer) currentSize() int {
	if rb.full {
		return rb.size
	}

	if rb.head >= rb.tail {
		return rb.head - rb.tail
	}

	return rb.size - rb.tail + rb.head
}

// getAllUnsafe returns all entries without locking (internal use).
func (rb *RingBuffer) getAllUnsafe() []LogEntry {
	if !rb.full && rb.head == rb.tail {
		return nil
	}

	var result []LogEntry

	if rb.full {
		// Buffer is full, read from tail to head
		for i := rb.tail; i != rb.head; i = (i + 1) % rb.size {
			result = append(result, rb.buffer[i])
		}
	} else {
		// Buffer is not full, read from 0 to head
		for i := 0; i < rb.head; i++ {
			result = append(result, rb.buffer[i])
		}
	}

	return result
}
