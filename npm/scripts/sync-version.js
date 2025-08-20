#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { execSync } from 'child_process';
import { fileURLToPath } from 'url';

// ES module equivalent of __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Colors for output
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m'
};

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

/**
 * Get version from Git tags
 */
function getGitVersion() {
  try {
    // Try to get the latest tag
    const gitTag = execSync('git describe --tags --abbrev=0 2>/dev/null', { encoding: 'utf8' }).trim();
    // Remove 'v' prefix if present
    return gitTag.replace(/^v/, '');
  } catch (err) {
    // No tags found, try to get from commit
    try {
      const shortHash = execSync('git rev-parse --short HEAD', { encoding: 'utf8' }).trim();
      return `0.0.0-dev.${shortHash}`;
    } catch (err2) {
      return '0.0.0-dev';
    }
  }
}

/**
 * Get version from Go source code
 */
function getGoVersion() {
  const mainGoPath = path.join(__dirname, '..', '..', 'cmd', 'mcp-server', 'main.go');
  
  if (!fs.existsSync(mainGoPath)) {
    log('Warning: main.go not found', 'yellow');
    return null;
  }
  
  const content = fs.readFileSync(mainGoPath, 'utf8');
  
  // Look for Version variable
  const versionMatch = content.match(/Version\s*=\s*"([^"]+)"/);
  if (versionMatch && versionMatch[1] !== 'dev') {
    return versionMatch[1];
  }
  
  return null;
}

/**
 * Get current package.json version
 */
function getPackageVersion() {
  const packagePath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
  return packageJson.version;
}

/**
 * Update package.json version
 */
function updatePackageVersion(version) {
  const packagePath = path.join(__dirname, '..', 'package.json');
  const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
  
  const oldVersion = packageJson.version;
  packageJson.version = version;
  
  fs.writeFileSync(packagePath, JSON.stringify(packageJson, null, 2) + '\n');
  
  return oldVersion;
}

/**
 * Main synchronization logic
 */
function main() {
  log('Version Synchronization Tool', 'cyan');
  log('============================', 'cyan');
  console.log('');
  
  // Get versions from different sources
  const gitVersion = getGitVersion();
  const goVersion = getGoVersion();
  const packageVersion = getPackageVersion();
  
  log(`Git version:     ${gitVersion || 'not found'}`, 'yellow');
  log(`Go version:      ${goVersion || 'not found'}`, 'yellow');
  log(`Package version: ${packageVersion}`, 'yellow');
  console.log('');
  
  // Determine which version to use (priority order)
  let targetVersion = null;
  let source = '';
  
  // Command line argument takes precedence
  const cliVersion = process.argv[2];
  if (cliVersion) {
    targetVersion = cliVersion.replace(/^v/, '');
    source = 'CLI argument';
  }
  // Then Git tag
  else if (gitVersion && !gitVersion.includes('dev')) {
    targetVersion = gitVersion;
    source = 'Git tag';
  }
  // Then Go source
  else if (goVersion) {
    targetVersion = goVersion;
    source = 'Go source';
  }
  // Default to current package version
  else {
    targetVersion = packageVersion;
    source = 'package.json (unchanged)';
  }
  
  // Update if different
  if (targetVersion !== packageVersion) {
    const oldVersion = updatePackageVersion(targetVersion);
    log(`✅ Updated package.json version:`, 'green');
    log(`   ${oldVersion} → ${targetVersion}`, 'green');
    log(`   Source: ${source}`, 'cyan');
  } else {
    log(`✅ Version already synchronized: ${targetVersion}`, 'green');
    log(`   Source: ${source}`, 'cyan');
  }
  
  // Verify the update
  const newPackageVersion = getPackageVersion();
  if (newPackageVersion !== targetVersion) {
    log(`❌ Version update failed!`, 'red');
    process.exit(1);
  }
  
  console.log('');
  log('Version synchronization complete!', 'green');
}

// Run if called directly
try {
  main();
} catch (err) {
  log(`Error: ${err.message}`, 'red');
  process.exit(1);
}

export {
  getGitVersion,
  getGoVersion,
  getPackageVersion,
  updatePackageVersion
};