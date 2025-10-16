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
export type { MCPTool, Result, Success, Failure, ToolContext } from './types/index.js';

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

// Tool creation helper for external consumers
/** @public */
export { tool } from './types/tool.js';

// All tools with new interface
/** @public */
export {
  ALL_TOOLS,
  analyzeRepoTool,
  buildImageTool,
  deployTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  validateDockerfileTool,
  verifyDeployTool,
} from './tools/index.js';

// Export utilities for external consumers (telemetry integration)
/** @public */
export { extractSchemaShape } from './lib/zod-utils.js';

// Export Zod types for TypeScript consumers
/** @public */
export type { ZodRawShape } from 'zod';
