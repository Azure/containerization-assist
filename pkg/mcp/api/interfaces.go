// Package api provides backward compatibility aliases for interfaces that have been moved to application/api.
// This file serves as a compatibility shim during the architecture migration.
// DEPRECATED: Use pkg/mcp/application/api directly.
package api

import "github.com/Azure/container-kit/pkg/mcp/application/api"

// Backward compatibility constants - only the ones actually used in the codebase
const (
	// Categories - from constants.go
	CategoryAnalyze       = api.CategoryAnalyze
	CategoryBuild         = api.CategoryBuild
	CategoryDeploy        = api.CategoryDeploy
	CategoryScan          = api.CategoryScan
	CategoryGeneral       = api.CategoryGeneral
	CategoryUtility       = api.CategoryUtility
	CategorySession       = api.CategorySession
	CategoryOrchestration = api.CategoryOrchestration

	// Status - from constants.go
	StatusActive      = api.StatusActive
	StatusInactive    = api.StatusInactive
	StatusError       = api.StatusError
	StatusMaintenance = api.StatusMaintenance
	StatusDeprecated  = api.StatusDeprecated

	// Error types - from constants.go
	ErrTypeSystem     = api.ErrTypeSystem
	ErrTypeValidation = api.ErrTypeValidation
	ErrTypeResource   = api.ErrTypeResource
	ErrTypeInternal   = api.ErrTypeInternal
	ErrTypeTimeout    = api.ErrTypeTimeout
	ErrTypeAuth       = api.ErrTypeAuth
	ErrTypeNetwork    = api.ErrTypeNetwork

	ErrorTypeValidation    = api.ErrorTypeValidation
	ErrorTypeExecution     = api.ErrorTypeExecution
	ErrorTypeNetwork       = api.ErrorTypeNetwork
	ErrorTypeFileSystem    = api.ErrorTypeFileSystem
	ErrorTypeConfiguration = api.ErrorTypeConfiguration
	ErrorTypePermission    = api.ErrorTypePermission
	ErrorTypeTimeout       = api.ErrorTypeTimeout

	// Capabilities - from constants.go
	CapabilityMetrics     = api.CapabilityMetrics
	CapabilityEvents      = api.CapabilityEvents
	CapabilityValidation  = api.CapabilityValidation
	CapabilityPersistence = api.CapabilityPersistence

	// Error strategies - from constants.go  
	ErrorStrategyFail     = api.ErrorStrategyFail
	ErrorStrategyContinue = api.ErrorStrategyContinue
	ErrorStrategySkip     = api.ErrorStrategySkip
)

// Backward compatibility type aliases for core types
type Tool = api.Tool
type ToolInput = api.ToolInput
type ToolOutput = api.ToolOutput
type ToolSchema = api.ToolSchema
type ToolMetadata = api.ToolMetadata
type ToolCategory = api.ToolCategory
type ToolStatus = api.ToolStatus
type RetryPolicy = api.RetryPolicy

// Backward compatibility for other types defined in the api package
type Session = api.Session
type Registry = api.Registry
type Orchestrator = api.Orchestrator
type MCPServer = api.MCPServer

// Re-export commonly used error variable
var ErrorInvalidInput = api.ErrorInvalidInput