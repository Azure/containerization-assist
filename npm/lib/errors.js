/**
 * Custom error classes for the Container Kit MCP NPM package
 * Provides better error context and debugging information
 */

/**
 * Base error class for MCP tool-related errors
 */
class MCPToolError extends Error {
  /**
   * @param {string} message - Error message
   * @param {string} tool - Name of the tool that failed
   * @param {Object} params - Parameters passed to the tool
   * @param {Error|null} originalError - Original error if wrapping another error
   */
  constructor(message, tool, params, originalError = null) {
    super(message);
    this.name = 'MCPToolError';
    this.tool = tool;
    this.params = params;
    this.originalError = originalError;
    this.timestamp = new Date().toISOString();
    
    // Capture stack trace
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, this.constructor);
    }
  }

  /**
   * Convert error to JSON for serialization
   * @returns {Object} JSON representation of the error
   */
  toJSON() {
    return {
      name: this.name,
      message: this.message,
      tool: this.tool,
      params: this.params,
      timestamp: this.timestamp,
      stack: this.stack,
      originalError: this.originalError?.message
    };
  }
}

/**
 * Error thrown when a binary is not found for the current platform
 */
class BinaryNotFoundError extends Error {
  /**
   * @param {string} platform - Operating system platform
   * @param {string} arch - CPU architecture
   * @param {string} searchPath - Path where binary was expected
   */
  constructor(platform, arch, searchPath) {
    super(`Binary not found for ${platform}-${arch} at: ${searchPath}`);
    this.name = 'BinaryNotFoundError';
    this.platform = platform;
    this.arch = arch;
    this.searchPath = searchPath;
    
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, this.constructor);
    }
  }

  /**
   * Get helpful suggestions for resolving the error
   * @returns {string[]} Array of suggestion strings
   */
  getSuggestions() {
    return [
      'Run "npm run build:current" to build for your platform',
      'Check if your platform is supported',
      'Report an issue at: https://github.com/Azure/containerization-assist/issues'
    ];
  }
}

/**
 * Error thrown when tool execution fails
 */
class ToolExecutionError extends MCPToolError {
  /**
   * @param {string} tool - Name of the tool that failed
   * @param {Object} params - Parameters passed to the tool
   * @param {number} exitCode - Process exit code
   * @param {string} stderr - Standard error output
   * @param {string} stdout - Standard output
   */
  constructor(tool, params, exitCode, stderr, stdout) {
    super(`Tool ${tool} failed with exit code ${exitCode}`, tool, params);
    this.name = 'ToolExecutionError';
    this.exitCode = exitCode;
    this.stderr = stderr;
    this.stdout = stdout;
  }

  /**
   * Check if the error is retryable based on exit code
   * @returns {boolean} Whether the operation should be retried
   */
  isRetryable() {
    // Timeout and kill signals are potentially retryable
    return this.exitCode === 124 || this.exitCode === 137;
  }
}

/**
 * Error thrown when invalid parameters are provided to a tool
 */
class InvalidParametersError extends MCPToolError {
  /**
   * @param {string} tool - Name of the tool
   * @param {Object} params - Invalid parameters
   * @param {string[]} errors - Validation error messages
   */
  constructor(tool, params, errors) {
    const message = `Invalid parameters for tool ${tool}: ${errors.join(', ')}`;
    super(message, tool, params);
    this.name = 'InvalidParametersError';
    this.validationErrors = errors;
  }
}

/**
 * Error thrown when tool output cannot be parsed
 */
class ParseError extends MCPToolError {
  /**
   * @param {string} tool - Name of the tool
   * @param {string} output - Raw output that couldn't be parsed
   * @param {Error} parseError - Original parse error
   */
  constructor(tool, output, parseError) {
    super(`Failed to parse output from tool ${tool}`, tool, {}, parseError);
    this.name = 'ParseError';
    this.output = output;
  }
}

/**
 * Error thrown when a timeout occurs
 */
class TimeoutError extends MCPToolError {
  /**
   * @param {string} tool - Name of the tool
   * @param {number} timeout - Timeout value in milliseconds
   * @param {Object} params - Parameters passed to the tool
   */
  constructor(tool, timeout, params) {
    super(`Tool ${tool} timed out after ${timeout}ms`, tool, params);
    this.name = 'TimeoutError';
    this.timeout = timeout;
  }
}

module.exports = {
  MCPToolError,
  BinaryNotFoundError,
  ToolExecutionError,
  InvalidParametersError,
  ParseError,
  TimeoutError
};