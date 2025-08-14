#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { platform, arch } = process;

// ANSI color codes for terminal output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m'
};

// Helper function for colored output
function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

// Platform mapping
const platformMap = {
  'darwin': 'darwin',
  'linux': 'linux',
  'win32': 'win'
};

// Architecture mapping
const archMap = {
  'x64': 'x64',
  'arm64': 'arm64',
  'arm': 'arm64', // Map arm to arm64 as fallback
};

// Get platform and architecture
const mappedPlatform = platformMap[platform];
const mappedArch = archMap[arch] || arch;

if (!mappedPlatform) {
  log(`✗ Unsupported platform: ${platform}`, 'red');
  log(`  Supported platforms: macOS, Linux, Windows`, 'yellow');
  process.exit(1);
}

// Construct binary name
let binaryName = `mcp-server-${mappedPlatform}-${mappedArch}`;
if (platform === 'win32') {
  binaryName += '.exe';
}

// Paths
const binDir = path.join(__dirname, '..', 'bin');
const binaryPath = path.join(binDir, binaryName);
const linkPath = path.join(binDir, platform === 'win32' ? 'mcp-server.exe' : 'mcp-server');

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  log(`✗ Binary not found for your platform: ${binaryName}`, 'red');
  log(`  Platform: ${platform} (${arch})`, 'yellow');
  log(`  Expected binary: ${binaryPath}`, 'yellow');
  
  // Provide helpful suggestions
  log('\n  Possible solutions:', 'cyan');
  log('  1. Build from source:', 'cyan');
  log(`     cd ${path.dirname(__dirname)}`, 'yellow');
  log('     npm run build:current', 'yellow');
  log('  2. Check if your platform is supported', 'cyan');
  log('  3. Report an issue: https://github.com/Azure/container-kit/issues', 'cyan');
  
  // List available binaries
  log('\n  Available binaries:', 'cyan');
  try {
    const files = fs.readdirSync(binDir);
    const binaries = files.filter(f => f.startsWith('mcp-server-'));
    if (binaries.length > 0) {
      binaries.forEach(b => log(`    - ${b}`, 'yellow'));
    } else {
      log('    No binaries found. Run: npm run build', 'yellow');
    }
  } catch (err) {
    log('    Could not list binaries', 'yellow');
  }
  
  process.exit(1);
}

// Make binary executable (Unix-like systems)
if (platform !== 'win32') {
  try {
    fs.chmodSync(binaryPath, 0o755);
    log(`✓ Made binary executable: ${binaryName}`, 'green');
  } catch (err) {
    log(`⚠ Could not make binary executable: ${err.message}`, 'yellow');
  }
}

// Create or update symlink/copy
try {
  // Remove existing link/file if it exists
  if (fs.existsSync(linkPath)) {
    fs.unlinkSync(linkPath);
  }
  
  // On Windows, copy the file instead of symlinking
  if (platform === 'win32') {
    fs.copyFileSync(binaryPath, linkPath);
    log(`✓ Copied binary to: mcp-server.exe`, 'green');
  } else {
    // Create relative symlink for Unix-like systems
    fs.symlinkSync(binaryName, linkPath);
    log(`✓ Created symlink: mcp-server -> ${binaryName}`, 'green');
  }
} catch (err) {
  log(`⚠ Could not create link: ${err.message}`, 'yellow');
  log(`  You can still use the binary directly: ${binaryName}`, 'yellow');
}

// Success message
console.log('');
log('╔═══════════════════════════════════════════════════════════╗', 'bright');
log('║  Container Kit MCP Server installed successfully! 🚀      ║', 'bright');
log('╚═══════════════════════════════════════════════════════════╝', 'bright');
console.log('');
log(`  Platform:     ${platform} (${arch})`, 'cyan');
log(`  Binary:       ${binaryName}`, 'cyan');
log(`  Install path: ${binDir}`, 'cyan');
console.log('');
log('  Usage:', 'green');
log('    npx container-kit-mcp --help', 'yellow');
log('    npx ckmcp --version', 'yellow');
console.log('');
log('  Documentation: https://github.com/Azure/container-kit', 'cyan');
console.log('');