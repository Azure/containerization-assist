package ai

import (
	"os"
	"path/filepath"
)

// FileReader interface defines methods for reading files
type FileReader interface {
	ReadFile(path string) (string, error)
	FileExists(path string) bool
	ListDirectory(path string) ([]string, error)
}

// DefaultFileReader provides a standard implementation of FileReader
type DefaultFileReader struct {
	BaseDir string
}

func (r *DefaultFileReader) ReadFile(path string) (string, error) {
	fullPath := filepath.Join(r.BaseDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *DefaultFileReader) FileExists(path string) bool {
	fullPath := filepath.Join(r.BaseDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (r *DefaultFileReader) ListDirectory(path string) ([]string, error) {
	fullPath := filepath.Join(r.BaseDir, path)
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
