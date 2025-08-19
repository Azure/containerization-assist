const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');
const constants = require('./constants');
const { BinaryNotFoundError, ToolExecutionError, TimeoutError, ParseError } = require('./errors');
const debug = require('./debug');

/**
 * Determine the binary path based on platform
 * @returns {string} Absolute path to the MCP binary for current platform
 * @throws {BinaryNotFoundError} If binary doesn't exist for current platform
 */
function getBinaryPath() {
  const platform = os.platform();
  const arch = os.arch();
  
  // Map Node.js platform/arch to our binary directory names
  let platformDir;
  if (platform === 'darwin') {
    platformDir = arch === 'arm64' ? 'darwin-arm64' : 'darwin-x64';
  } else if (platform === 'linux') {
    platformDir = arch === 'arm64' ? 'linux-arm64' : 'linux-x64';
  } else if (platform === 'win32') {
    platformDir = arch === 'arm64' ? 'win32-arm64' : 'win32-x64';
  } else {
    throw new Error(`Unsupported platform: ${platform} ${arch}`);
  }
  
  let binaryName = constants.BINARY_NAME;
  if (platform === constants.PLATFORMS.WIN32) {
    binaryName = constants.BINARY_NAME_WIN;
  }
  
  // Return platform-specific binary path
  const binaryPath = path.join(__dirname, '..', 'bin', platformDir, binaryName);
  
  // Check if binary exists
  if (!fs.existsSync(binaryPath)) {
    debug.error('executor', `Binary not found`, { platform, arch, binaryPath });
    throw new BinaryNotFoundError(platform, arch, binaryPath);
  }
  
  return binaryPath;
}

/**
 * Execute a tool via subprocess
 * @param {string} toolName - Name of the tool to execute (e.g., 'analyze_repository')
 * @param {Object} params - Tool parameters to pass via environment variable
 * @returns {Promise<Object>} Tool execution result parsed from JSON output
 * @throws {ToolExecutionError} If tool exits with non-zero code
 * @throws {ParseError} If tool output cannot be parsed as JSON
 */
async function executeTool(toolName, params) {
  debug.log('executor', `Executing tool: ${toolName}`, { params });
  
  return new Promise((resolve, reject) => {
    const binary = getBinaryPath();
    
    // Add session_id if not provided (for tools that need it)
    if (!params.session_id && needsSessionId(toolName)) {
      params.session_id = generateSessionId();
    }
    
    // Call binary with tool command
    const args = ['tool', toolName];
    const env = {
      ...process.env,
      [constants.ENV_VARS.TOOL_PARAMS]: JSON.stringify(params)
    };
    
    debug.trace('executor', 'Spawning process', { binary, args });
    
    const child = spawn(binary, args, { env });
    
    let stdout = '';
    let stderr = '';
    
    child.stdout.on('data', (data) => {
      stdout += data.toString();
    });
    
    child.stderr.on('data', (data) => {
      stderr += data.toString();
    });
    
    child.on('close', (code) => {
      if (code === constants.EXIT_SUCCESS) {
        try {
          const result = JSON.parse(stdout);
          debug.log('executor', `Tool ${toolName} succeeded`, { result });
          resolve(result);
        } catch (e) {
          debug.error('executor', `Failed to parse tool output`, e);
          reject(new ParseError(toolName, stdout, e));
        }
      } else {
        // Try to parse error from stdout first (tool mode outputs errors as JSON)
        try {
          const errorResult = JSON.parse(stdout);
          if (errorResult.error) {
            debug.error('executor', `Tool returned error`, { tool: toolName, error: errorResult.error });
            reject(new ToolExecutionError(toolName, params, code, stderr, stdout));
          } else {
            reject(new ToolExecutionError(toolName, params, code, stderr, stdout));
          }
        } catch (e) {
          debug.error('executor', `Tool failed with unparseable output`, { code, stderr, stdout });
          reject(new ToolExecutionError(toolName, params, code, stderr, stdout));
        }
      }
    });
    
    child.on('error', (err) => {
      debug.error('executor', `Failed to spawn process for tool ${toolName}`, err);
      reject(new ToolExecutionError(toolName, params, -1, '', err.message));
    });
  });
}

/**
 * Check if a tool needs a session_id parameter
 * @param {string} toolName - Name of the tool to check
 * @returns {boolean} True if the tool requires a session_id, false otherwise
 */
function needsSessionId(toolName) {
  const toolsNeedingSession = [
    'analyze_repository',
    'generate_dockerfile',
    'build_image',
    'scan_image',
    'tag_image',
    'push_image',
    'generate_k8s_manifests',
    'prepare_cluster',
    'deploy_application',
    'verify_deployment',
    'workflow_status'
  ];
  
  return toolsNeedingSession.includes(toolName);
}

/**
 * Generate a unique session ID for workflow tracking
 * Format: session-TIMESTAMP-RANDOM
 * @returns {string} Generated session ID
 * @example
 * generateSessionId() // "session-2024-01-15T10-30-45-abc123def"
 */
function generateSessionId() {
  const timestamp = new Date().toISOString()
    .replace(/[:.]/g, '-')
    .slice(0, -5);
  const random = Math.random().toString(36).substr(2, 9);
  return `session-${timestamp}-${random}`;
}

module.exports = {
  executeTool,
  generateSessionId
};