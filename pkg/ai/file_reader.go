package ai

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	// LoggingCallback is called when a file operation is logged (if not nil)
	LoggingCallback func(message string)
)

// ReadFile reads a file from the specified base directory with optional logging
func ReadFile(baseDir, path string) (string, error) {
	if LoggingCallback != nil {
		message := fmt.Sprintf("📄 LLM reading file: %s", path)
		LoggingCallback(message)
	}

	fullPath := filepath.Join(baseDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FileExists checks if a file exists in the specified base directory with optional logging
func FileExists(baseDir, path string) bool {
	if LoggingCallback != nil {
		message := fmt.Sprintf("🔍 LLM checking if file exists: %s", path)
		LoggingCallback(message)
	}

	fullPath := filepath.Join(baseDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ListDirectory lists files in a directory with optional logging
func ListDirectory(baseDir, path string) ([]string, error) {
	if LoggingCallback != nil {
		message := fmt.Sprintf("📂 LLM listing directory: %s", path)
		LoggingCallback(message)
	}

	fullPath := filepath.Join(baseDir, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		files = append(files, name)
	}
	return files, nil
}
