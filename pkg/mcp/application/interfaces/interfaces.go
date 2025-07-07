package interfaces

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

type Tool = api.Tool
type ToolInput = api.ToolInput
type ToolOutput = api.ToolOutput
type ToolSchema = api.ToolSchema
type ToolExample = api.ToolExample
type ToolMetadata = api.ToolMetadata
type ToolCategory = api.ToolCategory
type ToolStatus = api.ToolStatus

type Registry = api.Registry
type RegistryOption = api.RegistryOption
type RegistryConfig = api.RegistryConfig
type RetryPolicy = api.RetryPolicy
type ObservableRegistry = api.ObservableRegistry
type RegistryMetrics = api.RegistryMetrics
type RegistryEventType = api.RegistryEventType
type RegistryEvent = api.RegistryEvent
type RegistryEventCallback = api.RegistryEventCallback

type Session = api.Session

type Orchestrator = api.Orchestrator

type Workflow = api.Workflow
type WorkflowStep = api.WorkflowStep
type WorkflowTemplate = api.WorkflowTemplate
type WorkflowResult = api.WorkflowResult
type StepResult = api.StepResult
type WorkflowStatus = api.WorkflowStatus

type MCPServer = api.MCPServer
type GomcpManager = api.GomcpManager

type Transport = api.Transport

type ErrorType = api.ErrorType

type Logger = api.Logger

type BuildArgs = api.BuildArgs
type BuildResult = api.BuildResult
type BuildStatus = api.BuildStatus
type BuildInfo = api.BuildInfo
type BuildState = api.BuildState
type BuildStrategy = api.BuildStrategy
type CacheStats = api.CacheStats
type RegistryStats = api.RegistryStats

type ToolFactory = api.ToolFactory
type ToolCreator = api.ToolCreator

var (
	ErrorTypeValidation    = api.ErrorTypeValidation
	ErrorTypeExecution     = api.ErrorTypeExecution
	ErrorTypeNetwork       = api.ErrorTypeNetwork
	ErrorTypeFileSystem    = api.ErrorTypeFileSystem
	ErrorTypeConfiguration = api.ErrorTypeConfiguration
	ErrorTypePermission    = api.ErrorTypePermission
	ErrorTypeTimeout       = api.ErrorTypeTimeout

	CategoryAnalyze       = api.CategoryAnalyze
	CategoryBuild         = api.CategoryBuild
	CategoryDeploy        = api.CategoryDeploy
	CategoryScan          = api.CategoryScan
	CategoryGeneral       = api.CategoryGeneral
	CategoryUtility       = api.CategoryUtility
	CategorySession       = api.CategorySession
	CategoryOrchestration = api.CategoryOrchestration

	StatusActive      = api.StatusActive
	StatusInactive    = api.StatusInactive
	StatusError       = api.StatusError
	StatusMaintenance = api.StatusMaintenance
	StatusDeprecated  = api.StatusDeprecated

	BuildStateQueued    = api.BuildStateQueued
	BuildStateRunning   = api.BuildStateRunning
	BuildStateCompleted = api.BuildStateCompleted
	BuildStateFailed    = api.BuildStateFailed
	BuildStateCancelled = api.BuildStateCancelled

	BuildStrategyDocker   = api.BuildStrategyDocker
	BuildStrategyBuildkit = api.BuildStrategyBuildkit
	BuildStrategyPodman   = api.BuildStrategyPodman
	BuildStrategyKaniko   = api.BuildStrategyKaniko
)

var (
	WithNamespace      = api.WithNamespace
	WithTags           = api.WithTags
	WithPriority       = api.WithPriority
	WithEnabled        = api.WithEnabled
	WithMetadata       = api.WithMetadata
	WithConcurrency    = api.WithConcurrency
	WithTimeout        = api.WithTimeout
	WithRetryPolicy    = api.WithRetryPolicy
	WithCache          = api.WithCache
	WithRateLimit      = api.WithRateLimit
	DefaultRetryPolicy = api.DefaultRetryPolicy
)
