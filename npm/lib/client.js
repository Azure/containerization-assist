const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// Singleton connection
let connection = null;

class MCPConnection {
  constructor() {
    this.server = null;
    this.messageId = 0;
    this.pendingRequests = new Map();
    this.buffer = '';
    this.isClosing = false;
  }

  getBinaryPath() {
    const platform = process.platform;
    const binDir = path.join(__dirname, '..', 'bin');
    
    // First, try the platform-agnostic symlink/copy
    const defaultBinary = path.join(binDir, platform === 'win32' ? 'mcp-server.exe' : 'mcp-server');
    if (fs.existsSync(defaultBinary)) {
      return defaultBinary;
    }
    
    // Fallback to platform-specific binary
    const archMap = { 'x64': 'x64', 'arm64': 'arm64' };
    const platformMap = { 'darwin': 'darwin', 'linux': 'linux', 'win32': 'win' };
    
    const mappedPlatform = platformMap[platform];
    const mappedArch = archMap[process.arch] || process.arch;
    
    let binaryName = `mcp-server-${mappedPlatform}-${mappedArch}`;
    if (platform === 'win32') {
      binaryName += '.exe';
    }
    
    const specificBinary = path.join(binDir, binaryName);
    if (fs.existsSync(specificBinary)) {
      return specificBinary;
    }
    
    return null;
  }

  async connect() {
    if (this.server) return;
    
    const binaryPath = this.getBinaryPath();
    if (!binaryPath) {
      throw new Error('MCP server binary not found. Please reinstall the package.');
    }
    
    this.server = spawn(binaryPath, [], {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: process.env,
      windowsHide: true
    });
    
    this.server.stdout.on('data', (data) => {
      this.buffer += data.toString();
      this.processBuffer();
    });
    
    this.server.stderr.on('data', (data) => {
      if (process.env.DEBUG || process.env.MCP_DEBUG) {
        console.error('[MCP Server]', data.toString());
      }
    });
    
    this.server.on('error', (err) => {
      if (!this.isClosing) {
        console.error(`MCP server error: ${err.message}`);
        this.cleanup();
      }
    });
    
    this.server.on('exit', (code, signal) => {
      if (!this.isClosing && code !== 0) {
        console.error(`MCP server exited unexpectedly: code=${code}, signal=${signal}`);
      }
      this.cleanup();
    });
    
    // Give server time to start
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  processBuffer() {
    const lines = this.buffer.split('\n');
    this.buffer = lines.pop() || '';
    
    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        const message = JSON.parse(line);
        this.handleMessage(message);
      } catch (err) {
        // Ignore non-JSON lines
      }
    }
  }
  
  handleMessage(message) {
    if (message.id && this.pendingRequests.has(message.id)) {
      const { resolve, reject } = this.pendingRequests.get(message.id);
      this.pendingRequests.delete(message.id);
      
      if (message.error) {
        reject(new Error(message.error.message || 'Unknown error'));
      } else {
        resolve(message.result);
      }
    }
  }
  
  async callTool(toolName, args) {
    if (!this.server) {
      await this.connect();
    }
    
    const messageId = ++this.messageId;
    const request = {
      jsonrpc: '2.0',
      id: messageId,
      method: 'tools/call',
      params: {
        name: toolName,
        arguments: args
      }
    };
    
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pendingRequests.delete(messageId);
        reject(new Error(`Timeout calling ${toolName}`));
      }, process.env.MCP_TIMEOUT || 30000);
      
      this.pendingRequests.set(messageId, {
        resolve: (result) => {
          clearTimeout(timeout);
          resolve(result);
        },
        reject: (error) => {
          clearTimeout(timeout);
          reject(error);
        }
      });
      
      if (process.env.DEBUG || process.env.MCP_DEBUG) {
        console.error('[MCP Request]', JSON.stringify(request));
      }
      
      this.server.stdin.write(JSON.stringify(request) + '\n');
    });
  }
  
  cleanup() {
    this.isClosing = true;
    if (this.server) {
      this.server.kill();
      this.server = null;
    }
    // Clear pending requests
    for (const [id, { reject }] of this.pendingRequests) {
      reject(new Error('Connection closed'));
    }
    this.pendingRequests.clear();
  }
  
  disconnect() {
    this.cleanup();
  }
}

// Get or create singleton connection
async function getConnection() {
  if (!connection) {
    connection = new MCPConnection();
    await connection.connect();
  }
  return connection;
}

// Reset connection (useful for error recovery)
function resetConnection() {
  if (connection) {
    connection.disconnect();
    connection = null;
  }
}

// Cleanup on exit
process.on('exit', () => {
  if (connection) {
    connection.disconnect();
  }
});

process.on('SIGINT', () => {
  if (connection) {
    connection.disconnect();
  }
  process.exit(0);
});

process.on('SIGTERM', () => {
  if (connection) {
    connection.disconnect();
  }
  process.exit(0);
});

module.exports = { getConnection, resetConnection };