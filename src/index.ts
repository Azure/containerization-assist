/**
 * Clean, idiomatic TypeScript API for Container Assist
 * Provides strongly-typed runtime with dependency injection support
 */

// Primary API - AppRuntime with type safety and dependency injection
/** @public */
export { createApp } from './app/index.js';
/** @public */
export type { TransportConfig } from './app/index.js';

// New AppRuntime interface with precise typing
/** @public */
export type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
  CreateAppRuntime,
} from './types/runtime.js';

// Core types
/** @public */
export type { MCPTool, Result, Success, Failure } from './types/index.js';

// Export tool helper utilities
export {
  getToolLogger,
  createToolTimer,
  createStandardizedToolTracker,
} from './lib/tool-helpers.js';

// Export parameter defaulting utilities
export {
  withDefaults,
  buildParams,
  K8S_DEFAULTS,
  CONTAINER_DEFAULTS,
  ACA_DEFAULTS,
  BUILD_DEFAULTS,
  getToolDefaults,
  ParameterBuilder,
} from './lib/param-defaults.js';

// Export result handling utilities
export {
  propagateFailure,
  mapResult,
  chainResults,
  combineResults,
  tryExecute,
  tryExecuteAsync,
  unwrapOrThrow,
  unwrapOr,
  isSuccess,
  isFailure,
  mapError,
  withErrorContext,
} from './lib/result-utils.js';
