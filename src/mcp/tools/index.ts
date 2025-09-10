/**
 * MCP Tools Helper Index
 *
 * This module provides a clean public API for all tool standardization helpers.
 * These utilities implement the "Golden Path" pattern for consistent tool behavior.
 *
 * @example Basic tool implementation using helpers
 * ```typescript
 * import { getSession, updateSession, aiGenerate, formatStandardResponse } from '@mcp/tools';
 *
 * export const myTool = async (params, context) => {
 *   // 1. Simple session resolution
 *   const sess = await getSession(params.sessionId, context);
 *   if (!sess.ok) return Failure(sess.error);
 *
 *   // 2. AI generation with registry
 *   const result = await aiGenerate(context.logger, context, {
 *     promptName: 'my-prompt',
 *     promptArgs: params,
 *     expectation: 'json'
 *   });
 *
 *   // 3. Simple session update
 *   if (result.ok) {
 *     await updateSession(sess.value.id, {
 *       completed_steps: [...(sess.value.state.completed_steps || []), 'my-step']
 *     }, context);
 *   }
 *
 *   // 4. Return standardized response
 *   return formatStandardResponse(result, sess.value.id);
 * };
 * ```
 */

// =============================================================================
// SESSION MANAGEMENT HELPERS
// =============================================================================

/**
 * Session management utilities for consistent session handling across tools.
 *
 * @example Getting a session with optional sessionId
 * ```typescript
 * const sess = await getSession(params.sessionId, context);
 * if (!sess.ok) return Failure(sess.error);
 * const { id: sessionId, state } = sess.value;
 * ```
 */
export * from './session-helpers';

// =============================================================================
// AI INTEGRATION HELPERS
// =============================================================================

/**
 * AI invocation helpers with centralized prompt registry and fallback logic.
 *
 * @example AI generation with fallback
 * ```typescript
 * const result = await aiGenerate(logger, context, {
 *   promptName: 'dockerfile-generation',
 *   promptArgs: { framework: 'node', requirements: [] },
 *   expectation: 'dockerfile',
 *   fallbackBehavior: 'retry',
 *   maxRetries: 3
 * });
 * ```
 */
export * from './ai-helpers';

// =============================================================================
// TOOL EXECUTION WRAPPER
// =============================================================================

/**
 * NOTE: Tool wrapper has been replaced with direct implementation pattern.
 * Use selective progress reporting with createStandardProgress() instead.
 *
 * @example Direct tool implementation (current pattern)
 * ```typescript
 * async function myToolImpl(params: MyParams, context: ToolContext): Promise<Result<MyResult>> {
 *   const progress = context.progress ? createStandardProgress(context.progress) : undefined;
 *   if (progress) await progress('VALIDATING');
 *   // Implementation logic
 *   if (progress) await progress('COMPLETE');
 *   return Success(result);
 * }
 * export const myTool = myToolImpl;
 * ```
 */

// =============================================================================
// RESPONSE FORMATTING
// =============================================================================

/**
 * Response standardization utilities for consistent tool return shapes.
 *
 * @example Formatting a standard response
 * ```typescript
 * // Returns: { ok: true, sessionId: '123', data: result, message: 'Success' }
 * return formatStandardResponse(result, sessionId);
 *
 * // Tool-specific formatters also available:
 * return responseFormatters.dockerfile(content, sessionId);
 * return responseFormatters.manifest(yaml, sessionId);
 * ```
 */
export { formatStandardResponse, responseFormatters } from './response-formatter';

// =============================================================================
// PROGRESS REPORTING
// =============================================================================

/**
 * Standardized progress reporting with 4-stage pattern.
 *
 * @example Using standard progress stages
 * ```typescript
 * const progress = createStandardProgress(context.progressReporter);
 *
 * await progress('VALIDATING');   // 10%
 * await progress('EXECUTING');    // 50%
 * await progress('FINALIZING');   // 90%
 * await progress('COMPLETE');     // 100%
 * ```
 */
export { createStandardProgress, STANDARD_STAGES } from '../utils/progress-helper';

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/**
 * Re-export commonly used types for convenience
 */
export type { AIGenerateOptions, AIResponse } from './ai-helpers';

export type { DockerfileResponse, ManifestResponse, AnalysisResponse } from './response-formatter';

// =============================================================================
// UTILITY EXPORTS
// =============================================================================

/**
 * SDK Tool Registry for tool management
 */
export type { SDKToolRegistry } from './registry';

/**
 * Validation utilities
 */
export type { ValidationContext, ValidationResult } from './validator';
