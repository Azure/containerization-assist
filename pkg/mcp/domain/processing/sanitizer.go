package processing

// Data Processing Package
//
// This package provides comprehensive data processing utilities including sanitization,
// schema validation, and preference management. The original large file (1286 LOC) has
// been split into focused modules following WORKSTREAM ETA standards for code readability
// and maintainability.
//
// Processing functionality is distributed across these files:
//
// - sanitizer_core.go: Core data sanitization utilities and string processing
// - sanitizer_schema.go: JSON schema processing and validation
// - sanitizer_preferences.go: User preference storage and management
// - sanitizer_types.go: Type definitions and conversion utilities
// - sanitizer_utils.go: Utility functions and helper methods
//
// All processing methods follow these patterns:
// - Type-safe data conversion with fallback to interface{} for compatibility
// - Configurable processing modes (strict vs. lenient)
// - Comprehensive error handling with detailed context
// - Performance optimization through caching and lazy evaluation
// - Thread-safe operations using mutex protection
//
// This modular structure ensures each file has a single responsibility and
// remains under the 800 LOC limit while maintaining comprehensive processing coverage.
