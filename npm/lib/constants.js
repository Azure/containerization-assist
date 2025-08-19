/**
 * Constants for the Container Kit MCP NPM package
 * Centralizes all magic numbers and configuration values
 */

module.exports = {
  // Timeouts (in milliseconds)
  DEFAULT_COMMAND_TIMEOUT: 120000,  // 2 minutes
  MAX_COMMAND_TIMEOUT: 600000,      // 10 minutes
  MCP_PING_TIMEOUT: 2000,           // 2 seconds for ping
  STDIO_START_TIMEOUT: 3000,        // 3 seconds for stdio startup
  TEST_COMMAND_TIMEOUT: 5000,       // 5 seconds for test commands
  
  // Buffer sizes
  MAX_OUTPUT_BUFFER: 10 * 1024 * 1024,  // 10MB max buffer
  MAX_LINE_LENGTH: 2000,                 // Max characters per line
  
  // Retry configuration
  DEFAULT_MAX_RETRIES: 3,          // Maximum retry attempts
  DEFAULT_RETRY_DELAY: 1000,       // Initial retry delay (1 second)
  DEFAULT_BACKOFF_FACTOR: 2,       // Exponential backoff multiplier
  MAX_RETRY_DELAY: 30000,          // Maximum retry delay (30 seconds)
  
  // File permissions
  EXECUTABLE_PERMISSIONS: 0o755,   // rwxr-xr-x
  READABLE_PERMISSIONS: 0o644,     // rw-r--r--
  EXECUTABLE_BITS: 0o111,          // Check for any execute permission
  
  // Process exit codes
  EXIT_SUCCESS: 0,                 // Successful execution
  EXIT_FAILURE: 1,                 // General failure
  EXIT_TIMEOUT: 124,               // Command timeout
  EXIT_KILLED: 137,                // Process was killed (SIGKILL)
  
  // Platform identifiers
  PLATFORMS: {
    DARWIN: 'darwin',
    LINUX: 'linux',
    WIN32: 'win32'
  },
  
  // Architecture identifiers
  ARCHITECTURES: {
    X64: 'x64',
    ARM64: 'arm64',
    ARM: 'arm'
  },
  
  // Binary names
  BINARY_NAME: 'container-kit-mcp',
  BINARY_NAME_WIN: 'container-kit-mcp.exe',
  
  // Environment variables
  ENV_VARS: {
    DEBUG: 'DEBUG_MCP',
    DEBUG_TRACE: 'DEBUG_MCP_TRACE',
    NON_INTERACTIVE: 'NON_INTERACTIVE',
    TOOL_PARAMS: 'TOOL_PARAMS'
  }
};