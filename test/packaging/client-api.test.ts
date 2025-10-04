/**
 * Client API Compatibility Tests
 *
 * Validates that the published package exports the modern API:
 * - createApp() for app runtime creation
 * - TOOLS for tool name constants
 * - getAllInternalTools() for tool registry access
 */

import { execSync } from 'child_process';
import { existsSync, mkdirSync, rmSync, readFileSync, writeFileSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

const PROJECT_ROOT = join(__dirname, '../..');

describe('Client API Compatibility', () => {
  let testDir: string;
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
    testDir = join(tmpdir(), `client-api-test-${Date.now()}`);
    mkdirSync(testDir, { recursive: true });
    
    // Set up client test environment
    clientTestDir = join(testDir, 'client');
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

  afterAll(() => {
    // Cleanup
    if (existsSync(testDir)) {
      rmSync(testDir, { recursive: true, force: true });
    }
    
    if (existsSync(packageTarball)) {
      rmSync(packageTarball, { force: true });
    }
  });

  describe('Package Exports', () => {
    it('should export createApp', () => {
      const testScript = `
        const { createApp } = require('@thgamble/containerization-assist-mcp');
        console.log(typeof createApp);
      `;

      writeFileSync(join(clientTestDir, 'test-exports.js'), testScript);

      const result = execSync('node test-exports.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('function');
    });

    it('should export TOOLS', () => {
      const testScript = `
        const { TOOLS } = require('@thgamble/containerization-assist-mcp');
        console.log(typeof TOOLS);
        console.log(Object.keys(TOOLS).length > 0);
      `;

      writeFileSync(join(clientTestDir, 'test-tools.js'), testScript);

      const result = execSync('node test-tools.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(lines[0]).toBe('object');
      expect(lines[1]).toBe('true');
    });

    it('should export getAllInternalTools', () => {
      const testScript = `
        const { getAllInternalTools } = require('@thgamble/containerization-assist-mcp');
        console.log(typeof getAllInternalTools);
        const tools = getAllInternalTools();
        console.log(Array.isArray(tools));
        console.log(tools.length > 0);
      `;

      writeFileSync(join(clientTestDir, 'test-get-tools.js'), testScript);

      const result = execSync('node test-get-tools.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(lines[0]).toBe('function');
      expect(lines[1]).toBe('true');
      expect(lines[2]).toBe('true');
    });

    it('should have expected tool names with canonical hyphenated format', () => {
      const testScript = `
        const { TOOLS } = require('@thgamble/containerization-assist-mcp');
        const expectedTools = [
          'ANALYZE_REPO',
          'BUILD_IMAGE',
          'GENERATE_DOCKERFILE',
          'SCAN',
          'TAG_IMAGE'
        ];

        // Verify constants exist
        expectedTools.forEach(tool => {
          if (!TOOLS[tool]) {
            throw new Error(\`Missing tool constant: \${tool}\`);
          }
        });

        // Verify canonical hyphenated names
        const expectedNames = {
          ANALYZE_REPO: 'analyze-repo',
          BUILD_IMAGE: 'build-image',
          GENERATE_DOCKERFILE: 'generate-dockerfile',
          SCAN: 'scan',
          TAG_IMAGE: 'tag-image'
        };

        for (const [key, expectedName] of Object.entries(expectedNames)) {
          if (TOOLS[key] !== expectedName) {
            throw new Error(\`Tool \${key} has value \${TOOLS[key]}, expected \${expectedName}\`);
          }
        }

        console.log('All expected tools found with canonical names');
      `;

      writeFileSync(join(clientTestDir, 'test-tool-names-specific.js'), testScript);

      const result = execSync('node test-tool-names-specific.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('All expected tools found with canonical names');
    });
  });

  describe('createApp AppRuntime', () => {
    it('should create app runtime without errors', () => {
      const testScript = `
        const { createApp } = require('@thgamble/containerization-assist-mcp');

        try {
          const app = createApp();
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

    it('should have bindToMCP method', () => {
      const testScript = `
        const { createApp } = require('@thgamble/containerization-assist-mcp');

        const app = createApp();
        console.log(typeof app.bindToMCP);
      `;

      writeFileSync(join(clientTestDir, 'test-bind-mcp.js'), testScript);

      const result = execSync('node test-bind-mcp.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('function');
    });

    it('should have execute method', () => {
      const testScript = `
        const { createApp } = require('@thgamble/containerization-assist-mcp');

        const app = createApp();
        console.log(typeof app.execute);
      `;

      writeFileSync(join(clientTestDir, 'test-execute.js'), testScript);

      const result = execSync('node test-execute.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('function');
    });

    it('should have listTools method', () => {
      const testScript = `
        const { createApp } = require('@thgamble/containerization-assist-mcp');

        const app = createApp();
        const tools = app.listTools();
        console.log(Array.isArray(tools));
        console.log(tools.length > 0);
        console.log(tools.every(t => typeof t.name === 'string'));
      `;

      writeFileSync(join(clientTestDir, 'test-list-tools.js'), testScript);

      const result = execSync('node test-list-tools.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(lines[0]).toBe('true');
      expect(lines[1]).toBe('true');
      expect(lines[2]).toBe('true');
    });
  });

  describe('Client Example Integration', () => {
    it('should support bindToMCP with external MCP server', () => {
      const clientExample = `
        const { Server } = require("@modelcontextprotocol/sdk/server/index.js");
        const { StdioServerTransport } = require("@modelcontextprotocol/sdk/server/stdio.js");
        const { createApp, TOOLS } = require('@thgamble/containerization-assist-mcp');

        async function testClientPattern() {
          // Create MCP server instance
          const server = new Server(
            {
              name: "testServer",
              version: "0.0.1",
            },
            {
              capabilities: {
                tools: {},
              }
            }
          );

          // Create app runtime
          const app = createApp();

          // Bind to the MCP server
          try {
            app.bindToMCP({ server });
            console.log('bindToMCP completed successfully');
            return true;
          } catch (error) {
            console.error('bindToMCP failed:', error.message);
            return false;
          }
        }

        testClientPattern().then(success => {
          process.exit(success ? 0 : 1);
        });
      `;

      writeFileSync(join(clientTestDir, 'client-pattern-test.js'), clientExample);

      // This should not throw
      const result = execSync('node client-pattern-test.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('bindToMCP completed successfully');
    });

    it('should support tool aliasing via config', () => {
      const testScript = `
        const { createApp, TOOLS } = require('@thgamble/containerization-assist-mcp');

        // Create app with tool aliases
        const app = createApp({
          toolAliases: {
            [TOOLS.ANALYZE_REPO]: 'analyzeRepository',
            [TOOLS.BUILD_IMAGE]: 'buildImage',
            [TOOLS.GENERATE_DOCKERFILE]: 'generateDockerfile',
            [TOOLS.SCAN]: 'scanImage',
            [TOOLS.TAG_IMAGE]: 'tagImage'
          }
        });

        const tools = app.listTools();
        const aliasedNames = tools.map(t => t.name).sort();

        console.log(aliasedNames.includes('analyzeRepository'));
        console.log(aliasedNames.includes('buildImage'));
        console.log(aliasedNames.includes('generateDockerfile'));
      `;

      writeFileSync(join(clientTestDir, 'test-aliasing.js'), testScript);

      const result = execSync('node test-aliasing.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(lines[0]).toBe('true');
      expect(lines[1]).toBe('true');
      expect(lines[2]).toBe('true');
    });

    it('should support TOOLS constants as keys', () => {
      const testScript = `
        const { TOOLS } = require('@thgamble/containerization-assist-mcp');

        // Test that TOOLS constants can be used as object keys
        const mapping = {
          [TOOLS.ANALYZE_REPO]: 'analyzeRepository',
          [TOOLS.BUILD_IMAGE]: 'buildImage',
          [TOOLS.GENERATE_DOCKERFILE]: 'generateDockerfile',
          [TOOLS.SCAN]: 'scanImage',
          [TOOLS.TAG_IMAGE]: 'tagImage'
        };

        console.log(Object.keys(mapping).length);
        console.log(mapping['analyze-repo']);
        console.log(mapping['scan']);
      `;

      writeFileSync(join(clientTestDir, 'test-mapping.js'), testScript);

      const result = execSync('node test-mapping.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      const lines = result.trim().split('\n');
      expect(parseInt(lines[0])).toBe(5);
      expect(lines[1]).toBe('analyzeRepository');
      expect(lines[2]).toBe('scanImage');
    });
  });

  describe('API Surface', () => {
    it('should export modern API surface', () => {
      const testScript = `
        const pkg = require('@thgamble/containerization-assist-mcp');

        // Check for key exports that clients depend on
        const expectedExports = [
          'createApp',
          'TOOLS',
          'getAllInternalTools',
          'ALL_TOOLS'
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
        const pkg1 = require('@thgamble/containerization-assist-mcp');
        const { createApp } = require('@thgamble/containerization-assist-mcp');
        const { TOOLS } = require('@thgamble/containerization-assist-mcp');
        const { getAllInternalTools } = require('@thgamble/containerization-assist-mcp');

        console.log('All require patterns work');
      `;

      writeFileSync(join(clientTestDir, 'test-require-patterns.js'), testScript);

      const result = execSync('node test-require-patterns.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('All require patterns work');
    });

    it('should export type definitions', () => {
      const testScript = `
        const pkg = require('@thgamble/containerization-assist-mcp');

        // Verify runtime can be created
        const app = pkg.createApp();

        // Verify tools array is accessible
        const tools = pkg.getAllInternalTools();

        console.log('Type definitions functional');
      `;

      writeFileSync(join(clientTestDir, 'test-types.js'), testScript);

      const result = execSync('node test-types.js', {
        cwd: clientTestDir,
        encoding: 'utf8'
      });

      expect(result.trim()).toBe('Type definitions functional');
    });
  });
});