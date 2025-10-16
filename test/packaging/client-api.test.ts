/**
 * Client API Compatibility Tests
 * 
 * Validates that the published package continues to support
 * programmatic usage patterns as shown in the client example.
 */

import { execSync } from 'child_process';
import { existsSync, rmSync, readFileSync, writeFileSync, mkdirSync } from 'fs';
import { join } from 'path';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';

const PROJECT_ROOT = join(__dirname, '../..');

describe('Client API Compatibility', () => {
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let packageTarball: string;
  let clientTestDir: string;

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
    const tempResult = createTestTempDir('client-api-test-');
    testDir = tempResult.dir;
    cleanup = tempResult.cleanup;

    // Set up client test environment
    clientTestDir = join(testDir.name, 'client');
    mkdirSync(clientTestDir, { recursive: true });

    // Create package.json for client test
    const clientPackageJson = {
      name: 'client-test',
      version: '1.0.0',
      type: 'commonjs',
      main: 'index.js'
    };
    writeFileSync(
      join(clientTestDir, 'package.json'),
      JSON.stringify(clientPackageJson, null, 2)
    );

    // Install our package and dependencies
    execSync(`npm install "${packageTarball}"`, {
      cwd: clientTestDir,
      stdio: 'pipe'
    });

    // Install MCP SDK (required for client example)
    execSync('npm install @modelcontextprotocol/sdk', {
      cwd: clientTestDir,
      stdio: 'pipe'
    });
  }, 180000); // 3 minutes timeout

  afterAll(async () => {
    // Cleanup
    await cleanup();

    if (existsSync(packageTarball)) {
      rmSync(packageTarball, { force: true });
    }
  });

  describe('Package Exports', () => {
    it('should export createContainerAssistServer', () => {
      const testScript = `
        const { createContainerAssistServer } = require('containerization-assist-mcp');
        console.log(typeof createContainerAssistServer);
      `;

      writeFileSync(join(clientTestDir, 'test-exports.js'), testScript);

      const result = execSync('node test-exports.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('function');
    });

    it('should export TOOL_NAMES', () => {
      const testScript = `
        const { TOOL_NAMES } = require('containerization-assist-mcp');
        console.log(typeof TOOL_NAMES);
        console.log(Object.keys(TOOL_NAMES).length > 0);
      `;

      writeFileSync(join(clientTestDir, 'test-tool-names.js'), testScript);

      const result = execSync('node test-tool-names.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(lines[0]).toBe('object');
      expect(lines[1]).toBe('true');
    });

    it('should have expected tool names', () => {
      const testScript = `
        const { TOOL_NAMES } = require('containerization-assist-mcp');
        const expectedTools = [
          'ANALYZE_REPO',
          'BUILD_IMAGE', 
          'GENERATE_DOCKERFILE',
          'SCAN_IMAGE',
          'TAG_IMAGE'
        ];
        
        expectedTools.forEach(tool => {
          if (!TOOL_NAMES[tool]) {
            throw new Error(\`Missing tool: \${tool}\`);
          }
        });
        
        console.log('All expected tools found');
      `;

      writeFileSync(join(clientTestDir, 'test-tool-names-specific.js'), testScript);

      const result = execSync('node test-tool-names-specific.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('All expected tools found');
    });
  });

  describe('createContainerAssistServer Factory', () => {
    it('should create server instance without errors', () => {
      const testScript = `
        const { createContainerAssistServer } = require('containerization-assist-mcp');

        try {
          const server = createContainerAssistServer();
          console.log('instantiated');
        } catch (error) {
          console.error('Error:', error.message);
          process.exit(1);
        }
      `;

      writeFileSync(join(clientTestDir, 'test-instantiation.js'), testScript);

      const result = execSync('node test-instantiation.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('instantiated');
    });

    it('should have registerTools method', () => {
      const testScript = `
        const { createContainerAssistServer } = require('containerization-assist-mcp');

        const server = createContainerAssistServer();
        console.log(typeof server.registerTools);
      `;

      writeFileSync(join(clientTestDir, 'test-register-tools.js'), testScript);

      const result = execSync('node test-register-tools.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('function');
    });
  });

  describe('Client Example Integration', () => {
    it('should support the exact client usage pattern', () => {
      // Create a simplified version of the client example
      const clientExample = `
        const { McpServer } = require("@modelcontextprotocol/sdk/server/mcp.js");
        const { createContainerAssistServer, TOOL_NAMES } = require('containerization-assist-mcp');

        function testClientPattern() {
          const server = new McpServer(
            {
              name: "testServer",
              version: "0.0.1",
            },
            {
              capabilities: {
                logging: {},
              }
            }
          );

          const caServer = createContainerAssistServer();

          // Test the registerTools method with the expected signature
          try {
            caServer.registerTools({ server }, {
              tools: [
                TOOL_NAMES.ANALYZE_REPO,
                TOOL_NAMES.BUILD_IMAGE,
                TOOL_NAMES.GENERATE_DOCKERFILE
              ],
              nameMapping: {
                [TOOL_NAMES.ANALYZE_REPO]: 'analyzeRepository',
                [TOOL_NAMES.BUILD_IMAGE]: 'buildImage',
                [TOOL_NAMES.GENERATE_DOCKERFILE]: 'generateDockerfile'
              }
            });
            
            console.log('registerTools completed successfully');
            return true;
          } catch (error) {
            console.error('registerTools failed:', error.message);
            return false;
          }
        }

        const success = testClientPattern();
        process.exit(success ? 0 : 1);
      `;

      writeFileSync(join(clientTestDir, 'client-pattern-test.js'), clientExample);

      // This should not throw
      const result = execSync('node client-pattern-test.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('registerTools completed successfully');
    });

    it('should support tool name mapping', () => {
      const testScript = `
        const { TOOL_NAMES } = require('containerization-assist-mcp');
        
        // Test that tool names can be used as object keys
        const mapping = {
          [TOOL_NAMES.ANALYZE_REPO]: 'analyzeRepository',
          [TOOL_NAMES.BUILD_IMAGE]: 'buildImage',
          [TOOL_NAMES.GENERATE_DOCKERFILE]: 'generateDockerfile',
          [TOOL_NAMES.SCAN_IMAGE]: 'scanImage',
          [TOOL_NAMES.TAG_IMAGE]: 'tagImage'
        };
        
        console.log(Object.keys(mapping).length);
      `;

      writeFileSync(join(clientTestDir, 'test-mapping.js'), testScript);

      const result = execSync('node test-mapping.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(parseInt(result.trim())).toBe(5);
    });
  });

  describe('Backward Compatibility', () => {
    it('should maintain consistent API surface', () => {
      const testScript = `
        const pkg = require('containerization-assist-mcp');
        
        // Check for key exports that clients depend on
        const expectedExports = [
          'createContainerAssistServer',
          'TOOL_NAMES'
        ];
        
        const missingExports = expectedExports.filter(exp => !(exp in pkg));
        
        if (missingExports.length > 0) {
          console.error('Missing exports:', missingExports);
          process.exit(1);
        }
        
        console.log('All expected exports present');
      `;

      writeFileSync(join(clientTestDir, 'test-api-surface.js'), testScript);

      const result = execSync('node test-api-surface.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('All expected exports present');
    });

    it('should handle CommonJS require correctly', () => {
      const testScript = `
        // Test different require patterns
        const pkg1 = require('containerization-assist-mcp');
        const { createContainerAssistServer } = require('containerization-assist-mcp');
        const { TOOL_NAMES } = require('containerization-assist-mcp');
        
        console.log('All require patterns work');
      `;

      writeFileSync(join(clientTestDir, 'test-require-patterns.js'), testScript);

      const result = execSync('node test-require-patterns.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('All require patterns work');
    });
  });
});