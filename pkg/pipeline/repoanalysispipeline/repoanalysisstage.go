package repoanalysispipeline

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// RepoAnalysisStage implements the pipeline.PipelineStage interface for repository analysis
var _ pipeline.PipelineStage = &RepoAnalysisStage{}

type RepoAnalysisStage struct {
	AIClient *ai.AzOpenAIClient
	Parser   pipeline.Parser
}

// Initialize prepares the pipeline state with initial repo analysis-related values
func (p *RepoAnalysisStage) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// No specific initialization needed for repo analysis
	return nil
}

// Generate creates the repository analysis if needed
func (p *RepoAnalysisStage) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	// Nothing to generate for repo analysis
	return nil
}

// GetErrors returns repo analysis-related errors from the state
func (p *RepoAnalysisStage) GetErrors(state *pipeline.PipelineState) string {
	// Check if there's any analysis error stored in metadata
	if err, ok := state.Metadata[pipeline.RepoAnalysisErrorKey].(string); ok && err != "" {
		return err
	}
	return ""
}

func (p *RepoAnalysisStage) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	// Nothing to write for repo analysis
	return nil
}

// Deploy handles any deployment steps needed after repo analysis
func (p *RepoAnalysisStage) Deploy(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}) error {
	// Print the repository analysis for visibility during deployment
	if analysis, ok := state.Metadata[pipeline.RepoAnalysisResultKey].(string); ok && analysis != "" {
		logger.Infof("\n📋 Repository Analysis Results:")
		logger.Info(analysis)

		// Also print the file operation summary if available
		if calls, ok := state.Metadata[pipeline.RepoAnalysisCallsKey].(string); ok && calls != "" {
			logger.Infof("\n🔎 Files Accessed During Analysis:")
			logger.Info(calls)
		}
	}

	return nil
}

// Run executes the repository analysis pipeline
func (p *RepoAnalysisStage) Run(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}, options pipeline.RunnerOptions) error {

	targetDir := options.TargetDirectory
	logger.Infof("Starting repository analysis for: %s\n", targetDir)
	logger.Infof("\n🔍 Analyzing repository at: %s\n", targetDir)
	logger.Info("\n⚙️ LLM File Operations (real-time):")

	// Create a slice to store operation logs and implement real-time logging
	var operationLogs []string

	// Set up callback for file operations
	ai.LoggingCallback = func(message string) {
		logger.Info(message)
		operationLogs = append(operationLogs, message)
	}

	// Analyze the repository content
	repoAnalysis, err := AnalyzeRepositoryWithFileAccess(ctx, p.AIClient, state, targetDir)

	// Clear callback
	ai.LoggingCallback = nil

	if err != nil {
		state.Metadata[pipeline.RepoAnalysisErrorKey] = fmt.Sprintf("repository analysis failed: %v", err)
		return fmt.Errorf("repository analysis failed: %v", err)
	}

	// Store the analysis results in the pipeline state metadata
	state.Metadata[pipeline.RepoAnalysisResultKey] = repoAnalysis

	// Print out the LLM function call summary
	logger.Info("\n📊 LLM Analysis Summary:")
	logger.Infof("- Total file operations: %d\n", len(operationLogs))

	// Format the file operation logs for better readability
	fileOperations := FormatFileOperationLogs(operationLogs)

	// Store the function calls in the metadata
	state.Metadata[pipeline.RepoAnalysisCallsKey] = fileOperations

	// Print file operation summary
	logger.Info("\n🔎 Summary of Files Accessed During Analysis:")
	logger.Info(fileOperations)

	logger.Info("\n📋 Repository Analysis Results:")
	logger.Info(repoAnalysis)

	logger.Info("✅ Repository analysis completed successfully")

	return nil
}

// AnalyzeRepositoryWithFileAccess uses AI with file access tools to analyze the repository for containerization requirements
func AnalyzeRepositoryWithFileAccess(ctx context.Context, client *ai.AzOpenAIClient, state *pipeline.PipelineState, targetDir string) (string, error) {
	// Create prompt for LLM to analyze repository with file access tools
	promptText := fmt.Sprintf(`
You are an expert in containerizing applications. Your task is to analyze this repository and identify all the information needed to properly containerize it.

You have access to the following tools to help with your analysis:
1. read_file: Use this to read the contents of a specific file in the repository
2. list_directory: Use this to list files and directories in a specific directory
3. file_exists: Use this to check if a specific file exists in the repository

IMPORTANT: You MUST actively use these tools to properly analyze the repository. Do not rely solely on the repository structure provided. You need to:
- Use list_directory to explore directories that might contain important configuration files
- Use read_file to examine the content of key files like package.json, requirements.txt, Dockerfile, etc.
- Use file_exists to check if certain important files exist in different locations

High-level repository structure:
%s

Your goal is to examine the repository thoroughly and provide a detailed report on everything needed to containerize this application effectively. Follow these steps:

1. First, explore the repository structure to understand the project organization
   - Start by using list_directory on the root directory and key subdirectories
   - Look for project organization indicators

2. Look for key configuration files to identify the project type and dependencies
   - For Node.js: check for package.json, package-lock.json, yarn.lock
   - For Python: check for requirements.txt, setup.py, pyproject.toml, Pipfile
   - For Java: check for pom.xml, build.gradle, settings.gradle
   - For Go: check for go.mod, go.sum
   - Use read_file to examine the content of these files when you find them

3. Examine build configuration files to understand the build process
   - Look for Makefiles, build scripts, CI/CD configs
   - Use read_file to understand the build steps

4. Check for environment configuration files
   - Look for .env files, config.json, application.properties, etc.
   - Use read_file to understand required environment variables
   - Specifically look for database connection strings and credentials

5. Look for main application entry points
   - Check for files like main.go, app.py, index.js, etc.
   - Use read_file to confirm entry points

6. Identify specific runtime requirements
   - Look for clues in configuration files about required services
   - Check for any documentation on dependencies

7. IMPORTANT: Note that if you find a Dockerfile at the root of the repository, it might be a sample and does not necessarily directly define the environment. You should still analyze the codebase to determine the actual requirements.

8. IMPORTANT: Look thoroughly for database connection configurations and patterns
   - Look for database connection strings or configuration in .env files, config files, or application code
   - Check for database client libraries in dependency files (like mongoose, sequelize, sqlx, gorm, sqlalchemy, etc.)
   - Search for code patterns that indicate database connections:
     - For SQL databases: Look for imports/requires of database drivers (mysql, postgres, sqlite, sqlserver)
     - For NoSQL: Look for MongoDB, Redis, DynamoDB, Cassandra client imports
     - For Azure: Look for CosmosDB, Azure SQL, Azure Database for MySQL/PostgreSQL connections
   - Check for connection pooling configurations
   - Identify database authentication methods used (username/password, managed identity, etc.)

9. CRITICAL: After completing your initial analysis, continue searching deeper for database-related code and configurations:
   - Scan source code files for database-related imports, connection strings, and query patterns
   - Look beyond obvious configuration files into application logic files
   - Search for environment variable references that might be used for database connections
   - Check for database migration scripts or schema definitions
   - Look for ORM configuration files or model definitions

Based on your analysis, provide a structured report on:
1. Project type and frameworks identified
2. Language and runtime requirements (specific versions if found)
3. Build process and tools required
4. Dependencies that need to be installed
5. Required environment variables
6. Entry point or startup commands
7. Ports that should be exposed
8. Database connections and requirements:
   - Database type(s) used (MySQL, PostgreSQL, MongoDB, Redis, etc.)
   - Connection methods (direct, ORM, client library)
   - Authentication methods
   - Connection pooling configurations if found
   - Data persistence requirements
9. Any other relevant information for containerization

This information will be used to create an accurate Dockerfile and Kubernetes manifests.
`, state.RepoFileTree)

	// Get LLM analysis using the file access tools
	content, _, err := client.GetChatCompletionWithFileTools(ctx, promptText, targetDir)
	if err != nil {
		return "", err
	}

	return content, nil
}
