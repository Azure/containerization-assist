/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

export * from './categories';
export * from './core';
export * from './tool';
export * from './topics';

/**
 * Tool execution context
 *
 * @remarks
 * ToolContext provides essential utilities for tool execution:
 * - `logger`: Structured logging with Pino
 * - `signal`: Optional AbortSignal for cancellation
 * - `progress`: Optional progress reporting callback
 *
 * @public
 */
export type { ToolContext } from '../mcp/context';

