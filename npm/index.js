#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// ANSI color codes for error messages
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  cyan: '\x1b[36m'
};

function logError(message) {
  console.error(`${colors.red}${message}${colors.reset}`);
}

function logWarning(message) {
  console.error(`${colors.yellow}${message}${colors.reset}`);
}

function logInfo(message) {
  console.error(`${colors.cyan}${message}${colors.reset}`);
}

// Determine the binary path
function getBinaryPath() {
  const platform = process.platform;
  const binDir = path.join(__dirname, 'bin');
  
  // First, try the platform-agnostic symlink/copy
  const defaultBinary = path.join(binDir, platform === 'win32' ? 'mcp-server.exe' : 'mcp-server');
  if (fs.existsSync(defaultBinary)) {
    return defaultBinary;
  }
  
  // Fallback to platform-specific binary
  const archMap = {
    'x64': 'x64',
    'arm64': 'arm64',
  };
  
  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'win'
  };
  
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
  
  // No binary found
  return null;
}

// Main execution
function main() {
  const binaryPath = getBinaryPath();
  
  if (!binaryPath) {
    logError('Containerization Assist MCP Server binary not found!');
    logWarning('\nThis usually means the post-install script failed.');
    logInfo('\nTry these solutions:');
    logInfo('  1. Reinstall the package:');
    logInfo('     npm uninstall @container-assist/mcp-server');
    logInfo('     npm install @container-assist/mcp-server');
    logInfo('\n  2. Build from source:');
    logInfo('     cd ' + __dirname);
    logInfo('     npm run build:current');
    logInfo('\n  3. Check your platform is supported:');
    logInfo(`     Platform: ${process.platform}`);
    logInfo(`     Architecture: ${process.arch}`);
    process.exit(1);
  }
  
  // Get command line arguments (skip node and script name)
  const args = process.argv.slice(2);
  
  // Check if running in debug mode
  const isDebug = args.includes('--debug') || process.env.MCP_DEBUG === 'true';
  
  if (isDebug) {
    logInfo(`Starting Containerization Assist MCP Server`);
    logInfo(`Binary: ${binaryPath}`);
    logInfo(`Arguments: ${args.join(' ')}`);
    logInfo(`Working Directory: ${process.cwd()}`);
  }
  
  // Spawn the Go binary with stdio inheritance for MCP protocol
  const child = spawn(binaryPath, args, {
    stdio: 'inherit',        // Critical for MCP stdio transport
    env: {
      ...process.env,       // Pass through all environment variables
      // Ensure Go environment variables are set if needed
      GOSUMDB: process.env.GOSUMDB || 'sum.golang.org',
      GOPROXY: process.env.GOPROXY || ''
    },
    cwd: process.cwd(),     // Use current working directory
    windowsHide: true       // Hide console window on Windows
  });
  
  // Handle child process exit
  child.on('exit', (code, signal) => {
    if (signal) {
      if (isDebug) {
        logWarning(`Server terminated by signal: ${signal}`);
      }
      process.exit(1);
    } else if (code !== 0 && code !== null) {
      if (isDebug) {
        logError(`Server exited with code: ${code}`);
      }
      process.exit(code);
    } else {
      // Clean exit
      process.exit(0);
    }
  });
  
  // Handle errors
  child.on('error', (err) => {
    if (err.code === 'ENOENT') {
      logError('Containerization Assist MCP Server binary not found!');
      logError(`Expected at: ${binaryPath}`);
      logInfo('\nPlease reinstall the package:');
      logInfo('  npm uninstall -g @container-assist/mcp-server');
      logInfo('  npm install -g @container-assist/mcp-server');
    } else if (err.code === 'EACCES') {
      logError('Permission denied to execute the binary!');
      logInfo('\nTry fixing permissions:');
      logInfo(`  chmod +x "${binaryPath}"`);
    } else {
      logError(`Failed to start Containerization Assist MCP Server: ${err.message}`);
      if (isDebug) {
        console.error(err);
      }
    }
    process.exit(1);
  });
  
  // Handle termination signals gracefully
  const signals = ['SIGINT', 'SIGTERM', 'SIGQUIT'];
  signals.forEach(signal => {
    process.on(signal, () => {
      if (isDebug) {
        logInfo(`Received ${signal}, shutting down...`);
      }
      child.kill(signal);
    });
  });
}

// Run if this is the main module
if (require.main === module) {
  main();
}

// Export for programmatic use
module.exports = {
  getBinaryPath,
  spawn: main
};