package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer(t *testing.T) {
	t.Run("basic add and get", func(t *testing.T) {
		rb := NewRingBuffer(5)

		// Add entries
		for i := 0; i < 3; i++ {
			rb.Add(LogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Message %d", i),
			})
		}

		// Get entries
		entries := rb.GetEntries()
		assert.Len(t, entries, 3)
		assert.Equal(t, "Message 0", entries[0].Message)
		assert.Equal(t, "Message 1", entries[1].Message)
		assert.Equal(t, "Message 2", entries[2].Message)
	})

	t.Run("overflow behavior", func(t *testing.T) {
		rb := NewRingBuffer(3)

		// Add more entries than capacity
		for i := 0; i < 5; i++ {
			rb.Add(LogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Message %d", i),
			})
		}

		// Should only have the last 3 entries
		entries := rb.GetEntries()
		assert.Len(t, entries, 3)
		assert.Equal(t, "Message 2", entries[0].Message)
		assert.Equal(t, "Message 3", entries[1].Message)
		assert.Equal(t, "Message 4", entries[2].Message)
	})

	t.Run("filter by level", func(t *testing.T) {
		rb := NewRingBuffer(10)
		now := time.Now()

		// Add entries with different levels
		rb.Add(LogEntry{Timestamp: now, Level: "debug", Message: "Debug message"})
		rb.Add(LogEntry{Timestamp: now, Level: "info", Message: "Info message"})
		rb.Add(LogEntry{Timestamp: now, Level: "warn", Message: "Warn message"})
		rb.Add(LogEntry{Timestamp: now, Level: "error", Message: "Error message"})

		// Filter by warn level
		entries := rb.GetEntriesFiltered("warn", time.Time{}, "")
		assert.Len(t, entries, 2) // warn and error
		assert.Equal(t, "warn", entries[0].Level)
		assert.Equal(t, "error", entries[1].Level)
	})

	t.Run("filter by time", func(t *testing.T) {
		rb := NewRingBuffer(10)
		now := time.Now()

		// Add entries at different times
		rb.Add(LogEntry{Timestamp: now.Add(-5 * time.Minute), Level: "info", Message: "Old message"})
		rb.Add(LogEntry{Timestamp: now.Add(-2 * time.Minute), Level: "info", Message: "Recent message"})
		rb.Add(LogEntry{Timestamp: now, Level: "info", Message: "Current message"})

		// Filter by time (last 3 minutes)
		entries := rb.GetEntriesFiltered("", now.Add(-3*time.Minute), "")
		assert.Len(t, entries, 2)
		assert.Equal(t, "Recent message", entries[0].Message)
		assert.Equal(t, "Current message", entries[1].Message)
	})

	t.Run("filter by pattern", func(t *testing.T) {
		rb := NewRingBuffer(10)
		now := time.Now()

		// Add entries
		rb.Add(LogEntry{
			Timestamp: now,
			Level:     "info",
			Message:   "Processing request",
			Fields:    map[string]interface{}{"path": "/api/users"},
		})
		rb.Add(LogEntry{
			Timestamp: now,
			Level:     "info",
			Message:   "Database query",
			Fields:    map[string]interface{}{"table": "users"},
		})
		rb.Add(LogEntry{
			Timestamp: now,
			Level:     "info",
			Message:   "Cache hit",
			Fields:    map[string]interface{}{"key": "config"},
		})

		// Filter by pattern
		entries := rb.GetEntriesFiltered("", time.Time{}, "users")
		assert.Len(t, entries, 2)
		// Both entries contain "users" either in message or fields
	})

	t.Run("clear buffer", func(t *testing.T) {
		rb := NewRingBuffer(5)

		// Add entries
		for i := 0; i < 3; i++ {
			rb.Add(LogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Message %d", i),
			})
		}

		assert.Equal(t, 3, rb.Size())

		// Clear
		rb.Clear()
		assert.Equal(t, 0, rb.Size())
		assert.Len(t, rb.GetEntries(), 0)
	})

	t.Run("concurrent access", func(t *testing.T) {
		rb := NewRingBuffer(100)
		done := make(chan bool)

		// Writer goroutine
		go func() {
			for i := 0; i < 50; i++ {
				rb.Add(LogEntry{
					Timestamp: time.Now(),
					Level:     "info",
					Message:   fmt.Sprintf("Message %d", i),
				})
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()

		// Reader goroutine
		go func() {
			for i := 0; i < 10; i++ {
				entries := rb.GetEntries()
				_ = entries // Just access, don't assert (concurrent test)
				time.Sleep(5 * time.Millisecond)
			}
			done <- true
		}()

		// Wait for both to complete
		<-done
		<-done

		// Final check
		assert.Equal(t, 50, rb.Size())
	})
}
