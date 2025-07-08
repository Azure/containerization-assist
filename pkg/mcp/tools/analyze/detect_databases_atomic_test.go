package analyze

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicDetectDatabasesTool(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "database-detection-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files to simulate different database scenarios
	testCases := []struct {
		name          string
		files         map[string]string
		expectedDBs   []DatabaseType
		minConfidence float64
	}{
		{
			name: "PostgreSQL in docker-compose",
			files: map[string]string{
				"docker-compose.yml": `
version: '3.8'
services:
  postgres:
    image: postgres:13
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
`,
			},
			expectedDBs:   []DatabaseType{PostgreSQL},
			minConfidence: 0.3,
		},
		{
			name: "MySQL with environment variables",
			files: map[string]string{
				".env": `
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_DATABASE=myapp
MYSQL_USER=root
MYSQL_PASSWORD=secret
`,
				"package.json": `{
  "dependencies": {
    "mysql2": "^3.0.0"
  }
}`,
			},
			expectedDBs:   []DatabaseType{MySQL},
			minConfidence: 0.2,
		},
		{
			name: "MongoDB connection string",
			files: map[string]string{
				"config.js": `
const config = {
  database: 'mongodb://localhost:27017/myapp',
  host: 'localhost'
};
`,
			},
			expectedDBs:   []DatabaseType{MongoDB},
			minConfidence: 0.1,
		},
		{
			name: "Redis configuration",
			files: map[string]string{
				"redis.conf": `
port 6379
bind 127.0.0.1
save 900 1
`,
				".env": `
REDIS_HOST=localhost
REDIS_PORT=6379
`,
			},
			expectedDBs:   []DatabaseType{Redis},
			minConfidence: 0.4,
		},
		{
			name: "Multiple databases",
			files: map[string]string{
				"docker-compose.yml": `
version: '3.8'
services:
  postgres:
    image: postgres:13
  redis:
    image: redis:6
  web:
    build: .
`,
				"package.json": `{
  "dependencies": {
    "pg": "^8.0.0",
    "redis": "^4.0.0"
  }
}`,
			},
			expectedDBs:   []DatabaseType{PostgreSQL, Redis},
			minConfidence: 0.2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test directory for this case
			testDir := filepath.Join(tmpDir, tc.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			// Create test files
			for filename, content := range tc.files {
				filePath := filepath.Join(testDir, filename)
				err := os.MkdirAll(filepath.Dir(filePath), 0755)
				if err != nil {
					t.Fatalf("Failed to create file directory: %v", err)
				}

				err = os.WriteFile(filePath, []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to write test file %s: %v", filename, err)
				}
			}

			// Create and execute the tool
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			tool := NewAtomicDetectDatabasesTool(logger)
			params := DatabaseDetectionParams{
				RepositoryPath: testDir,
				ScanDepth:      3,
				IncludeConfig:  true,
			}

			ctx := context.Background()
			result, err := tool.Execute(ctx, params)
			if err != nil {
				t.Fatalf("Tool execution failed: %v", err)
			}

			// Verify results
			if !result.Success {
				t.Errorf("Expected successful detection, got failure")
			}

			// Check that expected databases were detected
			detectedTypes := make(map[DatabaseType]bool)
			for _, db := range result.DatabasesFound {
				detectedTypes[db.Type] = true

				// Check confidence levels
				if db.Confidence < tc.minConfidence {
					t.Errorf("Database %s has low confidence %f, expected at least %f",
						db.Type, db.Confidence, tc.minConfidence)
				}

				// Check that evidence sources are present
				if len(db.EvidenceSources) == 0 {
					t.Errorf("Database %s has no evidence sources", db.Type)
				}
			}

			// Verify all expected databases were detected
			for _, expectedDB := range tc.expectedDBs {
				if !detectedTypes[expectedDB] {
					t.Errorf("Expected database %s was not detected. Found: %v",
						expectedDB, detectedTypes)
				}
			}

			// Verify metadata
			if result.Metadata.ScanPath != testDir {
				t.Errorf("Expected scan path %s, got %s", testDir, result.Metadata.ScanPath)
			}

			if result.Metadata.ScanDuration <= 0 {
				t.Errorf("Expected positive scan duration, got %v", result.Metadata.ScanDuration)
			}
		})
	}
}

func TestAtomicDetectDatabasesToolValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	tool := NewAtomicDetectDatabasesTool(logger)
	ctx := context.Background()

	// Test invalid parameters
	invalidParams := DatabaseDetectionParams{
		RepositoryPath: "", // Empty path should fail validation
	}

	_, err := tool.Execute(ctx, invalidParams)
	if err == nil {
		t.Error("Expected validation error for empty repository path")
	}

	// Test validation method directly
	err = tool.Validate(ctx, invalidParams)
	if err == nil {
		t.Error("Expected validation error for empty repository path")
	}
}

func TestAtomicDetectDatabasesToolMetadata(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	tool := NewAtomicDetectDatabasesTool(logger)
	metadata := tool.GetMetadata()

	// Verify metadata fields
	if metadata.Name != "atomic_detect_databases" {
		t.Errorf("Expected name 'atomic_detect_databases', got '%s'", metadata.Name)
	}

	if metadata.Version == "" {
		t.Error("Expected non-empty version")
	}

	if metadata.Category != "analyze" {
		t.Errorf("Expected category 'analyze', got '%s'", metadata.Category)
	}

	// Check capabilities
	expectedCapabilities := []string{
		"postgresql_detection",
		"mysql_detection",
		"mongodb_detection",
		"redis_detection",
		"docker_compose_analysis",
		"environment_variable_detection",
		"configuration_file_analysis",
	}

	for _, capability := range expectedCapabilities {
		found := false
		for _, c := range metadata.Capabilities {
			if c == capability {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", capability)
		}
	}
}
