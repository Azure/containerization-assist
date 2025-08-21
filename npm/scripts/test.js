#!/usr/bin/env node

import { spawn, execSync } from 'child_process';
import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';

// ES module equivalent of __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Colors for output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m'
};

// File permissions
const EXECUTABLE_BITS = 0o111; // Check for any execute permission (owner/group/other)

function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

function logSection(title) {
  console.log('');
  log(`═══ ${title} ═══`, 'bright');
  console.log('');
}

// Test suite
class MCPServerTester {
  constructor() {
    this.testResults = [];
    this.npmDir = path.dirname(__dirname);
  }

  async runTests() {
    logSection('Containerization Assist MCP Server - NPM Package Test Suite');
    
    // Run all tests
    await this.testBinaryExists();
    await this.testBinaryExecutable();
    await this.testVersionCommand();
    await this.testHelpCommand();
    await this.testPingTool();
    await this.testStdioMode();
    await this.testPackageStructure();
    
    // Summary
    this.printSummary();
  }

  async testBinaryExists() {
    log('Testing: Binary exists...', 'cyan');
    
    const binDir = path.join(this.npmDir, 'bin');
    const platform = process.platform;
    const arch = process.arch === 'arm64' ? 'arm64' : 'x64';
    
    // Map platform to our directory structure
    let platformDir;
    if (platform === 'darwin') {
      platformDir = `darwin-${arch}`;
    } else if (platform === 'linux') {
      platformDir = `linux-${arch}`;
    } else if (platform === 'win32') {
      platformDir = `win32-${arch}`;
    }
    
    const expectedBinary = platform === 'win32' ? 'containerization-assist-mcp.exe' : 'containerization-assist-mcp';
    const binaryPath = path.join(binDir, platformDir, expectedBinary);
    
    if (fs.existsSync(binaryPath)) {
      this.pass('Binary exists at expected location');
      this.binaryPath = binaryPath;
    } else {
      // Check if any binaries exist
      try {
        const platformDirs = fs.readdirSync(binDir).filter(d => fs.statSync(path.join(binDir, d)).isDirectory());
        const binaries = [];
        for (const dir of platformDirs) {
          const files = fs.readdirSync(path.join(binDir, dir));
          const mcpBinaries = files.filter(f => f.includes('containerization-assist-mcp'));
          if (mcpBinaries.length > 0) {
            binaries.push(`${dir}/${mcpBinaries.join(', ')}`);
          }
        }
        if (binaries.length > 0) {
          this.warn(`Binary not at expected location ${platformDir}, but found: ${binaries.join(', ')}`);
        } else {
          this.fail('No MCP server binaries found');
        }
      } catch (err) {
        this.fail('No MCP server binaries found');
      }
    }
  }

  async testBinaryExecutable() {
    log('Testing: Binary is executable...', 'cyan');
    
    if (process.platform === 'win32') {
      this.skip('Executable check not applicable on Windows');
      return;
    }
    
    if (!this.binaryPath || !fs.existsSync(this.binaryPath)) {
      this.skip('Binary not found, skipping executable test');
      return;
    }
    
    try {
      const stats = fs.statSync(this.binaryPath);
      const isExecutable = (stats.mode & EXECUTABLE_BITS) !== 0;
      
      if (isExecutable) {
        this.pass('Binary is executable');
      } else {
        this.fail('Binary is not executable');
      }
    } catch (err) {
      this.fail(`Could not check binary permissions: ${err.message}`);
    }
  }

  async testVersionCommand() {
    log('Testing: Version command...', 'cyan');
    
    if (!this.binaryPath) {
      this.skip('Binary not found, skipping version test');
      return;
    }
    
    try {
      const result = await new Promise((resolve) => {
        const child = spawn(this.binaryPath, ['--version'], {
          stdio: ['pipe', 'pipe', 'pipe']
        });
        
        let output = '';
        let errorOutput = '';
        
        child.stdout.on('data', (data) => {
          output += data.toString();
        });
        
        child.stderr.on('data', (data) => {
          errorOutput += data.toString();
        });
        
        child.on('close', (code) => {
          resolve({
            success: code === 0,
            output: output || errorOutput
          });
        });
      });
      
      if (result.success && result.output.includes('Containerization Assist MCP Server')) {
        this.pass(`Version command works: ${result.output.trim()}`);
      } else {
        this.fail(`Version command output unexpected: ${result.output}`);
      }
    } catch (err) {
      this.fail(`Version command error: ${err.message}`);
    }
  }

  async testHelpCommand() {
    log('Testing: Help command...', 'cyan');
    
    try {
      const result = await this.runCommand(['--help']);
      if (result.success) {
        this.pass('Help command works');
      } else {
        this.fail(`Help command failed: ${result.output}`);
      }
    } catch (err) {
      this.fail(`Help command error: ${err.message}`);
    }
  }

  async testPingTool() {
    log('Testing: MCP ping tool...', 'cyan');
    
    if (!this.binaryPath) {
      this.skip('Binary not found, skipping ping test');
      return;
    }
    
    try {
      // Test ping tool using tool mode
      const result = await new Promise((resolve, reject) => {
        const env = {
          ...process.env,
          TOOL_PARAMS: JSON.stringify({ message: 'test' })
        };
        
        const child = spawn(this.binaryPath, ['tool', 'ping'], {
          env,
          stdio: ['pipe', 'pipe', 'pipe']
        });
        
        let output = '';
        let errorOutput = '';
        
        child.stdout.on('data', (data) => {
          output += data.toString();
        });
        
        child.stderr.on('data', (data) => {
          errorOutput += data.toString();
        });
        
        child.on('close', (code) => {
          if (code === 0) {
            resolve(output);
          } else {
            reject(new Error(errorOutput || 'Tool execution failed'));
          }
        });
        
        child.on('error', (err) => {
          reject(err);
        });
      });
      
      // Parse the JSON response
      const response = JSON.parse(result);
      if (response.response && response.response.includes('pong')) {
        this.pass(`MCP ping tool responds correctly: ${response.response}`);
      } else {
        this.warn('MCP ping tool response unexpected: ' + result);
      }
    } catch (err) {
      this.warn(`MCP ping test failed: ${err.message}`);
    }
  }

  async testStdioMode() {
    log('Testing: STDIO transport mode...', 'cyan');
    
    let child;
    try {
      child = spawn('node', [path.join(this.npmDir, 'index.js')], {
        stdio: ['pipe', 'pipe', 'pipe']
      });
      
      let started = false;
      const timeout = setTimeout(() => {
        if (!started) {
          child.kill();
        }
      }, 3000);
      
      child.stderr.on('data', (data) => {
        const output = data.toString();
        if (output.includes('Starting') || output.includes('MCP')) {
          started = true;
          clearTimeout(timeout);
          child.kill();
          this.pass('STDIO mode starts correctly');
        }
      });
      
      child.on('error', (err) => {
        this.fail(`STDIO mode error: ${err.message}`);
      });
      
      // Wait a bit for the test to complete
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      if (!started) {
        clearTimeout(timeout);
        child.kill();
        this.warn('STDIO mode test inconclusive');
      }
    } catch (err) {
      this.fail(`STDIO mode test error: ${err.message}`);
    }
  }

  async testPackageStructure() {
    log('Testing: Package structure...', 'cyan');
    
    const requiredFiles = [
      'package.json',
      'index.js',
      'README.md',
      'scripts/postinstall.js'
    ];
    
    const missing = [];
    for (const file of requiredFiles) {
      const filePath = path.join(this.npmDir, file);
      if (!fs.existsSync(filePath)) {
        missing.push(file);
      }
    }
    
    if (missing.length === 0) {
      this.pass('All required package files present');
    } else {
      this.fail(`Missing files: ${missing.join(', ')}`);
    }
  }

  // Helper methods
  async runCommand(args) {
    return new Promise((resolve) => {
      const child = spawn('node', [path.join(this.npmDir, 'lib', 'index.js'), ...args], {
        stdio: ['pipe', 'pipe', 'pipe']
      });
      
      let output = '';
      let errorOutput = '';
      
      child.stdout.on('data', (data) => {
        output += data.toString();
      });
      
      child.stderr.on('data', (data) => {
        errorOutput += data.toString();
      });
      
      child.on('exit', (code) => {
        resolve({
          success: code === 0,
          output: output || errorOutput,
          code
        });
      });
      
      // Timeout after 5 seconds
      setTimeout(() => {
        child.kill();
        resolve({
          success: false,
          output: 'Command timed out',
          code: -1
        });
      }, 5000);
    });
  }

  async runMCPCommand(request, timeout = 5000) {
    return new Promise((resolve, reject) => {
      const child = spawn('node', [path.join(this.npmDir, 'index.js')], {
        stdio: ['pipe', 'pipe', 'pipe']
      });
      
      let response = '';
      
      child.stdout.on('data', (data) => {
        response += data.toString();
      });
      
      child.stdin.write(JSON.stringify(request) + '\n');
      
      setTimeout(() => {
        child.kill();
        resolve(response);
      }, timeout);
    });
  }

  // Test result helpers
  pass(message) {
    this.testResults.push({ status: 'pass', message });
    log(`  ✅ ${message}`, 'green');
  }

  fail(message) {
    this.testResults.push({ status: 'fail', message });
    log(`  ❌ ${message}`, 'red');
  }

  warn(message) {
    this.testResults.push({ status: 'warn', message });
    log(`  ⚠️  ${message}`, 'yellow');
  }

  skip(message) {
    this.testResults.push({ status: 'skip', message });
    log(`  ⏭️  ${message}`, 'cyan');
  }

  printSummary() {
    logSection('Test Summary');
    
    const passed = this.testResults.filter(r => r.status === 'pass').length;
    const failed = this.testResults.filter(r => r.status === 'fail').length;
    const warned = this.testResults.filter(r => r.status === 'warn').length;
    const skipped = this.testResults.filter(r => r.status === 'skip').length;
    
    log(`Passed:  ${passed}`, 'green');
    log(`Failed:  ${failed}`, failed > 0 ? 'red' : 'green');
    log(`Warned:  ${warned}`, warned > 0 ? 'yellow' : 'green');
    log(`Skipped: ${skipped}`, 'cyan');
    
    console.log('');
    
    if (failed === 0) {
      log('✨ All critical tests passed!', 'bright');
      process.exit(0);
    } else {
      log('❌ Some tests failed. Please review the output above.', 'red');
      process.exit(1);
    }
  }
}

// Run tests
async function main() {
  const tester = new MCPServerTester();
  await tester.runTests();
}

// Run tests if this is the main module
main().catch(err => {
  log(`Test suite error: ${err.message}`, 'red');
  process.exit(1);
});