/**
 * Simple debug logging for the Container Kit MCP NPM package
 * Activated via DEBUG_MCP environment variable
 */

import { ENV_VARS } from './constants.js';

/**
 * Simple debug logger
 */
class DebugLogger {
  constructor() {
    this.enabled = process.env[ENV_VARS.DEBUG] === 'true';
    this.verbose = process.env[ENV_VARS.DEBUG_TRACE] === 'true';
  }

  /**
   * Log a debug message
   * @param {string} category - Log category (e.g., 'executor', 'tool')
   * @param {string} message - Log message
   * @param {Object} [data] - Optional additional data
   */
  log(category, message, data = null) {
    if (!this.enabled) return;
    
    const timestamp = new Date().toISOString();
    const logMessage = `[${timestamp}] [${category}] ${message}`;
    
    if (data) {
      console.error(logMessage, JSON.stringify(data, null, 2));
    } else {
      console.error(logMessage);
    }
  }

  /**
   * Log verbose/trace information
   * @param {string} category - Log category
   * @param {string} message - Log message
   * @param {Object} [data] - Optional additional data
   */
  trace(category, message, data = null) {
    if (!this.verbose) return;
    this.log(`TRACE:${category}`, message, data);
  }

  /**
   * Log an error
   * @param {string} category - Log category
   * @param {string} message - Log message
   * @param {Error} error - Error object
   */
  error(category, message, error) {
    if (!this.enabled) return;
    
    const timestamp = new Date().toISOString();
    console.error(`[${timestamp}] [ERROR:${category}] ${message}`);
    console.error('  Error:', error.message);
    if (this.verbose && error.stack) {
      console.error('  Stack:', error.stack);
    }
  }
}

const logger = new DebugLogger();
export const log = logger.log.bind(logger);
export const trace = logger.trace.bind(logger);
export const error = logger.error.bind(logger);
export default logger;