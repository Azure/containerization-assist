package api

const (
	ErrTypeSystem     = "system"
	ErrTypeValidation = "validation"
	ErrTypeResource   = "resource"
	ErrTypeInternal   = "internal"
	ErrTypeTimeout    = "timeout"
	ErrTypeAuth       = "auth"
	ErrTypeNetwork    = "network"
)

const (
	ErrorTypeValidation    = "validation"
	ErrorTypeExecution     = "execution"
	ErrorTypeNetwork       = "network"
	ErrorTypeFileSystem    = "filesystem"
	ErrorTypeConfiguration = "configuration"
	ErrorTypePermission    = "permission"
	ErrorTypeTimeout       = "timeout"
)

const (
	CategoryAnalyze       = "analyze"
	CategoryAnalysis      = "analysis" // Alias for CategoryAnalyze
	CategoryBuild         = "build"
	CategoryDeploy        = "deploy"
	CategoryScan          = "scan"
	CategoryGeneral       = "general"
	CategoryUtility       = "utility"
	CategorySession       = "session"
	CategoryOrchestration = "orchestration"
)

const (
	StatusActive      = "active"
	StatusInactive    = "inactive"
	StatusError       = "error"
	StatusMaintenance = "maintenance"
	StatusDeprecated  = "deprecated"
)

const (
	CapabilityMetrics     = "metrics"
	CapabilityEvents      = "events"
	CapabilityValidation  = "validation"
	CapabilityPersistence = "persistence"
)

const (
	ErrorStrategyFail     = "fail"
	ErrorStrategyContinue = "continue"
	ErrorStrategySkip     = "skip"
)
