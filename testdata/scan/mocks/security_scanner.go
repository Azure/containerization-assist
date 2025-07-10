package mocks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Azure/container-kit/pkg/mcp/domain/scan"
)

// MockSecurityScanner provides mock implementations for security scanning
type MockSecurityScanner struct {
	ScanResults     map[string]string // image -> result file path
	ScanErrors      map[string]error  // image -> error to return
	ScanCalls       []ScanCall        // track all scan calls
	DefaultResponse string            // default response file
}

// ScanCall represents a call to the scanner
type ScanCall struct {
	Image     string
	ScanType  string
	Scanner   string
	Timestamp string
}

// NewMockSecurityScanner creates a new mock security scanner
func NewMockSecurityScanner() *MockSecurityScanner {
	return &MockSecurityScanner{
		ScanResults: make(map[string]string),
		ScanErrors:  make(map[string]error),
		ScanCalls:   make([]ScanCall, 0),
	}
}

// SetScanResult sets the result file to return for a specific image
func (m *MockSecurityScanner) SetScanResult(image, resultFile string) {
	m.ScanResults[image] = resultFile
}

// SetScanError sets an error to return for a specific image
func (m *MockSecurityScanner) SetScanError(image string, err error) {
	m.ScanErrors[image] = err
}

// SetDefaultResponse sets the default response file
func (m *MockSecurityScanner) SetDefaultResponse(file string) {
	m.DefaultResponse = file
}

// ScanImage mocks the security scanning process
func (m *MockSecurityScanner) ScanImage(ctx context.Context, image, scanType, scanner string) ([]byte, error) {
	// Record the call
	m.ScanCalls = append(m.ScanCalls, ScanCall{
		Image:     image,
		ScanType:  scanType,
		Scanner:   scanner,
		Timestamp: "2023-01-01T00:00:00Z",
	})

	// Check for specific error
	if err, exists := m.ScanErrors[image]; exists {
		return nil, err
	}

	// Check for specific result
	if resultFile, exists := m.ScanResults[image]; exists {
		return m.loadTestData(resultFile)
	}

	// Return default response
	if m.DefaultResponse != "" {
		return m.loadTestData(m.DefaultResponse)
	}

	// Default clean response
	return m.loadTestData("testdata/scan/trivy/response_clean.json")
}

// loadTestData loads test data from file
func (m *MockSecurityScanner) loadTestData(filename string) ([]byte, error) {
	// If filename is not absolute, make it relative to project root
	if !filepath.IsAbs(filename) {
		filename = filepath.Join("..", "..", "..", filename)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open test data file %s: %w", filename, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read test data file %s: %w", filename, err)
	}

	return data, nil
}

// GetScanCalls returns all recorded scan calls
func (m *MockSecurityScanner) GetScanCalls() []ScanCall {
	return m.ScanCalls
}

// Reset clears all recorded data
func (m *MockSecurityScanner) Reset() {
	m.ScanResults = make(map[string]string)
	m.ScanErrors = make(map[string]error)
	m.ScanCalls = make([]ScanCall, 0)
	m.DefaultResponse = ""
}

// MockSecretScanner provides mock implementations for secret scanning
type MockSecretScanner struct {
	ScanResults map[string][]scan.SecretMatch // file -> secrets found
	ScanErrors  map[string]error              // file -> error to return
	ScanCalls   []SecretScanCall              // track all scan calls
}

// SecretScanCall represents a call to the secret scanner
type SecretScanCall struct {
	FilePath  string
	Patterns  []string
	Timestamp string
}

// NewMockSecretScanner creates a new mock secret scanner
func NewMockSecretScanner() *MockSecretScanner {
	return &MockSecretScanner{
		ScanResults: make(map[string][]scan.SecretMatch),
		ScanErrors:  make(map[string]error),
		ScanCalls:   make([]SecretScanCall, 0),
	}
}

// SetScanResult sets secrets to return for a specific file
func (m *MockSecretScanner) SetScanResult(filePath string, secrets []scan.SecretMatch) {
	m.ScanResults[filePath] = secrets
}

// SetScanError sets an error to return for a specific file
func (m *MockSecretScanner) SetScanError(filePath string, err error) {
	m.ScanErrors[filePath] = err
}

// ScanFile mocks the secret scanning process
func (m *MockSecretScanner) ScanFile(ctx context.Context, filePath string, patterns []string) ([]scan.SecretMatch, error) {
	// Record the call
	m.ScanCalls = append(m.ScanCalls, SecretScanCall{
		FilePath:  filePath,
		Patterns:  patterns,
		Timestamp: "2023-01-01T00:00:00Z",
	})

	// Check for specific error
	if err, exists := m.ScanErrors[filePath]; exists {
		return nil, err
	}

	// Check for specific result
	if secrets, exists := m.ScanResults[filePath]; exists {
		return secrets, nil
	}

	// Default empty response
	return []scan.SecretMatch{}, nil
}

// GetScanCalls returns all recorded scan calls
func (m *MockSecretScanner) GetScanCalls() []SecretScanCall {
	return m.ScanCalls
}

// Reset clears all recorded data
func (m *MockSecretScanner) Reset() {
	m.ScanResults = make(map[string][]scan.SecretMatch)
	m.ScanErrors = make(map[string]error)
	m.ScanCalls = make([]SecretScanCall, 0)
}
