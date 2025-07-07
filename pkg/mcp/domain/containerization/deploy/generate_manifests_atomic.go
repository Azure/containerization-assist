package deploy

import (
	"context"
	"fmt"
	"time"

	// mcp import removed - using mcptypes

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/database_detectors"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/Azure/container-kit/pkg/mcp/services"
	"github.com/localrivet/gomcp/server"
)

// Type aliases for atomic manifest generation to maintain backward compatibility
type GenerateManifestsArgs = GenerateManifestsRequest
type GenerateManifestsResult = kubernetes.ManifestGenerationResult
type AtomicGenerateManifestsArgs = GenerateManifestsArgs
type AtomicGenerateManifestsResult = GenerateManifestsResult

// AtomicGenerateManifestsTool is a simple stub for backward compatibility
type AtomicGenerateManifestsTool struct {
	logger           *slog.Logger
	baseTool         *GenerateManifestsTool
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	databaseDetector *analyze.AtomicDetectDatabasesTool
}

// NewAtomicGenerateManifestsTool creates a basic atomic tool using unified session manager
func NewAtomicGenerateManifestsTool(adapter mcptypes.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicGenerateManifestsTool {
	toolLogger := logger.With("tool", "atomic_generate_manifests")
	return createAtomicGenerateManifestsTool(adapter, sessionManager, toolLogger)
}

// NewAtomicGenerateManifestsToolWithServices creates a basic atomic tool using service interfaces
func NewAtomicGenerateManifestsToolWithServices(adapter mcptypes.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicGenerateManifestsTool {
	toolLogger := logger.With("tool", "atomic_generate_manifests")

	baseToolInterface := NewGenerateManifestsTool(toolLogger, "/tmp/container-kit")
	baseTool, ok := baseToolInterface.(*GenerateManifestsTool)
	if !ok {
		// This should never happen, but we handle it gracefully
		toolLogger.Error("Failed to type assert GenerateManifestsTool")
		baseTool = &GenerateManifestsTool{logger: toolLogger}
	}

	return &AtomicGenerateManifestsTool{
		logger:           toolLogger,
		baseTool:         baseTool,
		sessionStore:     serviceContainer.SessionStore(),
		sessionState:     serviceContainer.SessionState(),
		databaseDetector: analyze.NewAtomicDetectDatabasesTool(toolLogger),
	}
}

// createAtomicGenerateManifestsTool is the common creation logic
func createAtomicGenerateManifestsTool(adapter mcptypes.TypedPipelineOperations, sessionManager session.UnifiedSessionManager, logger *slog.Logger) *AtomicGenerateManifestsTool {
	baseToolInterface := NewGenerateManifestsTool(logger, "/tmp/container-kit")
	baseTool, ok := baseToolInterface.(*GenerateManifestsTool)
	if !ok {
		// This should never happen, but we handle it gracefully
		logger.Error("Failed to type assert GenerateManifestsTool")
		baseTool = &GenerateManifestsTool{logger: logger}
	}
	// Initialize standardized session management
	return &AtomicGenerateManifestsTool{
		logger:           logger,
		baseTool:         baseTool,
		sessionStore:     nil, // Will be set when services are injected
		sessionState:     nil, // Will be set when services are injected
		databaseDetector: analyze.NewAtomicDetectDatabasesTool(logger),
	}
}

// GetName returns the tool name
func (t *AtomicGenerateManifestsTool) GetName() string {
	return "atomic_generate_manifests"
}

// Execute delegates to the base tool with session workspace handling
func (t *AtomicGenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Extract session ID from args to get workspace
	var sessionID string
	switch v := args.(type) {
	case GenerateManifestsArgs:
		sessionID = v.SessionID
	case map[string]interface{}:
		if sid, ok := v["session_id"].(string); ok {
			sessionID = sid
		}
	}

	// Get workspace directory from session if available
	if sessionID != "" && t.sessionStore != nil {
		apiSession, err := t.sessionStore.Get(context.Background(), sessionID)
		if err == nil {
			// Extract workspace directory from session metadata
			if workspaceDir, ok := apiSession.Metadata["workspace_dir"].(string); ok && workspaceDir != "" {
				// Create a new base tool with the correct workspace
				baseToolInterface := NewGenerateManifestsTool(t.logger, workspaceDir)
				if baseTool, ok := baseToolInterface.(*GenerateManifestsTool); ok {
					t.baseTool = baseTool
				}
			}
		}
	}

	// Convert args to ToolInput
	toolInput, ok := args.(api.ToolInput)
	if !ok {
		// If args is not ToolInput, try to convert from GenerateManifestsArgs
		if manifestArgs, ok := args.(GenerateManifestsArgs); ok {
			toolInput = api.ToolInput{
				SessionID: manifestArgs.SessionID,
				Data: map[string]interface{}{
					"app_name":        manifestArgs.AppName,
					"image_reference": manifestArgs.ImageReference,
					"port":            manifestArgs.Port,
					"namespace":       manifestArgs.Namespace,
					"include_ingress": manifestArgs.IncludeIngress,
					"ingress_host":    manifestArgs.IngressHost,
				},
			}
		} else {
			return api.ToolOutput{
				Success: false,
				Error:   "invalid argument type",
			}, errors.NewError().Messagef("invalid argument type: expected ToolInput or GenerateManifestsArgs, got %T", args).WithLocation().Build()
		}
	}

	return t.baseTool.Execute(ctx, toolInput)
}

// ExecuteWithContext executes the tool with the provided arguments using the standard MCP pattern
func (t *AtomicGenerateManifestsTool) ExecuteWithContext(ctx *server.Context, args *GenerateManifestsArgs) (*GenerateManifestsResult, error) {
	startTime := time.Now()

	t.logger.Info("Starting atomic manifest generation",
		"app_name", args.AppName,
		"session_id", args.SessionID)

	// Step 1: Handle session management using focused services
	sessionData, err := t.sessionStore.Get(context.Background(), args.SessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).Build()
	}

	// Convert to core.SessionState for compatibility
	session := &core.SessionState{
		SessionID: sessionData.ID,
		Metadata:  sessionData.Metadata,
	}
	t.logger.Info("Using session for manifest generation",
		"session_id", session.SessionID)

	// Step 2: Set up workspace directory from session
	workspaceDir := session.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = "/tmp/container-kit"
	}

	// Create a new base tool with the correct workspace
	baseToolInterface := NewGenerateManifestsTool(t.logger, workspaceDir)
	if baseTool, ok := baseToolInterface.(*GenerateManifestsTool); ok {
		t.baseTool = baseTool
	}

	// Update session ID in args to use the actual session ID
	args.SessionID = session.SessionID

	// Step 2.5: Run database detection if workspace contains a repository
	detectedDatabases, err := t.detectDatabases(ctx, session, workspaceDir)
	if err != nil {
		t.logger.Warn("Failed to detect databases, continuing without database configuration", "error", err)
		// Continue without database configuration
		detectedDatabases = nil
	} else if detectedDatabases != nil && len(detectedDatabases.DatabasesFound) > 0 {
		t.logger.Info("Detected databases in repository",
			"database_count", len(detectedDatabases.DatabasesFound))

		// Add database environment variables to deployment args
		if args.Environment == nil {
			args.Environment = []SecretValue{}
		}

		// Add environment variables for each detected database
		for _, db := range detectedDatabases.DatabasesFound {
			t.addDatabaseEnvironmentVars(args, db)
		}
	}

	// Step 3: Execute the base tool with the correct types
	// Convert GenerateManifestsArgs to ToolInput
	toolInput := api.ToolInput{
		SessionID: args.SessionID,
		Data: map[string]interface{}{
			"app_name":        args.AppName,
			"image_reference": args.ImageReference,
			"port":            args.Port,
			"namespace":       args.Namespace,
			"include_ingress": args.IncludeIngress,
			"ingress_host":    args.IngressHost,
		},
	}
	result, err := t.baseTool.Execute(context.Background(), toolInput)

	// Step 4: Update session metadata with execution result
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["last_tool_execution"] = map[string]interface{}{
		"tool":      "atomic_generate_manifests",
		"timestamp": time.Now(),
		"success":   result.Success,
	}

	if err != nil {
		t.logger.Error("Manifest generation failed", "error", err,
			"session_id", session.SessionID,
			"duration", time.Since(startTime))
		return nil, err
	}

	// Extract data from ToolOutput and create ManifestGenerationResult
	var typedResult *kubernetes.ManifestGenerationResult
	if result.Success {
		manifests, _ := result.Data["manifests"].([]kubernetes.GeneratedManifest)
		outputDir, _ := result.Data["output_dir"].(string)
		manifestCount, _ := result.Data["manifest_count"].(int)

		typedResult = &kubernetes.ManifestGenerationResult{
			Success:   true,
			Manifests: manifests,
			OutputDir: outputDir,
			Duration:  time.Since(startTime),
		}

		t.logger.Info("Manifest generation completed successfully",
			"session_id", session.SessionID,
			"output_dir", outputDir,
			"manifest_count", manifestCount,
			"duration", time.Since(startTime))
	} else {
		typedResult = &kubernetes.ManifestGenerationResult{
			Success: false,
			Error: &kubernetes.ManifestError{
				Message: result.Error,
			},
			Duration: time.Since(startTime),
		}
	}

	return typedResult, nil
}

func (t *AtomicGenerateManifestsTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// GetMetadata returns metadata for this tool
func (t *AtomicGenerateManifestsTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_generate_manifests",
		Description:  "Atomically generates Kubernetes manifests with database detection",
		Version:      "1.0.0",
		Category:     api.ToolCategory("deployment"),
		Dependencies: []string{"kubernetes"},
		Capabilities: []string{
			"manifest_generation",
			"database_detection",
			"session_management",
		},
		Requirements: []string{"workspace_access"},
		Tags:         []string{"kubernetes", "manifests", "atomic"},
		Status:       api.ToolStatus("active"),
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// SetAnalyzer is a compatibility method
func (t *AtomicGenerateManifestsTool) SetAnalyzer(analyzer interface{}) {
	// No-op for compatibility
	t.logger.Debug("SetAnalyzer called on atomic tool (no-op)")
}

// detectDatabases runs database detection on the repository
func (t *AtomicGenerateManifestsTool) detectDatabases(ctx *server.Context, session *core.SessionState, workspaceDir string) (*database_detectors.DatabaseDetectionResult, error) {
	// Check if repository exists in workspace
	repoPath := workspaceDir + "/repo"

	// Create detection parameters
	params := database_detectors.DatabaseDetectionParams{
		RepositoryPath: repoPath,
		ScanDepth:      3, // Default scan depth
		IncludeConfig:  true,
	}

	// Run database detection
	result, err := t.databaseDetector.Execute(context.Background(), params)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// addDatabaseEnvironmentVars adds environment variables for a detected database
func (t *AtomicGenerateManifestsTool) addDatabaseEnvironmentVars(args *GenerateManifestsArgs, db database_detectors.DetectedDatabase) {
	// Add environment variables based on database type
	switch db.Type {
	case database_detectors.PostgreSQL:
		t.addPostgreSQLEnvVars(args, db)
	case database_detectors.MySQL:
		t.addMySQLEnvVars(args, db)
	case database_detectors.MongoDB:
		t.addMongoDBEnvVars(args, db)
	case database_detectors.Redis:
		t.addRedisEnvVars(args, db)
	}
}

// addPostgreSQLEnvVars adds PostgreSQL environment variables
func (t *AtomicGenerateManifestsTool) addPostgreSQLEnvVars(args *GenerateManifestsArgs, db database_detectors.DetectedDatabase) {
	// Add standard PostgreSQL environment variables
	args.Environment = append(args.Environment, SecretValue{
		Name:  "POSTGRES_HOST",
		Value: db.ConnectionInfo.Host,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "POSTGRES_PORT",
		Value: fmt.Sprintf("%d", db.ConnectionInfo.Port),
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "POSTGRES_DATABASE",
		Value: db.ConnectionInfo.Database,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "POSTGRES_USER",
		Value: db.ConnectionInfo.Username,
	})

	// Add connection string
	connStr := fmt.Sprintf("postgresql://%s:%d/%s",
		db.ConnectionInfo.Host,
		db.ConnectionInfo.Port,
		db.ConnectionInfo.Database)
	args.Environment = append(args.Environment, SecretValue{
		Name:  "DATABASE_URL",
		Value: connStr,
	})

	t.logger.Info("Added PostgreSQL environment variables",
		"database_type", "postgresql",
		"host", db.ConnectionInfo.Host,
		"port", db.ConnectionInfo.Port)
}

// addMySQLEnvVars adds MySQL environment variables
func (t *AtomicGenerateManifestsTool) addMySQLEnvVars(args *GenerateManifestsArgs, db database_detectors.DetectedDatabase) {
	// Add standard MySQL environment variables
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MYSQL_HOST",
		Value: db.ConnectionInfo.Host,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MYSQL_PORT",
		Value: fmt.Sprintf("%d", db.ConnectionInfo.Port),
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MYSQL_DATABASE",
		Value: db.ConnectionInfo.Database,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MYSQL_USER",
		Value: db.ConnectionInfo.Username,
	})

	// Add connection string
	connStr := fmt.Sprintf("mysql://%s:%d/%s",
		db.ConnectionInfo.Host,
		db.ConnectionInfo.Port,
		db.ConnectionInfo.Database)
	args.Environment = append(args.Environment, SecretValue{
		Name:  "DATABASE_URL",
		Value: connStr,
	})

	t.logger.Info("Added MySQL environment variables",
		"database_type", "mysql",
		"host", db.ConnectionInfo.Host,
		"port", db.ConnectionInfo.Port)
}

// addMongoDBEnvVars adds MongoDB environment variables
func (t *AtomicGenerateManifestsTool) addMongoDBEnvVars(args *GenerateManifestsArgs, db database_detectors.DetectedDatabase) {
	// Add standard MongoDB environment variables
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MONGO_HOST",
		Value: db.ConnectionInfo.Host,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MONGO_PORT",
		Value: fmt.Sprintf("%d", db.ConnectionInfo.Port),
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MONGO_DATABASE",
		Value: db.ConnectionInfo.Database,
	})

	// Add connection string
	connStr := fmt.Sprintf("mongodb://%s:%d/%s",
		db.ConnectionInfo.Host,
		db.ConnectionInfo.Port,
		db.ConnectionInfo.Database)
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MONGO_URI",
		Value: connStr,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "MONGODB_URI",
		Value: connStr,
	})

	t.logger.Info("Added MongoDB environment variables",
		"database_type", "mongodb",
		"host", db.ConnectionInfo.Host,
		"port", db.ConnectionInfo.Port)
}

// addRedisEnvVars adds Redis environment variables
func (t *AtomicGenerateManifestsTool) addRedisEnvVars(args *GenerateManifestsArgs, db database_detectors.DetectedDatabase) {
	// Add standard Redis environment variables
	args.Environment = append(args.Environment, SecretValue{
		Name:  "REDIS_HOST",
		Value: db.ConnectionInfo.Host,
	})
	args.Environment = append(args.Environment, SecretValue{
		Name:  "REDIS_PORT",
		Value: fmt.Sprintf("%d", db.ConnectionInfo.Port),
	})

	// Add connection string
	connStr := fmt.Sprintf("redis://%s:%d",
		db.ConnectionInfo.Host,
		db.ConnectionInfo.Port)

	if db.ConnectionInfo.Database != "" && db.ConnectionInfo.Database != "0" {
		args.Environment = append(args.Environment, SecretValue{
			Name:  "REDIS_DB",
			Value: db.ConnectionInfo.Database,
		})
		connStr = fmt.Sprintf("redis://%s:%d/%s",
			db.ConnectionInfo.Host,
			db.ConnectionInfo.Port,
			db.ConnectionInfo.Database)
	}

	args.Environment = append(args.Environment, SecretValue{
		Name:  "REDIS_URL",
		Value: connStr,
	})

	t.logger.Info("Added Redis environment variables",
		"database_type", "redis",
		"host", db.ConnectionInfo.Host,
		"port", db.ConnectionInfo.Port)
}

// convertSessionStateToCore converts session.SessionState to core.SessionState
func convertSessionStateToCore(sessionState *session.SessionState) *core.SessionState {
	if sessionState == nil {
		return nil
	}

	return &core.SessionState{
		SessionID:    sessionState.SessionID,
		UserID:       "default-user", // Since session.SessionState doesn't have UserID, use default
		CreatedAt:    sessionState.CreatedAt,
		UpdatedAt:    sessionState.LastAccessed, // Map LastAccessed to UpdatedAt
		ExpiresAt:    sessionState.ExpiresAt,
		WorkspaceDir: sessionState.WorkspaceDir,

		// Repository state mapping
		RepositoryAnalyzed: sessionState.RepoAnalysis != nil,
		RepoURL:            sessionState.RepoURL,

		// Build state mapping
		ImageRef: sessionState.ImageRef.String(), // Convert ImageReference to string

		// Status mapping
		Status: "active", // Default status
		Stage:  "deploy",

		// Convert metadata
		Metadata: sessionState.RepoAnalysis,
	}
}
