/**
 * CLI Functionality Tests
 * 
 * Validates that the CLI works correctly after installation
 * and that all CLI commands function as expected.
 */

import { execSync, spawn } from 'child_process';
import { existsSync, mkdirSync, rmSync, readFileSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

const PROJECT_ROOT = join(__dirname, '../..');

describe('CLI Functionality', () => {
  let testDir: string;
  let packageTarball: string;
  let installDir: string;
  let cliPath: string;

  beforeAll(async () => {
    // Ensure project is built
    execSync('npm run build', { cwd: PROJECT_ROOT, stdio: 'pipe' });
    
    // Create package
    const result = execSync('npm pack', { 
      cwd: PROJECT_ROOT, 
      encoding: 'utf8' 
    }).trim();
    packageTarball = join(PROJECT_ROOT, result);
    
    // Create test directory
    testDir = join(tmpdir(), `cli-test-${Date.now()}`);
    mkdirSync(testDir, { recursive: true });
    
    // Set up installation
    installDir = join(testDir, 'install');
    mkdirSync(installDir, { recursive: true });
    execSync('npm init -y', { cwd: installDir, stdio: 'pipe' });
    execSync(`npm install "${packageTarball}"`, { 
      cwd: installDir, 
      stdio: 'pipe' 
    });
    
    // Get CLI path
    const packageJson = JSON.parse(readFileSync(
      join(installDir, 'node_modules/@thgamble/containerization-assist-mcp/package.json'), 
      'utf8'
    ));
    const binRelativePath = packageJson.bin['containerization-assist-mcp'];
    cliPath = join(installDir, 'node_modules/@thgamble/containerization-assist-mcp', binRelativePath);
  }, 180000); // 3 minutes timeout

  afterAll(() => {
    // Cleanup
    if (existsSync(testDir)) {
      rmSync(testDir, { recursive: true, force: true });
    }
    
    if (existsSync(packageTarball)) {
      rmSync(packageTarball, { force: true });
    }
  });

  describe('CLI Binary', () => {
    it('should exist and be executable', () => {
      expect(existsSync(cliPath)).toBe(true);
      
      // Check if it's a JavaScript file
      const content = readFileSync(cliPath, 'utf8');
      expect(content).toMatch(/^#!/); // Should have shebang
    });

    it('should show help without errors', () => {
      const result = execSync(`node "${cliPath}" --help`, { 
        encoding: 'utf8',
        cwd: installDir 
      });
      
      expect(result).toContain('containerization-assist-mcp');
      expect(result).toContain('MCP server for AI-powered containerization workflows');
      expect(result).toContain('Options:');
    });

    it('should show version without errors', () => {
      const result = execSync(`node "${cliPath}" --version`, { 
        encoding: 'utf8',
        cwd: installDir 
      });
      
      // Should contain version number
      expect(result.trim()).toMatch(/^\d+\.\d+\.\d+/);
    });
  });

  describe('CLI Commands', () => {
    it('should validate configuration without Docker', () => {
      try {
        const result = execSync(`node "${cliPath}" --validate`, { 
          encoding: 'utf8',
          cwd: installDir,
          timeout: 30000,
          env: { ...process.env, NODE_ENV: 'test' }
        });
        
        expect(result).toContain('Configuration validation');
      } catch (error: any) {
        // Expected to fail if Docker isn't available, but should be graceful
        expect(error.stdout || error.stderr).toContain('Docker');
      }
    });

    it('should list tools without errors', () => {
      const result = execSync(`node "${cliPath}" --list-tools`, { 
        encoding: 'utf8',
        cwd: installDir,
        timeout: 30000,
        env: { ...process.env, NODE_ENV: 'test' }
      });
      
      expect(result).toContain('Available MCP Tools');
      expect(result).toContain('analyze-repo');
      expect(result).toContain('generate-dockerfile');
      expect(result).toContain('build-image');
    });

    it('should perform health check', () => {
      try {
        const result = execSync(`node "${cliPath}" --health-check`, { 
          encoding: 'utf8',
          cwd: installDir,
          timeout: 30000,
          env: { ...process.env, NODE_ENV: 'test' }
        });
        
        expect(result).toContain('Health Check Results');
      } catch (error: any) {
        // May fail due to Docker availability, but should be graceful
        const output = error.stdout || error.stderr || '';
        expect(output).toContain('Health Check');
      }
    });
  });

  describe('CLI Server Mode', () => {
    it('should start server and respond to stdin', (done) => {
      const child = spawn('node', [cliPath], {
        cwd: installDir,
        stdio: ['pipe', 'pipe', 'pipe'],
        env: { ...process.env, NODE_ENV: 'test', MCP_QUIET: 'true' }
      });

      let stdout = '';
      let stderr = '';
      let hasResponded = false;

      child.stdout.on('data', (data) => {
        stdout += data.toString();
        
        // Look for server ready indication or MCP response
        if (!hasResponded && (stdout.includes('"result"') || stdout.includes('"error"'))) {
          hasResponded = true;
          child.kill();
          
          // Should contain valid JSON response
          try {
            const lines = stdout.trim().split('\n');
            const lastLine = lines[lines.length - 1];
            JSON.parse(lastLine);
            done();
          } catch (error) {
            done(new Error(`Invalid JSON response: ${stdout}`));
          }
        }
      });

      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('error', (error) => {
        if (!hasResponded) {
          done(error);
        }
      });

      child.on('exit', (code) => {
        if (!hasResponded) {
          if (code === 0) {
            done();
          } else {
            done(new Error(`CLI exited with code ${code}. Stderr: ${stderr}`));
          }
        }
      });

      // Send a basic MCP ping request
      setTimeout(() => {
        try {
          child.stdin.write(JSON.stringify({
            jsonrpc: '2.0',
            id: 1,
            method: 'ping'
          }) + '\n');
        } catch (error) {
          if (!hasResponded) {
            done(error);
          }
        }
      }, 2000);

      // Timeout after 10 seconds
      setTimeout(() => {
        if (!hasResponded) {
          child.kill();
          done(new Error(`Server didn't respond within timeout. Stdout: ${stdout}, Stderr: ${stderr}`));
        }
      }, 10000);
    }, 15000);
  });

  describe('Error Handling', () => {
    it('should handle invalid commands gracefully', () => {
      try {
        execSync(`node "${cliPath}" invalid-command`, { 
          encoding: 'utf8',
          cwd: installDir 
        });
        fail('Should have thrown an error');
      } catch (error: any) {
        expect(error.status).toBe(1);
        const output = error.stdout || error.stderr || '';
        expect(output).toContain('Unknown command');
      }
    });

    it('should handle invalid options gracefully', () => {
      try {
        execSync(`node "${cliPath}" --invalid-option`, { 
          encoding: 'utf8',
          cwd: installDir 
        });
        fail('Should have thrown an error');
      } catch (error: any) {
        expect(error.status).toBe(1);
        const output = error.stdout || error.stderr || '';
        expect(output).toMatch(/unknown option|error/i);
      }
    });
  });
});