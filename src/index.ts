/**
 * Clean, idiomatic TypeScript API for Container Assist
 */

// Primary API - simple function that creates the app
export { createApp } from './app/index.js';
export type { AppConfig, TransportConfig } from './app/index.js';

// Tool names and types
export { TOOLS, type ToolName } from './exports/tools.js';

// Core types
export type { Tool, Result, Success, Failure } from './types/index.js';
