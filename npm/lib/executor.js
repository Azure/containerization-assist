import { spawn } from 'child_process';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { platform as _platform, arch as _arch } from 'os';
import { existsSync } from 'fs';
import {
  BINARY_NAME,
  PLATFORMS,
  BINARY_NAME_WIN,
  ENV_VARS,
  EXIT_SUCCESS,
  DEFAULT_COMMAND_TIMEOUT,
  MAX_COMMAND_TIMEOUT,
  MAX_OUTPUT_BUFFER
} from './constants.js';
import { BinaryNotFoundError, ToolExecutionError, TimeoutError, ParseError } from './errors.js';
import { error as _error, log, trace } from './debug.js';

// ES module equivalent of __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Determine the binary path based on platform
 * @returns {string} Absolute path to the MCP binary for current platform
 * @throws {BinaryNotFoundError} If binary doesn't exist for current platform
 */
function getBinaryPath() {
  const platform = _platform();
  const arch = _arch();
  
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
  
  let binaryName = BINARY_NAME;
  if (platform === PLATFORMS.WIN32) {
    binaryName = BINARY_NAME_WIN;
  }
  
  // Return platform-specific binary path
  const binaryPath = join(__dirname, '..', 'bin', platformDir, binaryName);
  
  // Check if binary exists
  if (!existsSync(binaryPath)) {
    _error('executor', `Binary not found`, { platform, arch, binaryPath });
    throw new BinaryNotFoundError(platform, arch, binaryPath);
  }
  
  return binaryPath;
}

/**
 * Safely parse JSON that may contain code fences
 * @param {string} text - Raw text that may contain JSON with markdown fences
 * @returns {Object} Parsed JSON object
 * @throws {SyntaxError} If JSON is invalid after fence removal
 */
function safeParseJSON(text) {
  const trimmed = String(text).trim();
  const withoutFences = trimmed.replace(/^```json\s*/i, '').replace(/```$/i, '').trim();
  return JSON.parse(withoutFences);
}

/**
 * Execute a tool via subprocess with timeout and output guards
 * @param {string} toolName - Name of the tool to execute (e.g., 'analyze_repository')
 * @param {Object} params - Tool parameters to pass via environment variable
 * @param {{timeout?: number}} [opts] - Optional execution options
 * @returns {Promise<Object>} Tool execution result parsed from JSON output
 * @throws {ToolExecutionError} If tool exits with non-zero code
 * @throws {ParseError} If tool output cannot be parsed as JSON
 * @throws {TimeoutError} If execution exceeds timeout
 */
async function executeTool(toolName, params = {}, opts = {}) {
  log('executor', `Executing tool: ${toolName}`, { params });

  return new Promise((resolve, reject) => {
    const binary = getBinaryPath();

    // Add session_id if not provided (for tools that need it)
    if (!params.session_id && needsSessionId(toolName)) {
      params.session_id = generateSessionId();
    }

    // Call binary with tool command
    const args = ['tool', toolName];
    const env = {
      ...process.env, // ✅ Fixed: was previously { .process.env, ... }
      [ENV_VARS.TOOL_PARAMS]: JSON.stringify(params)
    };

    // Configure timeout
    const timeoutMsRaw = Number.isFinite(opts.timeout) ? opts.timeout : DEFAULT_COMMAND_TIMEOUT;
    const timeoutMs = Math.max(0, Math.min(timeoutMsRaw, MAX_COMMAND_TIMEOUT));

    trace('executor', 'Spawning process', { binary, args, timeoutMs });

    const child = spawn(binary, args, {
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
      shell: false,
      windowsHide: true
    });

    let stdout = '';
    let stderr = '';
    let killedByUs = false;

    const killWith = (err) => {
      if (!killedByUs) {
        killedByUs = true;
        try { child.kill(); } catch (_) {}
      }
      reject(err);
    };

    // Set up timeout
    const timer = setTimeout(() => {
      killWith(new TimeoutError(toolName, timeoutMs));
    }, timeoutMs);

    // Handle stdout with output buffer protection
    child.stdout.on('data', (data) => {
      stdout += data.toString();
      if (stdout.length > MAX_OUTPUT_BUFFER) {
        killWith(new ToolExecutionError(toolName, params, -1, stderr, 'stdout exceeded MAX_OUTPUT_BUFFER'));
      }
    });

    // Handle stderr with output buffer protection
    child.stderr.on('data', (data) => {
      stderr += data.toString();
      if (stderr.length > MAX_OUTPUT_BUFFER) {
        killWith(new ToolExecutionError(toolName, params, -1, stderr, 'stderr exceeded MAX_OUTPUT_BUFFER'));
      }
    });

    child.on('error', (err) => {
      clearTimeout(timer);
      _error('executor', `Failed to spawn process for tool ${toolName}`, err);
      reject(new ToolExecutionError(toolName, params, -1, '', err.message));
    });

    child.on('close', (code) => {
      clearTimeout(timer);
      if (killedByUs) return; // already rejected

      if (code === EXIT_SUCCESS) {
        try {
          const result = safeParseJSON(stdout);
          log('executor', `Tool ${toolName} succeeded`);
          resolve(result);
        } catch (e) {
          _error('executor', `Failed to parse tool output`, e);
          reject(new ParseError(toolName, stdout, e));
        }
      } else {
        // Try to parse error from stdout first (tool mode outputs errors as JSON)
        try {
          const maybeJSON = safeParseJSON(stdout);
          if (maybeJSON && maybeJSON.error) {
            _error('executor', `Tool returned error`, { tool: toolName, error: maybeJSON.error });
          }
        } catch (_) { /* ignore parse errors for error reporting */ }
        reject(new ToolExecutionError(toolName, params, code ?? -1, stderr, stdout));
      }
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

export { executeTool, generateSessionId };