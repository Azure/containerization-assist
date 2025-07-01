package constants

// HTTP status codes and error codes used throughout the system
const (
	// HTTP Status Codes
	StatusOK                  = 200
	StatusCreated             = 201
	StatusAccepted            = 202
	StatusNoContent           = 204
	StatusBadRequest          = 400
	StatusUnauthorized        = 401
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusConflict            = 409
	StatusUnprocessableEntity = 422
	StatusInternalServerError = 500
	StatusBadGateway          = 502
	StatusServiceUnavailable  = 503
	StatusGatewayTimeout      = 504
)

// Application Error Codes
const (
	// Generic error codes
	ErrorCodeSuccess      = 0
	ErrorCodeGeneral      = 1
	ErrorCodeInvalidInput = 2
	ErrorCodeNotFound     = 3
	ErrorCodeUnauthorized = 4
	ErrorCodeForbidden    = 5

	// Session error codes (100-199)
	ErrorCodeSessionNotFound = 100
	ErrorCodeSessionExpired  = 101
	ErrorCodeSessionInvalid  = 102
	ErrorCodeSessionLimit    = 103

	// Tool execution error codes (200-299)
	ErrorCodeToolNotFound   = 200
	ErrorCodeToolFailed     = 201
	ErrorCodeToolTimeout    = 202
	ErrorCodeToolValidation = 203
	ErrorCodeToolDependency = 204

	// Docker operation error codes (300-399)
	ErrorCodeDockerBuildFailed  = 300
	ErrorCodeDockerPushFailed   = 301
	ErrorCodeDockerPullFailed   = 302
	ErrorCodeDockerInvalidImage = 303
	ErrorCodeDockerTimeout      = 304

	// Kubernetes operation error codes (400-499)
	ErrorCodeKubernetesDeployFailed    = 400
	ErrorCodeKubernetesInvalidManifest = 401
	ErrorCodeKubernetesTimeout         = 402
	ErrorCodeKubernetesNotFound        = 403
	ErrorCodeKubernetesPermission      = 404

	// Security scan error codes (500-599)
	ErrorCodeScanFailed        = 500
	ErrorCodeScanTimeout       = 501
	ErrorCodeScanNotSupported  = 502
	ErrorCodeScanInvalidTarget = 503

	// File system error codes (600-699)
	ErrorCodeFileNotFound   = 600
	ErrorCodeFilePermission = 601
	ErrorCodeFileCorrupted  = 602
	ErrorCodeDiskSpace      = 603

	// Network error codes (700-799)
	ErrorCodeNetworkTimeout     = 700
	ErrorCodeNetworkUnreachable = 701
	ErrorCodeNetworkDNS         = 702
	ErrorCodeNetworkTLS         = 703
)

// Exit Codes for CLI operations
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitTimeout = 2
	ExitSignal  = 3
)

// Priority levels
const (
	PriorityLow      = 1
	PriorityMedium   = 2
	PriorityHigh     = 3
	PriorityCritical = 4
)

// Severity levels for security findings
const (
	SeverityUnknown  = 0
	SeverityLow      = 1
	SeverityMedium   = 2
	SeverityHigh     = 3
	SeverityCritical = 4
)
