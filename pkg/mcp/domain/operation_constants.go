package domain

const (
	// Operation error types used by deploy package
	OPERATION_MANIFEST_ERROR = "manifest_error"
	OPERATION_FILE_ERROR     = "file_error"
	OPERATION_NETWORK_ERROR  = "network_error"
	OPERATION_AUTH_ERROR     = "auth_error"
	OPERATION_TIMEOUT_ERROR  = "timeout_error"
)

// Operation status types
const (
	OPERATION_STATUS_PENDING   = "pending"
	OPERATION_STATUS_RUNNING   = "running"
	OPERATION_STATUS_COMPLETED = "completed"
	OPERATION_STATUS_FAILED    = "failed"
	OPERATION_STATUS_CANCELLED = "cancelled"
)

// Operation types
const (
	OPERATION_TYPE_DEPLOY  = "deploy"
	OPERATION_TYPE_BUILD   = "build"
	OPERATION_TYPE_ANALYZE = "analyze"
	OPERATION_TYPE_SCAN    = "scan"
)
