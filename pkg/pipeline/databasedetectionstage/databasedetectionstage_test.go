package databasedetectionstage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/container-copilot/pkg/pipeline"
)

// TestDatabaseDetectionStage_Initialize tests the Initialize method
func TestDatabaseDetectionStage_Initialize(t *testing.T) {
	// Create a test pipeline
	stage := &DatabaseDetectionStage{}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test initializing
	err := stage.Initialize(context.Background(), state, "/test/path")
	if err != nil {
		t.Errorf("Initialize should not return an error, got: %v", err)
	}
}

// TestDatabaseDetectionStage_Run tests the Run method
func TestDatabaseDetectionStage_Run(t *testing.T) {
	type testCase struct {
		name     string                    // Test case name
		content  string                    // Input file content
		expected []pipeline.DatabaseDetectionResult // Expected detected databases
	}

	tests := []testCase{
		{
			name: "Valid database content",
			content: `
                mysql 8.0.16
                <postgres.version>15.3</postgres.version>
                redis.version 7.0.11
				redistribution
                Cassandra version 14.5.6
            `,
			expected: []pipeline.DatabaseDetectionResult{
				{Type: "Cassandra", Version: "14.5.6"},
				{Type: "MySQL", Version: "8.0.16"},
				{Type: "PostgreSQL", Version: "15.3"},
				{Type: "Redis", Version: "7.0.11"},
			},
		},
		{
			name: "Non-database content",
			content: `
                Some random text
				Redistribution
                Not a database
                Just some words
            `,
			expected: []pipeline.DatabaseDetectionResult{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a temporary file with the test content
			tmpDir := t.TempDir()
			testFilePath := filepath.Join(tmpDir, "testfile.txt")
			if err := os.WriteFile(testFilePath, []byte(test.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Create a test pipeline
			stage := &DatabaseDetectionStage{}
			state := &pipeline.PipelineState{
				Metadata: make(map[pipeline.MetadataKey]any),
			}

			// Run the detection stage
			err := stage.Run(context.Background(), state, nil, pipeline.RunnerOptions{TargetDirectory: tmpDir})
			if err != nil {
				t.Errorf("Run should not return an error, got: %v", err)
			}

			// Validate detected databases
			detectedDatabases:= state.DetectedDatabases
			ok := state.Metadata["detectedDatabaseErrors"]
			if ok != nil {
				t.Fatalf("Run did not populate detected databases in metadata")
			}

			if len(detectedDatabases) != len(test.expected) {
				t.Errorf("Expected %d detected databases, got %d", len(test.expected), len(detectedDatabases))
			}

			for i, db := range detectedDatabases {
				if db.Type != test.expected[i].Type || db.Version != test.expected[i].Version {
					t.Errorf("Detected database mismatch. Expected: %v, Got: %v", test.expected[i], db)
				}
			}
		})
	}
}

// TestDatabaseDetectionStage_GetErrors tests the GetErrors method
func TestDatabaseDetectionStage_GetErrors(t *testing.T) {
	// Create a test pipeline
	stage := &DatabaseDetectionStage{
		errors: []error{
			os.ErrNotExist,
			os.ErrPermission,
		},
	}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test getting errors
	errors := stage.GetErrors(state)
	expected := "file does not exist\npermission denied"
	if errors != expected {
		t.Errorf("GetErrors should return concatenated error messages, expected: %s, got: %s", expected, errors)
	}
}

// TestDatabaseDetectionStage_WriteSuccessfulFiles tests the WriteSuccessfulFiles method
func TestDatabaseDetectionStage_WriteSuccessfulFiles(t *testing.T) {
	// Create a test pipeline
	stage := &DatabaseDetectionStage{}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test writing files (no-op for this stage)
	err := stage.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles should not return an error, got: %v", err)
	}
}

// TestDatabaseDetectionStage_DetectDatabases tests the detectDatabases method
func TestDatabaseDetectionStage_DetectDatabases(t *testing.T) {
	// Create a test pipeline
	stage := &DatabaseDetectionStage{}

	// Create a temp directory for testing
	tmpDir := t.TempDir()

	// Create test files with database-related content
	testFiles := map[string]string{
		"mysql.txt":      "mysql 8.0.16",
		"postgresql.xml": "<postgresql.version>15.3</postgresql.version>",
		"redis.txt":      "redis.version 7.0.11",
		"invalid.txt":    "redistribution",
	}

	for name, content := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", name, err)
		}
	}

	// Run the detection
	detectedDatabases, err := stage.detectDatabases(tmpDir)
	if err != nil {
		t.Errorf("detectDatabases should not return an error, got: %v", err)
	}

	// Validate detected databases
	expected := []pipeline.DatabaseDetectionResult{
		{Type: "MySQL", Version: "8.0.16"},
		{Type: "PostgreSQL", Version: "15.3"},
		{Type: "Redis", Version: "7.0.11"},
	}

	if len(detectedDatabases) != len(expected) {
		t.Errorf("Expected %d detected databases, got %d", len(expected), len(detectedDatabases))
	}

	for i, db := range detectedDatabases {
		if db.Type != expected[i].Type || db.Version != expected[i].Version {
			t.Errorf("Detected database mismatch. Expected: %v, Got: %v", expected[i], db)
		}
	}
}
