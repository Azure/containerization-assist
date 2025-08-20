/**
 * Constants for the Container Kit MCP NPM package
 * Centralizes all magic numbers and configuration values
 */

export const DEFAULT_COMMAND_TIMEOUT = 120000;
export const MAX_COMMAND_TIMEOUT = 600000;
export const MCP_PING_TIMEOUT = 2000;
export const STDIO_START_TIMEOUT = 3000;
export const TEST_COMMAND_TIMEOUT = 5000;
export const MAX_OUTPUT_BUFFER = 10 * 1024 * 1024;
export const MAX_LINE_LENGTH = 2000;
export const DEFAULT_MAX_RETRIES = 3;
export const DEFAULT_RETRY_DELAY = 1000;
export const DEFAULT_BACKOFF_FACTOR = 2;
export const MAX_RETRY_DELAY = 30000;
export const EXECUTABLE_PERMISSIONS = 0o755;
export const READABLE_PERMISSIONS = 0o644;
export const EXECUTABLE_BITS = 0o111;
export const EXIT_SUCCESS = 0;
export const EXIT_FAILURE = 1;
export const EXIT_TIMEOUT = 124;
export const EXIT_KILLED = 137;
export const PLATFORMS = {
  DARWIN: 'darwin',
  LINUX: 'linux',
  WIN32: 'win32'
};
export const ARCHITECTURES = {
  X64: 'x64',
  ARM64: 'arm64',
  ARM: 'arm'
};
export const BINARY_NAME = 'container-kit-mcp';
export const BINARY_NAME_WIN = 'container-kit-mcp.exe';
export const ENV_VARS = {
  DEBUG: 'DEBUG_MCP',
  DEBUG_TRACE: 'DEBUG_MCP_TRACE',
  NON_INTERACTIVE: 'NON_INTERACTIVE',
  TOOL_PARAMS: 'TOOL_PARAMS'
};