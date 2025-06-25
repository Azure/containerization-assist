package utils

import (
	"strings"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
}

// RingBuffer is a circular buffer for storing log entries
type RingBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	head     int
	count    int
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
		head:     0,
		count:    0,
	}
}

// Add adds a new log entry to the ring buffer
func (rb *RingBuffer) Add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.capacity

	if rb.count < rb.capacity {
		rb.count++
	}
}

// GetEntries returns all entries in chronological order
func (rb *RingBuffer) GetEntries() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]LogEntry, rb.count)

	if rb.count < rb.capacity {
		// Buffer not full yet, entries are from 0 to head-1
		copy(result, rb.entries[:rb.count])
	} else {
		// Buffer is full, entries wrap around
		// Copy from head to end
		firstPart := rb.capacity - rb.head
		copy(result, rb.entries[rb.head:])
		// Copy from beginning to head
		if rb.head > 0 {
			copy(result[firstPart:], rb.entries[:rb.head])
		}
	}

	return result
}

// GetEntriesFiltered returns entries matching the filter criteria
func (rb *RingBuffer) GetEntriesFiltered(level string, since time.Time, pattern string) []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	allEntries := rb.GetEntries()
	if len(allEntries) == 0 {
		return nil
	}

	// Filter entries
	var filtered []LogEntry
	for _, entry := range allEntries {
		// Filter by time
		if !since.IsZero() && entry.Timestamp.Before(since) {
			continue
		}

		// Filter by level
		if level != "" && !matchesLogLevel(entry.Level, level) {
			continue
		}

		// Filter by pattern (simple substring match)
		if pattern != "" && !containsPattern(entry, pattern) {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

// matchesLogLevel checks if the entry level matches or is more severe than the filter level
func matchesLogLevel(entryLevel, filterLevel string) bool {
	levels := map[string]int{
		"trace": 0,
		"debug": 1,
		"info":  2,
		"warn":  3,
		"error": 4,
		"fatal": 5,
		"panic": 6,
	}

	entryPriority, ok1 := levels[entryLevel]
	filterPriority, ok2 := levels[filterLevel]

	if !ok1 || !ok2 {
		return entryLevel == filterLevel
	}

	return entryPriority >= filterPriority
}

// containsPattern checks if the log entry contains the pattern
func containsPattern(entry LogEntry, pattern string) bool {
	// Check message
	if containsIgnoreCase(entry.Message, pattern) {
		return true
	}

	// Check fields
	for _, value := range entry.Fields {
		if str, ok := value.(string); ok && containsIgnoreCase(str, pattern) {
			return true
		}
	}

	return false
}

// Clear removes all entries from the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.head = 0
	rb.count = 0
}

// Size returns the current number of entries in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
