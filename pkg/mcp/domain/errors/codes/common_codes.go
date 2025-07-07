package codes

import "github.com/Azure/container-kit/pkg/mcp/domain/errors"

// Common error codes used across domains
const (
	// General validation errors
	VALIDATION_FAILED           errors.ErrorCode = "VALIDATION_FAILED"
	VALIDATION_SCHEMA_INVALID   errors.ErrorCode = "VALIDATION_SCHEMA_INVALID"
	VALIDATION_REQUIRED_MISSING errors.ErrorCode = "VALIDATION_REQUIRED_MISSING"
	VALIDATION_FORMAT_INVALID   errors.ErrorCode = "VALIDATION_FORMAT_INVALID"
	VALIDATION_RANGE_INVALID    errors.ErrorCode = "VALIDATION_RANGE_INVALID"

	// Network and connectivity errors
	NETWORK_ERROR                 errors.ErrorCode = "NETWORK_ERROR"
	NETWORK_TIMEOUT               errors.ErrorCode = "NETWORK_TIMEOUT"
	NETWORK_CONNECTION_REFUSED    errors.ErrorCode = "NETWORK_CONNECTION_REFUSED"
	NETWORK_DNS_RESOLUTION_FAILED errors.ErrorCode = "NETWORK_DNS_RESOLUTION_FAILED"
	NETWORK_UNREACHABLE           errors.ErrorCode = "NETWORK_UNREACHABLE"

	// Timeout errors
	TIMEOUT                   errors.ErrorCode = "TIMEOUT"
	TIMEOUT_EXCEEDED          errors.ErrorCode = "TIMEOUT_EXCEEDED"
	TIMEOUT_DEADLINE_EXCEEDED errors.ErrorCode = "TIMEOUT_DEADLINE_EXCEEDED"

	// Resource errors
	RESOURCE_NOT_FOUND      errors.ErrorCode = "RESOURCE_NOT_FOUND"
	RESOURCE_ALREADY_EXISTS errors.ErrorCode = "RESOURCE_ALREADY_EXISTS"
	RESOURCE_EXHAUSTED      errors.ErrorCode = "RESOURCE_EXHAUSTED"
	RESOURCE_LOCKED         errors.ErrorCode = "RESOURCE_LOCKED"
	RESOURCE_UNAVAILABLE    errors.ErrorCode = "RESOURCE_UNAVAILABLE"

	// File system errors
	FILE_NOT_FOUND         errors.ErrorCode = "FILE_NOT_FOUND"
	FILE_PERMISSION_DENIED errors.ErrorCode = "FILE_PERMISSION_DENIED"
	FILE_ALREADY_EXISTS    errors.ErrorCode = "FILE_ALREADY_EXISTS"
	FILE_IO_ERROR          errors.ErrorCode = "FILE_IO_ERROR"
	DIRECTORY_NOT_FOUND    errors.ErrorCode = "DIRECTORY_NOT_FOUND"

	// Configuration errors
	CONFIG_INVALID        errors.ErrorCode = "CONFIG_INVALID"
	CONFIG_MISSING        errors.ErrorCode = "CONFIG_MISSING"
	CONFIG_PARSE_ERROR    errors.ErrorCode = "CONFIG_PARSE_ERROR"
	CONFIG_FORMAT_INVALID errors.ErrorCode = "CONFIG_FORMAT_INVALID"

	// System errors
	SYSTEM_ERROR       errors.ErrorCode = "SYSTEM_ERROR"
	SYSTEM_UNAVAILABLE errors.ErrorCode = "SYSTEM_UNAVAILABLE"
	SYSTEM_OVERLOADED  errors.ErrorCode = "SYSTEM_OVERLOADED"
	SYSTEM_MAINTENANCE errors.ErrorCode = "SYSTEM_MAINTENANCE"
)
