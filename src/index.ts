/**
 * Clean, idiomatic TypeScript API for Container Assist
 * Provides strongly-typed runtime with dependency injection support
 */

// Primary API - AppRuntime with type safety and dependency injection
/** @public */
export { createApp } from './app/index.js';
/** @public */
export type { TransportConfig } from './app/index.js';

/** @public */
export type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
  CreateAppRuntime,
} from './types/runtime.js';

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
  K8S_DEFAULTS,
  CONTAINER_DEFAULTS,
  ACA_DEFAULTS,
  BUILD_DEFAULTS,
  getToolDefaults,
} from './lib/param-defaults.js';

/** @public */
export { tool } from './types/tool.js';

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
  verifyDeployTool,
} from './tools/index.js';

// Export utilities for external consumers (telemetry integration)
/** @public */
export { extractSchemaShape } from './lib/zod-utils.js';

// Export Zod types for TypeScript consumers
/** @public */
export type { ZodRawShape } from 'zod';
