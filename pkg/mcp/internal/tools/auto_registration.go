//go:generate go run ../../../../tools/register-tools/main.go -dry-run

package tools

// This file provides auto-registration capabilities for MCP tools.
// The //go:generate directive above runs tool discovery to identify
// which tools are ready for auto-registration vs need interface migration.
//
// Current approach uses AutoRegistrationAdapter to bridge between
// the current tool implementations and the unified mcptypes.Tool interface.
//
// To regenerate discovery: go generate ./pkg/mcp/internal/tools