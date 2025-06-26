package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_read_file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test file
	testContent := "Hello, World!\nThis is a test file."
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte(testContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		baseDir     string
		path        string
		expectError bool
		expected    string
	}{
		{
			name:        "read existing file",
			baseDir:     tempDir,
			path:        "test.txt",
			expectError: false,
			expected:    testContent,
		},
		{
			name:        "read non-existent file",
			baseDir:     tempDir,
			path:        "nonexistent.txt",
			expectError: true,
			expected:    "",
		},
		{
			name:        "read file with path traversal",
			baseDir:     tempDir,
			path:        "../test.txt",
			expectError: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReadFile(tt.baseDir, tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestReadFileWithLogging(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_read_file_logging")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Capture logging output
	var loggedMessages []string
	LoggingCallback = func(message string) {
		loggedMessages = append(loggedMessages, message)
	}
	defer func() { LoggingCallback = nil }()

	_, err = ReadFile(tempDir, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(loggedMessages) != 1 {
		t.Errorf("Expected 1 logged message, got %d", len(loggedMessages))
	}

	expectedLog := "📄 LLM reading file: test.txt"
	if !strings.Contains(loggedMessages[0], "test.txt") {
		t.Errorf("Expected log to contain 'test.txt', got: %s", loggedMessages[0])
	}

	if loggedMessages[0] != expectedLog {
		t.Errorf("Expected log %q, got %q", expectedLog, loggedMessages[0])
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_file_exists")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test file
	testFile := filepath.Join(tempDir, "exists.txt")
	err = os.WriteFile(testFile, []byte("content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		baseDir  string
		path     string
		expected bool
	}{
		{
			name:     "existing file",
			baseDir:  tempDir,
			path:     "exists.txt",
			expected: true,
		},
		{
			name:     "non-existent file",
			baseDir:  tempDir,
			path:     "nonexistent.txt",
			expected: false,
		},
		{
			name:     "directory itself",
			baseDir:  tempDir,
			path:     ".",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FileExists(tt.baseDir, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFileExistsWithLogging(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_file_exists_logging")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Capture logging output
	var loggedMessages []string
	LoggingCallback = func(message string) {
		loggedMessages = append(loggedMessages, message)
	}
	defer func() { LoggingCallback = nil }()

	FileExists(tempDir, "any_file.txt")

	if len(loggedMessages) != 1 {
		t.Errorf("Expected 1 logged message, got %d", len(loggedMessages))
	}

	expectedLog := "🔍 LLM checking if file exists: any_file.txt"
	if loggedMessages[0] != expectedLog {
		t.Errorf("Expected log %q, got %q", expectedLog, loggedMessages[0])
	}
}

func TestListDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_list_directory")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.go", "file3.md"}
	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err = os.WriteFile(filePath, []byte("content"), 0600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0750)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	tests := []struct {
		name        string
		baseDir     string
		path        string
		expectError bool
		expectCount int
		expectFiles []string
	}{
		{
			name:        "list existing directory",
			baseDir:     tempDir,
			path:        ".",
			expectError: false,
			expectCount: 4, // 3 files + 1 directory
			expectFiles: []string{"file1.txt", "file2.go", "file3.md", "subdir/"},
		},
		{
			name:        "list non-existent directory",
			baseDir:     tempDir,
			path:        "nonexistent",
			expectError: true,
			expectCount: 0,
			expectFiles: nil,
		},
		{
			name:        "list empty subdirectory",
			baseDir:     tempDir,
			path:        "subdir",
			expectError: false,
			expectCount: 0,
			expectFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ListDirectory(tt.baseDir, tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
				return
			}

			if len(result) != tt.expectCount {
				t.Errorf("Expected %d files, got %d: %v", tt.expectCount, len(result), result)
				return
			}

			// Check that all expected files are present
			for _, expectedFile := range tt.expectFiles {
				found := false
				for _, actualFile := range result {
					if actualFile == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %q not found in result: %v", expectedFile, result)
				}
			}
		})
	}
}

func TestListDirectoryWithLogging(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test_list_directory_logging")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Capture logging output
	var loggedMessages []string
	LoggingCallback = func(message string) {
		loggedMessages = append(loggedMessages, message)
	}
	defer func() { LoggingCallback = nil }()

	_, err = ListDirectory(tempDir, ".")
	if err != nil {
		t.Fatalf("ListDirectory failed: %v", err)
	}

	if len(loggedMessages) != 1 {
		t.Errorf("Expected 1 logged message, got %d", len(loggedMessages))
	}

	expectedLog := "📂 LLM listing directory: ."
	if loggedMessages[0] != expectedLog {
		t.Errorf("Expected log %q, got %q", expectedLog, loggedMessages[0])
	}
}

func TestLoggingCallbackGlobalState(t *testing.T) {
	// Test that LoggingCallback can be set and unset properly
	originalCallback := LoggingCallback
	defer func() { LoggingCallback = originalCallback }()

	// Initially should be nil (or whatever was set before)
	LoggingCallback = nil

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "test_logging_callback")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test that no logging happens when callback is nil
	_, _ = ReadFile(tempDir, "nonexistent.txt") // Will error but shouldn't log

	// Set a callback and test that logging happens
	var messages []string
	LoggingCallback = func(msg string) {
		messages = append(messages, msg)
	}

	_, _ = ReadFile(tempDir, "nonexistent.txt") // Will error but should log
	FileExists(tempDir, "nonexistent.txt")
	_, _ = ListDirectory(tempDir, ".")

	if len(messages) != 3 {
		t.Errorf("Expected 3 log messages, got %d: %v", len(messages), messages)
	}

	// Clear callback and test no more logging
	LoggingCallback = nil
	messages = nil

	_, _ = ReadFile(tempDir, "nonexistent.txt")
	if len(messages) != 0 {
		t.Errorf("Expected no log messages after clearing callback, got %d", len(messages))
	}
}
