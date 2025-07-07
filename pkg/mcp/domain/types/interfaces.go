// Package types - Legacy compatibility
// This package is kept for backward compatibility.
// All interface definitions have been moved to the canonical interfaces package.
// This file now only contains types that are specific to the types package and not duplicated elsewhere.
package types

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// Legacy type aliases for backward compatibility
type Tool = api.Tool
type ToolInput = api.ToolInput
type ToolOutput = api.ToolOutput
type ToolSchema = api.ToolSchema

// ConfigProvider provides configuration for the MCP system
// This interface is specific to the types package and not duplicated elsewhere
type ConfigProvider interface {
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	IsSet(key string) bool
}

// Additional types that are specific to this package can be added here.
// All other interface types are now aliased in interfaces_compat.go
