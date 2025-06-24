package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestLogger(t *testing.T) {
	t.Run("basic logging", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		logger := NewLoggerWithWriter("test", &buf)
		
		logger.Info("test message", Str("key", "value"))
		output := buf.String()
		
		if !strings.Contains(output, "test message") {
			t.Errorf("expected output to contain 'test message', got: %s", output)
		}
		if !strings.Contains(output, `"key":"value"`) {
			t.Errorf("expected output to contain field, got: %s", output)
		}
		if !strings.Contains(output, `"component":"test"`) {
			t.Errorf("expected output to contain component, got: %s", output)
		}
	})
	
	t.Run("log levels", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			logFunc  func(Logger, string, ...Field)
			expected string
		}{
			{
				name: "info",
				logFunc: func(l Logger, msg string, fields ...Field) {
					l.Info(msg, fields...)
				},
				expected: `"level":"info"`,
			},
			{
				name: "warn",
				logFunc: func(l Logger, msg string, fields ...Field) {
					l.Warn(msg, fields...)
				},
				expected: `"level":"warn"`,
			},
			{
				name: "debug",
				logFunc: func(l Logger, msg string, fields ...Field) {
					l.Debug(msg, fields...)
				},
				expected: `"level":"debug"`,
			},
		}
		
		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				logger := NewLoggerWithWriter("test", &buf)
				
				tc.logFunc(logger, "test message")
				output := buf.String()
				
				if !strings.Contains(output, tc.expected) {
					t.Errorf("expected output to contain %s, got: %s", tc.expected, output)
				}
			})
		}
	})
	
	t.Run("error logging", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		logger := NewLoggerWithWriter("test", &buf)
		
		testErr := errors.New("test error")
		logger.Error("error occurred", testErr, Str("context", "testing"))
		output := buf.String()
		
		if !strings.Contains(output, `"level":"error"`) {
			t.Errorf("expected error level, got: %s", output)
		}
		if !strings.Contains(output, "test error") {
			t.Errorf("expected error message, got: %s", output)
		}
		if !strings.Contains(output, `"context":"testing"`) {
			t.Errorf("expected context field, got: %s", output)
		}
	})
	
	t.Run("field types", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		logger := NewLoggerWithWriter("test", &buf)
		
		logger.Info("test", 
			Str("string", "value"),
			Int("int", 42),
			Int64("int64", int64(123)),
			Bool("bool", true),
			Any("any", map[string]string{"foo": "bar"}),
		)
		
		output := buf.String()
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}
		
		if result["string"] != "value" {
			t.Errorf("expected string field to be 'value', got: %v", result["string"])
		}
		if result["int"] != float64(42) {
			t.Errorf("expected int field to be 42, got: %v", result["int"])
		}
		if result["int64"] != float64(123) {
			t.Errorf("expected int64 field to be 123, got: %v", result["int64"])
		}
		if result["bool"] != true {
			t.Errorf("expected bool field to be true, got: %v", result["bool"])
		}
	})
	
	t.Run("with fields", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		logger := NewLoggerWithWriter("test", &buf)
		
		childLogger := logger.With(
			Str("service", "api"),
			Int("version", 1),
		)
		
		childLogger.Info("child message")
		output := buf.String()
		
		if !strings.Contains(output, `"service":"api"`) {
			t.Errorf("expected service field, got: %s", output)
		}
		if !strings.Contains(output, `"version":1`) {
			t.Errorf("expected version field, got: %s", output)
		}
	})
	
	t.Run("with context", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		logger := NewLoggerWithWriter("test", &buf)
		
		ctx := context.Background()
		contextLogger := logger.WithContext(ctx)
		
		contextLogger.Info("context message")
		output := buf.String()
		
		if !strings.Contains(output, "context message") {
			t.Errorf("expected context message, got: %s", output)
		}
	})
	
	t.Run("global logger", func(t *testing.T) {
		t.Parallel()
		// Reset global logger for test
		globalLogger = nil
		globalLoggerOnce = sync.Once{}
		
		logger1 := GetGlobalLogger()
		logger2 := GetGlobalLogger()
		
		if logger1 != logger2 {
			t.Error("expected global logger to be singleton")
		}
	})
}