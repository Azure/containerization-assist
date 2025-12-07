/**
 * Core Module Exports
 *
 * This module exports the foundational types and utilities that have
 * no MCP dependencies. All tool implementations should import
 * ToolContext from this module (via '@/core/context').
 */

// Context types and factory
export type { ToolContext, ProgressReporter, ContextOptions } from './context.js';
export { createToolContext } from './context.js';
