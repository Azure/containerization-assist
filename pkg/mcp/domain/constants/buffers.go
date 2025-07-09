package constants

// Buffer size constants for various I/O operations
const (
	// SmallBufferSize is used for small data operations (1KB)
	SmallBufferSize = 1024 // 1KB

	// MediumBufferSize is used for medium data operations (4KB)
	MediumBufferSize = 4096 // 4KB

	// LargeBufferSize is used for large data operations (64KB)
	LargeBufferSize = 65536 // 64KB

	// DefaultBufferSize is the default buffer size for general operations
	DefaultBufferSize = MediumBufferSize

	// FileReadBufferSize is optimized for file reading operations (8KB)
	FileReadBufferSize = 8192 // 8KB

	// FileWriteBufferSize is optimized for file writing operations (8KB)
	FileWriteBufferSize = 8192 // 8KB

	// NetworkBufferSize is optimized for network operations (32KB)
	NetworkBufferSize = 32768 // 32KB

	// StreamBufferSize is optimized for streaming operations (16KB)
	StreamBufferSize = 16384 // 16KB

	// LogBufferSize is used for log output buffering (2KB)
	LogBufferSize = 2048 // 2KB

	// JSONBufferSize is used for JSON operations (8KB)
	JSONBufferSize = 8192 // 8KB

	// CompressionBufferSize is used for compression operations (32KB)
	CompressionBufferSize = 32768 // 32KB

	// DatabaseBufferSize is used for database operations (4KB)
	DatabaseBufferSize = 4096 // 4KB

	// HTTPBufferSize is used for HTTP request/response buffering (16KB)
	HTTPBufferSize = 16384 // 16KB

	// DockerBuildBufferSize is used for Docker build log streaming (32KB)
	DockerBuildBufferSize = 32768 // 32KB

	// ScanOutputBufferSize is used for security scan output (16KB)
	ScanOutputBufferSize = 16384 // 16KB
)

// Common buffer size aliases for backward compatibility
const (
	// KB represents kilobyte
	KB = 1024

	// MB represents megabyte
	MB = 1024 * KB

	// GB represents gigabyte
	GB = 1024 * MB
)
